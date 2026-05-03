package api

import "strings"

type traderErrorHint struct {
	Code    string
	Message string
	Raw     string
}

func (s *Server) getTraderErrorHint(traderID string) *traderErrorHint {
	if loadErr := s.traderManager.GetLoadError(traderID); loadErr != nil {
		return buildTraderErrorHint(loadErr.Error())
	}

	records, err := s.store.Decision().GetLatestRecords(traderID, 1)
	if err != nil || len(records) == 0 {
		return nil
	}

	latest := records[len(records)-1]
	if latest == nil {
		return nil
	}

	raw := strings.TrimSpace(latest.ErrorMessage)
	if raw == "" {
		for i := len(latest.Decisions) - 1; i >= 0; i-- {
			if strings.TrimSpace(latest.Decisions[i].Error) != "" {
				raw = strings.TrimSpace(latest.Decisions[i].Error)
				break
			}
		}
	}

	return buildTraderErrorHint(raw)
}

func buildTraderErrorHint(raw string) *traderErrorHint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	lowerRaw := strings.ToLower(raw)
	switch {
	case strings.Contains(lowerRaw, "you can't complete this request under your current account mode"):
		return &traderErrorHint{
			Code:    "okx_account_mode_mismatch",
			Message: "OKX 账户模式与系统下单模式不兼容。请检查双向持仓(Hedge Mode)、禁用仅支持 net mode 的 Portfolio Margin，并使用 Cross 保证金。",
			Raw:     raw,
		}
	case strings.Contains(lowerRaw, "order's position side does not match user's setting"):
		return &traderErrorHint{
			Code:    "binance_position_mode_mismatch",
			Message: "交易所持仓模式与系统下单方向不匹配。请检查是否已切换到双向持仓(Hedge Mode)。",
			Raw:     raw,
		}
	default:
		return &traderErrorHint{
			Code:    "generic_trader_error",
			Message: raw,
			Raw:     raw,
		}
	}
}
