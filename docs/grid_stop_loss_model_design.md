# 网格策略风控与止损模型设计说明

## 文档目的

本文用于固化当前智能网格策略的完整风控设计，覆盖两层能力：

- 突破前的边际区准入风控
- 突破后的分层止损与风险退出

目标不是直接给出代码实现，而是形成一份可供产品、策略、开发共同对齐的工程化规格。

---

## 一、网格策略背景

### 1.1 当前策略特征

当前策略属于 AI 辅助的合约网格策略，核心特征是：

- 在一个给定价格区间内，按网格层级挂出多空开仓单
- 开仓后在相邻层级或指定退出层级挂 reduce-only 平仓单
- 资金分配支持 `uniform` 与 `gaussian`
- 空仓后允许基于最新账户权益整盘重挂
- AI 参与箱体环境、方向、是否调整网格等判断

从策略本质上看，这是一个偏均值回归 / 箱体震荡的模型，不是纯趋势追踪系统。

### 1.2 当前风险问题

历史实盘暴露出两类问题：

1. 在边界附近或突破附近继续补挂 / 补仓，会把最差位置的风险放大
2. 触发止损后，如果仍优先追求“更好价格退出”，容易把止损变成被动解套单

这意味着风控不能只做“5% 触发止损”这一件事，而要分成两层：

- 第一层：在边界与突破区域限制新增风险
- 第二层：当风险已经形成时，用状态机分层减仓

### 1.3 本文设计目标

新的风控模型需要同时满足：

- 仍适配网格策略，不把网格直接改成趋势策略
- 能限制边界附近的差位置新仓与补仓
- 能识别不同仓位位置下的真实风险差异
- 支持部分止损，避免一刀切
- 支持回撤止损，但不能无限等待
- 在硬风险场景下优先保证退出确定性
- 与账户级风控、爆仓距离、仓位占比联动

---

## 二、总体架构

完整风控链路建议分成 3 层：

1. `Entry Admission Layer`
   用于判断当前价格区域下，是否允许 `new_entry`、`add_entry` 或仅允许 `reduce_only`

2. `Risk State Layer`
   用于判断当前处于 `normal / warning / soft_reduce / hard_reduce / emergency`

3. `Execution Layer`
   用于执行取消开仓、部分减仓、强制减仓、风险暂停、恢复等动作

执行顺序：

1. 先判定价格区域与方向风险区
2. 再判定风险状态
3. 再决定是否允许首仓、补仓、仅减仓
4. 若进入止损层，则执行分层减仓

---

## 三、动作定义

为了避免“能不能挂单”这种模糊表述，所有动作必须拆成三类：

### 3.1 `new_entry`

该方向当前没有持仓，新开第一笔仓位。

### 3.2 `add_entry`

该方向当前已有持仓，再增加同方向仓位。

### 3.3 `reduce_only`

只允许平仓 / 减仓，不允许扩大该方向风险。

这三个动作是风控准入判断的最小单位。

---

## 四、风险对象与观察维度

### 4.1 SideRisk：整侧风险

按 `LONG` / `SHORT` 聚合观察：

- side 总数量
- side 加权均价
- side 未实现亏损金额
- side 未实现亏损占权益比例
- side 持仓名义金额 / 账户权益
- side 距爆仓距离

### 4.2 LotRisk：分 lot 风险

按库存 lot 观察：

- 每个 lot 入场价
- 当前 lot 亏损比例
- lot 所属 `source_level`
- lot 是否属于中轴重仓区域
- lot 是否属于突破后建立的差位置仓位

### 4.3 StructureRisk：结构风险

按市场结构观察：

- 当前价格相对 `short_box / mid_box / long_box` 的位置
- 是否已突破网格边界
- 是否属于趋势加速
- 波动率是否显著扩张
- 当前 regime 是否仍适合均值回归

### 4.4 FinalRisk 组合概念

最终风险由以下四部分共同决定：

- `PositionRisk`
- `StructureRisk`
- `LiquidationRisk`
- `ConcentrationRisk`

这意味着止损不应再由单一 `stop_loss_pct` 决定。

---

## 五、核心监控指标

### 5.1 仓位亏损指标

- `side_loss_pct = abs(current_price - side_avg_entry) / side_avg_entry`
- `worst_lot_loss_pct`
- `unrealized_loss_usd`
- `unrealized_loss_pct_of_equity`

### 5.2 仓位规模指标

- `position_percent`
- `effective_leverage`
- `side_notional / equity`
- `filled_levels_count`
- `center_mass_ratio`

其中：

`center_mass_ratio = 中轴附近已成交名义金额 / 整侧已成交名义金额`

这个值越高，说明风险越集中在最重仓区域。

### 5.3 结构偏离指标

- `distance_from_short_box`
- `distance_from_mid_box`
- `distance_from_grid_boundary`
- `breakout_level`
- `breakout_direction`

### 5.4 爆仓保护指标

- `liquidation_distance_pct`
- `loss_to_liq_ratio`

其中：

`loss_to_liq_ratio = 当前不利偏移 / 当前到爆仓的剩余距离`

### 5.5 回撤止损指标

- `side_peak_favorable_price`
- `retracement_from_peak`
- `open_profit_drawdown_pct`

这组指标用于浮盈保护和风险回撤观察。

---

## 六、边际区准入风控

### 6.1 设计目标

边际区准入风控用于解决两类问题：

1. 避免在箱体边缘、突破边缘开出位置最差的新仓
2. 避免已有逆势仓位在危险区域继续被补大

它不是止损替代器，而是风险放大器抑制器。

### 6.2 区域定义

基础输入：

- `current_price`
- `grid_upper_price`
- `grid_lower_price`
- `grid_spacing`
- `short_box_upper / short_box_lower`
- `long_box_upper / long_box_lower`

边界基准说明：

- 所有区域判断都必须基于“当前有效 grid 边界快照”
- 该快照至少包含：
  - `effective_grid_upper_price`
  - `effective_grid_lower_price`
  - `effective_grid_spacing`
- 该快照在一个 `context_version` 内保持不变
- AI、admission、stop-loss、reseed 必须共享同一份边界快照
- 禁止在同一 cycle 内一部分逻辑使用旧边界、另一部分逻辑使用新边界

定义参数：

- `edge_band = 2.0 * grid_spacing`
- `breakout_band = 2.0 * grid_spacing`
- `trend_confirm_cycles = 2`

区域定义：

1. `safe_zone`
   价格距离上下边界都大于 `edge_band`

2. `edge_zone_upper`
   `grid_upper_price - edge_band <= current_price <= grid_upper_price`

3. `edge_zone_lower`
   `grid_lower_price <= current_price <= grid_lower_price + edge_band`

4. `breakout_zone_upper`
   `grid_upper_price < current_price <= grid_upper_price + breakout_band`

5. `breakout_zone_lower`
   `grid_lower_price - breakout_band <= current_price < grid_lower_price`

6. `trend_zone_upper`
   `current_price > grid_upper_price + breakout_band`

7. `trend_zone_lower`
   `current_price < grid_lower_price - breakout_band`

补充确认条件：

- 如果价格连续 `N=2~3` 个周期仍处在 breakout/trend 外侧，则结构风险升级

### 6.3 方向风险区

区域必须和方向结合判断。

对 `SHORT`：

- `edge_zone_upper / breakout_zone_upper / trend_zone_upper` 是危险区

对 `LONG`：

- `edge_zone_lower / breakout_zone_lower / trend_zone_lower` 是危险区

因此每个周期都应计算：

- `short_risk_zone = safe / edge / breakout / trend`
- `long_risk_zone = safe / edge / breakout / trend`

此外，首仓保护区不是独立第二套市场结构，而是 admission 的前置覆盖层。
建议最终在执行层只消费一个方向化准入区域枚举：

- `safe`
- `first_entry_guard`
- `edge`
- `breakout`
- `trend`

优先级建议：

1. `trend`
2. `breakout`
3. `first_entry_guard`
4. `edge`
5. `safe`

说明：

- `trend / breakout` 优先级最高
- `first_entry_guard` 只在边界内生效，不覆盖 `breakout / trend`
- `edge` 仍是通用边际区，但对 `new_entry` 来说，会被 `first_entry_guard` 的更强规则先裁决

### 6.4 准入矩阵

#### Safe Zone

- `new_entry`: 允许
- `add_entry`: 允许
- `reduce_only`: 允许

#### Edge Zone

- `new_entry`: 禁止
- `add_entry`: 有条件允许
- `reduce_only`: 允许

`add_entry` 允许条件：

- 该方向已有持仓
- 当前风险状态必须是 `normal`
- 补仓后 `position_percent` 不超过阈值
- 补仓后 `effective_leverage` 不超过阈值
- 补仓后预测止损亏损不超过账户风险预算
- 单次补仓金额必须衰减
- 连续补仓次数不能超限

#### Breakout Zone

- `new_entry`: 禁止
- `add_entry`: 禁止
- `reduce_only`: 允许

#### Trend Zone

- `new_entry`: 禁止
- `add_entry`: 禁止
- `reduce_only`: 允许
- 强制交由止损状态机接管

### 6.5 补仓限制

补仓不是简单的开或不开，而是三档制：

#### `full_add`

- 仅在 `safe_zone`
- 金额按正常网格分配

#### `limited_add`

- 仅在 `edge_zone`
- 单次补仓金额 `<= normal_grid_order_notional * 0.3`
- 同一方向在同一边际阶段最多补 `1` 次
- 若价格未回到 `safe_zone`，不得再次补仓

#### `blocked_add`

- 在 `breakout_zone / trend_zone`
- 完全禁止

建议约束：

- `max_edge_add_count_per_side = 1`
- `min_cycles_between_edge_add = 3`

### 6.6 位置评分

增加 `entry_location_score`，作为开仓前评分项。

评分输入：

- 当前价格相对网格中轴的位置
- 当前价格相对边界的位置
- 是否已在边界外
- 是否连续多个周期停留在边界外
- 当前波动 / breakout 状态

输出分级：

- `good`
- `neutral`
- `bad`
- `forbidden`

映射建议：

- `safe_zone` 中部：`good`
- `safe_zone` 靠边：`neutral`
- `edge_zone`：`bad`
- `breakout / trend_zone`：`forbidden`

使用规则：

- `new_entry` 仅允许 `good / neutral`
- `add_entry` 仅允许 `neutral / bad`
- `forbidden` 一律拒绝

### 6.7 预估风险校验

任何 `add_entry` 之前，先模拟补仓后的风险。

输入：

- 当前该方向持仓均价
- 当前该方向持仓数量
- 拟补仓价格
- 拟补仓数量
- 当前 `hard_reduce` 触发阈值
- 当前权益

输出：

- 补仓后新均价
- 若价格继续走到 `hard_reduce` 阈值，预计亏损是多少
- 预计亏损占权益比例
- 预计仓位占比变化
- 预计爆仓距离是否恶化

拒绝条件建议：

- `projected_loss_eq_pct_after_add > 1.5%`
- `projected_position_percent_after_add > 25%`
- `projected_effective_leverage_after_add > 1.2`
- `projected_liq_distance < 12%`

---

## 七、分层止损状态机

新的止损模型建议改成五层状态机，而不是单一 5% 触发。

### 7.1 State 0: Normal

正常网格运行：

- 允许开仓
- 允许补仓
- 正常挂相邻平仓单

进入 `State 1` 的条件，满足任一即可：

- `side_loss_pct >= 2.5%`
- `worst_lot_loss_pct >= 3.5%`
- `position_percent >= 70%`
- 当前价突破 `mid_box`
- `effective_leverage` 超过策略安全阈值

