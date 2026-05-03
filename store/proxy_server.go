package store

import (
	"fmt"
	"nofx/crypto"
	"nofx/logger"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProxyServerStore struct {
	db *gorm.DB
}

type ProxyServer struct {
	ID             string                 `gorm:"primaryKey" json:"id"`
	UserID         string                 `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	Name           string                 `gorm:"column:name;not null;default:''" json:"name"`
	ProxyURL       crypto.EncryptedString `gorm:"column:proxy_url;default:''" json:"proxy_url"`
	Enabled        bool                   `gorm:"column:enabled;default:true" json:"enabled"`
	LastTestStatus string                 `gorm:"column:last_test_status;default:''" json:"last_test_status"`
	LastExitIP     string                 `gorm:"column:last_exit_ip;default:''" json:"last_exit_ip"`
	LastTestAt     *time.Time             `gorm:"column:last_test_at" json:"last_test_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

func (ProxyServer) TableName() string { return "proxy_servers" }

func NewProxyServerStore(db *gorm.DB) *ProxyServerStore {
	return &ProxyServerStore{db: db}
}

func (s *ProxyServerStore) initTables() error {
	if err := s.db.AutoMigrate(&ProxyServer{}); err != nil {
		return err
	}
	return nil
}

func (s *ProxyServerStore) List(userID string) ([]*ProxyServer, error) {
	var proxies []*ProxyServer
	err := s.db.Where("user_id = ?", userID).Order("updated_at DESC, created_at DESC").Find(&proxies).Error
	return proxies, err
}

func (s *ProxyServerStore) GetByID(userID, id string) (*ProxyServer, error) {
	var proxy ProxyServer
	err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (s *ProxyServerStore) GetByIDAny(id string) (*ProxyServer, error) {
	var proxy ProxyServer
	err := s.db.Where("id = ?", id).First(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (s *ProxyServerStore) Create(userID, name, proxyURL, lastExitIP string) (string, error) {
	if name == "" {
		name = "Proxy"
	}
	id := uuid.New().String()
	now := time.Now().UTC()
	lastTestStatus := ""
	var lastTestAt *time.Time
	if lastExitIP != "" {
		lastTestStatus = "passed"
		lastTestAt = &now
	}
	proxy := &ProxyServer{
		ID:             id,
		UserID:         userID,
		Name:           name,
		ProxyURL:       crypto.EncryptedString(proxyURL),
		Enabled:        true,
		LastTestStatus: lastTestStatus,
		LastExitIP:     lastExitIP,
		LastTestAt:     lastTestAt,
	}
	if err := s.db.Create(proxy).Error; err != nil {
		return "", err
	}
	return id, nil
}

func (s *ProxyServerStore) Update(userID, id, name, proxyURL, lastExitIP string, enabled bool) error {
	updates := map[string]interface{}{
		"name":       name,
		"enabled":    enabled,
		"updated_at": time.Now().UTC(),
	}
	if proxyURL != "" {
		updates["proxy_url"] = crypto.EncryptedString(proxyURL)
	}
	if lastExitIP != "" {
		now := time.Now().UTC()
		updates["last_exit_ip"] = lastExitIP
		updates["last_test_status"] = "passed"
		updates["last_test_at"] = &now
	}
	result := s.db.Model(&ProxyServer{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("proxy server not found: id=%s, userID=%s", id, userID)
	}
	return nil
}

func (s *ProxyServerStore) UpdateTestResult(userID, id, status, exitIP string) error {
	now := time.Now().UTC()
	result := s.db.Model(&ProxyServer{}).Where("id = ? AND user_id = ?", id, userID).Updates(map[string]interface{}{
		"last_test_status": status,
		"last_exit_ip":     exitIP,
		"last_test_at":     &now,
		"updated_at":       now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("proxy server not found: id=%s, userID=%s", id, userID)
	}
	return nil
}

func (s *ProxyServerStore) Delete(userID, id string) error {
	result := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&ProxyServer{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("proxy server not found: id=%s, userID=%s", id, userID)
	}
	logger.Infof("🗑️ Deleted proxy server: id=%s, userID=%s", id, userID)
	return nil
}
