package store

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type ShareLinkStore struct {
	db *gorm.DB
}

type ShareLink struct {
	Code      string    `gorm:"primaryKey;size:16" json:"code"`
	UserID    string    `gorm:"column:user_id;not null;index" json:"user_id"`
	TraderID  string    `gorm:"column:trader_id;not null;index" json:"trader_id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ShareLink) TableName() string {
	return "share_links"
}

func NewShareLinkStore(db *gorm.DB) *ShareLinkStore {
	return &ShareLinkStore{db: db}
}

func (s *ShareLinkStore) initTables() error {
	if err := s.db.AutoMigrate(&ShareLink{}); err != nil {
		return fmt.Errorf("failed to migrate share_links table: %w", err)
	}
	return nil
}

func (s *ShareLinkStore) CreateOrGet(userID, traderID string) (*ShareLink, error) {
	var existing ShareLink
	if err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).First(&existing).Error; err == nil {
		return &existing, nil
	}

	for i := 0; i < 5; i++ {
		code, err := generateShareCode(8)
		if err != nil {
			return nil, err
		}
		link := &ShareLink{
			Code:     code,
			UserID:   userID,
			TraderID: traderID,
		}
		if err := s.db.Create(link).Error; err == nil {
			return link, nil
		}
	}

	return nil, fmt.Errorf("failed to create share link")
}

func (s *ShareLinkStore) GetByCode(code string) (*ShareLink, error) {
	var link ShareLink
	if err := s.db.Where("code = ?", code).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func generateShareCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate share code: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(bytes)
	if len(code) > length {
		code = code[:length]
	}
	return code, nil
}
