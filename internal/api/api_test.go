package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlite "github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"

	"github.com/l31155/danmaku-overlay/internal/auth"
	"github.com/l31155/danmaku-overlay/internal/config"
	"github.com/l31155/danmaku-overlay/internal/db"
	"github.com/l31155/danmaku-overlay/internal/websocket"
)

func setupTestAPI(t *testing.T, token string) (*httptest.Server, *db.DBQueue, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dsn := "file:" + filepath.Join(tmpDir, "test.db") + "?cache=shared&mode=rwc"
	gormDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := gormDB.AutoMigrate(&db.Library{}, &db.Series{}, &db.Episode{}, &db.History{}, &db.Setting{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	dbq := db.NewDBQueue(gormDB)
	t.Cleanup(func() { dbq.Close() })

	hub := websocket.NewHub(context.Background())
	hub.Start()
	t.Cleanup(func() { hub.Stop() })

	cfg := &config.Config{LocalToken: token, DataDir: tmpDir}

	s := NewServer(dbq, hub, cfg, nil, nil, nil, nil, context.Background())

	mux := http.NewServeMux()
	authMiddleware := auth.TokenAuth(cfg.LocalToken)
	s.registerRoutes(mux, authMiddleware)
	handler := s.corsMiddleware(mux)

	ts := httptest.NewServer(handler)
	t.Cleanup(func() { ts.Close() })

	return ts, dbq, tmpDir
}

func TestHealthEndpoint(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", body["status"])
	}
	if body["database"] != "ready" {
		t.Fatalf("expected database ready, got %s", body["database"])
	}
}

func TestSearch_Empty(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["total"].(float64) != 0 {
		t.Fatalf("expected 0 results, got %v", result["total"])
	}
}

func TestSearch_WithData(t *testing.T) {
	ts, dbq, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	seriesData := []db.Series{
		{ID: "100", Title: "进击的巨人", AirDate: strPtr("2013-04")},
		{ID: "200", Title: "鬼灭之刃", AirDate: strPtr("2019-04")},
	}
	for _, s := range seriesData {
		if err := dbq.Write(func(tx *gorm.DB) error {
			return tx.Create(&s).Error
		}); err != nil {
			t.Fatalf("failed to insert series: %v", err)
		}
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/search?q=巨人", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["total"].(float64) != 1 {
		t.Fatalf("expected 1 result, got %v", result["total"])
	}
}

func TestSearch_ByAirdate(t *testing.T) {
	ts, dbq, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	seriesData := []db.Series{
		{ID: "100", Title: "Series A", AirDate: strPtr("2026-07")},
		{ID: "200", Title: "Series B", AirDate: strPtr("2026-07")},
		{ID: "300", Title: "Series C", AirDate: strPtr("2026-01")},
	}
	for _, s := range seriesData {
		if err := dbq.Write(func(tx *gorm.DB) error {
			return tx.Create(&s).Error
		}); err != nil {
			t.Fatalf("failed to insert series: %v", err)
		}
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/search?airdate=2026-07", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["total"].(float64) != 2 {
		t.Fatalf("expected 2 results, got %v", result["total"])
	}
}

func TestGetEpisodes_Empty(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/episodes", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var episodes []db.Episode
	if err := json.NewDecoder(resp.Body).Decode(&episodes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(episodes) != 0 {
		t.Fatalf("expected empty episodes, got %d", len(episodes))
	}
}

func TestGetEpisodes_Filter(t *testing.T) {
	ts, dbq, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	episodes := []db.Episode{
		{ID: "1000011", SeriesID: "100", DandanEpisodeID: 1, RelativePath: "ep1.mkv", FileMD5: "md5a", FileHash: "hasha"},
		{ID: "1000022", SeriesID: "100", DandanEpisodeID: 2, RelativePath: "ep2.mkv", FileMD5: "md5b", FileHash: "hashb"},
		{ID: "2000011", SeriesID: "200", DandanEpisodeID: 3, RelativePath: "ep3.mkv", FileMD5: "md5c", FileHash: "hashc"},
	}
	for _, ep := range episodes {
		if err := dbq.Write(func(tx *gorm.DB) error {
			return tx.Create(&ep).Error
		}); err != nil {
			t.Fatalf("failed to insert episode: %v", err)
		}
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/episodes?series_id=100", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result []db.Episode
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(result))
	}
}

func TestGetDanmaku(t *testing.T) {
	ts, dbq, tmpDir := setupTestAPI(t, "test-token")
	defer ts.Close()

	danmakuDir := filepath.Join(tmpDir, "danmaku")
	if err := os.MkdirAll(danmakuDir, 0755); err != nil {
		t.Fatalf("failed to create danmaku dir: %v", err)
	}
	danmakuPath := filepath.Join(danmakuDir, "1.json")
	danmakuContent := `[{"time":1.0,"text":"test danmaku","color":"#FFFFFF"}]`
	if err := os.WriteFile(danmakuPath, []byte(danmakuContent), 0644); err != nil {
		t.Fatalf("failed to write danmaku file: %v", err)
	}

	ep := db.Episode{
		ID:              "1000011",
		SeriesID:        "100",
		DandanEpisodeID: 1,
		RelativePath:    "ep1.mkv",
		FileMD5:         "md5",
		FileHash:        "hash",
		DanmakuPath:     &danmakuPath,
	}
	if err := dbq.Write(func(tx *gorm.DB) error {
		return tx.Create(&ep).Error
	}); err != nil {
		t.Fatalf("failed to insert episode: %v", err)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/episodes/1000011/danmaku", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var danmaku []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&danmaku); err != nil {
		t.Fatalf("failed to decode danmaku response: %v", err)
	}
	if len(danmaku) != 1 {
		t.Fatalf("expected 1 danmaku item, got %d", len(danmaku))
	}
}

func TestGetDanmaku_NotFound(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/episodes/9999999/danmaku", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetDanmaku_NoDanmakuFile(t *testing.T) {
	ts, dbq, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	ep := db.Episode{
		ID:              "1000011",
		SeriesID:        "100",
		DandanEpisodeID: 1,
		RelativePath:    "ep1.mkv",
		FileMD5:         "md5",
		FileHash:        "hash",
	}
	if err := dbq.Write(func(tx *gorm.DB) error {
		return tx.Create(&ep).Error
	}); err != nil {
		t.Fatalf("failed to insert episode: %v", err)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/episodes/1000011/danmaku", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var danmaku []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&danmaku); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(danmaku) != 0 {
		t.Fatalf("expected empty danmaku, got %d", len(danmaku))
	}
}

func TestUpdateProgress(t *testing.T) {
	ts, dbq, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	ep := db.Episode{
		ID:              "1000011",
		SeriesID:        "100",
		DandanEpisodeID: 1,
		RelativePath:    "ep1.mkv",
		FileMD5:         "md5",
		FileHash:        "hash",
	}
	if err := dbq.Write(func(tx *gorm.DB) error {
		return tx.Create(&ep).Error
	}); err != nil {
		t.Fatalf("failed to insert episode: %v", err)
	}

	body := `{"episode_id":"1000011","position":42.5}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/progress",
		jsonReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !result["ok"] {
		t.Fatalf("expected ok=true")
	}

	var history db.History
	err = dbq.Read(func(tx *gorm.DB) error {
		return tx.Where("episode_id = ?", "1000011").First(&history).Error
	})
	if err != nil {
		t.Fatalf("failed to query history: %v", err)
	}
	if history.Position != 42.5 {
		t.Fatalf("expected position 42.5, got %f", history.Position)
	}
}

func TestUpdateProgress_InvalidBody(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/progress",
		jsonReader("invalid json"))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestTriggerScan(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/scan", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// When scannerManager is nil, expect 503 Service Unavailable
	// When scannerManager is set, expect 202 Accepted
	if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 or 503, got %d", resp.StatusCode)
	}
}

func TestTriggerScrape(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/scrape", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// When scraper is nil, expect 503 Service Unavailable
	// When scraper is set, expect 202 Accepted
	if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 or 503, got %d", resp.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/search?q=test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthInvalidToken(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCORS(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/v1/search", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected CORS Allow-Origin header")
	}
	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("expected CORS Allow-Methods header")
	}
	if resp.Header.Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("expected CORS Allow-Headers header")
	}
}

func TestNotFound(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func jsonReader(s string) *strings.Reader {
	return strings.NewReader(s)
}

func TestGetSettings_Empty(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/settings", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var settings map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(settings) != 0 {
		t.Fatalf("expected empty settings, got %d", len(settings))
	}
}

func TestUpdateAndGetSettings(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	body := `{"locale":"zh-CN","theme":"dark"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/settings", jsonReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	req2, _ := http.NewRequest("GET", ts.URL+"/api/v1/settings", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()

	var settings map[string]json.RawMessage
	if err := json.NewDecoder(resp2.Body).Decode(&settings); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(settings) != 2 {
		t.Fatalf("expected 2 settings, got %d", len(settings))
	}

	var locale string
	if err := json.Unmarshal(settings["locale"], &locale); err != nil {
		t.Fatalf("failed to unmarshal locale: %v", err)
	}
	if locale != "zh-CN" {
		t.Fatalf("expected zh-CN, got %s", locale)
	}
}

func TestUpdateSettings_InvalidBody(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/settings", jsonReader("invalid"))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetLibraries_Empty(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/library", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var libraries []db.Library
	if err := json.NewDecoder(resp.Body).Decode(&libraries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(libraries) != 0 {
		t.Fatalf("expected empty libraries, got %d", len(libraries))
	}
}

func TestCreateAndGetLibrary(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	body := `{"root_path":"/media/videos"}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/library", jsonReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var lib db.Library
	if err := json.NewDecoder(resp.Body).Decode(&lib); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if lib.RootPath != "/media/videos" {
		t.Fatalf("expected /media/videos, got %s", lib.RootPath)
	}

	req2, _ := http.NewRequest("GET", ts.URL+"/api/v1/library", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()

	var libraries []db.Library
	if err := json.NewDecoder(resp2.Body).Decode(&libraries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(libraries) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libraries))
	}
}

func TestCreateLibrary_MissingRootPath(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	body := `{}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/library", jsonReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateLibrary_InvalidBody(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/library", jsonReader("invalid"))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetLibraryFiles_MissingLibraryID(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/library/files", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetLibraryFiles_Empty(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/library/files?library_id=999", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var files []LibraryFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected empty files, got %d", len(files))
	}
}

func TestGetLibraryFiles_WithData(t *testing.T) {
	ts, dbq, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	lib := db.Library{RootPath: "/media/anime"}
	if err := dbq.Write(func(tx *gorm.DB) error {
		return tx.Create(&lib).Error
	}); err != nil {
		t.Fatalf("failed to create library: %v", err)
	}

	series := db.Series{ID: "100", Title: "测试番剧"}
	if err := dbq.Write(func(tx *gorm.DB) error {
		return tx.Create(&series).Error
	}); err != nil {
		t.Fatalf("failed to create series: %v", err)
	}

	eps := []db.Episode{
		{ID: "1000011", SeriesID: series.ID, LibraryID: lib.ID, DandanEpisodeID: 1, RelativePath: "S01/E01.mp4", FileMD5: "md5_1", FileHash: "hash_1", EpIndex: ptr[float64](1), MatchStatus: "matched"},
		{ID: "1000022", SeriesID: series.ID, LibraryID: lib.ID, DandanEpisodeID: 2, RelativePath: "S01/E02.mp4", FileMD5: "md5_2", FileHash: "hash_2", EpIndex: ptr[float64](2), MatchStatus: "unmatched"},
	}
	for _, ep := range eps {
		if err := dbq.Write(func(tx *gorm.DB) error {
			return tx.Create(&ep).Error
		}); err != nil {
			t.Fatalf("failed to create episode: %v", err)
		}
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/library/files?library_id=1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var files []LibraryFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].SeriesTitle != "测试番剧" {
		t.Fatalf("expected series_title '测试番剧', got %s", files[0].SeriesTitle)
	}
	if files[0].RelativePath != "S01/E01.mp4" {
		t.Fatalf("expected RelativePath 'S01/E01.mp4', got %s", files[0].RelativePath)
	}
}

func TestGetLibraryFiles_InvalidLibraryID(t *testing.T) {
	ts, _, _ := setupTestAPI(t, "test-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/library/files?library_id=abc", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func ptr[T any](v T) *T {
	return &v
}

func strPtr(s string) *string {
	return &s
}
