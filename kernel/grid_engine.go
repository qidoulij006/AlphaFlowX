package kernel

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"strings"
	"time"
)

func formatGridPrice(price float64) string {
	switch {
	case price >= 1000:
		return fmt.Sprintf("%.2f", price)
	case price >= 1:
		return fmt.Sprintf("%.4f", price)
	case price >= 0.1:
		return fmt.Sprintf("%.5f", price)
	case price > 0:
		return fmt.Sprintf("%.6f", price)
	default:
		return fmt.Sprintf("%.2f", price)
	}
}

func formatGridPct(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

// ============================================================================
// Grid Trading Context and Types
// ============================================================================

// GridLevelInfo represents a single grid level's current state
type GridLevelInfo struct {
	Index             int     `json:"index"`               // Level index (0 = lowest)
	Price             float64 `json:"price"`               // Target price for this level
	State             string  `json:"state"`               // "empty", "pending", "filled"
	Side              string  `json:"side"`                // "buy" or "sell"
	OrderID           string  `json:"order_id"`            // Current order ID (if pending)
	OrderQuantity     float64 `json:"order_quantity"`      // Order quantity
	OrderPositionSide string  `json:"order_position_side"` // LONG/SHORT for pending order
	OrderReduceOnly   bool    `json:"order_reduce_only"`   // Whether pending order only reduces
	LinkedLevelIndex  int     `json:"linked_level_index"`  // Linked filled level for paired exits
	PositionSize      float64 `json:"position_size"`       // Position size (if filled)
	PositionEntry     float64 `json:"position_entry"`      // Entry price (if filled)
	PositionSide      string  `json:"position_side"`       // LONG/SHORT for filled position
	AllocatedUSD      float64 `json:"allocated_usd"`       // USD allocated to this level
	UnrealizedPnL     float64 `json:"unrealized_pnl"`      // Unrealized P&L (if filled)
}

type GridOrderSummary struct {
	Bucket             string    `json:"bucket"`
	Price              float64   `json:"price"`
	TotalQuantity      float64   `json:"total_quantity"`
	TotalNotional      float64   `json:"total_notional"`
	OrderCount         int       `json:"order_count"`
	LevelIndexes       []int     `json:"level_indexes,omitempty"`
	LinkedLevelIndexes []int     `json:"linked_level_indexes,omitempty"`
	SourceEntryPrices  []float64 `json:"source_entry_prices,omitempty"`
}

type GridCollapsedLevelWarning struct {
	Price        float64 `json:"price"`
	LevelIndexes []int   `json:"level_indexes"`
}

// GridContext contains all information needed for AI grid decision making
type GridContext struct {
	// Basic info
	Symbol       string  `json:"symbol"`
	CurrentTime  string  `json:"current_time"`
	CurrentPrice float64 `json:"current_price"`

	// Grid configuration
	GridCount       int     `json:"grid_count"`
	TotalInvestment float64 `json:"total_investment"`
	Leverage        int     `json:"leverage"`
	UpperPrice      float64 `json:"upper_price"`
	LowerPrice      float64 `json:"lower_price"`
	GridSpacing     float64 `json:"grid_spacing"`
	Distribution    string  `json:"distribution"`

	// Grid state
	Levels                 []GridLevelInfo             `json:"levels"`
	ActiveOrderCount       int                         `json:"active_order_count"`
	FilledLevelCount       int                         `json:"filled_level_count"`
	ActiveLongOrderCount   int                         `json:"active_long_order_count"`
	ActiveShortOrderCount  int                         `json:"active_short_order_count"`
	ReduceOnlyOrderCount   int                         `json:"reduce_only_order_count"`
	FilledLongLevelCount   int                         `json:"filled_long_level_count"`
	FilledShortLevelCount  int                         `json:"filled_short_level_count"`
	IsPaused               bool                        `json:"is_paused"`
	AggregatedOpenOrders   []GridOrderSummary          `json:"aggregated_open_orders,omitempty"`
	CollapsedLevelWarnings []GridCollapsedLevelWarning `json:"collapsed_level_warnings,omitempty"`

	// Market data
	ATR14           float64 `json:"atr14"`
	BollingerUpper  float64 `json:"bollinger_upper"`
	BollingerMiddle float64 `json:"bollinger_middle"`
	BollingerLower  float64 `json:"bollinger_lower"`
	BollingerWidth  float64 `json:"bollinger_width"` // Percentage
	EMA20           float64 `json:"ema20"`
	EMA50           float64 `json:"ema50"`
	EMADistance     float64 `json:"ema_distance"` // Percentage
	RSI14           float64 `json:"rsi14"`
	MACD            float64 `json:"macd"`
	MACDSignal      float64 `json:"macd_signal"`
	MACDHistogram   float64 `json:"macd_histogram"`
	FundingRate     float64 `json:"funding_rate"`
	Volume24h       float64 `json:"volume_24h"`
	PriceChange1h   float64 `json:"price_change_1h"`
	PriceChange4h   float64 `json:"price_change_4h"`

	// Account info
	TotalEquity          float64 `json:"total_equity"`
	AvailableBalance     float64 `json:"available_balance"`
	CurrentPosition      float64 `json:"current_position"` // Net position size
	CurrentLongPosition  float64 `json:"current_long_position"`
	CurrentShortPosition float64 `json:"current_short_position"`
	UnrealizedPnL        float64 `json:"unrealized_pnl"`

	// Performance
	TotalProfit   float64 `json:"total_profit"`
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	MaxDrawdown   float64 `json:"max_drawdown"`
	DailyPnL      float64 `json:"daily_pnl"`

	// Box indicators (Donchian Channels)
	BoxData *market.BoxData `json:"box_data,omitempty"`

	// Grid direction (neutral, long, short, long_bias, short_bias)
	CurrentDirection string `json:"current_direction,omitempty"`
}

// ============================================================================
// Grid Prompt Building
// ============================================================================

// BuildGridSystemPrompt builds the system prompt for grid trading AI
func BuildGridSystemPrompt(config *store.GridStrategyConfig, lang string) string {
	if lang == "zh" {
		return buildGridSystemPromptZh(config)
	}
	return buildGridSystemPromptEn(config)
}

func buildGridSystemPromptZh(config *store.GridStrategyConfig) string {
	return fmt.Sprintf(`# 你是一个专业的网格交易AI

## 角色定义
你是一个经验丰富的网格交易专家，负责管理 %s 的网格交易策略。你的任务是：
1. 判断当前市场状态（震荡/趋势/高波动）
2. 决定是否需要调整网格或暂停交易
3. 管理每个网格层级的订单
4. 使用经典双向闭环网格，不要把卖单简单理解为开空，也不要把买单简单理解为开多

## 网格配置
- 交易对: %s
- 网格层数: %d
- 总投资: %.2f USDT
- 杠杆: %dx
- 价格分布: %s

## 决策规则

### 市场状态判断
- **震荡市场** (适合网格): 布林带宽度 < 3%%，且价格仍在中期箱体内；EMA20/50 距离 < 1%% 时属于标准震荡，1%%-2%% 视为箱体内早期方向偏移，不应仅凭这一点直接暂停网格
- **趋势市场** (暂停网格): 价格有效突破中期/长期箱体，或布林带宽度 > 4%% 且 EMA20/50 距离 > 2%%，并伴随持续突破布林带
- **高波动市场** (谨慎): ATR异常放大, 价格剧烈波动

### 可执行的操作
- place_buy_limit: 在指定价格下买入限价单
- place_sell_limit: 在指定价格下卖出限价单
- cancel_order: 取消指定订单
- cancel_all_orders: 取消所有订单
- pause_grid: 暂停网格交易（趋势市场时）
- resume_grid: 恢复网格交易（震荡市场时）
- adjust_grid: 调整网格边界
- hold: 保持当前状态不操作

## 经典双向闭环网格语义
- 下方买单优先用于两种情况: 1) 若上方已有空头持仓，则该买单用于平空获利; 2) 若没有可平的空头，则该买单用于开多
- 上方卖单优先用于两种情况: 1) 若下方已有多头持仓，则该卖单用于平多获利; 2) 若没有可平的多头，则该卖单用于开空
- 你的职责是判断哪些价位应该挂买单、哪些价位应该挂卖单，以及是否需要暂停、撤单、调整网格
- 不要因为想“平多/平空”而改用其他 action，仍然输出 place_buy_limit 或 place_sell_limit，执行层会根据已有持仓自动决定是平仓还是开仓
- 趋势市场或突破市场中，应优先建议 pause_grid、cancel_order、cancel_all_orders、adjust_grid，而不是继续密集挂单
- 对于 cancel_order，优先使用 level_index 指向要撤掉的挂单层级；只有在上下文明确提供真实交易所订单号时才填写 order_id
- 禁止编造 order_id。不要输出 L1/L9、价格、中文描述、挂单类型、说明文字等伪订单号
- 若价格仍在中期箱体内，且布林带宽度 < 3%%、EMA20/50 距离仍未超过 2%%，不要仅因“轻微趋势感”就建议 pause_grid。此时应优先维持或补齐箱体内的完整网格挂单
- 若市场不再适合网格，允许取消未成交挂单；已成交仓位将由风控和止损逻辑继续管理
- 当上下文同时提供“网格层级详情”和“实际挂单摘要”时，优先依据“实际挂单摘要”描述真实挂单价格、数量和对应层级，不要只复述逻辑层级编号
- 若发现多个层级收敛到同一实际价格，应明确指出这意味着当前网格间距偏小或价格精度过粗
- 若已成交层存在，但“实际挂单摘要”为空或减仓挂单数量为0，不要写“已有挂单等待触发”。应明确说明当前没有真实挂单，或平仓单补挂失败，正在等待下一轮重试
- 只有当“实际挂单摘要”里确实存在真实开仓/平仓挂单时，才可以描述为“等待价格触发成交”
- 若市场判断为震荡且网格未暂停，但“实际挂单摘要”里没有任何做多挂单或做空挂单，不要简单输出 hold。应把这视为网格挂单不足，优先建议补挂缺失的开仓单，除非风险约束明确阻止这样做

## 输出格式
输出JSON数组，每个决策包含:
- symbol: 交易对
- action: 操作类型
- price: 价格（限价单用）
- quantity: 数量
- level_index: 网格层级索引
- order_id: 订单ID（仅当上下文明确提供真实交易所订单号时填写，否则留空）
- confidence: 置信度 0-100
- reasoning: 决策理由

示例:
[
  {"symbol": "BTCUSDT", "action": "place_buy_limit", "price": 94000, "quantity": 0.01, "level_index": 2, "confidence": 85, "reasoning": "第2层价格接近，下买单"},
  {"symbol": "BTCUSDT", "action": "hold", "confidence": 90, "reasoning": "市场震荡，保持当前网格"}
]
`, config.Symbol, config.Symbol, config.GridCount, config.TotalInvestment, config.Leverage, config.Distribution)
}

func buildGridSystemPromptEn(config *store.GridStrategyConfig) string {
	return fmt.Sprintf(`# You are a Professional Grid Trading AI

## Role Definition
You are an experienced grid trading expert managing a grid strategy for %s. Your tasks are:
1. Assess current market regime (ranging/trending/volatile)
2. Decide whether to adjust grid or pause trading
3. Manage orders at each grid level
4. Use a classic bidirectional closed-loop grid rather than interpreting every sell as opening short or every buy as opening long

## Grid Configuration
- Symbol: %s
- Grid Levels: %d
- Total Investment: %.2f USDT
- Leverage: %dx
- Distribution: %s

## Decision Rules

### Market Regime Assessment
- **Ranging Market** (ideal for grid): Bollinger width < 3%% and price remains inside the medium box; EMA20/50 distance < 1%% is standard ranging, while 1%%-2%% inside the medium box is only an early directional drift and should not pause the grid by itself
- **Trending Market** (pause grid): price breaks the medium/long box, or Bollinger width > 4%% with EMA20/50 distance > 2%% and persistent band breakout
- **High Volatility** (caution): ATR spike, erratic price movement

### Available Actions
- place_buy_limit: Place buy limit order at specified price
- place_sell_limit: Place sell limit order at specified price
- cancel_order: Cancel specific order
- cancel_all_orders: Cancel all orders
- pause_grid: Pause grid trading (in trending market)
- resume_grid: Resume grid trading (in ranging market)
- adjust_grid: Adjust grid boundaries
- hold: Maintain current state

## Classic Bidirectional Closed-Loop Grid Semantics
- A lower buy order has two possible intents: 1) if there is an existing short position from an upper level, the buy should be used to close that short for profit; 2) otherwise it is used to open a new long
- An upper sell order has two possible intents: 1) if there is an existing long position from a lower level, the sell should be used to close that long for profit; 2) otherwise it is used to open a new short
- Your job is to decide which prices should have buy orders, which should have sell orders, and whether to pause, cancel, or adjust the grid
- Do not switch to other actions just because the practical intent is close-long or close-short. Keep using place_buy_limit / place_sell_limit and let the execution layer infer whether it is reducing or opening
- In normal grid operation, do not emit close_long or close_short just to realize routine grid profit. Routine exits must stay as adjacent reduce-only limit orders. Reserve direct close_long / close_short only for explicit risk exits such as stop-loss, emergency liquidation, or operator-forced liquidation
- In trending or breakout conditions, prefer pause_grid, cancel_order, cancel_all_orders, or adjust_grid instead of continuing to seed dense grid orders
- For cancel_order, prefer level_index to identify which grid order should be removed; only fill order_id when a real exchange order ID is explicitly available in context
- Never invent order_id values. Do not output L1/L9 labels, prices, descriptive text, or synthetic IDs as order_id
- If price is still inside the medium box, Bollinger width is below 3%%, and EMA20/50 distance is still below 2%%, do not pause the grid just because of mild directional drift. Prefer maintaining or restoring full in-box grid coverage
- If the market is no longer suitable for grid trading, you may cancel unfilled orders; filled positions will continue to be handled by risk controls and stop-loss logic
- When both "Grid Levels Detail" and "Actual Open Orders Summary" are provided, prioritize the actual exchange order summary when describing live order prices, sizes, and linked levels instead of repeating only logical level indexes
- If multiple levels collapse into the same actual price, explicitly note that the grid spacing is too tight or the exchange price precision is too coarse
- If there are filled levels but the "Actual Open Orders Summary" is empty or the reduce-only order count is zero, do not claim that live exit orders are already waiting to be triggered. Explicitly state that no real working orders are present, or that exit re-seeding failed and the system is waiting to retry
- Only describe the system as "waiting for price to trigger orders" when real live orders are actually present in the exchange order summary
- If the market is ranging and the grid is not paused, but the "Actual Open Orders Summary" contains no long-entry or short-entry orders, do not simply output hold. Treat this as under-seeded grid coverage and prefer seeding the missing entry orders unless explicit risk constraints prevent it

## Output Format
Output JSON array, each decision contains:
- symbol: Trading pair
- action: Action type
- price: Price (for limit orders)
- quantity: Quantity
- level_index: Grid level index
- order_id: Order ID (only if a real exchange order ID is explicitly provided in context; otherwise leave empty)
- confidence: Confidence 0-100
- reasoning: Decision reason

Example:
[
  {"symbol": "BTCUSDT", "action": "place_buy_limit", "price": 94000, "quantity": 0.01, "level_index": 2, "confidence": 85, "reasoning": "Level 2 price approaching, place buy order"},
  {"symbol": "BTCUSDT", "action": "hold", "confidence": 90, "reasoning": "Market ranging, maintain current grid"}
]
`, config.Symbol, config.Symbol, config.GridCount, config.TotalInvestment, config.Leverage, config.Distribution)
}

// BuildGridUserPrompt builds the user prompt with current grid context
func BuildGridUserPrompt(ctx *GridContext, lang string) string {
	if lang == "zh" {
		return buildGridUserPromptZh(ctx)
	}
	return buildGridUserPromptEn(ctx)
}

func buildGridUserPromptZh(ctx *GridContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 当前时间: %s\n\n", ctx.CurrentTime))

	// Market data section
	sb.WriteString("## 市场数据\n")
	sb.WriteString(fmt.Sprintf("- 当前价格: $%s\n", formatGridPrice(ctx.CurrentPrice)))
	sb.WriteString(fmt.Sprintf("- 1小时涨跌: %s\n", formatGridPct(ctx.PriceChange1h)))
	sb.WriteString(fmt.Sprintf("- 4小时涨跌: %s\n", formatGridPct(ctx.PriceChange4h)))
	sb.WriteString(fmt.Sprintf("- ATR14: $%s (%s)\n", formatGridPrice(ctx.ATR14), formatGridPct(ctx.ATR14/ctx.CurrentPrice*100)))
	sb.WriteString(fmt.Sprintf("- 布林带: 上轨 $%s, 中轨 $%s, 下轨 $%s\n",
		formatGridPrice(ctx.BollingerUpper), formatGridPrice(ctx.BollingerMiddle), formatGridPrice(ctx.BollingerLower)))
	sb.WriteString(fmt.Sprintf("- 布林带宽度: %s\n", formatGridPct(ctx.BollingerWidth)))
	sb.WriteString(fmt.Sprintf("- EMA20: $%s, EMA50: $%s, 距离: %s\n",
		formatGridPrice(ctx.EMA20), formatGridPrice(ctx.EMA50), formatGridPct(ctx.EMADistance)))
	sb.WriteString(fmt.Sprintf("- RSI14: %.1f\n", ctx.RSI14))
	sb.WriteString(fmt.Sprintf("- MACD: %.4f, Signal: %.4f, Histogram: %.4f\n", ctx.MACD, ctx.MACDSignal, ctx.MACDHistogram))
	sb.WriteString(fmt.Sprintf("- 资金费率: %s\n", formatGridPct(ctx.FundingRate*100)))
	sb.WriteString("\n")

	// Box Indicator Section
	if ctx.BoxData != nil {
		sb.WriteString("## 箱体指标 (唐奇安通道)\n\n")
		sb.WriteString("| 箱体级别 | 上轨 | 下轨 | 宽度 |\n")
		sb.WriteString("|----------|------|------|------|\n")

		shortWidth := 0.0
		midWidth := 0.0
		longWidth := 0.0

		if ctx.BoxData.CurrentPrice > 0 {
			shortWidth = (ctx.BoxData.ShortUpper - ctx.BoxData.ShortLower) / ctx.BoxData.CurrentPrice * 100
			midWidth = (ctx.BoxData.MidUpper - ctx.BoxData.MidLower) / ctx.BoxData.CurrentPrice * 100
			longWidth = (ctx.BoxData.LongUpper - ctx.BoxData.LongLower) / ctx.BoxData.CurrentPrice * 100
		}

		sb.WriteString(fmt.Sprintf("| 短期 (3天) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.ShortUpper, ctx.BoxData.ShortLower, shortWidth))
		sb.WriteString(fmt.Sprintf("| 中期 (10天) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.MidUpper, ctx.BoxData.MidLower, midWidth))
		sb.WriteString(fmt.Sprintf("| 长期 (21天) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.LongUpper, ctx.BoxData.LongLower, longWidth))

		sb.WriteString(fmt.Sprintf("\n当前价格: %.2f\n", ctx.BoxData.CurrentPrice))

		// Check position relative to boxes
		price := ctx.BoxData.CurrentPrice
		if price > ctx.BoxData.LongUpper || price < ctx.BoxData.LongLower {
			sb.WriteString("⚠️ 突破: 价格突破长期箱体!\n")
		} else if price > ctx.BoxData.MidUpper || price < ctx.BoxData.MidLower {
			sb.WriteString("⚠️ 警告: 价格接近长期箱体边界\n")
		}
		sb.WriteString("\n")
	}

	// Account section
	sb.WriteString("## 账户状态\n")
	sb.WriteString(fmt.Sprintf("- 总权益: $%.2f\n", ctx.TotalEquity))
	sb.WriteString(fmt.Sprintf("- 可用余额: $%.2f\n", ctx.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- 当前持仓: %.4f (净头寸)\n", ctx.CurrentPosition))
	sb.WriteString(fmt.Sprintf("- 当前多仓: %.4f\n", ctx.CurrentLongPosition))
	sb.WriteString(fmt.Sprintf("- 当前空仓: %.4f\n", ctx.CurrentShortPosition))
	sb.WriteString(fmt.Sprintf("- 未实现盈亏: $%.2f\n", ctx.UnrealizedPnL))
	sb.WriteString("\n")

	// Grid state section
	sb.WriteString("## 网格状态\n")
	sb.WriteString(fmt.Sprintf("- 网格范围: $%s - $%s\n", formatGridPrice(ctx.LowerPrice), formatGridPrice(ctx.UpperPrice)))
	sb.WriteString(fmt.Sprintf("- 网格间距: $%s\n", formatGridPrice(ctx.GridSpacing)))
	sb.WriteString(fmt.Sprintf("- 活跃订单数: %d\n", ctx.ActiveOrderCount))
	sb.WriteString(fmt.Sprintf("- 多头相关挂单: %d\n", ctx.ActiveLongOrderCount))
	sb.WriteString(fmt.Sprintf("- 空头相关挂单: %d\n", ctx.ActiveShortOrderCount))
	sb.WriteString(fmt.Sprintf("- 减仓挂单: %d\n", ctx.ReduceOnlyOrderCount))
	sb.WriteString(fmt.Sprintf("- 已成交层数: %d\n", ctx.FilledLevelCount))
	sb.WriteString(fmt.Sprintf("- 多头持仓层数: %d\n", ctx.FilledLongLevelCount))
	sb.WriteString(fmt.Sprintf("- 空头持仓层数: %d\n", ctx.FilledShortLevelCount))
	sb.WriteString(fmt.Sprintf("- 网格已暂停: %v\n", ctx.IsPaused))
	if ctx.CurrentDirection != "" {
		directionDescZh := map[string]string{
			"neutral":    "中性 (50%买+50%卖)",
			"long":       "做多 (100%买)",
			"short":      "做空 (100%卖)",
			"long_bias":  "偏多 (70%买+30%卖)",
			"short_bias": "偏空 (30%买+70%卖)",
		}
		desc := directionDescZh[ctx.CurrentDirection]
		if desc == "" {
			desc = ctx.CurrentDirection
		}
		sb.WriteString(fmt.Sprintf("- 网格方向: %s\n", desc))
	}
	sb.WriteString("\n")

	if len(ctx.AggregatedOpenOrders) > 0 {
		sb.WriteString("## 实际挂单摘要（按真实交易所价格聚合）\n")
		for _, summary := range ctx.AggregatedOpenOrders {
			label := summary.Bucket
			switch summary.Bucket {
			case "long_entry":
				label = "做多挂单"
			case "short_entry":
				label = "做空挂单"
			case "long_exit":
				label = "多头平仓"
			case "short_exit":
				label = "空头平仓"
			}
			sb.WriteString(fmt.Sprintf("- %s: $%s, 共%d笔, 总数量 %.4f, 总金额 $%.2f",
				label, formatGridPrice(summary.Price), summary.OrderCount, summary.TotalQuantity, summary.TotalNotional))
			if len(summary.LevelIndexes) > 0 {
				sb.WriteString(fmt.Sprintf(", 挂单层 %s", formatGridPromptLevels(summary.LevelIndexes)))
			}
			if summary.Bucket == "long_exit" || summary.Bucket == "short_exit" {
				sb.WriteString(fmt.Sprintf(", 对应开仓层 %s", formatGridPromptLevelsOrUnknown(summary.LinkedLevelIndexes, "未知开仓层")))
				if len(summary.SourceEntryPrices) > 0 {
					sb.WriteString(fmt.Sprintf(", 持仓价 %s", formatGridPromptPrices(summary.SourceEntryPrices)))
				}
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(ctx.CollapsedLevelWarnings) > 0 {
		sb.WriteString("## 网格层级收敛警告\n")
		for _, warning := range ctx.CollapsedLevelWarnings {
			sb.WriteString(fmt.Sprintf("- %s 共享同一实际价格 $%s，这通常表示网格间距偏小或交易所价格精度过粗\n",
				formatGridPromptLevels(warning.LevelIndexes), formatGridPrice(warning.Price)))
		}
		sb.WriteString("\n")
	}

	// Grid levels detail
	sb.WriteString("## 网格层级详情\n")
	sb.WriteString("| 层级 | 价格 | 状态 | 方向 | 待成交方向 | 减仓单 | 订单数量 | 持仓方向 | 持仓数量 | 未实现盈亏 |\n")
	sb.WriteString("|------|------|------|------|------------|--------|----------|----------|----------|------------|\n")
	for _, level := range ctx.Levels {
		reduceOnly := "否"
		if level.OrderReduceOnly {
			reduceOnly = "是"
		}
		sb.WriteString(fmt.Sprintf("| %d | $%s | %s | %s | %s | %s | %.4f | %s | %.4f | $%s |\n",
			level.Index, formatGridPrice(level.Price), level.State, level.Side,
			level.OrderPositionSide, reduceOnly, level.OrderQuantity, level.PositionSide, level.PositionSize, formatGridPrice(level.UnrealizedPnL)))
	}
	sb.WriteString("\n")

	sb.WriteString("说明: `待成交方向` 表示挂单作用于 LONG/SHORT 哪一侧; `减仓单=是` 表示该单优先用于平已有仓位，而不是新开仓。\n\n")

	// Performance section
	sb.WriteString("## 绩效统计\n")
	sb.WriteString(fmt.Sprintf("- 总利润: $%.2f\n", ctx.TotalProfit))
	sb.WriteString(fmt.Sprintf("- 总交易次数: %d\n", ctx.TotalTrades))
	sb.WriteString(fmt.Sprintf("- 胜率: %.1f%%\n", float64(ctx.WinningTrades)/float64(max(ctx.TotalTrades, 1))*100))
	sb.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n", ctx.MaxDrawdown))
	sb.WriteString(fmt.Sprintf("- 今日盈亏: $%.2f\n", ctx.DailyPnL))
	sb.WriteString("\n")

	sb.WriteString("## 请分析以上数据，做出网格交易决策\n")
	sb.WriteString("输出JSON数组格式的决策列表。\n")

	return sb.String()
}

func buildGridUserPromptEn(ctx *GridContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Current Time: %s\n\n", ctx.CurrentTime))

	// Market data section
	sb.WriteString("## Market Data\n")
	sb.WriteString(fmt.Sprintf("- Current Price: $%s\n", formatGridPrice(ctx.CurrentPrice)))
	sb.WriteString(fmt.Sprintf("- 1h Change: %s\n", formatGridPct(ctx.PriceChange1h)))
	sb.WriteString(fmt.Sprintf("- 4h Change: %s\n", formatGridPct(ctx.PriceChange4h)))
	sb.WriteString(fmt.Sprintf("- ATR14: $%s (%s)\n", formatGridPrice(ctx.ATR14), formatGridPct(ctx.ATR14/ctx.CurrentPrice*100)))
	sb.WriteString(fmt.Sprintf("- Bollinger Bands: Upper $%s, Middle $%s, Lower $%s\n",
		formatGridPrice(ctx.BollingerUpper), formatGridPrice(ctx.BollingerMiddle), formatGridPrice(ctx.BollingerLower)))
	sb.WriteString(fmt.Sprintf("- Bollinger Width: %s\n", formatGridPct(ctx.BollingerWidth)))
	sb.WriteString(fmt.Sprintf("- EMA20: $%s, EMA50: $%s, Distance: %s\n",
		formatGridPrice(ctx.EMA20), formatGridPrice(ctx.EMA50), formatGridPct(ctx.EMADistance)))
	sb.WriteString(fmt.Sprintf("- RSI14: %.1f\n", ctx.RSI14))
	sb.WriteString(fmt.Sprintf("- MACD: %.4f, Signal: %.4f, Histogram: %.4f\n", ctx.MACD, ctx.MACDSignal, ctx.MACDHistogram))
	sb.WriteString(fmt.Sprintf("- Funding Rate: %s\n", formatGridPct(ctx.FundingRate*100)))
	sb.WriteString("\n")

	// Box Indicator Section
	if ctx.BoxData != nil {
		sb.WriteString("## Box Indicators (Donchian Channels)\n\n")
		sb.WriteString("| Box Level | Upper | Lower | Width |\n")
		sb.WriteString("|-----------|-------|-------|-------|\n")

		shortWidth := 0.0
		midWidth := 0.0
		longWidth := 0.0

		if ctx.BoxData.CurrentPrice > 0 {
			shortWidth = (ctx.BoxData.ShortUpper - ctx.BoxData.ShortLower) / ctx.BoxData.CurrentPrice * 100
			midWidth = (ctx.BoxData.MidUpper - ctx.BoxData.MidLower) / ctx.BoxData.CurrentPrice * 100
			longWidth = (ctx.BoxData.LongUpper - ctx.BoxData.LongLower) / ctx.BoxData.CurrentPrice * 100
		}

		sb.WriteString(fmt.Sprintf("| Short (3d) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.ShortUpper, ctx.BoxData.ShortLower, shortWidth))
		sb.WriteString(fmt.Sprintf("| Mid (10d) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.MidUpper, ctx.BoxData.MidLower, midWidth))
		sb.WriteString(fmt.Sprintf("| Long (21d) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.LongUpper, ctx.BoxData.LongLower, longWidth))

		sb.WriteString(fmt.Sprintf("\nCurrent Price: %.2f\n", ctx.BoxData.CurrentPrice))

		// Check position relative to boxes
		price := ctx.BoxData.CurrentPrice
		if price > ctx.BoxData.LongUpper || price < ctx.BoxData.LongLower {
			sb.WriteString("⚠️ BREAKOUT: Price outside long-term box!\n")
		} else if price > ctx.BoxData.MidUpper || price < ctx.BoxData.MidLower {
			sb.WriteString("⚠️ WARNING: Price approaching long-term box boundary\n")
		}
		sb.WriteString("\n")
	}

	// Account section
	sb.WriteString("## Account Status\n")
	sb.WriteString(fmt.Sprintf("- Total Equity: $%.2f\n", ctx.TotalEquity))
	sb.WriteString(fmt.Sprintf("- Available Balance: $%.2f\n", ctx.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- Current Position: %.4f (net)\n", ctx.CurrentPosition))
	sb.WriteString(fmt.Sprintf("- Current Long Position: %.4f\n", ctx.CurrentLongPosition))
	sb.WriteString(fmt.Sprintf("- Current Short Position: %.4f\n", ctx.CurrentShortPosition))
	sb.WriteString(fmt.Sprintf("- Unrealized PnL: $%.2f\n", ctx.UnrealizedPnL))
	sb.WriteString("\n")

	// Grid state section
	sb.WriteString("## Grid Status\n")
	sb.WriteString(fmt.Sprintf("- Grid Range: $%s - $%s\n", formatGridPrice(ctx.LowerPrice), formatGridPrice(ctx.UpperPrice)))
	sb.WriteString(fmt.Sprintf("- Grid Spacing: $%s\n", formatGridPrice(ctx.GridSpacing)))
	sb.WriteString(fmt.Sprintf("- Active Orders: %d\n", ctx.ActiveOrderCount))
	sb.WriteString(fmt.Sprintf("- Long-related Orders: %d\n", ctx.ActiveLongOrderCount))
	sb.WriteString(fmt.Sprintf("- Short-related Orders: %d\n", ctx.ActiveShortOrderCount))
	sb.WriteString(fmt.Sprintf("- Reduce-only Orders: %d\n", ctx.ReduceOnlyOrderCount))
	sb.WriteString(fmt.Sprintf("- Filled Levels: %d\n", ctx.FilledLevelCount))
	sb.WriteString(fmt.Sprintf("- Long Position Levels: %d\n", ctx.FilledLongLevelCount))
	sb.WriteString(fmt.Sprintf("- Short Position Levels: %d\n", ctx.FilledShortLevelCount))
	sb.WriteString(fmt.Sprintf("- Grid Paused: %v\n", ctx.IsPaused))
	if ctx.CurrentDirection != "" {
		directionDescEn := map[string]string{
			"neutral":    "Neutral (50% buy + 50% sell)",
			"long":       "Long (100% buy)",
			"short":      "Short (100% sell)",
			"long_bias":  "Long Bias (70% buy + 30% sell)",
			"short_bias": "Short Bias (30% buy + 70% sell)",
		}
		desc := directionDescEn[ctx.CurrentDirection]
		if desc == "" {
			desc = ctx.CurrentDirection
		}
		sb.WriteString(fmt.Sprintf("- Grid Direction: %s\n", desc))
	}
	sb.WriteString("\n")

	if len(ctx.AggregatedOpenOrders) > 0 {
		sb.WriteString("## Actual Open Orders Summary (grouped by real exchange price)\n")
		for _, summary := range ctx.AggregatedOpenOrders {
			label := summary.Bucket
			switch summary.Bucket {
			case "long_entry":
				label = "Long Entry"
			case "short_entry":
				label = "Short Entry"
			case "long_exit":
				label = "Long Exit"
			case "short_exit":
				label = "Short Exit"
			}
			sb.WriteString(fmt.Sprintf("- %s: $%s, %d orders, total qty %.4f, total notional $%.2f",
				label, formatGridPrice(summary.Price), summary.OrderCount, summary.TotalQuantity, summary.TotalNotional))
			if len(summary.LevelIndexes) > 0 {
				sb.WriteString(fmt.Sprintf(", order levels %s", formatGridPromptLevels(summary.LevelIndexes)))
			}
			if summary.Bucket == "long_exit" || summary.Bucket == "short_exit" {
				sb.WriteString(fmt.Sprintf(", source entry levels %s", formatGridPromptLevelsOrUnknown(summary.LinkedLevelIndexes, "Unknown entry level")))
				if len(summary.SourceEntryPrices) > 0 {
					sb.WriteString(fmt.Sprintf(", entry prices %s", formatGridPromptPrices(summary.SourceEntryPrices)))
				}
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(ctx.CollapsedLevelWarnings) > 0 {
		sb.WriteString("## Grid Level Convergence Warning\n")
		for _, warning := range ctx.CollapsedLevelWarnings {
			sb.WriteString(fmt.Sprintf("- %s share the same actual price $%s, which usually means the grid spacing is too tight or exchange price precision is too coarse\n",
				formatGridPromptLevels(warning.LevelIndexes), formatGridPrice(warning.Price)))
		}
		sb.WriteString("\n")
	}

	// Grid levels detail
	sb.WriteString("## Grid Levels Detail\n")
	sb.WriteString("| Level | Price | State | Side | Pending Side | Reduce Only | Order Qty | Position Side | Position | Unrealized PnL |\n")
	sb.WriteString("|-------|-------|-------|------|--------------|-------------|-----------|---------------|----------|----------------|\n")
	for _, level := range ctx.Levels {
		reduceOnly := "no"
		if level.OrderReduceOnly {
			reduceOnly = "yes"
		}
		sb.WriteString(fmt.Sprintf("| %d | $%s | %s | %s | %s | %s | %.4f | %s | %.4f | $%s |\n",
			level.Index, formatGridPrice(level.Price), level.State, level.Side,
			level.OrderPositionSide, reduceOnly, level.OrderQuantity, level.PositionSide, level.PositionSize, formatGridPrice(level.UnrealizedPnL)))
	}
	sb.WriteString("\n")

	sb.WriteString("Note: `Pending Side` shows whether the order acts on LONG or SHORT inventory; `Reduce Only=yes` means the order is primarily intended to close an existing position rather than open a new one.\n\n")

	// Performance section
	sb.WriteString("## Performance Stats\n")
	sb.WriteString(fmt.Sprintf("- Total Profit: $%.2f\n", ctx.TotalProfit))
	sb.WriteString(fmt.Sprintf("- Total Trades: %d\n", ctx.TotalTrades))
	sb.WriteString(fmt.Sprintf("- Win Rate: %.1f%%\n", float64(ctx.WinningTrades)/float64(max(ctx.TotalTrades, 1))*100))
	sb.WriteString(fmt.Sprintf("- Max Drawdown: %.2f%%\n", ctx.MaxDrawdown))
	sb.WriteString(fmt.Sprintf("- Daily PnL: $%.2f\n", ctx.DailyPnL))
	sb.WriteString("\n")

	sb.WriteString("## Please analyze the data above and make grid trading decisions\n")
	sb.WriteString("Output a JSON array of decisions.\n")

	return sb.String()
}

// ============================================================================
// Grid Decision Functions
// ============================================================================

// GetGridDecisions gets AI decisions for grid trading
func GetGridDecisions(ctx *GridContext, mcpClient mcp.AIClient, config *store.GridStrategyConfig, lang string) (*FullDecision, error) {
	startTime := time.Now()

	// Build prompts
	systemPrompt := BuildGridSystemPrompt(config, lang)
	userPrompt := BuildGridUserPrompt(ctx, lang)

	logger.Infof("🤖 [Grid] Calling AI for grid decisions...")

	// Call AI
	response, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	// Parse decisions from response
	decisions, err := parseGridDecisions(response, ctx.Symbol)
	if err != nil {
		logger.Warnf("Failed to parse grid decisions: %v", err)
		// Return hold decision as fallback
		decisions = []Decision{{
			Symbol:     ctx.Symbol,
			Action:     "hold",
			Confidence: 50,
			Reasoning:  "Failed to parse AI response, holding current state",
		}}
	}

	duration := time.Since(startTime).Milliseconds()
	logger.Infof("⏱️ [Grid] AI call duration: %d ms, decisions: %d", duration, len(decisions))

	// Extract chain of thought from response
	cotTrace := extractCoTTrace(response)

	return &FullDecision{
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt,
		CoTTrace:            cotTrace,
		Decisions:           decisions,
		RawResponse:         response,
		AIRequestDurationMs: duration,
		Timestamp:           time.Now(),
	}, nil
}

// parseGridDecisions parses AI response into grid decisions
func parseGridDecisions(response string, symbol string) ([]Decision, error) {
	// Try to find JSON array in response
	jsonStr := extractJSONArray(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonStr), &decisions); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate and set default symbol
	for i := range decisions {
		if decisions[i].Symbol == "" {
			decisions[i].Symbol = symbol
		}
		// Validate action
		if !isValidGridAction(decisions[i].Action) {
			logger.Warnf("Invalid grid action: %s", decisions[i].Action)
		}
	}

	return decisions, nil
}

// extractJSONArray extracts JSON array from AI response
func extractJSONArray(response string) string {
	// Try to find ```json code block first
	matches := reJSONFence.FindStringSubmatch(response)
	if len(matches) > 1 {
		return matches[1]
	}

	// Try to find raw JSON array
	matches = reJSONArray.FindStringSubmatch(response)
	if len(matches) > 0 {
		return matches[0]
	}

	return ""
}

// isValidGridAction checks if action is a valid grid action
func isValidGridAction(action string) bool {
	validActions := map[string]bool{
		"place_buy_limit":   true,
		"place_sell_limit":  true,
		"cancel_order":      true,
		"cancel_all_orders": true,
		"pause_grid":        true,
		"resume_grid":       true,
		"adjust_grid":       true,
		"hold":              true,
		// Also support standard actions for compatibility
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
	}
	return validActions[action]
}

// ============================================================================
// Grid Context Builder Helpers
// ============================================================================

// BuildGridContextFromMarketData builds grid context from market data
func BuildGridContextFromMarketData(mktData *market.Data, config *store.GridStrategyConfig) *GridContext {
	ctx := &GridContext{
		Symbol:       config.Symbol,
		CurrentTime:  time.Now().Format("2006-01-02 15:04:05"),
		CurrentPrice: mktData.CurrentPrice,

		// Grid config
		GridCount:       config.GridCount,
		TotalInvestment: config.TotalInvestment,
		Leverage:        config.Leverage,
		Distribution:    config.Distribution,

		// Market data
		PriceChange1h: mktData.PriceChange1h,
		PriceChange4h: mktData.PriceChange4h,
		FundingRate:   mktData.FundingRate,
	}

	// Extract indicators from timeframe data
	if mktData.TimeframeData != nil {
		if tf5m, ok := mktData.TimeframeData["5m"]; ok {
			if len(tf5m.BOLLUpper) > 0 {
				ctx.BollingerUpper = tf5m.BOLLUpper[len(tf5m.BOLLUpper)-1]
				ctx.BollingerMiddle = tf5m.BOLLMiddle[len(tf5m.BOLLMiddle)-1]
				ctx.BollingerLower = tf5m.BOLLLower[len(tf5m.BOLLLower)-1]
				if ctx.BollingerMiddle > 0 {
					ctx.BollingerWidth = (ctx.BollingerUpper - ctx.BollingerLower) / ctx.BollingerMiddle * 100
				}
			}
			ctx.ATR14 = tf5m.ATR14
			if len(tf5m.RSI14Values) > 0 {
				ctx.RSI14 = tf5m.RSI14Values[len(tf5m.RSI14Values)-1]
			}
		}
	}

	// Extract longer term context
	if mktData.LongerTermContext != nil {
		if ctx.ATR14 == 0 {
			ctx.ATR14 = mktData.LongerTermContext.ATR14
		}
		ctx.EMA50 = mktData.LongerTermContext.EMA50
	}

	ctx.EMA20 = mktData.CurrentEMA20
	ctx.MACD = mktData.CurrentMACD

	// Calculate EMA distance
	if ctx.EMA50 > 0 {
		ctx.EMADistance = (ctx.EMA20 - ctx.EMA50) / ctx.EMA50 * 100
	}

	return ctx
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatGridPromptLevels(levelIndexes []int) string {
	unique := make([]int, 0, len(levelIndexes))
	seen := make(map[int]struct{}, len(levelIndexes))
	for _, levelIndex := range levelIndexes {
		if levelIndex < 0 {
			continue
		}
		if _, ok := seen[levelIndex]; ok {
			continue
		}
		seen[levelIndex] = struct{}{}
		unique = append(unique, levelIndex)
	}
	if len(unique) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(unique))
	for _, levelIndex := range unique {
		parts = append(parts, fmt.Sprintf("L%d", levelIndex+1))
	}
	return strings.Join(parts, "、")
}

func formatGridPromptLevelsOrUnknown(levelIndexes []int, unknown string) string {
	formatted := formatGridPromptLevels(levelIndexes)
	if formatted == "-" {
		return unknown
	}
	return formatted
}

func formatGridPromptPrices(prices []float64) string {
	seen := make(map[string]struct{}, len(prices))
	parts := make([]string, 0, len(prices))
	for _, price := range prices {
		if price <= 0 {
			continue
		}
		formatted := fmt.Sprintf("$%s", formatGridPrice(price))
		if _, ok := seen[formatted]; ok {
			continue
		}
		seen[formatted] = struct{}{}
		parts = append(parts, formatted)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "、")
}
