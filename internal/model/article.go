package model

import "time"

type Article struct {
	Base
	Title       string     `gorm:"size:200;not null" json:"title"`
	Slug        string     `gorm:"size:180;not null;uniqueIndex" json:"slug"`
	Summary     string     `gorm:"size:500;not null;default:''" json:"summary"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	Cover       string     `gorm:"size:255;not null;default:''" json:"cover"`
	Category    string     `gorm:"size:80;not null;default:'';index" json:"category"`
	Status      int        `gorm:"not null;index" json:"status"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	AuthorID    string     `gorm:"size:26;not null;default:'';index" json:"author_id"`
}

func (Article) TableName() string {
	return "articles"
}

const (
	ArticleStatusDraft     = 0
	ArticleStatusPublished = 1
	ArticleStatusArchived  = 2
)

func (a Article) IsPublished() bool {
	return a.Status == ArticleStatusPublished
}
