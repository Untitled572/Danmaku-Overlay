package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/l31155/danmaku-overlay/internal/db"
)

func TestNewScraper(t *testing.T) {
	_, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	s := NewScraper(dbq, dir, nil)
	if s == nil {
		t.Fatal("NewScraper returned nil")
	}
	if s.dbQueue != dbq {
		t.Error("dbQueue not set correctly")
	}
	if s.clientDo == nil {
		t.Error("clientDo not set")
	}
}

func TestScrapeEpisode_Unmatched(t *testing.T) {
	gdb, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	origSearchURL := bangumiSearchURL
	origCommentURL := dandanCommentURL
	origSubjectURL := bangumiSubjectURL
	t.Cleanup(func() {
		bangumiSearchURL = origSearchURL
		dandanCommentURL = origCommentURL
		bangumiSubjectURL = origSubjectURL
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/search/subjects":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{{"id": 9999}},
			})
		case "/v0/subjects/9999":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":          "Test Anime",
				"name_cn":       "测试动画",
				"summary":       "Test summary",
				"date":          "2024-01-01",
				"total_episodes": 12,
				"images": map[string]interface{}{
					"large": "https://example.com/cover.jpg",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	bangumiSearchURL = ts.URL + "/v0/search/subjects"
	bangumiSubjectURL = ts.URL + "/v0/subjects/%d"
	dandanCommentURL = ts.URL + "/api/v2/comment/%d"

	series := &db.Series{Title: "Test Series"}
	if err := gdb.Create(series).Error; err != nil {
		t.Fatalf("failed to create series: %v", err)
	}

	ep := &db.Episode{
		SeriesID:     series.ID,
		RelativePath: "test/video.mkv",
		FileMD5:      "abc123hash",
		FileHash:     "def456hash",
		MatchStatus:  "unmatched",
	}
	if err := gdb.Create(ep).Error; err != nil {
		t.Fatalf("failed to create episode: %v", err)
	}

	s := NewScraper(dbq, dir, nil)
	s.clientDo = ts.Client().Do

	err := s.ScrapeEpisode(context.Background(), ep, series)
	if err != nil {
		t.Fatalf("ScrapeEpisode() error: %v", err)
	}

	var updatedSeries db.Series
	if err := gdb.First(&updatedSeries, series.ID).Error; err != nil {
		t.Fatalf("failed to fetch updated series: %v", err)
	}

	if updatedSeries.BangumiID == nil || *updatedSeries.BangumiID != 9999 {
		t.Errorf("BangumiID = %v, want 9999", updatedSeries.BangumiID)
	}

	if updatedSeries.Summary == nil {
		t.Error("Summary is nil, want 'Test summary'")
	} else if *updatedSeries.Summary != "Test summary" {
		t.Errorf("Summary = %q, want 'Test summary'", *updatedSeries.Summary)
	}

	if updatedSeries.NameCN == nil {
		t.Error("NameCN is nil, want '测试动画'")
	} else if *updatedSeries.NameCN != "测试动画" {
		t.Errorf("NameCN = %q, want '测试动画'", *updatedSeries.NameCN)
	}

	if updatedSeries.AirDate == nil {
		t.Error("AirDate is nil, want '2024-01'")
	} else if *updatedSeries.AirDate != "2024-01" {
		t.Errorf("AirDate = %q, want '2024-01'", *updatedSeries.AirDate)
	}
}

func TestScrapeEpisode_AlreadyMatched(t *testing.T) {
	gdb, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	series := &db.Series{Title: "Test Series"}
	if err := gdb.Create(series).Error; err != nil {
		t.Fatalf("failed to create series: %v", err)
	}

	ep := &db.Episode{
		SeriesID:        series.ID,
		RelativePath:    "test/video.mkv",
		FileMD5:         "abc123hash",
		FileHash:        "def456hash",
		MatchStatus:     "matched",
		DandanEpisodeID: 123456,
	}
	if err := gdb.Create(ep).Error; err != nil {
		t.Fatalf("failed to create episode: %v", err)
	}

	s := NewScraper(dbq, dir, nil)
	err := s.ScrapeEpisode(context.Background(), ep, series)
	if err != nil {
		t.Fatalf("ScrapeEpisode() error: %v", err)
	}
}

func TestScrapeEpisode_BangumiNotFound(t *testing.T) {
	gdb, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	origSearchURL := bangumiSearchURL
	origCommentURL := dandanCommentURL
	origSubjectURL := bangumiSubjectURL
	t.Cleanup(func() {
		bangumiSearchURL = origSearchURL
		dandanCommentURL = origCommentURL
		bangumiSubjectURL = origSubjectURL
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/search/subjects":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	bangumiSearchURL = ts.URL + "/v0/search/subjects"
	dandanCommentURL = ts.URL + "/api/v2/comment/%d"

	series := &db.Series{Title: "Unknown Series"}
	if err := gdb.Create(series).Error; err != nil {
		t.Fatalf("failed to create series: %v", err)
	}

	ep := &db.Episode{
		SeriesID:     series.ID,
		RelativePath: "test/video.mkv",
		FileMD5:      "abc123hash",
		FileHash:     "def456hash",
		MatchStatus:  "unmatched",
	}
	if err := gdb.Create(ep).Error; err != nil {
		t.Fatalf("failed to create episode: %v", err)
	}

	s := NewScraper(dbq, dir, nil)
	s.clientDo = ts.Client().Do

	err := s.ScrapeEpisode(context.Background(), ep, series)
	if err == nil {
		t.Fatal("ScrapeEpisode() expected error for no bangumi results, got nil")
	}
}

func TestScrapeAllUnmatched(t *testing.T) {
	gdb, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	origSearchURL := bangumiSearchURL
	origCommentURL := dandanCommentURL
	origSubjectURL := bangumiSubjectURL
	t.Cleanup(func() {
		bangumiSearchURL = origSearchURL
		dandanCommentURL = origCommentURL
		bangumiSubjectURL = origSubjectURL
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/search/subjects":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{{"id": 100}},
			})
		case "/v0/subjects/100":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":          "Bulk Test Series",
				"name_cn":       "批量测试系列",
				"summary":       "Bulk test summary",
				"date":          "2024-01-01",
				"total_episodes": 12,
				"images": map[string]interface{}{
					"large": "https://example.com/cover.jpg",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	bangumiSearchURL = ts.URL + "/v0/search/subjects"
	bangumiSubjectURL = ts.URL + "/v0/subjects/%d"
	dandanCommentURL = ts.URL + "/api/v2/comment/%d"

	series := &db.Series{Title: "Bulk Test Series"}
	if err := gdb.Create(series).Error; err != nil {
		t.Fatalf("failed to create series: %v", err)
	}

	for i := 0; i < 3; i++ {
		ep := &db.Episode{
			ID:           fmt.Sprintf("test_%d", i),
			SeriesID:     series.ID,
			RelativePath: filepath.Join("test", "[Sub] Video "+string(rune('A'+i))+" - 0"+string(rune('1'+i))+".mkv"),
			FileMD5:      "hash" + string(rune('a'+i)),
			FileHash:     "fhash" + string(rune('a'+i)),
			MatchStatus:  "unmatched",
			ScrapeStatus: "unscraped",
		}
		if err := gdb.Create(ep).Error; err != nil {
			t.Fatalf("failed to create episode %d: %v", i, err)
		}
	}

	s := NewScraper(dbq, dir, nil)
	s.clientDo = ts.Client().Do

	err := s.ScrapeAllUnmatched(context.Background())
	if err != nil {
		t.Fatalf("ScrapeAllUnmatched() error: %v", err)
	}

	var updatedSeries db.Series
	if err := gdb.First(&updatedSeries, series.ID).Error; err != nil {
		t.Fatalf("failed to fetch updated series: %v", err)
	}

	if updatedSeries.BangumiID == nil || *updatedSeries.BangumiID != 100 {
		t.Errorf("BangumiID = %v, want 100", updatedSeries.BangumiID)
	}
}
