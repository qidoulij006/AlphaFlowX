# Grid Monitor Report

- Trader: qidou.万U实盘.DeepSeekV3.2
- Trader ID: 3885bf4e_32162195-6112-4e49-aba3-cd5a2baf7b4e_deepseek_1775547230
- Cycle window: 2252 -> 2271
- Generated at: 2026-04-12 08:21:38 +0800
- Successful cycles: 16
- Failed cycles: 4
- Average AI request duration: 8169.35 ms

## Cycle Details
- Cycle 2252 at 2026-04-11 23:26:35.93866601+00:00: success
- Cycle 2253 at 2026-04-11 23:29:35.996615258+00:00: success
- Cycle 2254 at 2026-04-11 23:32:37.656440347+00:00: success
- Cycle 2255 at 2026-04-11 23:35:35.139503986+00:00: success
- Cycle 2256 at 2026-04-11 23:38:35.966040536+00:00: success
- Cycle 2257 at 2026-04-11 23:41:36.584526018+00:00: success
- Cycle 2258 at 2026-04-11 23:44:47.10510723+00:00: failed | error: failed to cancel order: invalid order ID: strconv.ParseInt: parsing "空头平仓: $0.092140": invalid syntax
- Cycle 2259 at 2026-04-11 23:47:36.132485037+00:00: success
- Cycle 2260 at 2026-04-11 23:50:48.201410163+00:00: failed | error: failed to cancel order: invalid order ID: strconv.ParseInt: parsing "空头平仓单_$0.092140": invalid syntax
- Cycle 2261 at 2026-04-11 23:53:37.172013336+00:00: success
- Cycle 2262 at 2026-04-11 23:56:28.529526049+00:00: success
- Cycle 2263 at 2026-04-11 23:59:22.757988258+00:00: success
- Cycle 2264 at 2026-04-12 00:02:22.029769324+00:00: success
- Cycle 2265 at 2026-04-12 00:05:24.04540259+00:00: success
- Cycle 2266 at 2026-04-12 00:08:35.237190397+00:00: failed | error: failed to cancel order: invalid order ID: strconv.ParseInt: parsing "空头平仓_0.092140": invalid syntax
- Cycle 2267 at 2026-04-12 00:11:24.661205349+00:00: success
- Cycle 2268 at 2026-04-12 00:14:24.244360676+00:00: success
- Cycle 2269 at 2026-04-12 00:17:34.145887438+00:00: failed | error: failed to cancel order: invalid order ID: strconv.ParseInt: parsing "空头平仓_0.092140": invalid syntax
- Cycle 2270 at 2026-04-12 00:18:25.914830646+00:00: success
- Cycle 2271 at 2026-04-12 00:21:24.733014613+00:00: success

## Failure Breakdown
- 2 x failed to cancel order: invalid order ID: strconv.ParseInt: parsing "空头平仓_0.092140": invalid syntax
- 1 x failed to cancel order: invalid order ID: strconv.ParseInt: parsing "空头平仓: $0.092140": invalid syntax
- 1 x failed to cancel order: invalid order ID: strconv.ParseInt: parsing "空头平仓单_$0.092140": invalid syntax

## Action Breakdown
- hold: 19
- place_sell_limit: 9
- cancel_order: 8
- cancel_all_orders: 1
- pause_grid: 1

## Open Orders Snapshot
- No open orders.

## Open Positions Snapshot
- DOGEUSDT SHORT qty=313131.0000 entry=0.09353701 status=OPEN
