# 看板接口准确版

更新时间：2026-04-26

本文档定义 APP 看板链路中 3 个容易出错的接口口径：

1. 风控模块接口
2. 历史持仓结构
3. 当前挂单接口

原则：

- 当前态优先使用交易所实时接口
- 历史态优先使用本地落库结构
- 不允许用统计接口替代风控接口
- 不允许用历史订单表替代当前挂单接口

---

## 1. 风控模块接口

### 正确接口

`GET /api/traders/:id/grid-risk`

用途：

- 展示网格交易员当前风险状态
- 展示当前持仓、当前挂单、网格区间、箱体边界、杠杆状态
- 作为看板“风险面板”的唯一可信数据源

不要再用以下接口替代：

- `GET /api/statistics`
- `GET /api/account`
- `GET /api/orders?status=NEW`

### 返回结构

```ts
type GridRiskInfo = {
  current_leverage: number
  effective_leverage: number
  recommended_leverage: number
  compounding_base: number

  current_position: number
  max_position: number
  position_percent: number
  current_long_position: number
  current_short_position: number
  current_long_value: number
  current_short_value: number

  entry_order_count: number
  reduce_only_order_count: number
  long_order_count: number
  short_order_count: number
  long_filled_levels: number
  short_filled_levels: number
  average_order_notional: number

  liquidation_price: number
  liquidation_distance: number

  regime_level: string

  short_box_upper: number
  short_box_lower: number
  mid_box_upper: number
  mid_box_lower: number
  long_box_upper: number
  long_box_lower: number
  current_price: number
  grid_upper_price: number
  grid_lower_price: number
  grid_boundary_source: string
  grid_spacing: number
  min_grid_spacing: number
  max_grid_spacing: number

  breakout_level: string
  breakout_direction: string

  current_grid_direction: string
  direction_change_count: number
  enable_direction_adjust: boolean

  grid_levels: Array<{
    index: number
    price: number
  }>

  grid_orders: Array<{
    order_id: string
    price: number
    quantity: number
    notional: number
    side: string
    position_side: string
    reduce_only: boolean
    level_index: number
    linked_level_index: number
    source_entry_price: number
    bucket: "long_entry" | "short_entry" | "long_exit" | "short_exit"
    status: string
  }>
}
```

### 字段口径

#### 账户与杠杆

- `current_leverage`
  配置中的当前网格杠杆。
- `effective_leverage`
  当前持仓名义价值 / 当前复利基准资金。
- `recommended_leverage`
  根据当前 regime 推导出的建议杠杆上限。
- `compounding_base`
  当前复利基准资金，不等于初始本金，不等于历史固定投资额。

#### 仓位

- `current_position`
  当前总持仓名义价值。
- `current_long_position`
  当前多头数量。
- `current_short_position`
  当前空头数量。
- `current_long_value`
  当前多头名义价值。
- `current_short_value`
  当前空头名义价值。
- `max_position`
  当前 regime 下允许的最大持仓名义价值。
- `position_percent`
  当前持仓名义价值 / 最大允许持仓名义价值。

#### 挂单与层级

- `entry_order_count`
  当前所有开仓挂单数量。
- `reduce_only_order_count`
  当前所有平仓挂单数量。
- `long_order_count`
  `position_side = LONG` 的挂单数量，包含开多和平多。
- `short_order_count`
  `position_side = SHORT` 的挂单数量，包含开空和平空。
- `long_filled_levels`
  当前 LONG 方向已成交层数。
- `short_filled_levels`
  当前 SHORT 方向已成交层数。
- `average_order_notional`
  当前所有挂单的平均名义金额。

#### 价格区间

- `current_price`
  当前市场价。
- `grid_upper_price`
  当前网格上边界。
- `grid_lower_price`
  当前网格下边界。
- `grid_boundary_source`
  当前边界来源。
- `grid_spacing`
  当前网格间距。
- `min_grid_spacing`
  当前允许的最小间距。
- `max_grid_spacing`
  当前允许的最大间距。

#### 箱体与方向

- `short_box_upper / short_box_lower`
  短周期箱体边界。
- `mid_box_upper / mid_box_lower`
  中周期箱体边界。
- `long_box_upper / long_box_lower`
  长周期箱体边界。
- `regime_level`
  当前市场状态等级。
- `breakout_level`
  当前突破级别。
- `breakout_direction`
  当前突破方向。
- `current_grid_direction`
  当前网格方向。
- `direction_change_count`
  方向切换次数。
- `enable_direction_adjust`
  是否开启方向动态调整。

### 看板使用建议

- 风险卡片直接读取 `grid-risk`
- 网格层级图直接读取 `grid_levels`
- 当前挂单风险分桶直接读取 `grid_orders.bucket`
- 不要用 `/api/orders` 自己推导 `reduce_only`

### 数据来源口径

- 持仓：来自交易所实时 `GetPositions()`
- 当前挂单：来自交易所实时 `GetOpenOrders(symbol)`
- 网格状态：来自运行中 grid state

因此 `grid-risk` 是实时风控视图，不是历史聚合视图。

---

## 2. 历史持仓结构

### 正确接口

`GET /api/positions/history?trader_id=<id>&limit=<n>`

用途：

- 展示已平仓历史
- 展示历史收益统计
- 展示按 symbol 和方向的聚合表现

### 返回结构

```ts
type PositionHistoryResponse = {
  positions: TraderPosition[]
  stats: TraderStats
  symbol_stats: SymbolStats[]
  direction_stats: DirectionStats[]
}
```

### positions 单条结构