### 7.2 State 1: Warning

风险预警阶段：

- 禁止该方向新增首仓
- 保留现有 reduce-only 平仓单
- 提高平仓优先级
- 启动回撤观察
- 默认不允许扩大同侧风险
- 若业务需要保留补仓口子，只允许在 `safe_zone` 做一次 `limited_add`
- `edge / breakout / trend` 下的同方向补仓一律禁止

进入 `State 2` 的条件，满足任一即可：

- `side_loss_pct >= 4%`
- 当前价脱离 `short_box` 并持续 2 至 3 个周期
- `position_percent >= 85%`
- `liquidation_distance_pct` 进入警戒区
- `center_mass_ratio` 过高且继续逆向扩张

### 7.3 State 2: Soft Reduce

部分止损阶段，目标是先降风险，而不是一次性出清。

动作：

- 取消该方向所有开仓单
- 执行第一段减仓，建议减 `25% ~ 35%`
- 优先减最差 lot 与中轴高权重 lot
- 剩余仓位继续观察，但禁止重新扩大同方向风险
- 剩余仓位的止损阈值自动收紧

进入 `State 3` 的条件，满足任一即可：

- `side_loss_pct >= 5%`
- `worst_lot_loss_pct >= 6%`
- 当前价突破 `long_box`
- `loss_to_liq_ratio` 达到危险区
- `State 2` 后风险未改善且继续恶化

### 7.4 State 3: Hard Reduce

强减仓阶段，目标是优先保证敞口明显下降。

动作：

- 再减 `35% ~ 50%`
- 若风险仍高，则继续分批减仓
- 该方向完全禁止新增仓位
- 策略进入 `paused_risk`
- 仅维护 reduce-only 订单

进入 `State 4` 的条件，满足任一即可：

- `liquidation_distance_pct` 接近红线
- `unrealized_loss_pct_of_equity` 超过账户级阈值
- `daily_loss_limit` 命中
- breakout 持续恶化且没有回归迹象

### 7.5 State 4: Emergency

最终保护层：

- 立即平掉该侧剩余仓位
- 取消所有 entry 单
- 仅保留必要 reduce-only / 直接平仓
- 全策略暂停，等待人工或更严格恢复条件

---

## 八、部分止损设计

部分止损不应随机减，也不应简单按总仓位比例一刀切。

### 8.1 减仓优先级

1. 最差 lot
   亏损比例最大的 lot 优先减

2. 中轴高权重 lot
   高斯分配下，中间层风险贡献更大

3. 结构差位置 lot
   已处于箱体外、突破后建立的仓位优先减

4. 占用时间过长且未回归的 lot

### 8.2 减仓档位

- `Soft Reduce 1`: 减 `30%`
- `Soft Reduce 2`: 再减 `25%`
- `Hard Reduce`: 余下仓位减到只保留 `20% ~ 30%` 观察仓

目标是：

- 不一触发就全平
- 也不因等待回撤而继续满额承压

---

## 九、回撤止损设计

回撤止损只适用于轻中度风险，不能代替硬止损。

### 9.1 盈利保护回撤止损

适用：

- 仓位一度已有明显浮盈

逻辑：

- 记录持仓建立后的最佳有利价格
- 当浮盈回撤超过既定阈值时，执行部分止盈 / 止损

建议：

- 已获利润回撤 `35% ~ 50%` 时，先减 `25% ~ 40%`

### 9.2 风险回撤失败止损

适用：

- 已进入 `warning / soft_reduce`
- 但市场未出现有效回归

逻辑：

- 给回撤等待一个时间窗和价格窗
- 如等待 `2~3` 个周期仍未改善，则升级为更主动减仓

结论：

- 回撤止损不能无限等待
- 必须设置最大等待周期与超时升级条件

---

## 十、执行价格原则

执行价格必须按风险等级分层。

### 10.1 轻风险

可允许改善价格退出，但离现价不能太远。

建议：

- 距现价不超过 `min(1.0 * grid_spacing, 0.6 * ATR14, 0.8%)`

### 10.2 中风险

以可成交为优先。

建议：

- `SHORT`：挂在 `current_price - 0.2~0.5 * grid_spacing`
- `LONG`：挂在 `current_price + 0.2~0.5 * grid_spacing`
- 超时未成交则再追一次

### 10.3 高风险 / Emergency

不再优先考虑优化价格，而是优先退出确定性。

结论：

- 轻风险可以稍等
- 中风险只能短等
- 高风险不能等

---

## 十一、账户级风险闸门

除了单侧止损，还应有账户级闸门。

### 11.1 `daily_loss_limit`

达到后：

- 停止新开仓
- 剩余仓位仅允许 reduce-only 管理

### 11.2 `max_drawdown_limit`

账户净值从本轮峰值回撤超过阈值后：

- 强制进入 `hard_reduce / emergency`

### 11.3 `liquidation_distance_guard`

建议：

- 距爆仓 `< 8%`：禁止加仓
- 距爆仓 `< 5%`：强制 `hard_reduce`
- 距爆仓 `< 3%`：直接 `emergency`

---

## 十二、恢复机制

止损后不能马上恢复开仓。

恢复条件至少满足：

- 该侧仓位已明显降低或清零
- 当前价重新回到 `mid_box` 或合理区间
- 连续 `N` 个周期没有继续 breakout
- `effective_leverage` 回落到安全值
- 风险冷却期结束

若是 `emergency`：

- 必须人工确认或更严格自动恢复条件
- 不能下一周期自动重新挂网格

---

## 十三、参数建议第一版

### 13.1 边际区参数

- `edge_band = 2.0 * grid_spacing`
- `breakout_band = 2.0 * grid_spacing`
- `trend_confirm_cycles = 2`
- `edge_add_notional_multiplier = 0.3`
- `max_edge_add_count_per_side = 1`
- `min_cycles_between_edge_add = 3`

### 13.2 风险预测约束

- `projected_position_percent_after_add <= 25%`
- `projected_loss_eq_pct_after_add <= 1.5%`
- `projected_effective_leverage_after_add <= 1.2`
- `projected_liq_distance >= 12%`

### 13.3 止损状态阈值

- `Warning`: `side_loss_pct >= 2.5%`
- `Soft Reduce`: `side_loss_pct >= 4.0%`
- `Hard Reduce`: `side_loss_pct >= 5.0%`
- `Emergency`: `unrealized_loss_pct_of_equity >= 8%` 或更高优先级账户闸门命中

---

## 十四、实现规格

### 14.1 输入字段

建议实现时依赖以下输入：

- 交易所实时持仓
- 实时 open orders
- `grid_inventory_lots`
- 当前网格边界与 spacing
- 当前账户权益
- 当前风险状态
- 当前 risk history
- ATR / RSI / EMA distance / breakout 结构信号

### 14.2 核心函数

建议拆成 4 个模块：

1. `classifyRiskZone(side, price, gridState) -> zone`
2. `evaluateEntryIntent(side, hasPosition, zone, riskState) -> actionPolicy`
3. `projectPostAddRisk(currentPosition, candidateOrder, thresholds) -> projectedRisk`
4. `enforceZoneEntryPolicy(candidateOrder, policy, projectedRisk) -> allow / reject`

### 14.3 输出结构

建议新增统一输出对象：

```json
{
  "side": "SHORT",
  "risk_zone": "edge",
  "risk_state": "warning",
  "allow_new_entry": false,
  "allow_add_entry": true,
  "reduce_only_only": false,
  "max_add_notional": 900.0,
  "entry_location_score": "bad",
  "policy_reason": "upper edge zone: first entry blocked, limited add only"
}
```

### 14.4 伪代码

```text
for each candidate order:
  side = candidate.position_side
  zone = classifyRiskZone(side, currentPrice, gridState)
  riskState = evaluateRiskState(side, account, lots, structure)
  policy = evaluateEntryIntent(side, hasSidePosition, zone, riskState)

  if policy.reduce_only_only:
    reject all non-reduce-only orders

  if candidate is new_entry and !policy.allow_new_entry:
    reject

  if candidate is add_entry:
    if !policy.allow_add_entry:
      reject
    projected = projectPostAddRisk(currentSidePosition, candidate, thresholds)
    if projected violates thresholds:
      reject
    clamp candidate notional to policy.max_add_notional

  if riskState in soft_reduce/hard_reduce/emergency:
    bypass normal entry flow
    route to reduce-only risk execution
```

### 14.5 与看板联动字段

建议在风险看板中增加：

- `current_risk_zone_long`
- `current_risk_zone_short`
- `allow_new_long`
- `allow_add_long`
- `allow_new_short`
- `allow_add_short`
- `entry_location_score_long`
- `entry_location_score_short`
- `projected_add_risk_summary`
- `policy_reason`

这样前端可以解释“为什么这时不让开 / 不让补”。

---

## 十五、针对本次案例的回放

如果把这套规则套回本次实盘事件，路径会是：

1. 11:25 起，价格接近上边界，已有 `L14/L15/L16` 空头
   - `SHORT` 进入 `edge_zone_upper`
   - 禁止 `new_entry`
   - 若允许补空，也只能小额、一次、且必须通过预测风险校验

2. 12:58 到 14:16，价格多次站到原网格上界外
   - `SHORT` 进入 `breakout_zone_upper`
   - 所有新空和补空都禁止
   - 只保留减仓与观察

3. 18:31 以后，价格到 `0.1099+`
   - `SHORT` 已进入 `trend_zone_upper`
   - 交由 `hard_reduce`

结论：

- 这套规则能阻止突破阶段继续把空头补大
- 但不能消除更早已经建立的旧空头风险
- 因此它能防止风险放大，但不能替代止损

---

## 十六、最终策略口径

不建议把规则写成：

- 边际范围内不开新仓，可以补仓

建议写成：

- 边际区禁止首仓，补仓限额限次；突破区禁止任何同方向加仓，只允许减仓；趋势区由分层止损状态机接管。

这个表达更准确，也更不容易在实现时被误解成“边缘还能一直补仓”。

---

## 十七、与现有业务逻辑的冲突分析

本文设计可以与当前网格业务兼容，但和现有实现存在若干职责重叠与状态冲突。若不先做统一编排，后续继续叠加规则会导致行为难以解释。

### 17.1 当前并行存在的五套风险链路

当前系统里，至少有以下五套逻辑都在影响“是否开仓、是否撤单、是否暂停、是否重建”：

1. `price breakout` 逻辑  
   当前价格突破网格边界后，`checkBreakout()/handleBreakout()` 会直接决定是否 `pause + cancel entry`

2. `box breakout` 逻辑  
   `checkBoxBreakout()/executeBreakoutAction()` 会基于 short/mid/long box 确认结果执行减仓、暂停、close_all 或方向调整

3. AI 决策逻辑  
   AI 会给出 `hold / pause_grid / adjust_grid / cancel_all_orders`

4. 边界金额 cap 逻辑  
   当前边界保守逻辑只会压缩单格名义金额，不会真正阻止挂单

5. 新止损状态机  
   `warning / soft_reduce / hard_reduce / emergency` 会执行部分减仓、强减仓和风险暂停

问题不在单条逻辑错误，而在它们都能直接影响执行层，却没有一个统一的总裁决器。

### 17.2 价格突破与止损状态机冲突

当前有两条独立链路：

- `RunGridCycle()` 开头先执行 `checkBreakout -> handleBreakout`
- `syncGridState()` 结束后再执行 `checkAndExecuteStopLoss`

两者都会：

- 修改暂停状态
- 撤销开仓单
- 影响退出路径

但它们的判定基础不同：

