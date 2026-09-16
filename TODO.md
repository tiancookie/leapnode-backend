# LeapNode 待办事项 (TODO)

> 维护人：萃萃 | 最后更新：2026-09-16
> 说明：记录"需要做但还没做"的事项，按优先级分层。已完成的批1-8不在此列。

---

## P0 — 阻塞生产上线（必须做）

### 1. 真实计费配置（当前用自用模式绕过）
- **现状**：批8 测 AI 调用时开了 New-API `SelfUseModeEnabled=true`，跳过计费才调通。
- **要做**：生产必须关自用模式，给每个上架模型在 New-API「分组与模型定价设置」配价格（输入/输出单价）。
- **依赖**：需要老大+龙龙定价策略（成本价 × 加价倍率）。
- **风险**：不配价直接开自用模式 = 所有调用不计费，商业模式失效。

### 2. admin / distributor 渠道 CRUD 改走 New-API
- **现状**：批8 只打通了 **merchant** 渠道 CRUD（走 New-API 建 channels+abilities）。
- **未做**：
  - `admin` 官方渠道 CRUD（admin_service.go CreateOfficialChannel/UpdateChannel/DeleteChannel）仍直接写库
  - `distributor` 选品落 abilities（distributor 启用模型时应确保对应 channel 的 abilities 生效）
- **判据**：这两个不接，官方渠道和分站选品的模型对 AI 路由仍"隐形"。

### 3. New-API 上游账号换高配
- **现状**：x5m5x 免费/低配账号并发限制严（测试时频繁 429 gateway_concurrency_limit）。
- **要做**：生产换更高并发配额的上游 key，或配置多个上游渠道做负载均衡。

---

## P1 — 功能完整性（上线前应做）

### 4. 支付网关接入
- **现状**：充值码✅、USDT✅；支付宝、Stripe 未实现。
- **要做**：接支付宝易支付 + Stripe（New-API 已有 topup/pay 接口可参考，但 LeapNode 分站级支付配置需自己实现回调）。
- **依赖**：涉及资金，需老大确认商户号/密钥。

### 5. 数据一致性：其余写操作走 New-API 复核
- **现状**：user quota、channel 已解耦走 New-API；token 删/改仍直接写库（批标注了缓存风险 TODO）。
- **要做**：联调后把 token 删/改改成调 New-API `DELETE /api/token/:id`、`PUT /api/token/`，消除脏缓存风险。

### 6. 定时任务：套餐周期重置
- **现状**：套餐订阅的 next_reset_at 字段已存，但没有定时任务扫描重置额度。
- **要做**：cron job 扫 subscriptions 表，到期重置 remain_quota。

### 7. 提现审核闭环
- **现状**：分站提现申请✅（写 withdrawal_requests 待审核），但总站审核打款流程（admin 侧 approve/reject + 扣分站 balance）未做。

---

## P2 — 安全 / 收尾（上线后可迭代）

### 8. 删除临时 bootstrap 端点
- **现状**：`/bootstrap/init-dist`、`/bootstrap/init-merchant` 已加 secret 保护（BOOTSTRAP_SECRET）。
- **要做**：所有测试结束后彻底删除这两个端点 + 相关代码。

### 9. 分站选品缺"商家模型池"接口
- **现状**：分站选品时前端不知道可选的 merchant_id/channel_id。
- **要做**：加接口列出可选品的模型池（含正确的 merchant_id/channel_id）。

### 10. New-API channel 契约固化到文档
- **已摸清**：`POST /api/channel/` body=`{mode:"single", channel:{type,name,key,base_url,models,group,status}}`；New-API 自动建 abilities（按 group|model）；删用 `DELETE /api/channel/:id`。
- **要做**：写进 ADR 或 API 契约文档，避免重复摸索。

---

## 已知技术债（记录，非紧急）

- distributor_id 语义历史上混用过 user.ID / site.id，现已统一为 site.id，但需回归测试确认所有分站接口口径一致。
- merchant CreateChannel 的 base_url 现在靠入参传，官方渠道/分站选品接入时需保持一致。
- New-API access_token 15分钟过期，NewAPIClient 已实现自动刷新（账密登录），但依赖 NEW_API_ADMIN_USERNAME/PASSWORD 环境变量。
