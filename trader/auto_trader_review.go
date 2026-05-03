package trader

import (
    "encoding/json"
    "fmt"
    "math"
    "nofx/logger"
    "nofx/store"
    "strings"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

const reviewLoopMinInterval = 30 * time.Minute

type reviewMetrics struct {
    WindowStart          time.Time `json:"window_start"`
    WindowEnd            time.Time `json:"window_end"`
    StartEquity          float64   `json:"start_equity"`
    EndEquity            float64   `json:"end_equity"`
    ReturnPct            float64   `json:"return_pct"`
    ProfitSum            float64   `json:"profit_sum"`
    LossSum              float64   `json:"loss_sum"`
    LossProfitRatio      float64   `json:"loss_profit_ratio"`
    DecisionCount        int       `json:"decision_count"`
    FillCount            int       `json:"fill_count"`
    ClosedPositionCount  int       `json:"closed_position_count"`
    CurrentPositionCount int       `json:"current_position_count"`
    TargetReturnPct      float64   `json:"target_return_pct"`
    MaxLossProfitRatio   float64   `json:"max_loss_profit_ratio"`
}

type strategyCandidateResponse struct {
    Name            string               `json:"name"`
    Description     string               `json:"description"`
    ChangeReason    string               `json:"change_reason"`
    AnalysisSummary string               `json:"analysis_summary"`
    Config          store.StrategyConfig `json:"config"`
}

func (at *AutoTrader) maybeRunReviewLoop() {
    if at.store == nil || !at.config.ReviewEnabled || at.IsGridStrategy() {
        return
    }
    if !at.lastReviewCheck.IsZero() && time.Since(at.lastReviewCheck) < reviewLoopMinInterval {
        return
    }
    at.lastReviewCheck = time.Now().UTC()

    pending, err := at.store.TraderReview().GetPendingByTrader(at.id)
    if err == nil && pending != nil {
        at.pauseForPendingCandidate("pending strategy candidate awaiting approval")
        return
    }
    if err != nil && err != gorm.ErrRecordNotFound {
        logger.Infof("⚠️ [%s] failed to check pending trader review candidate: %v", at.name, err)
    }

    metrics, decisions, fills, closedPositions, currentPositions, currentStrategy, err := at.evaluateReviewWindow()
    if err != nil {
        logger.Infof("⚠️ [%s] skipped review loop: %v", at.name, err)
        return
    }
    if metrics.ReturnPct >= metrics.TargetReturnPct && metrics.LossProfitRatio <= metrics.MaxLossProfitRatio {
        return
    }

    triggerReason := fmt.Sprintf(
        "4h return %.2f%% (target %.2f%%), loss/profit ratio %.4f (max %.4f)",
        metrics.ReturnPct,
        metrics.TargetReturnPct,
        metrics.LossProfitRatio,
        metrics.MaxLossProfitRatio,
    )
    at.pauseForPendingCandidate(triggerReason)

    candidate, err := at.generateStrategyCandidate(metrics, decisions, fills, closedPositions, currentPositions, currentStrategy, triggerReason)
    if err != nil {
        logger.Infof("❌ [%s] failed to generate strategy candidate: %v", at.name, err)
        return
    }
    logger.Infof("🧠 [%s] strategy candidate saved: %s", at.name, candidate.ID)
}

func (at *AutoTrader) evaluateReviewWindow() (*reviewMetrics, []*store.DecisionRecord, []*store.TraderFill, []*store.TraderPosition, []map[string]interface{}, *store.Strategy, error) {
    window := at.config.ReviewWindow
    if window <= 0 {
        window = 4 * time.Hour
    }
    windowEnd := time.Now().UTC()
    windowStart := windowEnd.Add(-window)

    equity, err := at.store.Equity().GetByTimeRange(at.id, windowStart, windowEnd)
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }
    if len(equity) < 2 {
        return nil, nil, nil, nil, nil, nil, fmt.Errorf("insufficient equity snapshots in review window")
    }
    if equity[0].Timestamp.After(windowStart.Add(30 * time.Minute)) {
        return nil, nil, nil, nil, nil, nil, fmt.Errorf("review window coverage is too short")
    }

    decisions, err := at.store.Decision().GetByTimeRange(at.id, windowStart, windowEnd)
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }

    startMs := windowStart.UnixMilli()
    endMs := windowEnd.UnixMilli()

    fills, err := at.store.Order().GetTraderFillsByTimeRange(at.id, startMs, endMs)
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }

    closedPositions, err := at.store.Position().GetClosedPositionsByTimeRange(at.id, startMs, endMs)
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }

    currentPositions, err := at.GetPositions()
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }

    fullConfig, err := at.store.Trader().GetFullConfig(at.userID, at.id)
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }
    if fullConfig.Strategy == nil {
        return nil, nil, nil, nil, nil, nil, fmt.Errorf("strategy not found for trader")
    }

    profitSum := 0.0
    lossSum := 0.0
    for _, pos := range closedPositions {
        if pos.RealizedPnL >= 0 {
            profitSum += pos.RealizedPnL
        } else {
            lossSum += -pos.RealizedPnL
        }
    }

    lossProfitRatio := 0.0
    switch {
    case profitSum > 0:
        lossProfitRatio = lossSum / profitSum
    case lossSum > 0:
        lossProfitRatio = math.Inf(1)
    }

    startEquity := equity[0].TotalEquity
    endEquity := equity[len(equity)-1].TotalEquity
    returnPct := 0.0
    if startEquity > 0 {
        returnPct = ((endEquity - startEquity) / startEquity) * 100
    }

    metrics := &reviewMetrics{
        WindowStart:          windowStart,
        WindowEnd:            windowEnd,
        StartEquity:          startEquity,
        EndEquity:            endEquity,
        ReturnPct:            returnPct,
        ProfitSum:            profitSum,
        LossSum:              lossSum,
        LossProfitRatio:      lossProfitRatio,
        DecisionCount:        len(decisions),
        FillCount:            len(fills),
        ClosedPositionCount:  len(closedPositions),
        CurrentPositionCount: len(currentPositions),
        TargetReturnPct:      at.config.ReviewTargetReturnPct,
        MaxLossProfitRatio:   at.config.ReviewMaxLossProfitRatio,
    }
    return metrics, decisions, fills, closedPositions, currentPositions, fullConfig.Strategy, nil
}

