package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/l31155/danmaku-overlay/internal/db"
	"github.com/l31155/danmaku-overlay/internal/utils"
	"gorm.io/gorm"
)

var (
	dandanCommentURL       = "https://api.dandanplay.net/api/v2/comment/%d"
	dandanMatchURL         = "https://api.dandanplay.net/api/v2/match"
	dandanSearchURL        = "https://api.dandanplay.net/api/v2/search/episodes"
	bangumiSearchURL       = "https://api.bgm.tv/v0/search/subjects"
	bangumiSubjectURL      = "https://api.bgm.tv/v0/subjects/%d"
	bangumiSubjectImageURL = "https://api.bgm.tv/v0/subjects/%d/image?type=large"

	danmakuCacheExpiration = 24 * time.Hour
)

type Scraper struct {
	dbQueue    *db.DBQueue
	dataDir    string
	dandanLim  *TokenBucket
	bangumiLim *TokenBucket
	client     *http.Client
	clientDo   func(*http.Request) (*http.Response, error)
	Progress   *Progress
}

type bangumiSearchRequest struct {
	Keyword string `json:"keyword"`
	Filter  struct {
		Type []int `json:"type"`
	} `json:"filter,omitempty"`
}

type bangumiSearchResponse struct {
	Data []struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		NameCN   string `json:"name_cn"`
		Summary  string `json:"summary"`
		Date     string `json:"date"`
		Images   struct {
			Large  string `json:"large"`
			Common string `json:"common"`
			Medium string `json:"medium"`
			Small  string `json:"small"`
			Grid   string `json:"grid"`
		} `json:"images"`
		Rating struct {
			Score float64 `json:"score"`
			Total int     `json:"total"`
		} `json:"rating"`
		TotalEpisodes int `json:"total_episodes"`
	} `json:"data"`
}

func NewScraper(dbq *db.DBQueue, dataDir string, progress *Progress) *Scraper {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &Scraper{
		dbQueue:    dbq,
		dataDir:    dataDir,
		dandanLim:  NewTokenBucket(2*time.Second, 1),
		bangumiLim: NewTokenBucket(2*time.Second, 1),
		client:     client,
		clientDo:   client.Do,
		Progress:   progress,
	}
}

func (s *Scraper) ScrapeEpisode(ctx context.Context, ep *db.Episode, series *db.Series) error {
	if ep.ScrapeStatus != "unscraped" {
		return nil
	}

	if series.BangumiID == nil && series.Summary == nil {
		if err := s.scrapeMetadata(ctx, series); err != nil {
			slog.Warn("metadata scrape failed", "series", series.Title, "error", err)
			return err
		}
	}

	return nil
}

func (s *Scraper) DownloadDanmaku(ctx context.Context, ep *db.Episode) error {
	// Check if we have a valid cached file
	if ep.DanmakuPath != nil && *ep.DanmakuPath != "" {
		if info, err := os.Stat(*ep.DanmakuPath); err == nil {
			if time.Since(info.ModTime()) < danmakuCacheExpiration {
				return nil // Cache is still valid
			}
		}
	}

	// If no DandanEpisodeID, try to match the video first
	if ep.DandanEpisodeID == 0 {
		if err := s.matchVideo(ctx, ep); err != nil {
			return fmt.Errorf("match video: %w", err)
		}
		if ep.DandanEpisodeID == 0 {
			return fmt.Errorf("no match found for video: %s", ep.RelativePath)
		}
	}

	if err := s.dandanLim.Wait(ctx); err != nil {
		return fmt.Errorf("wait dandan rate limit: %w", err)
	}

	url := fmt.Sprintf(dandanCommentURL, ep.DandanEpisodeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create comment request: %w", err)
	}

	req.Header.Set("User-Agent", "DanmakuOverlay/1.0")

	var resp *http.Response
	err = s.retryWithBackoff(ctx, 3, 1*time.Second, func() error {
		var retryErr error
		resp, retryErr = s.clientDo(req)
		if retryErr != nil {
			return retryErr
		}

		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("fetch danmaku after retries: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read danmaku response: %w", err)
	}

	danmakuDir := filepath.Join(s.dataDir, "danmaku")
	if err := os.MkdirAll(danmakuDir, 0755); err != nil {
		return fmt.Errorf("create danmaku dir: %w", err)
	}

	danmakuLines, err := parseDanDanXML(body)
	if err != nil {
		slog.Warn("parse danmaku xml failed", "error", err)
	}

	jsonData, err := json.Marshal(danmakuLines)
	if err != nil {
		return fmt.Errorf("marshal danmaku json: %w", err)
	}

	jsonPath := filepath.Join(danmakuDir, fmt.Sprintf("%s.json", ep.FileHash))
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write json file: %w", err)
	}

	pathStr := jsonPath
	if err := s.dbQueue.Write(func(tx *gorm.DB) error {
		return tx.Model(ep).Update("danmaku_path", &pathStr).Error
	}); err != nil {
		return fmt.Errorf("update danmaku path: %w", err)
	}

	return nil
}