- `price breakout` 基于当前价格相对网格边界
- `stop-loss state machine` 基于持仓亏损、lot 亏损、爆仓距离、权益亏损占比

如果不统一主状态，系统容易同时处于：

- `price_breakout_pause`
- `hard_reduce`

最终用户只会看到“暂停了”，但不知道是因为结构暂停还是风险暂停。

### 17.3 `price breakout` 与 `box breakout` 冲突

当前系统同时存在两套突破模型：

- 单周期网格边界突破
- 多周期 box 突破确认

两者的阈值、确认方式、动作都不同：

- 一个可能 `pause_grid`
- 一个可能 `reduce_position`
- 一个可能 `close_all`
- 一个可能 `adjust_direction`

如果后续又引入边际区准入，系统会变成“三套突破系统并行”。

建议：

- `price breakout` 降级为近场结构描述
- `box breakout` 作为中期结构确认
- 两者都不直接下动作，而是统一进入结构层输出

### 17.4 边界金额 cap 与边际区准入冲突

当前边界保守逻辑只做一件事：

- 常规区 cap 到 `uniform x 1.5`
- 边缘区 cap 到 `uniform x 1.2`

它不会禁止挂单，只会把挂单金额压小。

而本文设计中的边际区准入要求：

- `new_entry` 可被直接禁止
- `add_entry` 可被限次、限额、限状态
- `breakout / trend` 可全面禁止加仓

因此两者不是同一层规则：

- 当前逻辑：金额控制
- 新设计：行为准入控制

如果不重新分层，后面会出现“准入说不让开，但金额 cap 还在独立工作”的语义重叠。

### 17.5 AI hold / auto-seed 与边际区准入冲突

当前系统在 `hold` 和明显震荡条件下会触发：

- `autoSeedMissingEntryOrdersForRangingGrid()`

这条链路会自动修补缺失的开仓层。

但它当前只看：

- 是否明确震荡
- 是否缺失开仓层

它不看：

- `safe / edge / breakout / trend zone`
- `new_entry` 还是 `add_entry`
- 当前是否只应 `reduce_only`

因此如果未来直接叠加边际区准入，而不改 auto-seed，自动补单链会绕过新准入规则。

### 17.6 位置惩罚与位置准入重复建模

当前止损状态机里已经有：

- `EntryLocationPenalty`

它会按仓位位置直接压低 `warning / soft / hard` 阈值。

而边际区设计里又引入了：

- `price_zone`
- `entry_location_score`
- `forbidden / bad / neutral / good`

如果两套都保留且各自独立生效，会造成“双重处罚”：

- 一套在准入层阻止开仓
- 一套在止损层提前触发风险状态

设计上应统一成一套“位置风险语义”，而不是两套并行。

### 17.7 暂停恢复语义不统一

当前恢复主要围绕：

- `pause_grid`
- `recent_pause_guard`
- `false breakout recovery`
- `ranging protection`

展开。

但新的止损模型要求：

- `soft_reduce / hard_reduce / emergency` 也应有独立恢复规则

如果继续只用：

- `IsPaused`
- `PauseReason`

会把“突破暂停”和“风险暂停”混在一起，恢复时无法做到不同逻辑不同恢复条件。

---

## 十八、统一架构建议

### 18.1 设计目标

全局设计目标不是继续叠一套规则，而是把当前并行的多条风控链收敛成统一编排：

- 结构层只描述市场
- 风险层只描述风险状态
- 准入层只决定能不能开 / 能不能补
- 执行层只负责动作落地

### 18.2 建议的三层总架构

#### 第一层：Market Structure Layer

只负责输出结构状态，不直接触发执行动作。

输出建议：

- `price_zone_long`
- `price_zone_short`
- `box_breakout_level`
- `breakout_confirmed`
- `ranging_score`
- `trend_score`
- `direction_bias`

说明：

- `price breakout` 变成近场边界结构描述
- `box breakout` 变成中期结构确认
- 两者都不直接 pause 或减仓

#### 第二层：Risk Policy Layer

这是统一编排核心。

输入：

- 市场结构层输出
- live positions
- inventory lots
- account equity
- current risk state
- liquidation metrics

输出：

- `allow_new_long`
- `allow_add_long`
- `allow_new_short`
- `allow_add_short`
- `force_reduce_only_long`
- `force_reduce_only_short`
- `risk_state_long`
- `risk_state_short`
- `selected_global_risk_state`
- `pause_mode`
- `rebuild_mode`

其中建议把暂停显式分成：

- `none`
- `breakout_pause`
- `risk_pause`
- `manual_pause`

重建模式显式分成：

- `none`
- `adjust_grid_entries_only`
- `full_flat_reseed`

#### 第三层：Execution Layer

执行层只消费 policy，不再自行解释市场。

只负责：

- 哪些 entry 单允许下
- 哪些 entry 单必须撤
- 是否执行 partial reduce
- 是否只维护 reduce-only
- 是否允许 full reseed

执行层不再自己决定是否趋势、是否边界危险、是否应补仓。

### 18.3 职责边界

#### `breakout` 负责什么

- 只负责定义结构状态
- 不直接撤单
- 不直接 pause
- 不直接减仓

#### AI decision 负责什么

- 给出 `hold / adjust_grid / pause_grid` 建议
- 不拥有最终否决权
- 所有 AI 动作必须再经过 policy 裁决

#### entry admission 负责什么

- 决定 `new_entry / add_entry / reduce_only`
- 这是边际区准入的唯一落点
- 它必须位于 auto-seed 和 AI hold 之下，作为统一拦截器

#### stop-loss state machine 负责什么

- 只负责已有仓位如何减
- 不负责定义市场结构
- 不负责决定是否允许新增仓位

#### pause / recovery 负责什么

- 统一由 `pause_mode` 管理
- 每种 pause mode 有独立恢复条件

### 18.4 推荐的统一主状态

建议新增统一主状态，不再只依赖 `IsPaused + PauseReason + CurrentRiskState` 的松散组合。

主状态建议：

- `running_normal`
- `running_restricted`
- `paused_breakout`
- `paused_risk`
- `paused_manual`
- `recovering_after_breakout`
- `recovering_after_risk`

这样可以把：

- 风险状态
- 暂停状态
- 恢复阶段

统一起来，避免用户只能看到一个模糊的 `paused`。

### 18.5 关键链路重构建议

#### `handleBreakout()`

建议改造方向：

- 不再直接 `pause + cancel entry`
- 改为写入结构状态
- 由 policy 决定是否进入 `breakout_pause`

#### `checkBoxBreakout()`

建议改造方向：

- 不再直接 `reduce_position / close_all / adjust_direction`
- 改成输出更高等级的结构信号
- 由 policy 层统一裁决

#### `autoSeedMissingEntryOrdersForRangingGrid()`

建议改造方向：

- 保留 candidate 发现逻辑
- 所有 candidate 在挂单前必须走 `entry admission`
- 不能绕过边际区准入

#### `EntryLocationPenalty`

建议改造方向：

- 取消单独阈值压缩语义
- 改为从统一的 `price_zone + entry_location_score` 派生
- 避免位置风险被重复建模

### 18.6 看板与审计建议

为了让系统行为可解释，建议看板增加统一解释字段：

- `market_structure_state`
- `long_risk_zone`
- `short_risk_zone`
- `pause_mode`
- `resume_policy`
- `entry_policy_long`
- `entry_policy_short`
- `policy_reason`
- `suppressed_actions`

这样用户可以直接看到：

- 为什么现在不让开
- 为什么现在不让补
- 为什么现在暂停
- 为什么现在还不能恢复

### 18.7 最终全局判断

当前这套风控与止损模型没有根本性业务冲突，但如果直接叠到现有代码上，会出现：

- 多套突破系统并行
- 多个模块都能改暂停状态
- auto-seed 绕过准入层
- 位置风险双重建模
- 风险暂停与突破暂停恢复条件混淆

因此全局设计的优先级应是：

1. 先统一结构层、风险层、执行层职责
2. 再接入边际区准入
3. 再收口现有 breakout / stop-loss / auto-seed / AI hold 的优先级

结论：

- 当前问题不是哪条规则一定错
- 而是需要一个统一的风控编排层
- 否则规则越多，系统越不可解释

---

## 十九、AI 判断时点与业务执行时点冲突分析

### 19.1 问题定义

当前系统中，AI 的判断不是在一个冻结世界里完成的，而是在持续变化的运行态中完成的。

这意味着：

- AI 看到的是某一时刻的快照
- 执行发生在稍后的另一时刻
- AI 返回前后，业务逻辑自己还会继续修改状态

因此冲突的根因不是“AI 判断错”，而是：

- 判断时点
- 执行时点
- 后处理时点

三者没有被显式建模。

### 19.2 当前一个 grid cycle 的真实时序

当前 `RunGridCycle()` 大致顺序是：

1. 前置业务检查
   - `checkBreakout()`
   - `checkMaxDrawdown()`
   - `checkDailyLossLimit()`
   - `checkBoxBreakout()`
   - `checkFalseBreakoutRecovery()`
   - `enforceRecentPauseGuard()`
   - `checkAndMitigateTrappedInventoryRisk()`

2. 若已 paused
   - `cancel entry`
   - `syncGridState()`
   - `preserve exits`
   - 保存 paused 记录

3. 若未 paused
   - `buildGridContext()`
   - 请求 `GetGridDecisions(...)`
   - 按返回结果逐条执行

4. 执行后
   - `syncGridState()`
   - `ensurePairedExitOrdersForAllFilledLevels()`
   - `autoSeedMissingEntryOrdersForRangingGrid()`
   - 保存 decision record

注意：

- `syncGridState()` 内部还会继续触发：
  - stop-loss 检查
  - auto-adjust
  - flat exit 后 full reseed

因此 AI 不是唯一执行源，系统本身在 AI 前后都还会继续驱动状态变化。

### 19.3 三种时点

建议显式区分三个时点：

1. `observation time`
   AI 构建上下文的时间点

2. `execution time`
   AI 动作真正开始落地的时间点

3. `post-sync time`
   执行后系统自动修复、补单、止损、调整再次发生的时间点

如果这三个时点没有版本化，后续所有冲突都只能靠规则互相覆盖。

### 19.4 三类典型冲突

#### 冲突一：AI 看到的是旧结构，执行时结构已经变化

例子：

- `buildGridContext()` 时价格仍在 grid 内
- AI 认为可以 `hold` 或补单
- AI 返回时价格已经进入 `edge / breakout`
- 若执行前没有再做一次最新 policy 校验，AI 动作就已经过时

当前系统的问题：

- `checkBreakout()` 在 cycle 开头只做一次
- AI 返回后不会再做一次执行前快照核验

#### 冲突二：AI 想收缩风险，但执行层用另一个时点把它挡掉

当前 `executeGridDecision()` 里有 `ranging protection`：

- 如果执行时刻本地判断仍是 clearly ranging
- 会阻止某些 destructive action

这就出现：

- AI 用 observation time 判断要收缩
- 执行层用 execution time 判断“当前还像 ranging”
- 然后把 AI 的动作挡掉

问题不在谁对谁错，而在于：

- 两者使用了不同时间切片
- 没有统一的裁决层解释哪个时间切片优先

#### 冲突三：AI 执行完后，系统自动逻辑继续改状态

当前 AI 决策执行完之后，系统还会继续：

- `syncGridState()`
- 补 paired exits
- `autoSeedMissingEntryOrdersForRangingGrid()`
- `checkAndExecuteStopLoss()`
- `autoAdjustGrid()`

结果是：

- AI `hold`
  但后处理可能补了新的 entry

- AI `adjust_grid`
  但后处理可能又触发 stop-loss 或 rebuild