func (at *AutoTrader) pauseForPendingCandidate(reason string) {
    at.isRunningMutex.Lock()
    wasRunning := at.isRunning
    at.isRunning = false
    at.isRunningMutex.Unlock()

    at.closeStopMonitor()
    if at.store != nil {
        if err := at.store.Trader().UpdateStatus(at.userID, at.id, false); err != nil {
            logger.Infof("⚠️ [%s] failed to persist paused status: %v", at.name, err)
        }
    }
    if wasRunning {
        logger.Infof("⏸ [%s] trader auto-paused: %s", at.name, reason)
    }
}

func (at *AutoTrader) generateStrategyCandidate(metrics *reviewMetrics, decisions []*store.DecisionRecord, fills []*store.TraderFill, closedPositions []*store.TraderPosition, currentPositions []map[string]interface{}, currentStrategy *store.Strategy, triggerReason string) (*store.TraderReviewCandidate, error) {
    existingPending, err := at.store.TraderReview().GetPendingByTrader(at.id)
    if err == nil && existingPending != nil {
        return existingPending, nil
    }
    if err != nil && err != gorm.ErrRecordNotFound {
        return nil, err
    }

    currentConfig, err := currentStrategy.ParseConfig()
    if err != nil {
        return nil, err
    }

    analysisSummary := buildReviewSummary(metrics, decisions, fills, closedPositions, currentPositions)
    response, rawResponse, err := at.requestStrategyCandidateFromAI(currentConfig, metrics, analysisSummary)
    if err != nil {
        return nil, err
    }

    strategyID := uuid.New().String()
    if response.Name == "" {
        response.Name = fmt.Sprintf("%s candidate %s", at.name, time.Now().UTC().Format("20060102-1504"))
    }
    if response.Description == "" {
        response.Description = "Auto-generated candidate from trader closed-loop review"
    }
    if response.ChangeReason == "" {
        response.ChangeReason = triggerReason
    }
    if response.AnalysisSummary == "" {
        response.AnalysisSummary = analysisSummary
    }

    candidateStrategy := &store.Strategy{
        ID:          strategyID,
        UserID:      at.userID,
        Name:        response.Name,
        Description: response.Description,
        IsActive:    false,
        IsDefault:   false,
    }
    if err := candidateStrategy.SetConfig(&response.Config); err != nil {
        return nil, err
    }
    if err := at.store.Strategy().Create(candidateStrategy); err != nil {
        return nil, err
    }

    candidate := &store.TraderReviewCandidate{
        ID:                    uuid.New().String(),
        UserID:                at.userID,
        TraderID:              at.id,
        TraderName:            at.name,
        BaseStrategyID:        currentStrategy.ID,
        CandidateStrategyID:   strategyID,
        Status:                "pending_approval",
        WindowStart:           metrics.WindowStart,
        WindowEnd:             metrics.WindowEnd,
        TargetReturnPct:       metrics.TargetReturnPct,
        ActualReturnPct:       metrics.ReturnPct,
        MaxLossProfitRatio:    metrics.MaxLossProfitRatio,
        ActualLossProfitRatio: metrics.LossProfitRatio,
        TriggerReason:         triggerReason,
        AnalysisSummary:       response.AnalysisSummary,
        ChangeReason:          response.ChangeReason,
        CandidateConfig:       candidateStrategy.Config,
    }
    if err := at.store.TraderReview().RejectOtherPending(at.userID, at.id, candidate.ID); err != nil {
        return nil, err
    }
    if err := at.store.TraderReview().Create(candidate); err != nil {
        return nil, err
    }

    logger.Infof("📝 [%s] saved candidate strategy raw response length=%d", at.name, len(rawResponse))
    return candidate, nil
}

