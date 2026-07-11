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
	"gorm.io/gorm"
)

var (
	dandanCommentURL      = "https://api.dandanplay.net/api/v2/comment/%d"
	dandanMatchURL        = "https://api.dandanplay.net/api/v2/match"
	dandanSearchURL       = "https://api.dandanplay.net/api/v2/search/episodes"
	bangumiSearchURL      = "https://api.bgm.tv/v0/search/subjects"
	bangumiSubjectURL     = "https://api.bgm.tv/v0/subjects/%d"
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

func NewScraper(dbq *db.DBQueue, dataDir string) *Scraper {
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
	}
}

func (s *Scraper) ScrapeEpisode(ctx context.Context, ep *db.Episode, series *db.Series) error {
	if ep.MatchStatus != "unmatched" {
		return nil
	}

	if series.BangumiID == nil && series.Summary == nil {
		if err := s.scrapeMetadata(ctx, series); err != nil {
			slog.Warn("metadata scrape failed", "series", series.Title, "error", err)
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

	jsonPath := filepath.Join(danmakuDir, fmt.Sprintf("%d.json", ep.DandanEpisodeID))
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
	// This is a simple implementation - can be enhanced
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

	// 2. Fetch Bangumi Details
	if bgmID != 0 {
		time.Sleep(1 * time.Second) // rate limit
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(bangumiSubjectURL, bgmID), nil)
		req.Header.Set("User-Agent", "DanmakuOverlay/1.0")
		if bgmToken != "" {
			req.Header.Set("Authorization", "Bearer "+bgmToken)
		}
		if resp, err := s.clientDo(req); err == nil {
			if resp.StatusCode == 200 {
				var subject struct {
					Name          string `json:"name"`
					NameCN        string `json:"name_cn"`
					Summary       string `json:"summary"`
					Date          string `json:"date"`
					TotalEpisodes int    `json:"total_episodes"`
					Images        struct {
						Large string `json:"large"`
					} `json:"images"`
				}
				if json.NewDecoder(resp.Body).Decode(&subject) == nil {
					totalEps := uint(subject.TotalEpisodes)
					s.dbQueue.Write(func(tx *gorm.DB) error {
						updates := map[string]interface{}{
							"bangumi_id": bgmID,
							"summary":    subject.Summary,
							"total_eps":  totalEps,
							"air_date":   subject.Date,
							"name_cn":    subject.NameCN,
						}
						tx.Model(series).Updates(updates)
						series.BangumiID = &bgmID
						series.Summary = &subject.Summary
						series.TotalEps = &totalEps
						series.AirDate = &subject.Date
						series.NameCN = &subject.NameCN
						return nil
					})

					if subject.Images.Large != "" {
						s.downloadCover(ctx, series, subject.Images.Large, fmt.Sprintf("bgm_%d.jpg", bgmID))
					}
					slog.Info("bangumi metadata scraped successfully", "title", series.Title, "bangumi_id", bgmID)
					resp.Body.Close()
					return nil
				}
			}
			resp.Body.Close()
		}
	}

	// 3. TMDB Fallback
	tmdbKey := apiKeys["tmdb_api_key"]
	if tmdbKey != "" {
		slog.Info("bangumi failed or not found, trying tmdb", "title", series.Title)
		tmdbUrl := fmt.Sprintf("https://api.themoviedb.org/3/search/tv?query=%s&api_key=%s&language=zh-CN", url.QueryEscape(series.Title), tmdbKey)
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, tmdbUrl, nil)
		if resp, err := s.clientDo(req); err == nil {
			if resp.StatusCode == 200 {
				var tmdbResp struct {
					Results []struct {
						ID         uint   `json:"id"`
						Overview   string `json:"overview"`
						PosterPath string `json:"poster_path"`
					} `json:"results"`
				}
				if json.NewDecoder(resp.Body).Decode(&tmdbResp) == nil && len(tmdbResp.Results) > 0 {
					res := tmdbResp.Results[0]
					s.dbQueue.Write(func(tx *gorm.DB) error {
						tx.Model(series).Updates(map[string]interface{}{
							"summary": res.Overview,
						})
						series.Summary = &res.Overview
						return nil
					})
					if res.PosterPath != "" {
						imgUrl := "https://image.tmdb.org/t/p/w500" + res.PosterPath
						s.downloadCover(ctx, series, imgUrl, fmt.Sprintf("tmdb_%d.jpg", res.ID))
					}
				}
			}
			resp.Body.Close()
		}
	}

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
	if err != nil || resp.StatusCode != 200 {
		return
	}
	defer resp.Body.Close()

	coverDir := filepath.Join(s.dataDir, "covers")
	os.MkdirAll(coverDir, 0755)
	
	coverPath := filepath.Join(coverDir, filename)
	f, err := os.Create(coverPath)
	if err == nil {
		io.Copy(f, resp.Body)
		f.Close()
		relPath := "covers/" + filename
		s.dbQueue.Write(func(tx *gorm.DB) error {
			tx.Model(series).Update("cover_path", relPath)
			series.CoverPath = &relPath
			return nil
		})
	}
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
	var episodes []db.Episode
	if err := s.dbQueue.Read(func(tx *gorm.DB) error {
		return tx.Where("match_status = ?", "unmatched").Find(&episodes).Error
	}); err != nil {
		return fmt.Errorf("query unmatched episodes: %w", err)
	}

	slog.Info("scraping unmatched episodes", "count", len(episodes))

	for i, ep := range episodes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		slog.Info("scraping episode", "progress", fmt.Sprintf("%d/%d", i+1, len(episodes)), "file", ep.RelativePath)

		var series db.Series
		if ep.SeriesID == 0 {
			dirName := getSeriesTitle(ep.RelativePath)
			if err := s.dbQueue.Write(func(tx *gorm.DB) error {
				if err := tx.Where(db.Series{Title: dirName}).FirstOrCreate(&series).Error; err != nil {
					return err
				}
				ep.SeriesID = series.ID
				return tx.Model(&ep).Update("series_id", series.ID).Error
			}); err != nil {
				slog.Warn("failed to create/load series from path", "error", err)
				continue
			}
		} else {
			if err := s.dbQueue.Write(func(tx *gorm.DB) error {
				return tx.First(&series, ep.SeriesID).Error
			}); err != nil {
				slog.Warn("failed to load series", "series_id", ep.SeriesID, "error", err)
				continue
			}
		}

		if err := s.ScrapeEpisode(ctx, &ep, &series); err != nil {
			slog.Warn("failed to scrape episode", "file", ep.RelativePath, "error", err)
			continue
		}

		// 刮削成功或失败，都将其状态更新为 matched，避免下次继续扫描
		if err := s.dbQueue.Write(func(tx *gorm.DB) error {
			return tx.Model(&ep).Update("match_status", "matched").Error
		}); err != nil {
			slog.Warn("failed to update match status to matched", "error", err)
		}
	}

	return nil
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