func (s *Scraper) matchVideo(ctx context.Context, ep *db.Episode) error {
	// Try matching by file name and hash first
	if err := s.matchByFileHash(ctx, ep); err != nil {
		slog.Warn("match by file hash failed", "error", err, "file", ep.RelativePath)
	}

	// If still no match, try keyword search
	if ep.DandanEpisodeID == 0 {
		if err := s.matchByKeyword(ctx, ep); err != nil {
			slog.Warn("match by keyword failed", "error", err, "file", ep.RelativePath)
		}
	}

	return nil
}

func (s *Scraper) matchByFileHash(ctx context.Context, ep *db.Episode) error {
	if err := s.dandanLim.Wait(ctx); err != nil {
		return fmt.Errorf("wait dandan rate limit: %w", err)
	}

	matchReq := struct {
		FileName string `json:"fileName"`
		FileHash string `json:"fileHash"`
		FileSize int64  `json:"fileSize"`
	}{
		FileName: filepath.Base(ep.RelativePath),
		FileHash: ep.FileMD5,
		FileSize: 0,
	}

	reqBody, err := json.Marshal(matchReq)
	if err != nil {
		return fmt.Errorf("marshal match request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dandanMatchURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create match request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DanmakuOverlay/1.0")

	var resp *http.Response
	err = s.retryWithBackoff(ctx, 3, 1*time.Second, func() error {
		var retryErr error
		resp, retryErr = s.clientDo(req)
		if retryErr != nil {
			return retryErr
		}

		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("match request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read match response: %w", err)
	}

	var matchResp struct {
		IsMatched bool `json:"isMatched"`
		Matches   []struct {
			EpisodeID uint `json:"episodeId"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(body, &matchResp); err != nil {
		return fmt.Errorf("unmarshal match response: %w", err)
	}

	if matchResp.IsMatched && len(matchResp.Matches) > 0 {
		ep.DandanEpisodeID = matchResp.Matches[0].EpisodeID
		if err := s.dbQueue.Write(func(tx *gorm.DB) error {
			return tx.Model(ep).Update("dandan_episode_id", ep.DandanEpisodeID).Error
		}); err != nil {
			return fmt.Errorf("update dandan episode id: %w", err)
		}
	}

	return nil
}

func (s *Scraper) matchByKeyword(ctx context.Context, ep *db.Episode) error {
	if err := s.dandanLim.Wait(ctx); err != nil {
		return fmt.Errorf("wait dandan rate limit: %w", err)
	}

	// Extract keyword from file name (remove extension and common patterns)
	keyword := extractKeyword(ep.RelativePath)
	if keyword == "" {
		return fmt.Errorf("could not extract keyword from: %s", ep.RelativePath)
	}

	searchURL := fmt.Sprintf("%s?keyword=%s", dandanSearchURL, url.QueryEscape(keyword))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return fmt.Errorf("create search request: %w", err)
	}

	req.Header.Set("User-Agent", "DanmakuOverlay/1.0")

	var resp *http.Response
	err = s.retryWithBackoff(ctx, 3, 1*time.Second, func() error {
		var retryErr error
		resp, retryErr = s.clientDo(req)
		if retryErr != nil {
			return retryErr
		}

		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read search response: %w", err)
	}

	var searchResp struct {
		Animes []struct {
			Episodes []struct {
				EpisodeID uint `json:"episodeId"`
			} `json:"episodes"`
		} `json:"animes"`
	}
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return fmt.Errorf("unmarshal search response: %w", err)
	}

	// Use the first match
	if len(searchResp.Animes) > 0 && len(searchResp.Animes[0].Episodes) > 0 {
		ep.DandanEpisodeID = searchResp.Animes[0].Episodes[0].EpisodeID
		if err := s.dbQueue.Write(func(tx *gorm.DB) error {
			return tx.Model(ep).Update("dandan_episode_id", ep.DandanEpisodeID).Error
		}); err != nil {
			return fmt.Errorf("update dandan episode id: %w", err)
		}
	}

	return nil
}

func extractKeyword(filePath string) string {
	// Remove extension
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}

	// Remove common patterns like [1080p], [HEVC], etc.
	result := base
	for _, pattern := range []string{"[", "]", "(", ")"} {
		for {
			start := -1
			for i, ch := range result {
				if string(ch) == pattern {
					if pattern == "[" || pattern == "(" {
						start = i
					} else if start >= 0 {
						result = result[:start] + result[i+1:]
						start = -1
						break
					}
				}
			}
			if start >= 0 {
				break
			}
		}
	}

	// Remove numbers at the end (episode numbers)
	for i := len(result) - 1; i >= 0; i-- {
		if result[i] < '0' || result[i] > '9' {
			result = result[:i+1]
			break
		}
	}

	// Trim spaces and special characters
	result = strings.TrimSpace(result)
	result = strings.Trim(result, "-_. ")

	return result
}

func (s *Scraper) scrapeMetadata(ctx context.Context, series *db.Series) error {
	if series.Title == "" {
		return nil
	}

	apiKeys := s.getAPIKeys()
	bgmToken := apiKeys["bangumi_access_token"]

	if err := s.bangumiLim.Wait(ctx); err != nil {
		return fmt.Errorf("wait bangumi rate limit: %w", err)
	}

	// 1. Search Bangumi using POST /v0/search/subjects
	searchReq := bangumiSearchRequest{
		Keyword: series.Title,
		Filter: struct {
			Type []int `json:"type"`
		}{Type: []int{2}}, // 2 = anime
	}
	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return fmt.Errorf("marshal search request: %w", err)
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, bangumiSearchURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DanmakuOverlay/1.0")
	if bgmToken != "" {
		req.Header.Set("Authorization", "Bearer "+bgmToken)
	}

	var bgmID uint
	if resp, err := s.clientDo(req); err == nil {
		if resp.StatusCode == 200 {
			var searchResp bangumiSearchResponse
			if json.NewDecoder(resp.Body).Decode(&searchResp) == nil && len(searchResp.Data) > 0 {
				bgmID = searchResp.Data[0].ID
			}
		}
		resp.Body.Close()
	}

	if bgmID == 0 {
		return fmt.Errorf("bangumi search returned no results for: %s", series.Title)
	}

	// 2. Fetch Bangumi Details
	time.Sleep(1 * time.Second) // rate limit
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(bangumiSubjectURL, bgmID), nil)
	req.Header.Set("User-Agent", "DanmakuOverlay/1.0")
	if bgmToken != "" {
		req.Header.Set("Authorization", "Bearer "+bgmToken)
	}

	resp, err := s.clientDo(req)
	if err != nil {
		return fmt.Errorf("fetch bangumi subject: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bangumi subject API returned status %d", resp.StatusCode)
	}

	var subject struct {
		Name          string `json:"name"`
		NameCN        string `json:"name_cn"`
		Summary       string `json:"summary"`
		Date          string `json:"date"`
		TotalEpisodes int    `json:"total_episodes"`
		Rating        struct {
			Score float64 `json:"score"`
		} `json:"rating"`
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Images struct {
			Large string `json:"large"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&subject); err != nil {
		return fmt.Errorf("decode bangumi subject: %w", err)
	}

	totalEps := uint(subject.TotalEpisodes)

	var tagNames []string
	for _, tag := range subject.Tags {
		tagNames = append(tagNames, tag.Name)
	}
	tagsJSON, _ := json.Marshal(tagNames)
	tagsStr := string(tagsJSON)

	airDate := subject.Date
	if len(airDate) >= 7 {
		airDate = airDate[:7]
	}

	// Update series with BangumiID as ID
	bangumiIDStr := fmt.Sprintf("%d", bgmID)
	oldID := series.ID

	// Delete old temporary record if it exists
	if strings.HasPrefix(oldID, "temp_") {
		if err := s.dbQueue.Write(func(tx *gorm.DB) error {
			return tx.Where("id = ?", oldID).Delete(&db.Series{}).Error
		}); err != nil {
			return fmt.Errorf("delete temp series: %w", err)
		}
	}

	// Create new series with bangumiID as ID
	series.ID = bangumiIDStr
	series.BangumiID = &bgmID
	series.Summary = &subject.Summary
	series.TotalEps = &totalEps
	series.AirDate = &airDate
	series.NameCN = &subject.NameCN
	series.Rating = &subject.Rating.Score
	series.Tags = &tagsStr

	if err := s.dbQueue.Write(func(tx *gorm.DB) error {
		return tx.Create(series).Error
	}); err != nil {
		return fmt.Errorf("save series: %w", err)
	}

	if subject.Images.Large != "" {
		s.downloadCover(ctx, series, subject.Images.Large, fmt.Sprintf("bgm_%d.jpg", bgmID))
	}
	slog.Info("bangumi metadata scraped successfully", "title", series.Title, "bangumi_id", bgmID)

	return nil
}

func (s *Scraper) getAPIKeys() map[string]string {
	var settings []db.Setting
	keys := make(map[string]string)
	if err := s.dbQueue.Read(func(tx *gorm.DB) error {
		return tx.Where("key = ?", "api_keys").Limit(1).Find(&settings).Error
	}); err == nil && len(settings) > 0 {
		json.Unmarshal(settings[0].Value, &keys)
	}
	return keys
}

func (s *Scraper) downloadCover(ctx context.Context, series *db.Series, imgUrl, filename string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, imgUrl, nil)
	resp, err := s.clientDo(req)
	if err != nil {
		slog.Warn("failed to download cover", "url", imgUrl, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		slog.Warn("cover download failed with status", "url", imgUrl, "status", resp.StatusCode)
		return
	}

	coverDir := filepath.Join(s.dataDir, "covers")
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		slog.Error("failed to create cover directory", "error", err)
		return
	}

	tmpPath := filepath.Join(coverDir, filename+".tmp")
	finalPath := filepath.Join(coverDir, filename)

	f, err := os.Create(tmpPath)
	if err != nil {
		slog.Error("failed to create cover file", "path", tmpPath, "error", err)
		return
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		slog.Error("failed to write cover file", "path", tmpPath, "error", err)
		return
	}

	f.Close()

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		slog.Error("failed to rename cover file", "error", err)
		return
	}

	relPath := "covers/" + filename
	s.dbQueue.Write(func(tx *gorm.DB) error {
		return tx.Model(series).Update("cover_path", relPath).Error
	})
	series.CoverPath = &relPath
}

func getSeriesTitle(relPath string) string {
	dir := filepath.Dir(relPath)
	if dir == "." || dir == "/" {
		return "Unknown Series"
	}
	base := filepath.Base(dir)
	baseLower := strings.ToLower(base)
	// 如果父目录是 Season X 或 Specials，说明是二级目录结构，需要往上一级获取真正的番剧标题
	if strings.HasPrefix(baseLower, "season") || baseLower == "specials" || baseLower == "extra" {
		parent := filepath.Dir(dir)
		if parent != "." && parent != "/" {
			return filepath.Base(parent)
		}
	}
	return base
}

func (s *Scraper) ScrapeAllUnmatched(ctx context.Context) error {
	start := time.Now()

	var episodes []db.Episode
	if err := s.dbQueue.Read(func(tx *gorm.DB) error {
		return tx.Where("scrape_status = ?", "unscraped").Find(&episodes).Error
	}); err != nil {
		return fmt.Errorf("query unscraped episodes: %w", err)
	}

	slog.Info("scraping unscraped episodes", "count", len(episodes))

	if s.Progress != nil {
		s.Progress.SetScrapeRunning(len(episodes))
	}

	var scraped, failed int
	for i, ep := range episodes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if s.Progress != nil {
			s.Progress.UpdateScrapeProgress(i+1, len(episodes))
		}

		slog.Info("scraping episode", "progress", fmt.Sprintf("%d/%d", i+1, len(episodes)), "file", ep.RelativePath)

		// 1. Get series title from directory
		dirName := getSeriesTitle(ep.RelativePath)

		// 2. Find or create series
		var series db.Series
		if err := s.dbQueue.Read(func(tx *gorm.DB) error {
			return tx.Where("title = ?", dirName).First(&series).Error
		}); err != nil {
			// Series not found, create temporary series with temp ID
			tempID := fmt.Sprintf("temp_%d", time.Now().UnixNano())
			series = db.Series{
				ID:    tempID,
				Title: dirName,
			}
			if err := s.dbQueue.Write(func(tx *gorm.DB) error {
				return tx.Create(&series).Error
			}); err != nil {
				slog.Warn("failed to create series", "error", err)
				s.updateEpisodeStatus(&ep, "no_match")
				failed++
				continue
			}
		}

		// 3. Scrape metadata if needed
		if series.BangumiID == nil {
			if err := s.scrapeMetadata(ctx, &series); err != nil {
				slog.Warn("metadata scrape failed", "series", series.Title, "error", err)
				s.updateEpisodeStatus(&ep, "no_match")
				failed++
				continue
			}
		}

		// 4. Generate Episode ID
		if series.BangumiID == nil {
			slog.Warn("series has no bangumi_id", "series", series.Title)
			s.updateEpisodeStatus(&ep, "no_match")
			failed++
			continue
		}

		// Extract epIndex from filename
		filename := filepath.Base(ep.RelativePath)
		epIndex := utils.ExtractEpIndexFromFilename(filename)

		// Generate episode ID
		episodeID := utils.GenerateEpisodeID(*series.BangumiID, epIndex)

		// 5. Update episode with IDs
		// First check if episode already exists by relative_path
		var existingEp db.Episode
		if err := s.dbQueue.Read(func(tx *gorm.DB) error {
			return tx.Where("relative_path = ?", ep.RelativePath).First(&existingEp).Error
		}); err == nil {
			// Episode exists, update it in place using WHERE relative_path
			if err := s.dbQueue.Write(func(tx *gorm.DB) error {
				return tx.Model(&db.Episode{}).Where("relative_path = ?", ep.RelativePath).Updates(map[string]interface{}{
					"id":           episodeID,
					"series_id":    series.ID,
					"ep_index":     float64Ptr(float64(epIndex)),
				}).Error
			}); err != nil {
				slog.Warn("failed to update episode", "error", err)
				failed++
				continue
			}
		} else {
			// Episode not found, create new one
			ep.ID = episodeID
			ep.SeriesID = series.ID
			ep.EpIndex = float64Ptr(float64(epIndex))
			if err := s.dbQueue.Write(func(tx *gorm.DB) error {
				return tx.Create(&ep).Error
			}); err != nil {
				slog.Warn("failed to create episode", "error", err)
				failed++
				continue
			}
		}

		// 6. Update series current_ep
		s.updateSeriesCurrentEp(series.ID)

		// 7. Update scrape status
		s.updateEpisodeStatus(&ep, "completed")
		scraped++
	}

	elapsed := time.Since(start)
	if s.Progress != nil {
		s.Progress.SetScrapeCompleted(fmt.Sprintf("%d/%d scraped, %d failed, %s", scraped, len(episodes), failed, elapsed.Round(time.Millisecond).String()))
	}

	return nil
}

