package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) getTraderFromShareToken(c *gin.Context) (string, string, error) {
	code := c.Query("token")
	if code == "" {
		return "", "", fmt.Errorf("missing share code")
	}

	link, err := s.store.ShareLink().GetByCode(code)
	if err != nil {
		return "", "", err
	}

	if err := s.traderManager.LoadUserTradersFromStore(s.store, link.UserID); err != nil {
		return "", "", err
	}

	traderCfg, err := s.store.Trader().GetByID(link.TraderID)
	if err != nil {
		return "", "", err
	}
	if traderCfg.UserID != link.UserID {
		return "", "", fmt.Errorf("share token trader mismatch")
	}

	return link.UserID, link.TraderID, nil
}

func (s *Server) handleCreateShareLink(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	traderCfg, err := s.store.Trader().GetByID(traderID)
	if err != nil || traderCfg.UserID != userID {
		SafeNotFound(c, "Trader")
		return
	}

	link, err := s.store.ShareLink().CreateOrGet(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Generate share token", err)
		return
	}

	scheme := "https"
	if c.Request.TLS == nil {
		if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto
		} else {
			scheme = "http"
		}
	}

	baseURL := scheme + "://" + c.Request.Host
	c.JSON(http.StatusOK, gin.H{
		"token": link.Code,
		"url":   baseURL + "/share/" + link.Code,
	})
}

func (s *Server) handleCreatePublicCompetitionShareLink(c *gin.Context) {
	traderID := c.Param("id")

	traderCfg, err := s.store.Trader().GetByID(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	if !traderCfg.ShowInCompetition {
		SafeNotFound(c, "Trader")
		return
	}

	link, err := s.store.ShareLink().CreateOrGet(traderCfg.UserID, traderID)
	if err != nil {
		SafeInternalError(c, "Generate share token", err)
		return
	}

	scheme := "https"
	if c.Request.TLS == nil {
		if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto
		} else {
			scheme = "http"
		}
	}

	baseURL := scheme + "://" + c.Request.Host
	c.JSON(http.StatusOK, gin.H{
		"token": link.Code,
		"url":   baseURL + "/share/" + link.Code,
	})
}

func (s *Server) handleSharedTrader(c *gin.Context) {
	userID, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}

	trader, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	isRunning := trader.Trader.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   trader.Trader.ID,
		"trader_name": trader.Trader.Name,
		"ai_model":    trader.Trader.AIModelID,
		"exchange_id": trader.Trader.ExchangeID,
		"is_running":  isRunning,
		"strategy_id": trader.Trader.StrategyID,
		"created_at":  trader.Trader.CreatedAt,
		"strategy_name": func() string {
			if trader.Strategy != nil {
				return trader.Strategy.Name
			}
			return ""
		}(),
		"initial_balance": trader.Trader.InitialBalance,
	})
}

func (s *Server) handleSharedStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	c.JSON(http.StatusOK, trader.GetStatus())
}

func (s *Server) handleSharedAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	account, err := trader.GetAccountInfo()
	if err != nil {
		if IsRateLimitError(err) {
			SafeRateLimited(c, "Get account info", err)
			return
		}
		SafeInternalError(c, "Get account info", err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func (s *Server) handleSharedPositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	positions, err := trader.GetPositions()
	if err != nil {
		if IsRateLimitError(err) {
			SafeRateLimited(c, "Get positions", err)
			return
		}
		SafeInternalError(c, "Get positions", err)
		return
	}
	c.JSON(http.StatusOK, positions)
}

func (s *Server) handleSharedLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	limit := 5
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 100 {
				limit = 100
			}
		}
	}
	records, err := trader.GetStore().Decision().GetLatestRecords(trader.GetID(), limit)
	if err != nil {
		SafeInternalError(c, "Get decision log", err)
		return
	}
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	c.JSON(http.StatusOK, records)
}

func (s *Server) handleSharedStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	stats, err := trader.GetStore().Decision().GetStatistics(trader.GetID())
	if err != nil {
		SafeInternalError(c, "Get statistics", err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) handleSharedEquityHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}

	snapshots, err := s.store.Equity().GetLatest(traderID, 10000)
	if err != nil {
		SafeInternalError(c, "Get historical data", err)
		return
	}
	if len(snapshots) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`
		AvailableBalance float64 `json:"available_balance"`
		TotalPnL         float64 `json:"total_pnl"`
		TotalPnLPct      float64 `json:"total_pnl_pct"`
		PositionCount    int     `json:"position_count"`
		MarginUsedPct    float64 `json:"margin_used_pct"`
	}

	initialBalance := snapshots[0].Balance
	if initialBalance == 0 {
		initialBalance = 1
	}

	history := make([]EquityPoint, 0, len(snapshots))
	for _, snap := range snapshots {
		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (snap.UnrealizedPnL / initialBalance) * 100
		}
		history = append(history, EquityPoint{
			Timestamp:        snap.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      snap.TotalEquity,
			AvailableBalance: snap.Balance,
			TotalPnL:         snap.UnrealizedPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    snap.PositionCount,
			MarginUsedPct:    snap.MarginUsedPct,
		})
	}

	c.JSON(http.StatusOK, history)
}

func (s *Server) handleSharedPositionHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	limit := 100
	if limitStr := c.DefaultQuery("limit", "100"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}
	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}
	positions, err := store.Position().GetClosedPositions(trader.GetID(), limit)
	if err != nil {
		SafeInternalError(c, "Get position history", err)
		return
	}
	stats, _ := store.Position().GetFullStats(trader.GetID())
	symbolStats, _ := store.Position().GetSymbolStats(trader.GetID(), 10)
	directionStats, _ := store.Position().GetDirectionStats(trader.GetID())
	c.JSON(http.StatusOK, gin.H{
		"positions":       positions,
		"stats":           stats,
		"symbol_stats":    symbolStats,
		"direction_stats": directionStats,
	})
}

func (s *Server) handleSharedOrders(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	query := cloneQueryWithoutToken(c)
	query.Set("trader_id", traderID)
	c.Request.URL.RawQuery = query.Encode()
	c.Set("share_trader_id", traderID)
	s.handleOrders(c)
}

func (s *Server) handleSharedOpenOrders(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	query := cloneQueryWithoutToken(c)
	query.Set("trader_id", traderID)
	c.Request.URL.RawQuery = query.Encode()
	c.Set("share_trader_id", traderID)
	s.handleOpenOrders(c)
}

func (s *Server) handleSharedGridRisk(c *gin.Context) {
	_, traderID, err := s.getTraderFromShareToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired share link"})
		return
	}
	c.Params = append(c.Params, gin.Param{Key: "id", Value: traderID})
	s.handleGetGridRiskInfo(c)
}

func cloneQueryWithoutToken(c *gin.Context) url.Values {
	query := c.Request.URL.Query()
	query.Del("token")
	return query
}
