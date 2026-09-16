# LeapNode 返佣系统实现文档（批10）

本文档说明 LeapNode 返佣系统的完整闭环：**邀请注册 → 首充返佣 → 收益提现 → admin 审批**。

---

## 一、返佣规则配置

返佣比例与注册奖励目前**硬编码**在 `internal/services/aff_service.go` 顶部常量：

```go
const (
    LEVEL1_RATE     = 0.10 // 一级返佣比例：首充金额的 10%
    LEVEL2_RATE     = 0.05 // 二级返佣比例：首充金额的 5%
    REGISTER_REWARD = 0.5  // 注册奖励：$0.5 给直接邀请人
)
```

> **TODO**：后续实现 admin 配置接口（`aff_settings` 表 + `GET/PUT /api/admin/aff/settings`），
> 支持运营在后台动态调整比例，无需改代码重新部署。

### Quota 换算

- 系统内部金额统一用 **Quota** 单位存储（`500,000 Quota = $1.00`）。
- 对外 JSON 一律用美元：`utils.QuotaToDollars(quota)` / `utils.DollarsToQuota(dollars)`。

---

## 二、返佣自动触发

### 2.1 注册返佣

**入口**：`POST /api/dist/user/register`（`UserService.Register`）

**流程**：
1. 注册请求携带邀请码（兼容两个字段名：`aff_code` 表单字段，或 `ref` 邀请链接参数 `/register?ref=CODE`）。
2. 查 `users` 表 `WHERE aff_code = ?` 找到 inviter。
3. 通过 New-API 创建用户后，回写 `users.inviter_id = inviter.id`（若 inviter 是分站主还回写 `distributor_id`）。
4. 用户数据 `Save` 落库后，**同步**调用 `affService.GrantRegisterReward(inviter.id, newUser.id)`。
5. 发放失败仅记录日志，**不回滚注册**（返佣是附加动作，不应阻塞用户注册）。

**GrantRegisterReward 逻辑**：
- 防重复：先查 `affiliate_earnings` 是否已有 `(user_id=inviterID, invitee_id=inviteeID, event_type='register')` 记录，有则跳过。
- 事务内：给 inviter 的 `users.aff_quota += $0.5`，写一条 `affiliate_earnings`（`event_type='register', status=1`）。

### 2.2 首充返佣

**入口**：`TopupService.ConfirmCryptoOrderPaid`（充值到账确认，事务内写 `topup_orders`）

**流程**：
1. 充值到账，事务内写 `topup_orders`（`status=1` 已支付）。
2. 事务内查询 `topup_orders WHERE user_id=? AND status=1` 的**订单数**：
   - 若刚写入的这条是**唯一一条**（COUNT=1）→ 判定为首充。
3. **事务提交成功后**（非事务内），调用 `affService.GrantFirstTopupReward(userID, topupAmount)`。
   - 之所以放事务外：`GrantFirstTopupReward` 用 `s.db` 独立读写，需读到已提交的 `topup_orders` 行，避免与未提交事务的可见性竞态。

**GrantFirstTopupReward 逻辑**：
- 防重复：先查 `affiliate_earnings` 是否已有 `(invitee_id=userID, event_type='first_topup')` 记录，有则跳过（按 invitee 去重，一个用户的首充只返一次）。
- 查 invitee 的 `inviter_id`（一级邀请人）；若一级邀请人还有 `inviter_id`（二级邀请人）。
- 事务内：
  - 一级邀请人 `aff_quota += 首充金额 × 10%`，写 `affiliate_earnings`（`commission_rate=0.10`）。
  - 二级邀请人（若存在）`aff_quota += 首充金额 × 5%`，写 `affiliate_earnings`（`commission_rate=0.05`）。
  - 两条记录都是 `event_type='first_topup', status=1`。

---

## 三、admin 提现审批

