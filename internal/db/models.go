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
	ID        uint      `gorm:"primaryKey"`
	BangumiID *uint     `gorm:"uniqueIndex"`
	Title     string    `gorm:"not null;index"`
	NameCN    *string   `gorm:"index"`
	CoverPath *string
	TotalEps  *uint
	AirDate   *string
	Summary   *string
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (Series) TableName() string {
	return "series"
}

type Episode struct {
	ID              uint     `gorm:"primaryKey"`
	SeriesID        uint     `gorm:"index"`
	LibraryID       uint
	DandanEpisodeID uint     `gorm:"not null"`
	RelativePath    string   `gorm:"not null;uniqueIndex"`
	FileMD5         string   `gorm:"not null"`
	FileHash        string   `gorm:"not null;uniqueIndex"`
	DanmakuPath     *string
	EpIndex         *float64
	MatchStatus     string   `gorm:"default:matched;index"`
}

func (Episode) TableName() string {
	return "episodes"
}

type History struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"default:1"`
	EpisodeID uint      `gorm:"index;constraint:OnDelete:CASCADE"`
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
