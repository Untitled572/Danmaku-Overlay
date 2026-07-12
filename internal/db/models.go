package db

import (
	"encoding/json"
	"time"
)

type Library struct {
	ID       uint   `gorm:"primaryKey"`
	RootPath string `gorm:"not null"`
}

func (Library) TableName() string {
	return "libraries"
}

type Series struct {
	ID        string    `gorm:"primaryKey;uniqueIndex"` // BangumiID
	BangumiID *uint     `gorm:"uniqueIndex"`
	Title     string    `gorm:"not null;index"`
	NameCN    *string   `gorm:"index"`
	CoverPath *string
	TotalEps  *uint
	CurrentEp *uint     `gorm:"default:0"`
	AirDate   *string   // yyyy-mm 格式
	Rating    *float64
	Tags      *string   // JSON 数组
	Summary      *string
	LastPlayedAt time.Time
}

func (Series) TableName() string {
	return "series"
}

type Episode struct {
	ID              string    `gorm:"primaryKey;uniqueIndex"` // 格式: bangumiID+epIndex+check
	SeriesID        string    `gorm:"index"`                  // 关联 Series.ID (BangumiID)
	LibraryID       uint
	DandanEpisodeID uint      `gorm:"not null"`
	RelativePath    string    `gorm:"not null;uniqueIndex"`
	FileMD5         string    `gorm:"not null"`
	FileHash        string    `gorm:"not null;uniqueIndex"`
	DanmakuPath     *string
	EpIndex         *float64
	MatchStatus     string    `gorm:"default:unmatched;index"`
	ScrapeStatus    string    `gorm:"default:unscraped;index"`
	WatchProgress   float64 `gorm:"default:0"`
	LastPlayedAt    time.Time
}

func (Episode) TableName() string {
	return "episodes"
}

type History struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"default:1"`
	EpisodeID string    `gorm:"index;constraint:OnDelete:CASCADE"`
	Position  float64   `gorm:"default:0"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (History) TableName() string {
	return "history"
}

type Setting struct {
	ID     uint            `gorm:"primaryKey"`
	UserID uint            `gorm:"default:1"`
	Key    string          `gorm:"uniqueIndex"`
	Value  json.RawMessage `gorm:"not null"`
}

func (Setting) TableName() string {
	return "settings"
}