**Controller**：`internal/controllers/admin_withdrawal_controller.go`
**路由组**：`adminProtected`（`/api/admin`，需 `AuthRequired + AdminAuth`）

### 3.1 GET /api/admin/withdrawals — 提现申请列表

- 查询参数：
  - `status`（可选）：`0=pending 1=approved 2=rejected`
  - `limit`（默认 20，上限 200）、`offset`（默认 0）
- JOIN `users` 表回填 `username`。
- 响应：
  ```json
  {
    "success": true,
    "data": [
      {
        "id": 1, "user_id": 8, "username": "dist01",
        "amount": 50.0, "payment_method": "alipay", "account_info": "13800138000",
        "status": 0, "applied_at": "2026-09-15 10:00:00",
        "processed_at": "", "admin_remark": ""
      }
    ],
    "total": 23
  }
  ```

### 3.2 PUT /api/admin/withdrawals/:id/approve — 审批通过

- 请求体：`{"admin_remark": "已打款至支付宝"}`（可选）
- 业务逻辑（`AffService.ApproveWithdrawal`，**单事务**）：
  1. `SELECT ... FOR UPDATE` 行锁 `affiliate_payouts`，防并发重复审批。
  2. 校验 `status=0`（pending），否则报 `该提现申请已处理`。
  3. 校验 `users.aff_quota >= request.amount`，否则报 `用户返佣余额不足`（防重复审批扣成负数）。
  4. 扣款：`UPDATE users SET aff_quota = aff_quota - ?`。
  5. 写 `affiliate_earnings`（`event_type='withdraw', quota=负数, status=1`）。
  6. 更新 `affiliate_payouts`：`status=1, processed_at=NOW(), admin_remark=?`。
- 响应：`{"success": true, "message": "提现审批通过"}`

### 3.3 PUT /api/admin/withdrawals/:id/reject — 审批拒绝

- 请求体：`{"admin_remark": "账户信息有误，请重新提交"}`（**必填**）
- 业务逻辑（`AffService.RejectWithdrawal`，事务 + 行锁）：
  1. `SELECT ... FOR UPDATE` 行锁。
  2. 校验 `status=0`（pending）。
  3. 更新：`status=2, processed_at=NOW(), admin_remark=?`。
  4. **不扣款**（拒绝不影响 aff_quota）。
- 响应：`{"success": true, "message": "已拒绝提现申请"}`

---

## 四、返佣统计看板

### 4.1 GET /api/dist/aff/stats — 用户/分站自己的返佣统计

**路由组**：`affProtected`（`/api/dist`，需 `AuthRequired`）
**方法**：`AffService.GetAffStats`

| 字段 | 含义 | 计算方式 |
|------|------|---------|
| `total_earnings` | 历史总收益 | `SUM(affiliate_earnings.quota) WHERE user_id=? AND status=1 AND event_type NOT IN ('withdraw','transfer')` |
| `available_balance` | 当前可提现 | `users.aff_quota` |
| `total_withdrawn` | 已提现 | `-SUM(quota) WHERE event_type='withdraw'`（withdraw 记录为负数，取反） |
| `level1_invitees` | 一级邀请人数 | `COUNT(users) WHERE inviter_id=?` |
| `level2_invitees` | 二级邀请人数 | 一级邀请人的下线数 |
| `this_month_new_invitees` | 本月新增邀请 | `COUNT(users) WHERE inviter_id=? AND created_at >= 本月1号`（created_at 为 Unix 秒） |

### 4.2 GET /api/admin/aff/overview — 平台级返佣大盘

**路由组**：`adminProtected`（`/api/admin`）
**方法**：`AffService.GetAffOverview`