- AI `pause`
  但另一条恢复链可能后面又介入

这意味着 AI 决策只是中间一步，不是最终行为来源。

### 19.5 当前最危险的设计问题

当前最大的问题不是“AI 慢”，而是：

- AI 被记录成主判断源
- 但系统行为并不完全由 AI 决定

现在的审计记录主要体现：

- AI 想做什么
- 执行结果成功或失败

但没有完整体现：

- 执行前状态有没有漂移
- 哪些动作是 policy gate 拦掉的
- 哪些动作是 post-sync 自动追加的

这会导致用户看到日志时误以为：

- AI 的输出就等于系统最终行为

实际上并不是。

---

## 二十、统一时序设计建议

### 20.1 设计目标

时序设计的目标不是“让 AI 更快”，而是让系统显式管理：

- AI 看到了什么版本的世界
- 执行发生在什么版本的世界
- 世界变了以后，这份 decision 是否还有效

### 20.2 新的 cycle 结构

建议把一个 grid cycle 改成 7 个阶段。

#### 阶段 1：Pre-check Stage

先做硬风控和硬暂停检查：

- max drawdown
- daily loss
- emergency breakout
- emergency liquidation risk

这一层只处理“无需等 AI 的绝对约束”。

#### 阶段 2：Context Snapshot Stage

生成上下文快照，并附加版本信息：

- `context_version`
- `context_built_at`
- `market_price_at_context`
- `position_hash`
- `open_orders_hash`
- `pause_state_hash`
- `risk_state_hash`

这个快照一旦生成，就成为 AI advisory 的输入版本。

#### 阶段 3：AI Advisory Stage

AI 不再直接被定义为最终执行决策源，而是 advisory source。

AI 输出建议：

- 市场解释
- 建议模式
- 可选候选动作

例如：

- `desired_mode = running_restricted`
- `candidate_actions = [hold, adjust_grid, place_buy_limit]`

#### 阶段 4：Policy Compilation Stage

这是统一裁决核心。

输入：

- 最新实时状态
- context snapshot
- AI advisory
- system rules

输出统一 policy：

- 这份 AI decision 是否过期
- 当前是否允许新仓 / 补仓
- 是否应 pause
- 是否应 reduce-only
- 是否应进入 risk state upgrade

#### 阶段 5：Execution Stage

所有动作都必须过 policy gate 才能落地。

无论动作来源是：

- AI advisory
- auto-seed
- breakout handling
- stop-loss

都不能绕过这层。

#### 阶段 6：Post-sync Stage

执行后：

- sync exchange state
- rebuild lots / exit bindings
- refresh current state

但这一阶段的自动动作也必须继续经过 policy gate，不能变成第二执行器。

#### 阶段 7：Audit Stage

记录四类信息：

- context snapshot
- AI advisory
- compiled policy
- actual execution
- post-sync automatic actions

这样可以完整还原：

- AI 当时怎么看
- 系统为什么没完全照做
- 后续自动逻辑又做了什么

### 20.3 增加 decision freshness 机制

建议引入显式时效控制。

#### Freshness 输入

- `price_drift_pct_since_context`
- `position_changed_since_context`
- `orders_changed_since_context`
- `pause_state_changed_since_context`
- `risk_state_changed_since_context`

#### 过期条件建议

若满足任一，则 decision 视为 stale：

- 价格偏移超过阈值
- 当前持仓数量或方向发生变化
- 当前 open orders 结构发生变化
- pause state 变化
- risk state 升级

#### stale 处理建议

- 丢弃 decision
- 或降级为 `hold + re-evaluate`
- 或仅允许 `reduce-only` 类动作继续执行

### 20.4 AI 的职责收敛建议

AI 不适合承担强实时执行动作判断，例如：

- 此刻这 1 秒要不要挂某个具体单
- 此刻这 1 秒要不要补某个具体层

AI 更适合承担：

- 市场结构解释
- 模式建议
- 恢复时机建议
- 网格重建建议
- 风险收缩建议

因此建议把 AI 职责从：

- `direct action generator`

收敛为：

- `strategy advisory generator`

### 20.5 自动化动作的编排建议

当前系统的问题之一是：

- AI 是一个执行源
- post-sync 自动逻辑又是另一个执行源

建议改成：

- 所有自动逻辑都只生成 candidate actions
- 统一提交给 orchestrator
- orchestrator 根据最新 policy 决定是否执行

这样：

- `autoSeedMissingEntryOrdersForRangingGrid()`
- `ensurePairedExitOrders...`
- `stop-loss reduction`
- `autoAdjustGrid()`

都不再各自直写执行层。

### 20.6 推荐的新审计字段

为了支持时点分析，建议新增：

- `context_version`
- `context_built_at`
- `decision_received_at`
- `execution_started_at`
- `execution_finished_at`
- `decision_stale`
- `stale_reason`
- `policy_overrides`
- `post_sync_actions`

这样可以回答：

- AI 判断时用了哪个快照
- 执行时是否已经过期
- 哪些动作被 policy 覆盖
- 哪些动作是后处理追加的

### 20.7 最终设计判断

当前业务逻辑与 AI 判断时间的冲突，不是单个 bug，而是结构问题：

- observation time
- execution time
- post-sync time

没有统一版本化，也没有 freshness 机制。

因此设计改进重点应是：

1. 引入上下文快照版本
2. 引入 decision freshness / stale 语义
3. 所有动作统一过 policy gate
4. AI 从直接执行源收敛为 advisory source
5. post-sync 自动逻辑不能绕过 orchestrator

结论：

- 这不是“让 AI 更准”就能解决的问题
- 这是一个时序编排问题
- 必须在全局架构上把“判断时间”和“执行时间”分开设计

---

## 二十一、落地路线图

### 21.1 路线图目标

这套设计不能一次性整体替换当前系统，否则风险过高。

更合理的推进方式是：

- 先把状态与审计补齐
- 再把准入层接进来
- 再把多条风控链统一到 orchestrator
- 最后才收口 AI 与自动逻辑的职责

### 21.2 分阶段原则

每一阶段都要满足三个要求：

1. 能单独上线
2. 不破坏现有平仓链路
3. 可以通过看板和日志观察效果

---

### 21.3 阶段一：可观测性与状态统一

#### 目标

先不改实质行为，先把系统到底“为什么做了这件事”记录清楚。

#### 主要改造

1. 增加统一主状态

- `running_normal`
- `running_restricted`
- `paused_breakout`
- `paused_risk`
- `paused_manual`
- `recovering_after_breakout`
- `recovering_after_risk`

2. 增加统一暂停模式

- `pause_mode`
- `pause_source`
- `pause_started_at`
- `resume_policy`

3. 增加统一结构状态输出

- `price_zone_long`
- `price_zone_short`
- `box_breakout_level`
- `breakout_confirmed`
- `ranging_score`
- `trend_score`

4. 增加统一审计字段

- `context_version`
- `context_built_at`
- `decision_received_at`
- `execution_started_at`
- `execution_finished_at`
- `decision_stale`
- `stale_reason`
- `policy_overrides`
- `post_sync_actions`

#### 本阶段不改什么

- 不改变现有下单逻辑
- 不改变 stop-loss 实际执行逻辑
- 不改变 AI 提示词和动作格式

#### 价值

- 先把系统行为解释清楚
- 为后续 policy gate 和 orchestrator 做观测基础

#### 风险点

- 数据结构和看板字段会增多
- 但这类改动风险低、可回滚

---

### 21.4 阶段二：边际区准入层接入

#### 目标

把“能不能开 / 能不能补 / 只能减仓”从金额 cap 升级为真正的准入规则。

#### 主要改造

1. 引入 `Market Structure Layer`

输出：

- `safe / edge / breakout / trend`

按方向分成：

- `long_risk_zone`
- `short_risk_zone`

2. 引入 `Entry Admission Evaluator`

输出：

- `allow_new_long`
- `allow_add_long`
- `allow_new_short`
- `allow_add_short`
- `max_add_notional`
- `policy_reason`

3. 统一接入开仓入口

以下链路都必须接 admission：

- AI 下单动作
- auto-seed 缺失层
- flat 后 full reseed
- 调整网格后的 entry 重挂

4. 现有金额 cap 降级为 admission 子规则

变成：

- “先决定能不能开”
- “如果能开，再决定 cap 到多少”

#### 本阶段不改什么

- 暂不改 stop-loss 状态机
- 暂不改 breakout 和 box breakout 的执行方式

#### 价值

- 先堵住“边缘还能继续补大”的入口问题
- 这是最直接减少风险扩大的阶段

#### 风险点

- 可能导致部分交易员短期挂单数下降
- 需要看板明确展示“为什么现在不让补”

---

### 21.5 阶段三：风险状态机与暂停恢复统一

#### 目标

把 `price breakout`、`box breakout`、`risk pause` 统一到同一个 pause / recovery 体系。

#### 主要改造

1. `price breakout` 降级为结构信号

- 不再直接 pause
- 不再直接 cancel entry

2. `box breakout` 降级为结构确认信号

- 不再直接 close_all / reduce_position / adjust_direction

3. 统一由 `Risk Policy Layer` 输出：

- `pause_mode`
- `risk_state`
- `rebuild_mode`

4. 恢复逻辑拆分

- `breakout_pause` 恢复条件
- `risk_pause` 恢复条件
- `manual_pause` 恢复条件

5. `EntryLocationPenalty` 收口

- 不再作为独立阈值修正器
- 统一纳入 `price_zone + entry_location_score + risk_state`

#### 本阶段不改什么

- 暂不改 AI advisory 模型
- 暂不完全移除现有 auto-adjust 行为

#### 价值

- 解决“多个模块都在 pause / recover”的冲突
- 让恢复条件可解释

#### 风险点

- 需要非常仔细处理向后兼容
- 否则可能影响已有暂停恢复路径

---

### 21.6 阶段四：统一执行编排层

#### 目标

让 AI、auto-seed、stop-loss、grid rebuild 都不再直接写执行层，而是统一走 orchestrator。

#### 主要改造

1. 引入 `Policy Compilation Stage`

输入：

- context snapshot
- AI advisory
- latest live state
- system rules

输出：

- compiled policy

2. 引入 `Execution Orchestrator`

所有 candidate actions 都要走这里。

来源包括：

- AI advisory
- auto-seed
- stop-loss
- rebuild
- breakout derived actions

3. 引入 `decision freshness`

判断：

- decision 是否过期
- 若过期应丢弃、降级还是仅保留 reduce-only

4. post-sync 自动逻辑收口

- post-sync 不再是独立执行器
- 只生成 candidate actions，再交给 orchestrator

#### 本阶段会改变什么

- AI 不再是直接动作执行源
- 自动逻辑不再能绕过 policy gate

#### 价值

- 这是整个设计真正完成统一的阶段
- 能解决判断时间与执行时间冲突

#### 风险点

- 实现复杂度最高
- 上线前必须有完整回归和灰度观察

---

### 21.7 阶段五：AI 职责收敛

#### 目标

让 AI 从“直接动作判断器”收敛成“策略 advisory”。

#### 主要改造

1. AI 输出从：

- `place order / cancel / adjust / pause`

逐步收敛成：

- `market interpretation`
- `desired mode`
- `strategy advisory`
- `optional candidate actions`

2. 执行动作更多由系统规则化模块决定

例如：

- 是否补单
- 是否进入 risk pause
- 是否 full reseed

这些不再依赖 AI 实时逐条判断。

#### 价值

- 时点冲突显著减小
- 行为一致性显著提升

#### 风险点

