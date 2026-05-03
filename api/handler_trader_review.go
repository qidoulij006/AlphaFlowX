package api

import (
    "encoding/json"
    "net/http"
    "time"

    "nofx/logger"

    "github.com/gin-gonic/gin"
)

func (s *Server) handleListTraderReviewCandidates(c *gin.Context) {
    userID := c.GetString("user_id")
    traderID := c.Param("id")

    if _, err := s.store.Trader().GetFullConfig(userID, traderID); err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
        return
    }

    candidates, err := s.store.TraderReview().ListByTrader(userID, traderID, 20)
    if err != nil {
        SafeInternalError(c, "Failed to get trader review candidates", err)
        return
    }

    result := make([]gin.H, 0, len(candidates))
    for _, candidate := range candidates {
        var cfg map[string]interface{}
        _ = json.Unmarshal([]byte(candidate.CandidateConfig), &cfg)
        result = append(result, gin.H{
            "id":                       candidate.ID,
            "status":                   candidate.Status,
            "base_strategy_id":         candidate.BaseStrategyID,
            "candidate_strategy_id":    candidate.CandidateStrategyID,
            "window_start":             candidate.WindowStart,
            "window_end":               candidate.WindowEnd,
            "target_return_pct":        candidate.TargetReturnPct,
            "actual_return_pct":        candidate.ActualReturnPct,
            "max_loss_profit_ratio":    candidate.MaxLossProfitRatio,
            "actual_loss_profit_ratio": candidate.ActualLossProfitRatio,
            "trigger_reason":           candidate.TriggerReason,
            "analysis_summary":         candidate.AnalysisSummary,
            "change_reason":            candidate.ChangeReason,
            "candidate_config":         cfg,
            "approved_at":              candidate.ApprovedAt,
            "created_at":               candidate.CreatedAt,
            "updated_at":               candidate.UpdatedAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{"candidates": result})
}

func (s *Server) handleApproveTraderReviewCandidate(c *gin.Context) {
    userID := c.GetString("user_id")
    traderID := c.Param("id")
    candidateID := c.Param("candidateId")

    fullConfig, err := s.store.Trader().GetFullConfig(userID, traderID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
        return
    }

    candidate, err := s.store.TraderReview().Get(userID, traderID, candidateID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Candidate does not exist"})
        return
    }
    if candidate.Status != "pending_approval" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Candidate is not pending approval"})
        return
    }

    now := time.Now().UTC()
    if err := s.store.Trader().UpdateStrategyID(userID, traderID, candidate.CandidateStrategyID); err != nil {
        SafeInternalError(c, "Failed to switch trader strategy", err)
        return
    }
    if err := s.store.TraderReview().UpdateStatus(userID, traderID, candidateID, "approved", &now); err != nil {
        SafeInternalError(c, "Failed to approve candidate", err)
        return
    }

    if loaded, err := s.traderManager.GetTrader(traderID); err == nil && loaded != nil {
        loaded.Stop()
    }
    s.traderManager.RemoveTrader(traderID)
    if err := s.traderManager.LoadUserTradersFromStore(s.store, userID); err != nil {
        logger.Infof("⚠️ failed to reload trader after candidate approval: %v", err)
    }

    c.JSON(http.StatusOK, gin.H{
        "message":               "Candidate approved and strategy switched. Trader remains stopped until you start it manually.",
        "trader_id":             traderID,
        "trader_name":           fullConfig.Trader.Name,
        "strategy_id":           candidate.CandidateStrategyID,
        "candidate_id":          candidate.ID,
        "manual_restart_needed": true,
    })
}