func (s *Scraper) updateEpisodeStatus(ep *db.Episode, status string) {
	s.dbQueue.Write(func(tx *gorm.DB) error {
		return tx.Model(&db.Episode{}).Where("relative_path = ?", ep.RelativePath).Updates(map[string]interface{}{
			"scrape_status": status,
			"match_status":  status,
		}).Error
	})
}

func (s *Scraper) updateSeriesCurrentEp(seriesID string) {
	var count int64
	s.dbQueue.Read(func(tx *gorm.DB) error {
		return tx.Model(&db.Episode{}).
			Where("series_id = ? AND scrape_status = ?", seriesID, "completed").
			Count(&count).Error
	})

	currentEp := uint(count)
	s.dbQueue.Write(func(tx *gorm.DB) error {
		return tx.Model(&db.Series{}).Where("id = ?", seriesID).Update("current_ep", currentEp).Error
	})
}

func float64Ptr(f float64) *float64 {
	return &f
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func isRetryableError(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	if respErr, ok := err.(*HTTPError); ok && respErr.StatusCode >= 500 {
		return true
	}
	return false
}

func (s *Scraper) retryWithBackoff(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i == maxRetries {
				break
			}

			if !isRetryableError(err) {
				return err
			}

			delay := baseDelay * time.Duration(1<<uint(i))
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			slog.Warn("retrying after error", "attempt", i+1, "delay", delay, "error", err)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		} else {
			return nil
		}
	}
	return lastErr
}