- 需要同步调整提示词与看板说明
- 用户会从“AI 直接交易”感知转向“AI 辅助策略控制”

---

## 二十二、模块拆解建议

### 22.1 推荐新增模块

1. `market_structure_evaluator`
2. `risk_policy_evaluator`
3. `entry_admission_evaluator`
4. `decision_freshness_evaluator`
5. `execution_orchestrator`
6. `pause_recovery_policy`
7. `audit_context_snapshot`

### 22.2 推荐收口的旧模块

后续应逐步降权或收口这些“直接执行型”逻辑：

- `handleBreakout()`
- `executeBreakoutAction()`
- `autoSeedMissingEntryOrdersForRangingGrid()`
- `EntryLocationPenalty` 独立阈值修正
- post-sync 中直接触发执行动作的逻辑

不是立刻删除，而是逐步变成：

- 生成候选信号
- 交给统一编排层

---

## 二十三、看板与灰度策略建议

### 23.1 看板先行

建议每一阶段上线前，先把对应可观测字段加到看板：

- 当前结构区间
- 当前准入策略
- 当前 pause mode
- 当前 risk state
- 当前 decision freshness
- 当前 policy override

### 23.2 灰度顺序

建议灰度顺序：

1. 只开观察，不改动作
2. 先对 auto-seed 接入 admission
3. 再对 AI entry 动作接 admission
4. 再统一 pause / recovery
5. 最后引入 orchestrator

### 23.3 回滚策略

每个阶段都应保留：

- feature flag
- per-trader enable switch
- 看板对比观察入口

避免一次性全量切换。

---

## 二十四、最终实施建议

如果从工程优先级排序，我建议是：

1. 先做阶段一  
   统一状态与审计，先把系统解释清楚

2. 再做阶段二  
   把边际区准入落在 entry admission 上，先堵风险放大入口

3. 再做阶段三  
   收口暂停与恢复，统一风险状态语义

4. 最后做阶段四和阶段五  
   引入 orchestrator 和 AI 职责收敛

结论：

- 这套设计是可以落地的
- 但必须按阶段推进，不能一次性把所有控制权重写
- 否则最容易破坏的是：平仓链路、恢复链路和现有交易员稳定性

---

## 二十五、开发任务拆分版

### 25.1 使用方式

本章节用于把前面的设计收敛成可执行的开发任务清单。

每个任务包含：

- 目标
- 主要改动点
- 依赖
- 验收标准

建议按任务组推进，而不是按文件推进。

---

### 25.2 任务组 A：统一状态与审计

#### A1. 主状态与暂停模式建模

目标：

- 给当前网格运行态增加统一主状态与 pause mode

主要改动点：

- 在 grid state 中新增：
  - `main_state`
  - `pause_mode`
  - `pause_source`
  - `resume_policy`
  - `recovering_mode`

依赖：

- 无

验收标准：

- 任一交易员在看板上能明确看到当前属于：
  - 正常运行
  - 受限运行
  - 突破暂停
  - 风险暂停
  - 人工暂停

#### A2. context snapshot 版本化

目标：

- 显式记录 AI 所看到的上下文版本

主要改动点：

- 给每次 `buildGridContext()` 生成：
  - `context_version`
  - `context_built_at`
  - `market_price_at_context`
  - `position_hash`
  - `open_orders_hash`
  - `pause_state_hash`
  - `risk_state_hash`

依赖：

- A1

验收标准：

- decision record 中可追溯本次 AI 使用的上下文版本

#### A3. 审计字段补齐

目标：

- 完整记录 AI、policy 和 post-sync 的关系

主要改动点：

- decision record / risk history 增加：
  - `decision_received_at`
  - `execution_started_at`
  - `execution_finished_at`
  - `decision_stale`
  - `stale_reason`
  - `policy_overrides`
  - `post_sync_actions`

依赖：

- A2

验收标准：

- 任一次 cycle 都能回答：
  - AI 看到了什么
  - 哪些动作被覆盖
  - 后处理追加了什么

---

### 25.3 任务组 B：结构层与准入层

#### B1. Market Structure Evaluator

目标：

- 把 `price breakout` 和 `box breakout` 收敛成统一结构输出

主要改动点：

- 新增结构评估模块，输出：
  - `long_risk_zone`
  - `short_risk_zone`
  - `box_breakout_level`
  - `breakout_confirmed`
  - `ranging_score`
  - `trend_score`

依赖：

- A1

验收标准：

- 同一时刻只存在一套结构结论
- 看板能显示当前结构区间和 breakout 等级

#### B2. Entry Admission Evaluator

目标：

- 统一判定 `new_entry / add_entry / reduce_only`

主要改动点：

- 新增 admission evaluator，输出：
  - `allow_new_long`
  - `allow_add_long`
  - `allow_new_short`
  - `allow_add_short`
  - `max_add_notional`
  - `policy_reason`

依赖：

- B1

验收标准：

- 对任一候选单，系统能明确返回：
  - 是否允许
  - 为什么允许 / 不允许

#### B3. 预测风险校验器

目标：

- 在 add_entry 前模拟补仓后的风险

主要改动点：

- 新增 projected risk evaluator，输出：
  - `projected_avg_entry`
  - `projected_loss_eq_pct`
  - `projected_position_percent`
  - `projected_effective_leverage`
  - `projected_liq_distance`

依赖：

- B2

验收标准：

- 任一补仓动作都能显示补仓后预测风险
- 超阈值补仓会被 admission 拒绝

---

### 25.4 任务组 C：接管所有 entry 入口

#### C1. AI 下单动作接 admission

目标：

- AI 候选 entry 单不能绕过准入层

主要改动点：

- `place_buy_limit / place_sell_limit` 在真正执行前先过 admission evaluator

依赖：

- B2
- B3

验收标准：

- AI 提议的边际区首仓会被拦截
- 拒绝原因可见

#### C2. auto-seed 接 admission

目标：

- 自动补缺失层也不能绕过准入层

主要改动点：

- `autoSeedMissingEntryOrdersForRangingGrid()` 中的 candidate entries 全部接 admission

依赖：

- C1

验收标准：

- 在 edge / breakout 区，auto-seed 不会继续补出被禁止的 entry

#### C3. flat reseed / adjust_grid reseed 接 admission

目标：

- full reseed 与 adjust 后 entry 重挂也必须受统一准入约束

主要改动点：

- `seedFullEntryGrid()`
- `adjust_grid` 后重挂 entry

依赖：

- C2

验收标准：

- 空仓整盘重挂不再无条件把所有层都挂出
- 受准入规则限制的层会被正确跳过

---

### 25.5 任务组 D：风险状态机与暂停恢复统一

#### D1. 收口 price breakout 与 box breakout 动作权

目标：

- breakout 模块不再直接下执行动作

主要改动点：

- `handleBreakout()` 改为输出结构事件
- `executeBreakoutAction()` 改为输出建议事件

依赖：

- B1

验收标准：

- breakout 模块本身不再直接 `pause/cancel/close_all`

#### D2. 统一 pause / recovery policy

目标：

- 用统一 pause mode 替代散落的 `IsPaused + PauseReason`

主要改动点：

- 定义：
  - `breakout_pause`
  - `risk_pause`
  - `manual_pause`
- 定义对应恢复条件

依赖：

- D1
- A1

验收标准：

- 看板能明确显示当前 pause mode
- 不同 pause mode 走不同恢复条件

#### D3. 收口 EntryLocationPenalty

目标：

- 避免位置风险双重建模

主要改动点：

- 用统一的 `zone + location_score + risk_state` 代替独立 penalty 阈值修正

依赖：

- B1
- D2

验收标准：

- 同一仓位不会同时被两套位置机制重复处罚

---

### 25.6 任务组 E：统一执行编排

#### E1. Decision Freshness Evaluator

目标：

- 在执行前判断 AI decision 是否已过期

主要改动点：

- 新增 freshness evaluator：
  - `price_drift_pct_since_context`
  - `position_changed_since_context`
  - `orders_changed_since_context`
  - `pause_changed_since_context`
  - `risk_changed_since_context`

依赖：

- A2

验收标准：

- 系统能明确标记 decision 是否 stale

#### E2. Policy Compilation Stage

目标：

- 把 AI advisory、结构、风险和系统规则统一编译成 policy

主要改动点：

- 新增 compiled policy 输出：
  - 准入策略
  - pause 模式
  - rebuild 模式
  - reduce-only 模式
  - override 理由

依赖：

- E1
- B2
- D2

验收标准：

- 对任一 cycle，都能产出一份完整 policy

#### E3. Execution Orchestrator

目标：

- 所有动作统一走 orchestrator

主要改动点：

- AI actions
- auto-seed actions
- stop-loss actions
- rebuild actions
- breakout derived actions

全部变为 candidate actions，由 orchestrator 统一落地

依赖：

- E2

验收标准：

- 没有 candidate action 能绕过 orchestrator

#### E4. post-sync 自动动作收口

目标：

- post-sync 不再是独立第二执行器

主要改动点：

- `syncGridState()` 后触发的自动补单 / 调整 / 风险动作改成 candidate actions

依赖：

- E3

验收标准：

- post-sync 动作也能在审计里看到来源与 override 理由

---

### 25.7 任务组 F：AI 职责收敛

#### F1. AI 输出收敛为 advisory

目标：

- AI 从 direct action source 收敛成 advisory source

主要改动点：

- AI 输出重点改为：
  - market interpretation
  - desired mode
  - recovery suggestion
  - optional candidate actions

依赖：

- E3

验收标准：

- AI 输出即使过期，也不会直接驱动错误下单

#### F2. 提示词与看板同步收敛

目标：

- 让产品口径与实现一致

主要改动点：

- 更新提示词
- 更新看板说明
- 更新审计展示文案

依赖：

- F1

验收标准：

- 用户理解从“AI 直接交易”切换为“AI 辅助策略控制”

---

## 二十六、任务依赖图

推荐依赖顺序：

1. A1 -> A2 -> A3
2. B1 -> B2 -> B3
3. C1 -> C2 -> C3
4. D1 -> D2 -> D3
5. E1 -> E2 -> E3 -> E4
6. F1 -> F2

推荐实施顺序：

1. 先做 A
2. 再做 B + C
3. 再做 D
4. 再做 E
5. 最后做 F

---

## 二十七、阶段验收标准

### 阶段一验收

- 看板能解释当前主状态、pause mode、risk state
- decision record 可追溯 context version

### 阶段二验收

- edge / breakout 区首仓和补仓限制生效
- auto-seed 不再绕过准入层

### 阶段三验收

- breakout pause 与 risk pause 语义分离
- 恢复逻辑按 pause mode 生效

### 阶段四验收

- 所有动作统一过 orchestrator
- stale decision 能被识别并处理

### 阶段五验收

- AI 输出收敛为 advisory
- 审计中能区分 AI 建议、policy 决定和最终执行

---

## 二十八、最终工程建议

如果只给开发团队一条总原则，我建议是：

- 先统一状态与审计
- 再统一准入
- 再统一暂停与恢复
- 最后统一编排与 AI 职责

不要直接从：

- 改 AI 提示词
- 改止损参数
- 改某个补单逻辑

这种局部入口开始。

因为当前真正的问题不是某个点的参数不够好，而是：

- 系统里同时存在多个决策源和多个执行源

这件事必须先从架构上收口。

---

## 二十九、项目排期版

### 29.1 目标

本章节用于把前面的任务清单进一步压缩成可执行排期方案。

排期维度包括：

- 优先级
- 预估工作量
- 风险等级
- 是否需要灰度
- 是否需要看板先行

说明：

- 这里的工作量采用粗粒度估算：`S / M / L / XL`
- 风险等级采用：`低 / 中 / 高`