func (at *AutoTrader) requestStrategyCandidateFromAI(currentConfig *store.StrategyConfig, metrics *reviewMetrics, analysisSummary string) (*strategyCandidateResponse, string, error) {
    configJSON, _ := json.MarshalIndent(currentConfig, "", "  ")
    metricsJSON, _ := json.MarshalIndent(metrics, "", "  ")

    systemPrompt := `You are a crypto trading strategy reviewer.
Return only one valid JSON object.
Do not use markdown fences.
The "config" field must be a complete StrategyConfig object, not a partial patch.
Keep risk/reward disciplined and avoid overtrading.`

    userPrompt := fmt.Sprintf(`Trader "%s" has failed its review window.

Target:
- Rolling 4h return >= %.2f%%
- Loss/profit ratio <= %.4f

Metrics:
%s

Current strategy config:
%s

Observed 4h analysis:
%s

Generate one improved full strategy candidate and return JSON with this shape:
{
  "name": "short candidate name",
  "description": "short description",
  "change_reason": "why the current strategy failed and what was changed",
  "analysis_summary": "concise summary of the 4h review",
  "config": { complete StrategyConfig JSON object }
}

The candidate must explicitly revise prompt sections, trading frequency, entry standards, decision process, and risk parameters.
Do not propose auto-restart.`, at.name, metrics.TargetReturnPct, metrics.MaxLossProfitRatio, string(metricsJSON), string(configJSON), analysisSummary)

    rawResponse, err := at.mcpClient.CallWithMessages(systemPrompt, userPrompt)
    if err != nil {
        return nil, "", err
    }

    payload, err := extractJSONObject(rawResponse)
    if err != nil {
        return nil, rawResponse, err
    }

    var response strategyCandidateResponse
    if err := json.Unmarshal([]byte(payload), &response); err != nil {
        return nil, rawResponse, fmt.Errorf("failed to parse strategy candidate JSON: %w", err)
    }

    if response.Config.RiskControl.MinRiskRewardRatio <= 0 {
        response.Config.RiskControl.MinRiskRewardRatio = currentConfig.RiskControl.MinRiskRewardRatio
    }
    if response.Config.RiskControl.MinRiskRewardRatio < 3.0 {
        response.Config.RiskControl.MinRiskRewardRatio = 3.0
    }
    if response.Config.Language == "" {
        response.Config.Language = currentConfig.Language
    }
    return &response, rawResponse, nil
}

