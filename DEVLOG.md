# DEVLOG

随手记一些开发过程中的取舍、踩坑、和"为什么不这样写"。
不是文档，只是给以后回看时留个上下文。

---

## 2026-06 提交前最后一周

### Agent Wallet owner 模型
- 一开始想直接用 `tx_context::sender()` 做强校验，但要 zkLogin 用户自己签 PTB，
  对前端/agent 端来说要重写一套签名流程，黑客松时间不够。
- 妥协方案：**trusted-gateway** —— gateway 校验 zkLogin session，然后把验证过的
  `agent_address` 作为参数传进 `execute_trade`。Move 里只校验这个地址是否等于
  `wallet.agent_address`。
- 风险点：gateway 是受信的，意味着 gateway 私钥被偷就能伪造 agent 操作。
  写在了 README 的 Security Considerations 里。生产环境再上 zkLogin sponsored tx。

### zkLogin ephemeral key 缓存
- 想过用 Redis 存，但 Redis 是"optional"依赖，zkLogin 主链路不应该因为 Redis 挂了就崩。
- 所以放在内存里：`internal/gateway/zklogin/ephemeral.go` 那个 map。
- 副作用：gateway 重启用户要重新走一遍 OAuth。前端有重试逻辑就还好。
- 后来清了几个 helper（`GetKey` / `RemoveKey` / `CleanupExpired`），全仓 grep 都没人调。
  留着只会让新人误以为这些是对外 API，删了。

### Move 里那个被删掉的 `MAX_SLIPPAGE_BPS`
- 之前留了一个 `const MAX_SLIPPAGE_BPS: u64 = 500;` 加 `#[allow(unused_const)]`，
  注释里还写"7. Slippage does not exceed MAX_SLIPPAGE_BPS (Guardian check)"。
- 其实这个限制是 gateway 端 Guardian 算的，Move 里没有任何代码读这个常量。
- 留着的话 review 的人会以为链上有这个保护，但其实没有。删了，把注释也对齐了。

### Walrus blob ID 怎么用
- Walrus 的 blob ID 是内容寻址的（SHA256-like），存到 Move `MemoryObject` 里当 key。
- 本来想用 blob 内容本身当 memory 索引，后来发现同一份 LLM 输出可能产生不同 blob
  （timestamp 字段），所以还是用 task_id 做主键，blob_id 当 reference。

### Kafka vs 同步执行
- 真实想做成"提交即返，异步落链"。但 demo 模式不能这样 —— 评委要看到立即结果。
- 加了 `HACKATHON_DEMO_MODE=true` 时 `EnableSynchronousDemoProcessing` 这条路径，
  跳过 Kafka，直接 executor.ExecutePTB()。生产模式走原路径。
- 缺点：demo 模式代码路径和真实路径不完全一样。但这是 hackathon，先把故事讲圆。

---

## 一些还没做的 TODO（如果以后继续维护）

- [ ] `MemoryObject` 应该有 "owner 可以删除" 的接口，目前没写
- [ ] DeepBook 的 order book 失败回滚 —— 现在的处理是记录到 `ErrorDetail`，没回滚 wallet budget spent
- [ ] zkLogin session 在多 gateway 实例下共享 —— 现在内存 map，扩多副本要换 Redis
- [ ] Guardian 的 slippage 阈值应该可配置，现在硬编码 5%
- [ ] `agent_wallet.move` 的 protocol allowlist 写入是 owner 创建时定的，没给 agent 端"申请加入"流程

---

## 试过但没用的方案

- ❌ 用 `SuiTransactionBlockResponse::effects` 反查 object ID 来拿 wallet ID
  → 太脆，event 更稳定。改成解析 `WalletCreated` event。
- ❌ Move 端做 time window check 用 `tx_context::epoch_timestamp_ms()` 精确到毫秒
  → 实际 epoch 是 ~24h 一格，毫秒精度没意义，回归 epoch 整数。
- ❌ PTB 里塞 `Clock` 对象做"截止时间"硬校验
  → Clock 要 shared object，PTB 复杂度直接 *2。hackathon 阶段不划算。

---

## 一些零碎偏好

- 错误信息习惯 `fmt.Errorf("xxx failed: %w", err)`，这样调用方可以 `errors.Is`。
- Go 文件不喜欢超过 ~400 行，超了就拆。`agent_wallet.go` 现在已经偏长了，下次大改要拆。
- Move 里每个 public fun 上面的 doc comment 我尽量都写上 "为什么不那样" —— 半年后回看最有用
  的是 "为什么这里妥协了"，不是 "这个函数做什么"。