---

### 29.2 排期原则

推荐遵循四条原则：

1. 先做可观测性，再做行为变更
2. 先堵风险放大入口，再改核心执行链
3. 先做局部灰度能力，再做统一 orchestrator
4. 先让系统可解释，再让 AI 职责收敛

这意味着：

- 不应一开始就直接改 stop-loss 或 AI 提示词
- 不应跳过审计与看板直接做统一编排

---

### 29.3 推荐排期总览

#### Wave 1：状态、审计、看板先行

包含：

- A1 主状态与 pause mode
- A2 context snapshot 版本化
- A3 审计字段补齐

优先级：

- `P0`

工作量：

- `M`

风险等级：

- `低`

是否需要灰度：

- `否`

是否需要看板先行：

- `是`

目标：

- 先让系统行为可解释
- 为后续所有动作改造建立观测基础

#### Wave 2：边际区准入与补仓风险控制

包含：

- B1 Market Structure Evaluator
- B2 Entry Admission Evaluator
- B3 预测风险校验
- C1 AI entry 接 admission
- C2 auto-seed 接 admission
- C3 reseed 接 admission

优先级：

- `P0`

工作量：

- `L`

风险等级：

- `中`

是否需要灰度：

- `是`

是否需要看板先行：

- `是`

目标：

- 尽快堵住边缘区继续放大风险的问题
- 这是最早能产生实质风控收益的一波

#### Wave 3：统一 pause / recovery 与风险状态语义

包含：

- D1 收口 breakout 动作权
- D2 统一 pause / recovery policy
- D3 收口位置 penalty

优先级：

- `P1`

工作量：

- `L`

风险等级：

- `中到高`

是否需要灰度：

- `是`

是否需要看板先行：

- `是`

目标：

- 解决“多个模块都能 pause/recover”的问题
- 为 orchestrator 做状态统一准备

#### Wave 4：统一执行编排

包含：

- E1 decision freshness evaluator
- E2 policy compilation stage
- E3 execution orchestrator
- E4 post-sync 自动动作收口

优先级：

- `P1`

工作量：

- `XL`

风险等级：

- `高`

是否需要灰度：

- `强制需要`

是否需要看板先行：

- `强制需要`

目标：

- 真正解决多个执行源并行的问题
- 真正解决 AI 判断时间与业务执行时间冲突

#### Wave 5：AI 职责收敛

包含：

- F1 AI advisory 化
- F2 提示词与产品口径同步

优先级：

- `P2`

工作量：

- `M`

风险等级：

- `中`

是否需要灰度：

- `是`

是否需要看板先行：

- `建议需要`

目标：

- 降低 AI 直接驱动执行的时点风险
- 让系统职责更加稳定

---

### 29.4 周期建议

如果按正常节奏推进，建议粗略分为 5 个阶段周期。

#### 第 1 周期

目标：

- 完成 Wave 1

交付：

- 新状态字段
- 新审计字段
- 看板展示基础字段

上线策略：

- 全量可开，因为主要是观测增强

#### 第 2-3 周期

目标：

- 完成 Wave 2

交付：

- 边际区准入
- 补仓风险预测
- AI / auto-seed / reseed 接 admission

上线策略：

- 先按交易员灰度
- 先观察“挂单数下降”和“补单频率下降”是否符合预期

#### 第 4-5 周期

目标：

- 完成 Wave 3

交付：

- 统一 pause mode
- 统一 recovery policy
- breakout 动作权收口

上线策略：

- 必须灰度
- 必须观察恢复路径是否异常变慢或变频繁

#### 第 6-8 周期

目标：

- 完成 Wave 4

交付：

- freshness evaluator
- compiled policy
- execution orchestrator
- post-sync 动作收口

上线策略：

- 小流量灰度
- 最好按交易员 / 交易所 / 策略分组逐步开启

#### 第 9 周期以后

目标：

- 完成 Wave 5

交付：

- AI advisory 化
- 提示词与看板口径统一

上线策略：

- 在 orchestrator 稳定后再推进

---

### 29.5 每波次的看板先行要求

#### Wave 1 看板要求

- 主状态
- pause mode
- current risk state
- context version

#### Wave 2 看板要求

- long/short risk zone
- allow_new / allow_add
- max_add_notional
- policy_reason

#### Wave 3 看板要求

- recovery mode
- resume policy
- pause source

#### Wave 4 看板要求

- decision stale
- stale reason
- policy overrides
- post-sync actions

#### Wave 5 看板要求

- advisory summary
- compiled policy summary
- actual execution summary

原则：

- 没有对应可观测字段，不建议上线该波次的行为改造

---

### 29.6 灰度策略建议

建议按以下顺序灰度：

1. 仅观测，不改行为
2. 先灰度 admission 到 auto-seed
3. 再灰度 admission 到 AI entry
4. 再灰度 pause / recovery 统一
5. 最后灰度 orchestrator

原因：

- auto-seed 是相对更可控的入口
- AI entry 比 auto-seed 更容易影响挂单结构
- pause / recovery 影响整个状态流转
- orchestrator 会影响所有执行源

建议灰度粒度：

- per trader
- per strategy
- per exchange

并保留 feature flag：

- `enable_context_snapshot_v2`
- `enable_entry_admission`
- `enable_pause_mode_unification`
- `enable_execution_orchestrator`
- `enable_ai_advisory_mode`

---

### 29.7 风险最高的三个实施点

#### 风险点一：统一 pause / recovery

原因：

- 它会影响现有恢复路径
- 最容易造成“该恢复的不恢复，或不该恢复的恢复”

建议：

- 独立灰度
- 上线前准备回滚开关

#### 风险点二：post-sync 动作收口

原因：

- 当前很多稳定性靠 post-sync 兜底
- 一旦收口不完整，容易出现 exit 缺失或 entry 漏挂

建议：

- 先只做审计
- 再逐步把 candidate actions 纳入 orchestrator

#### 风险点三：AI advisory 化

原因：

- 这是用户心智和系统职责的变化
- 如果前面的 orchestrator 没稳定，AI advisory 化会放大不确定性

建议：

- 最后做
- 必须等 orchestrator 已经稳定运行

---

### 29.8 最值得优先做的两件事

如果资源有限，我建议优先做：

1. `Wave 1`
   因为先把状态和审计补齐，后面所有改造才有观察基础

2. `Wave 2`
   因为边际区准入能最直接减少风险放大

这是最具性价比的前两步。

---

### 29.9 排期版最终建议

如果把整个项目压缩成一句话：

- 第一阶段先让系统可解释
- 第二阶段先堵住风险继续放大的入口
- 第三阶段再统一暂停与恢复
- 第四阶段再统一执行编排
- 最后再让 AI 退居 advisory 角色

这个顺序最稳，也最符合当前系统现状。

---

## 三十、过去 24 小时回放对比与首仓保护带融合设计

### 30.1 目的

这一章固化两件事：

1. 用真实过去 24 小时交易结果，回放新方案的大致效果
2. 将“固定 22% + 动态 10xATR”的首仓保护带融合进现有 admission / stop-loss / AI orchestration 设计，避免它成为一条孤立规则

说明：

- 这里的回放是基于真实成交、真实挂单、真实日志做规则重放
- 不是完整逐 K 线回测引擎
- 适合回答“新规则大概率会不会挡住这次亏损、会不会伤害已有盈利”这类问题

### 30.2 样本与口径

本次回放样本使用交易员：

- `3885bf4e_32162195-6112-4e49-aba3-cd5a2baf7b4e_deepseek_1775547230`

主要数据源：

- `trader_positions`
- `grid_inventory_lots`
- `trader_orders`
- `grid_risk_events`
- `nofx_2026-04-29.log`

两个必须区分的边界口径：

1. 白天主要运行边界

- `grid_lower_price = 0.092639`
- `grid_upper_price = 0.10197`

2. 止损发生时有效边界

- `grid_lower_price = 0.09356`
- `grid_upper_price = 0.112`
- `grid_spacing = 0.0009705263`

注意：

- 这次“边际比例首仓测试”是按用户要求，使用白天主要运行边界来做
- 这次“为什么仍然发生止损”的解释，则必须承认止损时真实生效边界已经扩大到 `0.112`

### 30.3 真实过去 24 小时结果

样本窗口内，真实实盘主要结果如下。

已实现盈利空单：

- `SHORT 63432 @ 0.09972629 -> 0.09923629`
- `SHORT 10868 @ 0.10000000 -> 0.09951000`

已实现结果：

- 毛收益约 `+36.41 USDT`
- 手续费约 `2.96 USDT`
- 已实现净收益约 `+33.45 USDT`

止损主仓：

- 主仓均价约 `0.10015943`
- 主仓名义数量 `70442`
- 真实已实现亏损约 `-608.60 USDT`

这组结果说明：

- 过去 24 小时不是“全部都坏”
- 它是“前面有正常盈利，后面一组空头在单边突破中被较晚地分段止损”

### 30.4 使用新止损模型回放的核心结论

如果只引入：

- 边际区准入
- 更早的 `soft_reduce / hard_reduce`

而不改变样本里的原始安全区首仓，那么结论是：

1. 原先盈利的单，大概率仍然继续盈利
2. 原先止损的主仓，大概率仍然会亏损
3. 但亏损规模预计明显缩小

原因：

- 这次主亏仓的几笔核心 entry 并不是在真正的边际坏位置开出的
- 它们更接近“安全区正常开仓后，遭遇后续单边突破”
- 所以边际区准入无法直接抹掉这组仓位
- 真正改善结果的是更早启动分层减仓

基于样本重放，若更早进入强减仓，已实现亏损可从：

- 真实约 `-608.60 USDT`

改善到大致：

- 回放约 `-332.60 USDT`

改善幅度大约：

- `+276.00 USDT`

这说明：

- 新风控体系的第一收益，不是“所有亏损都不再发生”
- 而是“同样判断错误时，亏得更轻”

### 30.5 10% 首仓边际距离测试结论

按用户要求，使用白天主要运行边界：

- `upper = 0.10197`
- `lower = 0.092639`

定义：

- 上边界附近顶部 `X%` 区域内，不允许开 `SHORT` 首仓
- 下边界附近底部 `X%` 区域内，不允许开 `LONG` 首仓

当 `X = 10%` 时：

- 上边界危险区起点约 `0.101037`

而这次关键几笔空头首仓价格大致在：

- `0.09972629`
- `0.10000000`
- `0.10049335`

因此：

- `10%` 完全挡不住这次主亏链

### 30.6 边际比例扫描结论

按白天边界继续放大比例，扫描结果如下：

| 边界比例 | 上边界危险区起点 | 是否挡住主亏首仓 | 是否挡住较小盈利空单 | 是否挡住较大盈利空单 |
|----------|------------------|------------------|----------------------|----------------------|
| 10% | 0.101037 | 否 | 否 | 否 |
| 15% | 0.100570 | 否 | 否 | 否 |
| 20% | 0.100104 | 否 | 否 | 否 |
| 21% | 0.100010 | 否 | 否 | 否 |
| 22% | 0.099917 | 是 | 是 | 否 |
| 24% | 0.099731 | 是 | 是 | 否 |
| 25% | 0.099637 | 是 | 是 | 是 |

关键临界值：

- 挡住 `0.10049335`：至少约 `15.83%`
- 挡住 `0.10000000`：至少约 `21.11%`
- 挡住 `0.09972629`：至少约 `24.05%`

因此可得：

1. `10%` 不够
2. `20%` 仍然不够
3. `22%` 是第一档真正能挡住这次主亏首仓的有效值
4. `25%` 已经明显过于激进，会开始伤害原本较大的盈利单