func buildReviewSummary(metrics *reviewMetrics, decisions []*store.DecisionRecord, fills []*store.TraderFill, closedPositions []*store.TraderPosition, currentPositions []map[string]interface{}) string {
    lines := []string{
        fmt.Sprintf("Window: %s to %s UTC", metrics.WindowStart.Format(time.RFC3339), metrics.WindowEnd.Format(time.RFC3339)),
        fmt.Sprintf("Return: %.2f%% on equity %.2f -> %.2f", metrics.ReturnPct, metrics.StartEquity, metrics.EndEquity),
        fmt.Sprintf("Loss/profit ratio: %.4f with profit_sum=%.2f and loss_sum=%.2f", metrics.LossProfitRatio, metrics.ProfitSum, metrics.LossSum),
        fmt.Sprintf("Decisions=%d, fills=%d, closed_positions=%d, open_positions=%d", metrics.DecisionCount, metrics.FillCount, metrics.ClosedPositionCount, metrics.CurrentPositionCount),
    }

    if len(decisions) > 0 {
        lines = append(lines, "Recent decisions:")
        start := 0
        if len(decisions) > 5 {
            start = len(decisions) - 5
        }
        for _, decision := range decisions[start:] {
            lines = append(lines, fmt.Sprintf("- %s success=%t decisions=%d err=%s", decision.Timestamp.Format(time.RFC3339), decision.Success, len(decision.Decisions), decision.ErrorMessage))
        }
    }

    if len(closedPositions) > 0 {
        lines = append(lines, "Closed positions:")
        start := 0
        if len(closedPositions) > 5 {
            start = len(closedPositions) - 5
        }
        for _, pos := range closedPositions[start:] {
            lines = append(lines, fmt.Sprintf("- %s %s pnl=%.2f hold=%s close_reason=%s", pos.Symbol, pos.Side, pos.RealizedPnL, formatDurationMs(pos.ExitTime-pos.EntryTime), pos.CloseReason))
        }
    }

    if len(fills) > 0 {
        lines = append(lines, "Recent fills:")
        start := 0
        if len(fills) > 5 {
            start = len(fills) - 5
        }
        for _, fill := range fills[start:] {
            lines = append(lines, fmt.Sprintf("- %s %s qty=%.4f price=%.4f realized=%.2f", fill.Symbol, fill.Side, fill.Quantity, fill.Price, fill.RealizedPnL))
        }
    }

    if len(currentPositions) > 0 {
        lines = append(lines, "Current positions:")
        for i, pos := range currentPositions {
            if i >= 5 {
                break
            }
            lines = append(lines, fmt.Sprintf("- %v %v unrealized=%v leverage=%v", pos["symbol"], pos["side"], pos["unrealized_pnl"], pos["leverage"]))
        }
    }

    return strings.Join(lines, "\n")
}

func extractJSONObject(raw string) (string, error) {
    trimmed := strings.TrimSpace(raw)
    if strings.HasPrefix(trimmed, "```") {
        trimmed = strings.TrimPrefix(trimmed, "```json")
        trimmed = strings.TrimPrefix(trimmed, "```JSON")
        trimmed = strings.TrimPrefix(trimmed, "```")
        trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
    }

    start := strings.Index(trimmed, "{")
    end := strings.LastIndex(trimmed, "}")
    if start < 0 || end < 0 || end < start {
        return "", fmt.Errorf("no JSON object found in AI response")
    }
    return trimmed[start : end+1], nil
}
