# Grid Maintenance Rules

日期: 2026-04-13
适用范围: AlphaFlowX / NOFX 智能网格策略维护、排障、迭代

## 1. 总原则

- 不走官方镜像，只使用本地代码和本地镜像。
- 不做半截修复，必须到“已部署、已生效、已验证”。
- 不用前端现象直接下结论，先找执行层真相源。
- 任何结论必须区分三层：
  - AI 决策层
  - 后端执行层
  - 同步 / 展示层

## 2. 真相源优先级

### 交易是否真实发生

按以下优先级判断：
1. 交易所 live open orders / positions
2. `trader_fills`
3. `trader_orders`
4. 前端页面显示

### 周期是否真实运行

按以下优先级判断：
1. `decision_records`
2. 运行日志
3. 前端 dashboard

### 网格逐笔业务逻辑

按以下优先级判断：
1. `grid_inventory_lots`
2. `trader_fills`
3. `trader_orders`
4. `trader_positions`

说明:
- `trader_positions` 是聚合仓位表，不适合解释逐笔网格成交和平仓。

## 3. 网格策略判断规则

### 震荡逻辑

- 震荡箱体内应优先维持完整开仓网格。
- 若缺失开仓挂单，应优先补单，而不是直接 `hold`。
- 若当前价格上下 `1 x grid spacing` 内已各有一张开多和开空，可抑制中心重复补单。

### 趋势逻辑

- 趋势市场下必须取消全部开仓挂单。
- 趋势市场下应保留平仓挂单。
- 趋势暂停不应再继续补新的开仓单。

### 恢复逻辑

- 暂停后恢复不能过快。
- 恢复至少要满足：
  - 冷却时间
  - 连续确认次数
  - 回到中期箱体内
  - 指标阈值回归可接受范围

### 平仓逻辑

- 常规网格止盈应优先使用相邻层被动限价平仓。
- 不应轻易走 `MARKET close_long / close_short`。
- 若相邻层价格过近，小于最小允许间隔，则按最小允许间隔重算平仓价。

## 4. 执行层规则

- 取消挂单时，以交易所实时 `GetOpenOrders` 为真相源。
- 取消开仓挂单时：
  - 取消 `reduceOnly=false`
  - 保留 `reduceOnly=true`
- 平仓单不能按总仓位盲挂，必须先扣除已挂 reduce-only 数量。
- 平仓锚点必须优先使用每笔 lot 的 `source_level_index`，不得优先用“当前最近层”替代。
- 同价位允许：
  - 一张开仓单
  - 一张来自相邻层的平仓单
  并存。
- pause 分支不能 silent return，必须继续写周期记录。

## 5. 固定排障顺序

1. 先判断是否是显示问题
   - 前端
   - 接口返回
   - 数据库
   - 交易所 live 状态
2. 再判断是否是周期问题
   - `decision_records` 是否继续写
   - 日志是否继续跑 cycle
   - 是否进入 pause / resume 分支
3. 再判断是否是执行问题
   - AI 给了什么动作
   - 执行层是否真正执行
   - 是否被保护逻辑拦截
   - 是否被交易所拒单
4. 最后才判断是否是策略设计问题
   - 箱体是否合理
   - spacing 是否合理
   - 恢复门槛是否合理
   - 平仓锚点是否合理

## 6. 固定核查字段

每次查 grid trader，固定优先检查：

- `decision_records`
  - 最新周期
  - action
  - reasoning
- `trader_orders`
  - `NEW/PARTIALLY_FILLED`
  - `type`
  - `reduce_only`
  - `order_action`
- `trader_fills`
  - `is_maker`
  - `price`
  - `quantity`
- `grid_inventory_lots`
  - `source_level_index`
  - `exit_level_index`
  - `remaining_qty`
  - `status`
- 交易所 live
  - open orders
  - positions

## 7. 高优先级风险信号

出现以下任一情况，直接判为高优先级异常：

- `pause_grid` 后交易所后台仍有 `reduceOnly=false` 挂单
- `decision_records` 停住，但 trader 进程还在运行
- 平仓价格只比开仓价格多/少一个 tick
- 出现 `MARKET close_short / close_long`，但并非明确风控强平
- DB 显示 0 挂单，但交易所后台仍有挂单
- 恢复后第一轮一次性补大量 entry orders
- lot 没有稳定 `source_level_index / exit_level_index`

## 8. 后续设计原则

- AI 决策层负责方向和意图。
- 执行层负责真实交易约束和纠偏。
- 同步层负责真相还原。
- 展示层只展示，不反向定义业务逻辑。

补充:
- AI 不应决定过多执行细节。
- 执行层必须兜底纠偏。
- 展示层不能反向当作真相源。

## 9. 已确认存在过的漏洞

1. 趋势决策被震荡保护错误拦截。
2. pause 后不再写新周期记录。
3. 趋势暂停时漏删开仓挂单。
4. 常规网格场景错误走 `MARKET close_short / close_long`。
5. 平仓价只差一个 tick，不符合相邻层业务逻辑。
6. DB 订单镜像与交易所后台真实挂单不一致。
7. 聚合仓位表误导逐笔网格业务判断。

## 10. 当前仍需持续盯防的风险

1. 恢复阶段是否仍会一次性补太多开仓单。
2. lot 的 `source_level_index / exit_level_index` 是否始终可靠。
3. 相邻层平仓是否已完全摆脱聚合 / 跳层影响。
4. live open orders 是否已完全成为执行真相源。
5. AI 提示词变动是否会再次影响执行放行 / 阻断条件。

## 11. 最终判断

- 如果目标是严格贴业务逻辑，当前本地增强版优于官方原始逻辑。
- 如果目标是实现简单、维护轻、少故障，官方原始逻辑更稳。
- 在当前业务目标下，正确方向不是回滚到官方，而是继续把以下三条收口：
  - lot 真相源
  - live open orders 真相源
  - 恢复节奏控制
