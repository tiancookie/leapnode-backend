# LeapNode 待办事项 (TODO)

> 维护人：萃萃 | 最后更新：2026-09-16 02:40
> 说明：记录"需要做但还没做"的事项，按优先级分层。已完成的批1-10不在此列。

---

## P0 — 阻塞生产上线（必须做，本周）

### 1. 真实计费配置 ⚠️ 最高优先级
- **现状**：批8 测 AI 调用时开了 New-API `SelfUseModeEnabled=true`，跳过计费才调通。
- **要做**：生产必须关自用模式，给每个上架模型在 New-API「分组与模型定价设置」配价格（输入/输出单价）。
- **依赖**：需要老大+龙龙定价策略（成本价 × 加价倍率）。
- **风险**：不配价直接开自用模式 = 所有调用不计费，商业模式失效。
- **预计工时**：配置 1h（定价策略确定后），验证测试 2h

---

### 2. New-API 上游账号换高配 ⚠️
- **现状**：x5m5x 免费/低配账号并发限制严（测试时频繁 429 gateway_concurrency_limit）。
- **要做**：生产换更高并发配额的上游 key，或配置多个上游渠道做负载均衡。
- **依赖**：需老大确认上游账号方案
- **预计工时**：配置 30min（账号确定后）

---

## P1 — 功能完整性（上线前应做，1-2周内）

### 3. 支付网关接入 💰
- **现状**：充值码✅、USDT✅；支付宝、Stripe 未实现。
- **要做**：接支付宝易支付 + Stripe（New-API 已有 topup/pay 接口可参考，但 LeapNode 分站级支付配置需自己实现回调）。
- **依赖**：涉及资金，需老大确认商户号/密钥。
- **预计工时**：支付宝易支付 4-6h，Stripe 3-4h（含测试）

---

### 4. Token 删/改走 New-API 🔧
- **现状**：user quota、channel 已解耦走 New-API；token 删/改仍直接写库（批7标注了缓存风险 TODO）。
- **要做**：改成调 New-API `DELETE /api/token/:id`、`PUT /api/token/`，消除脏缓存风险。
- **预计工时**：1-2h（含测试）

---

### 5. 套餐周期重置定时任务 ⏰
- **现状**：套餐订阅的 next_reset_at 字段已存，但没有定时任务扫描重置额度。
- **要做**：cron job 扫 subscriptions 表，到期重置 remain_quota。
- **预计工时**：2-3h（含测试）

---

### 6. Admin UpdateChannel 改 models 走 New-API 🔧
- **现状**：批9 已改 Create/Delete 走 New-API ✅；Update 的 models 变更仍待接 New-API PUT（已加风险标注，改 key/name/status 不影响路由）。
- **要做**：AdminService.UpdateChannel 改走 New-API PUT，更新后重建 abilities（刷新路由缓存）。
- **优先级**：低（改 models 是低频场景）
- **预计工时**：1h

---

## P2 — 安全/收尾（上线后可迭代）

### 7. 删除临时 bootstrap 端点 🗑️
- **现状**：`/bootstrap/init-dist`、`/bootstrap/init-merchant` 已加 BOOTSTRAP_SECRET 保护。
- **要做**：所有测试结束后彻底删除这两个端点 + 相关代码。
- **预计工时**：30min

---

### 8. 分站选品缺"商家模型池"接口 📋
- **现状**：分站选品时前端不知道可选的 merchant_id/channel_id。
- **要做**：加接口列出可选品的模型池（含正确的 merchant_id/channel_id）。
- **预计工时**：2-3h

---

### 9. New-API channel 契约固化到文档 📄
- **已摸清**：`POST /api/channel/` body=`{mode:"single", channel:{type,name,key,base_url,models,group,status}}`；New-API 自动建 abilities（按 group|model）；删用 `DELETE /api/channel/:id`。
- **要做**：写进 ADR 或 API 契约文档，避免重复摸索。
- **预计工时**：1h

---

## 已知技术债（记录，非紧急）

- **distributor_id 语义**：历史上混用过 user.ID / site.id，现已统一为 site.id，需回归测试确认所有分站接口口径一致。
- **merchant CreateChannel 的 base_url**：现在靠入参传，官方渠道/分站选品接入时需保持一致。
- **New-API access_token 15分钟过期**：NewAPIClient 已实现自动刷新（账密登录），依赖 NEW_API_ADMIN_USERNAME/PASSWORD 环境变量。

---

## 🎯 本周行动计划（按优先级）

### 立即行动（需老大决策）
1. ⚠️ **老大+龙龙确认定价策略** → 我配置 New-API 模型价格 → 关闭自用模式 → 验证计费
2. ⚠️ **老大确认 New-API 上游账号方案**（升级现有 / 多渠道负载均衡）
3. 💰 **老大确认支付商户号**（支付宝易支付 / Stripe）

### 技术收尾（我自主完成）
4. 🔧 实现 Token 删/改走 New-API（1-2h）
5. ⏰ 实现套餐周期重置定时任务（2-3h）
6. 🔧 实现 Admin UpdateChannel 走 New-API（1h，低优先级）

### 验收准备（下周）
7. 完整回归测试（批1-10所有端点）
8. 真机验证计费正确性（关闭自用模式后）
9. 🗑️ 删除 bootstrap 端点
10. 📋 补充分站选品"商家模型池"接口

---

## 📊 工时预估

- **P0（阻塞上线）**：3-4h（定价/账号确定后）
- **P1（功能完整）**：10-15h
- **P2（收尾）**：3-4h
- **验收测试**：4-6h

**总计：20-30h（1-2周内可完成所有P0+P1）**

---

## ✅ 批1-10 已完成清单（不在 TODO 中）

- ✅ 批1：公开接口（6个）
- ✅ 批2：用户认证（3个 + protected routes）
- ✅ 批3：Token管理（5个）
- ✅ 批4：充值码兑换（3个）
- ✅ 批5：USDT充值（4个） + 套餐订阅（3个）
- ✅ 批6：密钥分组管理（6个）
- ✅ 批7：分站运营工具（12个）
- ✅ 批8：商家中心 + Channel专项（20+个）
- ✅ 批9：官方渠道 + 分站选品 + 分站财务/仪表盘（20+个）
- ✅ 批10：返佣系统完整闭环（14个）

**实际实现：150个端点，超出原始设计109个API**