### 30.7 为什么选择 22% 作为固定锚点标准

结合这次样本，`22%` 有三个特点：

1. 它是第一档明确有效的值

- 可以挡住这次 `0.10000000` 一带的主亏首仓

2. 它保留了更大一笔已证实盈利的安全区空单

- `0.09972629` 这笔仍可保留

3. 它只牺牲了一笔较小盈利空单

- `0.10000000 -> 0.09951000` 这笔约 `+5.33 USDT`

从样本性价比看，`22%` 比 `25%` 更适合作为首仓保护标准。

### 30.8 首仓保护带的正确定位

这里必须明确：

- `22%` 不是新的统一边际区定义
- `22%` 也不应再单独作为最终唯一门槛
- 最终应使用“固定带 + 动态带”的融合保护带

也就是说，以后系统不应只有一个 `edge_zone` 概念，而应拆成两层：

1. 一般边际区

- 继续沿用原设计
- 例如 `2 * grid_spacing` 或其他动态带宽
- 用于控制补仓、金额 cap、warning 阶段保守化

2. 首仓保护区

- 使用“固定比例带宽”和“ATR 动态带宽”中的更严格者
- 对危险方向首仓直接禁入
- 当前建议默认值：
  - 固定带：`22%`
  - 动态带：`10 x ATR14`

这样设计的原因是：

- 补仓、止损、AI 判断更适合用动态结构带宽
- 首仓准入更适合用简单、稳定、可解释的硬边界

### 30.8.1 首仓保护带的精确定义

这里把首仓保护带的数学语义固定为：

```text
range = effective_grid_upper_price - effective_grid_lower_price
fixed_band = range * 0.22
atr_band = 10 * ATR14
guard_band = max(fixed_band, atr_band)
```

对 `SHORT` 首仓保护区：

```text
short_first_entry_guard_price
= effective_grid_upper_price - guard_band
```

当：

```text
current_price >= short_first_entry_guard_price
and current_price <= effective_grid_upper_price
and current_short_position == 0
```

则禁止 `SHORT new_entry`。

对 `LONG` 首仓保护区：

```text
long_first_entry_guard_price
= effective_grid_lower_price + guard_band
```

当：

```text
current_price <= long_first_entry_guard_price
and current_price >= effective_grid_lower_price
and current_long_position == 0
```

则禁止 `LONG new_entry`。

解释：

- 固定带部分：`22%` 指的是整个上下边界区间宽度的 `22%`
- 动态带部分：`10xATR14` 指的是用当时真实波动去衡量保护带宽度
- 最终实际生效的是两者的 `max`
- 不是把两者相加
- 这样做的目标是既保留稳定性，又保留波动自适应能力

### 30.8.2 首仓保护带的运行基准

首仓保护带必须使用“当前有效 grid 边界快照 + 当前有效 ATR14 快照”来计算。

运行规则：

1. 在 cycle 开始或 context 构建前，冻结一份：

- `effective_grid_upper_price`
- `effective_grid_lower_price`
- `effective_grid_spacing`
- `effective_atr14`
- `context_version`

2. 当次 cycle 内：

- AI 解释
- entry admission
- auto-seed
- adjust 后 entry 重挂
- flat 后 reseed

都必须使用这同一份快照。

3. 只有下一个 cycle 或显式重建快照后，首仓保护带才允许更新。

这样可以避免：

- 首仓门槛用旧边界
- reseed 用新边界
- 同一轮内出现自相矛盾的准入结果

### 30.9 与现有 admission 设计的融合方式

建议把 admission 的区域体系改成三层，而不是一层：

1. `safe_zone`

- 允许正常首仓
- 允许正常补仓

2. `first_entry_guard_zone`

- 禁止危险方向首仓
- 允许已有仓位在满足条件时小额补仓

3. `edge / breakout / trend`

- 继续承接原来的边际区、突破区、趋势区逻辑

也就是说，执行顺序改成：

1. 先判断 `trend`
2. 再判断 `breakout`
3. 再判断是否落入 `first_entry_guard_zone`
4. 再判断 `edge`
5. 最后才是 `safe`

执行规则改成：

1. 如果是 `trend / breakout`

- 无论是否已有持仓，都禁止同方向新增风险

2. 如果是 `first_entry_guard_zone`

- 若该方向当前无持仓，则拒绝 `new_entry`
- 若已有持仓，则继续走后续 `edge / safe` 的补仓规则

3. 如果不是 `first_entry_guard_zone`

- 再进入原有的 `edge / breakout / trend` admission 规则

这个顺序很重要。  
首仓保护带是“前置首仓门槛”，不是对原有 edge-zone 的替代。

### 30.10 推荐的方向化规则

对 `SHORT`：

- 当价格进入 `short_first_entry_guard_price ~ upper` 区域时
- 若当前没有 `SHORT` 持仓
- 禁止开新的 `SHORT` 首仓

对 `LONG`：

- 当价格进入 `lower ~ long_first_entry_guard_price` 区域时
- 若当前没有 `LONG` 持仓
- 禁止开新的 `LONG` 首仓

注意：

- 这里只约束危险方向
- 另一侧的减仓或对冲平仓不受此规则阻断

### 30.11 与补仓逻辑如何融合

首仓保护带不应直接扩展成“保护带内禁止补仓”，否则会过度保守。

建议：

1. 首仓保护带只用于 `new_entry`
2. `add_entry` 继续由原有 `edge / breakout / trend` 规则控制
3. 如果已经进入 `warning`

- 只允许在 `safe_zone` 做一次 `limited_add`
- `edge / breakout / trend` 下的同方向补仓全部禁止

4. 如果进入 `soft_reduce` 及以上

- 全面禁止补仓

这能避免两类极端：

- 过宽：首仓规则一刀切扩展到所有动作
- 过松：首仓挡住了，但补仓仍在危险区无限继续

### 30.12 与金额 cap 逻辑如何融合

融合顺序建议固定为：

1. 先过首仓保护带门槛
2. 再过 `edge / breakout / trend` 准入规则
3. 最后再应用 `uniform x 1.5 / 1.2` 的金额 cap

原因：

- 如果首仓本来就不该开，金额 cap 没有意义
- 金额上限只是风险压缩，不是风险准入

因此：

- 首仓保护带的优先级高于 `1.5x / 1.2x` cap
- 首仓保护带本身不参与金额缩放，只参与准入判定

### 30.13 与 AI 判断的融合方式

AI 侧不要自己重复计算首仓保护带。  
建议：

1. 在 context 中明确提供字段

- `in_short_first_entry_guard_zone`
- `in_long_first_entry_guard_zone`
- `first_entry_guard_threshold_pct`
- `first_entry_guard_atr_multiplier`
- `short_first_entry_guard_price`
- `long_first_entry_guard_price`
- `effective_grid_upper_price`
- `effective_grid_lower_price`
- `effective_atr14`
- `context_version`

2. AI 可以据此解释当前结构

- 比如说明“价格已进入空头首仓保护区，不建议首次做空”

3. 但最终执行仍由 admission gate 决定

这样做的好处是：

- AI 与执行层看到同一套结构语义
- 但不会把硬准入逻辑分散到 prompt reasoning 里
- 同时也把边界基准和版本固定下来，避免 AI 与执行层使用不同门槛

### 30.14 与止损模型的关系

首仓保护带不是止损模型的一部分，它位于止损之前。

职责划分应是：

1. 首仓门槛

- 尽量减少危险首仓出现

2. 边际区准入

- 控制已有风险是否允许扩大

3. 分层止损

- 当风险已经形成后，负责尽早减损

这样三层关系才清晰：

- 首仓保护带负责“少犯错”
- `edge admission` 负责“别把错放大”
- `soft/hard/emergency` 负责“错了以后亏轻一点”

### 30.14.1 首仓保护带必须覆盖的入口

为了避免规则只拦住一部分动作，以下链路都必须接入同一首仓门槛：

1. AI 直接提议的 entry 动作
2. `autoSeedMissingEntryOrdersForRangingGrid()` 生成的 candidate entry
3. `adjust_grid` 后的 entry 重挂
4. `flat exit` 后的 `full reseed`
5. 任何人工或系统触发的 entry 重建逻辑

如果只接一部分入口，行为会变成：

- AI 不能开
- 但 auto-seed 还能补
- 或 reseed 还能重新挂

这会直接破坏规则一致性。

### 30.15 对现有方案的最终修订建议

将原文中的“边际区禁止首仓，补仓限额限次；突破区禁止任何同方向加仓，只允许减仓；趋势区由分层止损状态机接管。”

修订为：

- 首仓保护区：危险方向首仓直接禁入，保护带使用 `max(22% * range, 10 * ATR14)`
- 一般边际区：禁止首仓，补仓限额限次
- 突破区：禁止任何同方向加仓，只允许减仓
- 趋势区：交由分层止损状态机接管

这个版本更符合实盘观察，也更便于工程落地。

### 30.16 当前推荐口径

当前推荐的融合口径如下：

1. 固定带使用 `22% * range`
2. 动态带使用 `10 * ATR14`
3. 最终保护带取两者 `max`
4. 不替代原有 `edge / breakout / trend` 设计
5. 首仓门槛优先级高于金额 cap
6. AI 只读这个结构，不直接执行这个规则
7. 止损模型继续承担后续风险收缩职责

一句话总结：

- 这不是新的全部边际逻辑
- 它是加在现有体系最前面的一道“首仓硬门槛”
- 形式上是 `max(固定带, 动态带)`
- 用来挡最容易形成主亏链的那类首仓

### 30.17 过去 7 天回放结果

在这名交易员过去 7 天真实数据上，使用如下规则回放：

```text
guard_band = max(0.22 * (upper - lower), 10 * ATR14)
SHORT 首仓禁止线 = upper - guard_band
LONG  首仓禁止线 = lower + guard_band
```

回放口径：

- 使用真实 `trader_positions`
- 使用真实 `decision_records` 中当时的 `ATR14` 与网格边界
- 按“首仓 campaign”重放
- 如果某轮首仓会被挡掉，则该轮同方向仓位链视为不存在

用途限定：

- 这一节只用于验证首仓保护带的方向性有效性
- 只说明“过去样本中，这条规则大概率会挡掉什么、保留什么”
- 不作为未来真实落地后的收益承诺
- 不作为策略收益预测
- 不作为产品对外宣称的业绩依据

原因：

- 这是基于真实成交的规则重放，不是完整逐 K 线逐订单撮合回测
- 它适合做准入策略比较，不适合做未来收益外推

回放结果：

- 过去 7 天真实净收益约：`590.64 USDT`
- 回放后净收益约：`1199.28 USDT`
- 净改善约：`+608.64 USDT`

盈利损失：

- 被挡掉的盈利 campaign 只有 1 轮
- 净收益损失约：`4.89 USDT`
- 占过去 7 天全部盈利 campaign 收益约：`0.39%`

亏损减少：

- 被挡掉的主亏 campaign 约：`-613.53 USDT`

因此，这 7 天样本里：

1. 盈利会下降，但下降很小
2. 主要收益来自挡掉一轮大亏损首仓链
3. 净效果明显为正

样本内被挡掉的两轮关键 campaign：

1. `2026-04-29 08:56`

- `SHORT first entry @ 0.10000000`
- 实际净收益约：`+4.89 USDT`

2. `2026-04-29 09:50`

- `SHORT first entry @ 0.10015943`
- 实际净结果约：`-613.53 USDT`

另外，这 7 天样本里主导保护带的主要还是固定带 `22%`，不是 `10xATR`：