```ts
type TraderPosition = {
  id: number
  trader_id: string
  exchange_id: string
  exchange_type: string
  exchange_position_id: string

  symbol: string
  side: string

  entry_quantity: number
  quantity: number
  entry_price: number
  entry_order_id: string
  entry_time: number

  exit_price: number
  exit_order_id: string
  exit_time: number

  realized_pnl: number
  fee: number
  leverage: number

  status: "OPEN" | "CLOSED"
  close_reason: string
  source: string

  created_at: number
  updated_at: number
}
```

### 字段口径

- 所有时间字段均为 Unix 毫秒时间戳
- `positions/history` 页面应主要消费 `status = CLOSED`
- `realized_pnl` 是已实现盈亏，不包含未平仓浮盈亏
- `fee` 是该仓位累计手续费
- `side` 表示持仓方向，仅为 `LONG` 或 `SHORT`
- `entry_quantity` 是建仓总数量
- `quantity` 在历史记录里不要被解释为“当前剩余数量”，历史页统一按成交记录展示

### stats 结构

```ts
type TraderStats = {
  total_trades: number
  win_trades: number
  loss_trades: number
  win_rate: number
  profit_factor: number
  sharpe_ratio: number
  total_pnl: number
  total_fee: number
  avg_win: number
  avg_loss: number
  max_drawdown_pct: number
}
```

口径：

- 全部基于 `CLOSED` 持仓计算
- 不包含未平仓持仓
- `max_drawdown_pct` 是按已平仓 PnL 序列回推，不是实时权益曲线回撤

### symbol_stats 结构

```ts
type SymbolStats = {
  symbol: string
  total_trades: number
  win_trades: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
  avg_hold_mins: number
}
```

### direction_stats 结构

```ts
type DirectionStats = {
  side: string
  trade_count: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
}
```

### 看板使用建议

- 历史持仓列表页：只用 `positions`
- 历史收益摘要：用 `stats`
- 币种表现排行：用 `symbol_stats`
- 多空方向表现：用 `direction_stats`

不要再把：

- `/api/trades`
- `/api/orders`
- `/api/statistics`

混成“历史持仓页”的数据源。

---

## 3. 当前挂单接口

当前挂单必须区分：

1. 交易所实时挂单
2. 本地订单历史表

### 3.1 实时挂单接口

#### 正确接口

`GET /api/open-orders?trader_id=<id>&symbol=<symbol>`

用途：

- 展示“此刻真实挂在交易所上的订单”
- 展示当前委托簿
- 展示当前网格挂单

这是当前挂单看板的唯一可信接口。

#### 返回结构

```ts
type OpenOrder = {
  order_id: string
  symbol: string
  side: string
  position_side: string
  type: string
  price: number
  stop_price: number
  quantity: number
  status: string
}
```

#### 字段口径

- `order_id`
  交易所订单号
- `side`
  订单方向，`BUY` 或 `SELL`
- `position_side`
  仓位方向，`LONG` 或 `SHORT`
- `price`
  限价单价格
- `stop_price`
  条件单触发价，普通限价单可为 0
- `quantity`
  订单数量
- `status`
  实时订单状态

### 3.2 本地订单接口

#### 接口

`GET /api/orders?trader_id=<id>&symbol=<optional>&status=<optional>&limit=<optional>`

用途：

- 订单历史
- 审计
- 订单详情
- 订单状态变更回放

不要把它当作“当前挂单接口”。

#### 结构

```ts
type TraderOrder = {
  id: number
  trader_id: string
  exchange_id: string
  exchange_type: string
  exchange_order_id: string
  client_order_id: string
  symbol: string
  side: string
  position_side: string
  type: string
  time_in_force: string
  quantity: number
  price: number
  stop_price: number
  status: string
  filled_quantity: number
  avg_fill_price: number
  commission: number
  commission_asset: string
  leverage: number
  reduce_only: boolean
  close_position: boolean
  working_type: string
  price_protect: boolean
  order_action: string
  related_position_id: number
  created_at: number
  updated_at: number
  filled_at: number
}
```

### 准确使用规则

- 当前挂单页：用 `/api/open-orders`
- 订单历史页：用 `/api/orders`
- 最近成交页：用 `/api/recent-fills`
- 某一订单的成交明细：用 `/api/orders/:id/fills`

### 为什么过去会不准

因为 `trader_orders` 是本地镜像表，虽然会同步，但它不是实时交易所委托簿：

- 同步有周期
- `NEW` 不等于交易所当前一定还挂着
- 某些撤单后，本地状态可能稍晚更新

所以看板上凡是写“当前挂单”，必须改成使用 `open-orders`。

---

## 最终推荐的看板数据源

### 风控卡片

- `GET /api/traders/:id/grid-risk`

### 历史持仓页

- `GET /api/positions/history?trader_id=...`

### 当前挂单页

- `GET /api/open-orders?trader_id=...&symbol=...`

### 订单历史页

- `GET /api/orders?trader_id=...`

### 最近成交页

- `GET /api/recent-fills?trader_id=...&symbol=...`

---

## 禁止混用清单

- 禁止用 `/api/statistics` 代替风控接口
- 禁止用 `/api/account` 推导当前网格风险
- 禁止用 `/api/orders?status=NEW` 代替当前挂单接口
- 禁止用 `/api/trades` 代替历史持仓结构
- 禁止把实时交易所态和本地落库历史态混成一个列表

---

## 实施建议

APP 前端建议固定成下面的职责分层：

- 风控模块：只认 `grid-risk`
- 历史持仓模块：只认 `positions/history`
- 当前挂单模块：只认 `open-orders`
- 历史订单模块：只认 `orders`

这样可以彻底解决“过去一直不准确”的问题。