| 字段 | 含义 | 计算方式 |
|------|------|---------|
| `total_distributed` | 平台总发放 | `SUM(affiliate_earnings.quota) WHERE status=1 AND event_type NOT IN ('withdraw','transfer')` |
| `pending_withdrawal` | 待提现总额 | `SUM(users.aff_quota)` |
| `pending_requests` | 待审核提现申请数 | `COUNT(affiliate_payouts) WHERE status=0` |
| `top_inviters` | 返佣收益 TOP10 | 按 `user_id` 聚合收益倒序取 10，含 `username / total_earnings / invitees_count` |

---

## 五、防重复设计（关键）

所有返佣的幂等性由**业务逻辑去重**保证（查 `affiliate_earnings` 表）：

| 事件 | 去重条件 |
|------|---------|
| 注册返佣 | `(user_id=inviterID, invitee_id=inviteeID, event_type='register')` 已存在则跳过 |
| 首充返佣 | `(invitee_id=userID, event_type='first_topup')` 已存在则跳过（按 invitee 去重，一级+二级一起发一次） |

首充检测本身也有兜底：`topup_orders` 中 `status=1` 的订单数 `> 1` 时不再判定为首充。

> **建议（后续加固）**：为 `affiliate_earnings` 增加唯一约束
> `UNIQUE(invitee_id, event_type)`（首充）/ `UNIQUE(user_id, invitee_id, event_type)`（注册），
> 在数据库层兜底并发场景下的重复写入。当前依赖业务逻辑去重，正常单实例串行调用无问题。

---

## 六、并发与事务保证

- **提现审批**：`SELECT ... FOR UPDATE` 行锁 `affiliate_payouts`，扣款 + 写历史 + 更新 status 在**单事务**内原子完成。重复审批被 `status=0` 校验拦截。
- **充值首充检测**：首充判定在充值事务内完成，返佣发放在事务提交后触发，避免读到未提交数据。
- **aff_quota 扣减**：全部用 `gorm.Expr("aff_quota - ?", ...)` 原子表达式，不做「读-改-写」。

---

## 七、数据表映射

| 逻辑名 | 实际表名 | 说明 |
|--------|---------|------|
| `aff_history` | `affiliate_earnings` | 返佣历史（`event_type`: register/first_topup/transfer/withdraw） |
| `withdrawal_requests` | `affiliate_payouts` | 提现申请（`status`: 0=pending 1=approved 2=rejected 3=paid） |
| `users` | `users` | 扩展列：`aff_code / inviter_id / aff_quota`（LeapNode 直接写库，不走 New-API） |
| `topup_orders` | `topup_orders` | 充值订单（首充检测数据源，`status=1` 表示已支付） |

---

## 八、涉及文件

| 文件 | 改动 |
|------|------|
| `internal/services/aff_service.go` | 返佣常量、`GrantRegisterReward`/`GrantFirstTopupReward` 重写（防重复+一二级返佣）、提现审批（List/Approve/Reject）、统计（Stats/Overview） |
| `internal/services/user_service.go` | 注册流程接受 `ref`，触发注册返佣 |
| `internal/services/topup_service.go` | 首充检测 + 事务后触发首充返佣 |
| `internal/controllers/aff_controller.go` | `GetAffStats` / `GetAffOverview` |
| `internal/controllers/admin_withdrawal_controller.go` | **新增** 提现审批 3 接口 |
| `cmd/server/main.go` | 注册路由（`/api/dist/aff/stats`、`/api/admin/withdrawals/*`、`/api/admin/aff/overview`） |

---

## 九、API 速查

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/dist/user/register` | 公开 | 注册（接受 `aff_code`/`ref`，触发注册返佣） |
| GET | `/api/dist/aff/stats` | 用户 | 个人返佣统计 |
| GET | `/api/admin/withdrawals` | admin | 提现申请列表（`?status=&limit=&offset=`） |
| PUT | `/api/admin/withdrawals/:id/approve` | admin | 审批通过 |
| PUT | `/api/admin/withdrawals/:id/reject` | admin | 审批拒绝 |
| GET | `/api/admin/aff/overview` | admin | 平台返佣大盘 |