- 被挡掉的两轮 campaign 中，最终起作用的都是固定带
- 说明在当前 DOGE 样本下，`10xATR` 更像动态辅助项
- 固定带仍然是主裁决项

解释：

- 这不代表 `10xATR` 没价值
- 它的作用是为未来更高或更低波动 regime 提供动态修正
- 但在当前 7 天样本里，`22%` 本身已经足够严格

---

## 三十一、按专业量化实施标准的最终修正

这一章用于处理前文剩余的实施级风险，把方案从“设计可行”进一步收敛到“更适合真实量化生产环境落地”。

目标：

- 降低样本过拟合
- 降低工程集成歧义
- 降低灰度上线风险
- 提高策略可解释性与可回滚性

### 31.1 首仓保护带参数不再作为不可变常数，而作为默认锚点参数

前文的固定带 `22%` 与动态带 `10xATR14` 来自真实样本验证，足以作为第一版默认值，但不应被定义成永远固定不可变。

建议修正为：

- `first_entry_guard_threshold_pct_default = 0.22`
- `first_entry_guard_threshold_pct_min = 0.18`
- `first_entry_guard_threshold_pct_max = 0.26`
- `first_entry_guard_atr_multiplier_default = 10.0`
- `first_entry_guard_atr_multiplier_min = 8.0`
- `first_entry_guard_atr_multiplier_max = 12.0`

执行原则：

1. 第一版生产默认值使用 `0.22`
2. 不允许策略在运行中无约束自适应漂移
3. 只允许通过：
   - 策略配置
   - 灰度实验
   - 离线回放结果
   调整该参数

原因：

- 对量化系统来说，“先有稳定默认值，再做可控校准”优于“直接写死永不调整”
- 这样既保留了这次样本的经验，也避免未来对其他品种过拟合

### 31.2 首仓保护带参数的校准方法

建议把以下两个参数都纳入标准离线回放校准，而不是人工拍脑袋：

- `first_entry_guard_threshold_pct`
- `first_entry_guard_atr_multiplier`

建议目标函数：

```text
score =
  w1 * risk_reduction
+ w2 * realized_pnl_retention
+ w3 * entry_coverage_retention
- w4 * false_block_rate
```

其中：

- `risk_reduction`：亏损链被阻断或减轻的程度
- `realized_pnl_retention`：原本盈利样本被保留的程度
- `entry_coverage_retention`：正常首仓覆盖率
- `false_block_rate`：被错误阻挡的优质首仓比例

建议：

- 先用 `0.18 / 0.20 / 0.22 / 0.24 / 0.26` 做离线扫描
- 再用 `8 / 9 / 10 / 11 / 12 x ATR14` 做离线扫描
- 按 symbol 分类评估
- 若跨品种差异较大，再考虑按波动率分层设参数

第一阶段不建议：

- 每个周期动态调 `22%`
- 每个周期动态调 `10xATR14` 倍数
- 用在线 AI 输出去即时修改这两个阈值

### 31.3 运行态中的参数分层

为避免逻辑混乱，建议把参数分成三类：

1. 结构参数

- `first_entry_guard_threshold_pct`
- `first_entry_guard_atr_multiplier`
- `edge_band_multiplier`
- `breakout_band_multiplier`
- `trend_confirm_cycles`

2. 风险参数

- `warning_loss_pct`
- `soft_reduce_loss_pct`
- `hard_reduce_loss_pct`
- `emergency_unrealized_loss_eq_pct`

3. 执行参数

- `limited_add_notional_multiplier`
- `max_edge_add_count_per_side`
- `min_cycles_between_edge_add`
- `reduce_timeout_cycles`

原则：

- 结构参数不要在盘中高频变
- 风险参数只允许版本化更新
- 执行参数允许有限灰度

### 31.4 Warning 阶段补仓的最终统一规则

为彻底消除歧义，`warning` 下的补仓逻辑最终统一为：

1. 默认：`warning` 不允许任何同方向补仓
2. 若业务明确要求保留缓冲口，则只能启用 `warning_safe_limited_add`
3. `warning_safe_limited_add` 的定义：
   - 仅限 `safe_zone`
   - 每个方向每次 `warning` 状态生命周期最多一次
   - 金额 `<= normal_grid_order_notional * 0.2`
   - 必须通过 projected risk check

默认推荐：

- 第一版生产关闭 `warning_safe_limited_add`
- 在 `Wave 1-3` 强制关闭，不参与灰度
- 只有在二阶段离线回放和 shadow 统计都验证通过后，才允许单独实验开启

原因：

- 从专业风险控制角度，`warning` 最重要的是停止扩大错误，而不是给系统继续找更好均价的机会

### 31.5 “一次 limited_add”的重置条件

前文“一次补仓”的语义，这里明确写死：

一个方向的 `limited_add_budget` 只有在满足以下全部条件时才能重置：

1. 该方向风险状态回到 `normal`
2. 该方向当前风险区回到 `safe_zone`
3. 连续 `N=3` 个周期保持 `safe_zone`
4. 当前该方向没有未完成的 risk-reduce action

如果只满足部分条件，不重置。

说明：

- 重置按“方向 + 风险状态生命周期”管理
- 不是按每个 cycle 自动清零

### 31.6 effective grid boundary snapshot 的最终规范

这里对快照时点做最终规范。

每个 cycle 应分两份快照：

1. `cycle_open_snapshot`

- 在本轮决策开始前生成
- 包含：
  - `effective_grid_upper_price`
  - `effective_grid_lower_price`
  - `effective_grid_spacing`
  - `current_price`
  - `positions_hash`
  - `open_orders_hash`
  - `risk_state_hash`

2. `execution_snapshot`

- 在真正落地动作前重新拉一遍
- 用于 stale check 和 pre-execution gate

执行规则：

- AI advisory 基于 `cycle_open_snapshot`
- 真正执行必须再过 `execution_snapshot`
- 若两者差异超过阈值，则：
  - entry 类动作失效
  - reduce-only 类动作仍允许继续

这样是更标准的量化落地方式：

- 允许观察与执行分离
- 但不允许陈旧快照继续开风险仓

### 31.7 stale check 的明确触发条件

建议新增 `decision_stale` 规则：

当满足任一条件时，本轮 AI entry/admission 建议视为过期：

- `abs(current_price - snapshot_price) >= 0.5 * effective_grid_spacing`
- `positions_hash` 变化
- `open_orders_hash` 变化
- `risk_state_hash` 变化
- `pause_mode` 变化

过期后处理规则：

- `new_entry` 直接废弃
- `add_entry` 直接废弃
- `reduce_only` 可继续执行
- `pause / risk reduce` 可继续执行

### 31.8 统一准入入口的最终要求

所有会产生 entry 的链路，必须只经过一个函数：

```text
evaluate_entry_admission(intent, side, snapshot, policy_state) -> decision
```

这里的 `intent` 至少包括：

- `ai_new_entry`
- `ai_add_entry`
- `auto_seed_entry`
- `adjust_reseed_entry`
- `flat_reseed_entry`
- `manual_entry_rebuild`

返回值至少包括：

- `allowed`
- `reason_code`
- `zone`
- `risk_state`
- `max_notional`
- `stale`

量化系统里，真正降低风险的关键不是规则写得多，而是：

- 所有入口最终只认一个裁决函数

### 31.9 不同动作的优先级最终排序

建议最终动作优先级固定如下：

1. `emergency_exit`
2. `hard_reduce`
3. `soft_reduce`
4. `pause_transition`
5. `reduce_only_maintenance`
6. `entry_cancel`
7. `entry_place`

解释：

- 风险降低动作永远优先于首仓保护或补仓逻辑
- `22%` 只决定 `entry_place`
- 它不能阻塞已经触发的减仓或退出动作

### 31.10 首仓保护带与止损门槛的边界

这里再次明确边界：

- `max(22% * range, 10 * ATR14)` first_entry_guard 是 pre-trade filter
- `edge/breakout/trend admission` 是 risk expansion filter
- `warning/soft/hard/emergency` 是 post-trade risk control

禁止把这三层混用成一套条件。

例如：

- 不要因为触发 `first_entry_guard` 就进入 `warning`
- 不要因为进入 `warning` 就反向修改固定带或 ATR 倍数
- 不要因为 `22%` 阻挡了首仓，就跳过 `breakout` 结构判定

### 31.11 首仓保护带的跨品种适配建议

若未来扩展到更多 symbol，建议优先按这三类分桶校准：

1. 低波动大盘类
2. 中波动主流山寨类
3. 高波动小市值类

初始建议：

- 低波动：`0.20 ~ 0.22`
- 中波动：`0.22 ~ 0.24`
- 高波动：`0.24 ~ 0.26`

ATR 倍数初始建议：

- 低波动：`8x ~ 10x ATR14`
- 中波动：`10x ~ 11x ATR14`
- 高波动：`11x ~ 12x ATR14`

但这只是二阶段优化建议。第一阶段仍建议：

- 所有 symbol 统一默认 `0.22`
- 先收集样本，再拆桶

### 31.12 上线前的必备看板字段

如果没有下面这些看板字段，不建议上线相关行为：

- `effective_grid_upper_price`
- `effective_grid_lower_price`
- `first_entry_guard_threshold_pct`
- `first_entry_guard_atr_multiplier`
- `first_entry_guard_fixed_band`
- `first_entry_guard_atr_band`
- `first_entry_guard_active_source`
- `short_first_entry_guard_price`
- `long_first_entry_guard_price`
- `short_entry_zone`
- `long_entry_zone`
- `entry_admission_reason_code`
- `decision_stale`
- `stale_reason`
- `admission_source`

这样才能在盘中回答：

- 这笔首仓为什么被拒
- 是被 `22%` 拦住，还是被 breakout 拦住
- 是固定带生效，还是 ATR 带生效
- 是 AI 被拒，还是 auto-seed 被拒

### 31.13 上线方式的专业修正

从量化生产标准看，正确顺序不是“文档定了就全开”，而是：

1. 先上看板与审计，不改行为
2. 再上 `admission shadow mode`
3. 再只灰度 `ai_new_entry + auto_seed_entry`
4. 再灰度 `adjust_reseed_entry + flat_reseed_entry`
5. 最后再接统一 orchestrator

其中 `shadow mode` 指：

- 真实不拦单
- 但完整记录如果新规则生效，会拦哪笔、原因是什么

这是专业量化系统很常见的落地方式，能先验证逻辑，再改变行为。

### 31.14 回滚与保险丝

建议所有关键能力都具备独立 feature flag：

- `enable_first_entry_guard`
- `enable_first_entry_guard_shadow`
- `enable_entry_admission_v2`
- `enable_warning_safe_limited_add`
- `enable_execution_snapshot_gate`
- `enable_execution_orchestrator`

并且必须支持：

- per trader
- per symbol
- per strategy

如果上线后出现异常，应能做到：

- 只关闭 `first_entry_guard`
- 不影响已有 stop-loss
- 不影响 reduce-only 维护

### 31.15 最终专业版结论

把前面的风险修正后，最终建议是：

1. `22%` 保留，但作为可校准默认值，不作为永恒硬编码
2. `warning` 默认禁止同向补仓
3. `limited_add` 的重置条件必须状态化，不按周期自动清零
4. admission 必须唯一入口化
5. AI 只能给 advisory，entry 最终由 execution snapshot 下的 admission gate 决定
6. 上线前先 shadow，再灰度，再统一 orchestrator

一句话总结：

- 这套方案现在已经可以按成熟量化系统的方式落地
- 但前提不是“直接全开”
- 而是“参数可校准、执行可审计、入口可统一、异常可回滚”
