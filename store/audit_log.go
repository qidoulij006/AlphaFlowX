package store

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// AuditLogStore stores admin audit entries.
type AuditLogStore struct {
	db *gorm.DB
}

// AuditLog represents a control-plane action record.
type AuditLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ActorID    string    `gorm:"column:actor_id;not null;index" json:"actor_id"`
	ActorEmail string    `gorm:"column:actor_email;not null" json:"actor_email"`
	Action     string    `gorm:"column:action;not null;index" json:"action"`
	TargetType string    `gorm:"column:target_type;not null" json:"target_type"`
	TargetID   string    `gorm:"column:target_id;not null" json:"target_id"`
	Summary    string    `gorm:"column:summary;not null" json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

func NewAuditLogStore(db *gorm.DB) *AuditLogStore {
	return &AuditLogStore{db: db}
}

func (s *AuditLogStore) initTables() error {
	return s.db.AutoMigrate(&AuditLog{})
}

func (s *AuditLogStore) Create(entry *AuditLog) error {
	return s.db.Create(entry).Error
}

func (s *AuditLogStore) ListRecent(limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 20
	}
	var logs []AuditLog
	err := s.db.Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

func (s *AuditLogStore) ListFiltered(limit int, action, query string) ([]AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	tx := s.db.Model(&AuditLog{})
	if action = strings.TrimSpace(action); action != "" {
		tx = tx.Where("action = ?", action)
	}
	if query = strings.TrimSpace(query); query != "" {
		like := "%" + query + "%"
		tx = tx.Where(
			"actor_email LIKE ? OR target_type LIKE ? OR target_id LIKE ? OR summary LIKE ?",
			like, like, like, like,
		)
	}
	var logs []AuditLog
	err := tx.Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, err
}
