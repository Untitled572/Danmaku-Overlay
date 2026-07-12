package db

import (
	"encoding/json"
	"testing"

	sqlite "github.com/ncruces/go-sqlite3/gormlite"
	gorm "gorm.io/gorm"
)

func setupMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}

	if err := db.AutoMigrate(&Library{}, &Series{}, &Episode{}, &History{}, &Setting{}); err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	return db
}

func TestAutoMigrateCreatesTables(t *testing.T) {
	db := setupMemoryDB(t)

	tables, err := db.Migrator().GetTables()
	if err != nil {
		t.Fatalf("failed to get tables: %v", err)
	}

	expected := map[string]bool{"libraries": true, "series": true, "episodes": true, "history": true, "settings": true}
	for _, table := range tables {
		delete(expected, table)
	}
	for table := range expected {
		t.Errorf("table %q was not created", table)
	}
}

func TestCreateLibrary(t *testing.T) {
	db := setupMemoryDB(t)

	lib := Library{RootPath: "/media/anime"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatalf("failed to create library: %v", err)
	}

	if lib.ID == 0 {
		t.Error("library ID should not be zero after creation")
	}
}

func TestCreateSeries(t *testing.T) {
	db := setupMemoryDB(t)

	bangumiID := uint(42)
	series := Series{
		ID:        "42",
		BangumiID: &bangumiID,
		Title:     "Test Anime",
		CoverPath: strPtr("/covers/test.jpg"),
		TotalEps:  uintPtr(12),
		Summary:   strPtr("A test anime series"),
	}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("failed to create series: %v", err)
	}

	if series.ID == "" {
		t.Error("series ID should not be empty after creation")
	}
}

func TestSeriesBangumiIDUniqueConstraint(t *testing.T) {
	db := setupMemoryDB(t)

	bangumiID := uint(99)
	s1 := Series{ID: "99", BangumiID: &bangumiID, Title: "Series A"}
	if err := db.Create(&s1).Error; err != nil {
		t.Fatalf("failed to create first series: %v", err)
	}

	s2 := Series{ID: "99", BangumiID: &bangumiID, Title: "Series B"}
	err := db.Create(&s2).Error
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate BangumiID, got nil")
	}
}

func TestCreateEpisode(t *testing.T) {
	db := setupMemoryDB(t)

	lib := Library{RootPath: "/media/anime"}
	db.Create(&lib)

	bangumiID := uint(10)
	series := Series{ID: "10", BangumiID: &bangumiID, Title: "Series"}
	db.Create(&series)

	ep := Episode{
		ID:              "100011",
		SeriesID:        series.ID,
		LibraryID:       lib.ID,
		DandanEpisodeID: 1,
		RelativePath:    "season1/ep01.mkv",
		FileMD5:         "abc123",
		FileHash:        "hash001",
		MatchStatus:     "matched",
	}
	if err := db.Create(&ep).Error; err != nil {
		t.Fatalf("failed to create episode: %v", err)
	}

	if ep.ID == "" {
		t.Error("episode ID should not be empty after creation")
	}
}

func TestEpisodeFileHashUniqueConstraint(t *testing.T) {
	db := setupMemoryDB(t)

	lib := Library{RootPath: "/media/anime"}
	db.Create(&lib)

	bangumiID := uint(20)
	series := Series{ID: "20", BangumiID: &bangumiID, Title: "Series"}
	db.Create(&series)

	ep1 := Episode{
		ID:              "200011",
		SeriesID:        series.ID,
		LibraryID:       lib.ID,
		DandanEpisodeID: 1,
		RelativePath:    "ep01.mkv",
		FileMD5:         "md5_1",
		FileHash:        "samehash",
	}
	if err := db.Create(&ep1).Error; err != nil {
		t.Fatalf("failed to create first episode: %v", err)
	}

	ep2 := Episode{
		ID:              "200022",
		SeriesID:        series.ID,
		LibraryID:       lib.ID,
		DandanEpisodeID: 2,
		RelativePath:    "ep02.mkv",
		FileMD5:         "md5_2",
		FileHash:        "samehash",
	}
	err := db.Create(&ep2).Error
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate FileHash, got nil")
	}
}

func TestEpisodeRelativePathUniqueConstraint(t *testing.T) {
	db := setupMemoryDB(t)

	lib := Library{RootPath: "/media/anime"}
	db.Create(&lib)

	bangumiID := uint(30)
	series := Series{ID: "30", BangumiID: &bangumiID, Title: "Series"}
	db.Create(&series)

	ep1 := Episode{
		ID:              "300011",
		SeriesID:        series.ID,
		LibraryID:       lib.ID,
		DandanEpisodeID: 1,
		RelativePath:    "same/path.mkv",
		FileMD5:         "md5_1",
		FileHash:        "hash_a",
	}
	if err := db.Create(&ep1).Error; err != nil {
		t.Fatalf("failed to create first episode: %v", err)
	}

	ep2 := Episode{
		ID:              "300022",
		SeriesID:        series.ID,
		LibraryID:       lib.ID,
		DandanEpisodeID: 2,
		RelativePath:    "same/path.mkv",
		FileMD5:         "md5_2",
		FileHash:        "hash_b",
	}
	err := db.Create(&ep2).Error
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate RelativePath, got nil")
	}
}

func TestCreateHistory(t *testing.T) {
	db := setupMemoryDB(t)

	lib := Library{RootPath: "/media/anime"}
	db.Create(&lib)

	series := Series{ID: "40", Title: "Anime"}
	db.Create(&series)

	ep := Episode{
		ID:              "400011",
		SeriesID:        series.ID,
		LibraryID:       lib.ID,
		DandanEpisodeID: 1,
		RelativePath:    "ep.mkv",
		FileMD5:         "md5",
		FileHash:        "hash_h",
	}
	db.Create(&ep)

	hist := History{
		EpisodeID: ep.ID,
		Position:  123.5,
	}
	if err := db.Create(&hist).Error; err != nil {
		t.Fatalf("failed to create history: %v", err)
	}

	if hist.ID == 0 {
		t.Error("history ID should not be zero after creation")
	}
}

func TestCreateSetting(t *testing.T) {
	db := setupMemoryDB(t)

	val := json.RawMessage(`"dark"`)
	setting := Setting{
		Key:   "theme",
		Value: val,
	}
	if err := db.Create(&setting).Error; err != nil {
		t.Fatalf("failed to create setting: %v", err)
	}

	if setting.ID == 0 {
		t.Error("setting ID should not be zero after creation")
	}
}

func TestSettingKeyUniqueConstraint(t *testing.T) {
	db := setupMemoryDB(t)

	s1 := Setting{Key: "locale", Value: json.RawMessage(`"en"`)}
	if err := db.Create(&s1).Error; err != nil {
		t.Fatalf("failed to create first setting: %v", err)
	}

	s2 := Setting{Key: "locale", Value: json.RawMessage(`"zh"`)}
	err := db.Create(&s2).Error
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate setting key, got nil")
	}
}

func TestSeriesWithNilBangumiID(t *testing.T) {
	db := setupMemoryDB(t)

	s1 := Series{ID: "temp1", Title: "No Bangumi ID 1"}
	if err := db.Create(&s1).Error; err != nil {
		t.Fatalf("failed to create series with nil BangumiID: %v", err)
	}

	s2 := Series{ID: "temp2", Title: "No Bangumi ID 2"}
	if err := db.Create(&s2).Error; err != nil {
		t.Fatalf("failed to create another series with nil BangumiID: %v", err)
	}
}

func strPtr(s string) *string {
	return &s
}

func uintPtr(u uint) *uint {
	return &u
}
