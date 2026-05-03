package store

import (
    "fmt"
    "time"

    "gorm.io/gorm"
)

// TraderReviewStore stores closed-loop review outcomes and strategy candidates.
type TraderReviewStore struct {
    db *gorm.DB
}

// TraderReviewCandidate stores one generated strategy candidate for a trader.
type TraderReviewCandidate struct {
    ID                    string     `gorm:"primaryKey" json:"id"`
    UserID                string     `gorm:"column:user_id;not null;index" json:"user_id"`
    TraderID              string     `gorm:"column:trader_id;not null;index" json:"trader_id"`
    TraderName            string     `gorm:"column:trader_name;not null" json:"trader_name"`
    BaseStrategyID        string     `gorm:"column:base_strategy_id;default:''" json:"base_strategy_id"`
    CandidateStrategyID   string     `gorm:"column:candidate_strategy_id;not null" json:"candidate_strategy_id"`
    Status                string     `gorm:"column:status;not null;default:pending_approval;index" json:"status"`
    WindowStart           time.Time  `gorm:"column:window_start;not null" json:"window_start"`
    WindowEnd             time.Time  `gorm:"column:window_end;not null" json:"window_end"`
    TargetReturnPct       float64    `gorm:"column:target_return_pct;not null" json:"target_return_pct"`
    ActualReturnPct       float64    `gorm:"column:actual_return_pct;not null" json:"actual_return_pct"`
    MaxLossProfitRatio    float64    `gorm:"column:max_loss_profit_ratio;not null" json:"max_loss_profit_ratio"`
    ActualLossProfitRatio float64    `gorm:"column:actual_loss_profit_ratio;not null" json:"actual_loss_profit_ratio"`
    TriggerReason         string     `gorm:"column:trigger_reason;type:text" json:"trigger_reason"`
    AnalysisSummary       string     `gorm:"column:analysis_summary;type:text" json:"analysis_summary"`
    ChangeReason          string     `gorm:"column:change_reason;type:text" json:"change_reason"`
    CandidateConfig       string     `gorm:"column:candidate_config;type:text" json:"candidate_config"`
    ApprovedAt            *time.Time `gorm:"column:approved_at" json:"approved_at,omitempty"`
    CreatedAt             time.Time  `json:"created_at"`
    UpdatedAt             time.Time  `json:"updated_at"`
}

func (TraderReviewCandidate) TableName() string { return "trader_review_candidates" }

func NewTraderReviewStore(db *gorm.DB) *TraderReviewStore {
    return &TraderReviewStore{db: db}
}

func (s *TraderReviewStore) initTables() error {
    return s.db.AutoMigrate(&TraderReviewCandidate{})
}

func (s *TraderReviewStore) Create(candidate *TraderReviewCandidate) error {
    return s.db.Create(candidate).Error
}

func (s *TraderReviewStore) Get(userID, traderID, candidateID string) (*TraderReviewCandidate, error) {
    var candidate TraderReviewCandidate
    err := s.db.Where("id = ? AND user_id = ? AND trader_id = ?", candidateID, userID, traderID).First(&candidate).Error
    if err != nil {
        return nil, err
    }
    return &candidate, nil
}

func (s *TraderReviewStore) GetPendingByTrader(traderID string) (*TraderReviewCandidate, error) {
    var candidate TraderReviewCandidate
    err := s.db.Where("trader_id = ? AND status = ?", traderID, "pending_approval").
        Order("created_at DESC").
        First(&candidate).Error
    if err != nil {
        return nil, err
    }
    return &candidate, nil
}

func (s *TraderReviewStore) ListByTrader(userID, traderID string, limit int) ([]*TraderReviewCandidate, error) {
    var candidates []*TraderReviewCandidate
    query := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).Order("created_at DESC")
    if limit > 0 {
        query = query.Limit(limit)
    }
    if err := query.Find(&candidates).Error; err != nil {
        return nil, err
    }
    return candidates, nil
}

func (s *TraderReviewStore) UpdateStatus(userID, traderID, candidateID, status string, approvedAt *time.Time) error {
    updates := map[string]interface{}{
        "status":     status,
        "updated_at": time.Now().UTC(),
    }
    if approvedAt != nil {
        updates["approved_at"] = approvedAt.UTC()
    }
    return s.db.Model(&TraderReviewCandidate{}).
        Where("id = ? AND user_id = ? AND trader_id = ?", candidateID, userID, traderID).
        Updates(updates).Error
}

func (s *TraderReviewStore) RejectOtherPending(userID, traderID, exceptID string) error {
    query := s.db.Model(&TraderReviewCandidate{}).
        Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, "pending_approval")
    if exceptID != "" {
        query = query.Where("id <> ?", exceptID)
    }
    if err := query.Updates(map[string]interface{}{
        "status":     "superseded",
        "updated_at": time.Now().UTC(),
    }).Error; err != nil {
        return fmt.Errorf("failed to supersede pending candidates: %w", err)
    }
    return nil
}
