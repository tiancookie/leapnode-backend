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

### 2. admin / distributor 渠道 CRUD 改走 New-API ✅ 批9完成
- **merchant** 渠道 CRUD：批8 已走 New-API ✅
- **admin** 官方渠道：批9 已改 Create/Delete 走 New-API ✅；Update 的 models 变更仍待接 New-API PUT（已加风险标注，改 key/name/status 不影响路由）
- **distributor** 选品：批9 确认不建 channel，靠上游商家/官方渠道的 abilities 路由，无需改代码 ✅
- **真机验证**：官方渠道建立成功，New-API 日志坐实 glm-5.2→channel 2 路由计费成功。
- **遗留**：admin UpdateChannel 改 models 时需走 New-API PUT + abilities 重建（低频场景，已标注 TODO）。

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

## 批10 — 返佣系统（商业闭环最后一块）

### 10.1 返佣规则配置
- **现状**：一级/二级返佣比例硬编码或未配置。
- **要做**：admin 可配置一级/二级返佣比例（aff_settings 表或配置文件），默认值：一级 10%、二级 5%。

### 10.2 返佣自动触发
- **现状**：aff_service.go 已有 GrantRegisterReward/GrantFirstTopupReward 方法，但未挂到注册/充值流程。
- **要做**：
  1. 注册时：若有 ref 参数，写 users.inviter_id + 触发 GrantRegisterReward（给邀请人固定奖励 $0.5）
  2. 首充时：查 inviter_id，若存在触发 GrantFirstTopupReward（一级 10% + 二级 5%）
  3. 防重复：用 aff_history.event_type 去重（同一 invitee_id + register/first_topup 只记录一次）

### 10.3 admin 提现审批
- **现状**：用户提现申请已写 withdrawal_requests 表（status=0 pending）。
- **要做**：
  1. `GET /api/admin/withdrawals` — 查询待审核列表（分页，含用户名/金额/提现方式/申请时间）
  2. `PUT /api/admin/withdrawals/:id/approve` — 通过（扣用户 aff_quota + 更新 status=1 + 记录 aff_history.withdraw + 备注打款时间）
  3. `PUT /api/admin/withdrawals/:id/reject` — 拒绝（更新 status=2 + 填写拒绝原因 admin_remark）

### 10.4 返佣统计看板
- **要做**：
  1. `GET /api/dist/aff/stats` — 分站/用户自己的返佣统计（总收益/可提现余额/已提现/一级邀请人数/二级邀请人数/本月新增）
  2. `GET /api/admin/aff/overview` — 平台级返佣大盘（总发放金额/待提现金额/提现申请数/活跃邀请人数 TOP10）

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
