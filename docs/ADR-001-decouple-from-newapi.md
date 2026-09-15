# ADR-001：LeapNode 与 New-API 的数据访问解耦

**日期**：2026-09-15
**状态**：✅ 已决策，待执行
**决策者**：Terry（老大）+ 萃萃

---

## 背景

LeapNode 采用**路线A：两进程 + 共享 PostgreSQL** 架构（与原 final-tech-design v3.0 一致）：

```
Nginx（统一入口，待建）
 ├→ LeapNode-frontend（复用 SubRouter 静态页）
 ├→ LeapNode-backend（/api/dist/* /api/admin/* /api/merchant/* /api/distributor/*）
 └→ New-API（/v1/* AI 调用引擎 + Token 鉴权 + 计费）
        │
        └─ 共享 PostgreSQL
```

## 问题

当前 LeapNode-backend **直接用 GORM 读写 New-API 的原生表**（users / tokens / channels / redemptions / abilities / logs），造成**数据库层耦合**：
- 两个进程同时写同一批表 → 数据竞争、一致性风险
- New-API 有内存缓存（token cache / user quota cache），LeapNode 绕过它直接改库 → 缓存与库不一致
- New-API 升级改表结构 → LeapNode 跟着崩

## 决策

**职责边界明确化：**

1. **New-API 的表只由 New-API 写**
   - users / tokens / channels / redemptions / logs / abilities
   - LeapNode 需要改这些表时，**调用 New-API 的管理 API**，不直接写库

2. **LeapNode 只独占自己的 18 张扩展表**
   - merchant_* / distributor_* / packages / topup_orders / crypto_topup_orders
   - aff_history / referral_config / withdrawal_requests / kol_applications 等

3. **只读场景可保留直接查库**（性能考量，读不产生一致性问题）
   - 如统计报表 SELECT COUNT/SUM，可继续直接读 New-API 表
   - 但**所有写操作**（Create/Update/Delete）必须走 New-API API

## New-API 可用的管理 API（已确认存在）

| 操作 | New-API 接口 | 替代 LeapNode 当前的直接写 |
|------|-------------|------------------------|
| 注册用户 | `POST /api/user/register` | user_service.Register 的 db.Create(&user) |
| 登录 | `POST /api/user/login` | - |
| 改密 | 走 self 接口 | user_service db.Update password |
| 管理用户（改等级/禁用/加额度） | `POST /api/user/manage`（AdminAuth） | admin_service 各种 db.Model(&User).Update |
| 建 Token | `POST /api/token`（UserAuth） | token_service.CreateToken 的 db.Create(&token) |
| 删 Token | `DELETE /api/token` | token_service.DeleteToken |
| 渠道 CRUD | `/api/channel/*`（RootAuth） | admin/merchant service 的 channel 操作 |
| 兑换码 | New-API 有 redemption 管理接口 | topup_service 的兑换 |

## 需要改造的写操作清单（诊断结果）

**直接写 New-API 表的位置（必须改成调 API）：**
- `user_service.go:179` Register → db.Create(&user)
- `user_service.go:235` 改密 → db.Update password
- `user_service.go:244` 改语言 → db.Update language
- `user_service.go:69` aff_service 回填 aff_code → db.Update
- `token_service.go:195` CreateToken → db.Create(&token)
- `token_service.go:256` UpdateToken
- `token_service.go` DeleteToken 软删除
- `admin_service.go:243/249` 用户改等级/禁用
- `admin_service.go:510` 建 channel
- `admin_service.go:546` 改 channel
- `topup_service` 兑换码扣 quota / 加 quota（涉及 users.quota）
- `aff_service` 返佣加 aff_quota（users.aff_quota）
- `distributor_service.go:431` 调整用户 quota

**保留直接读（只读，无需改）：**
- 所有 Count/SUM 统计
- 列表查询（GetUsers/GetModels 等）

## 关键难点

1. **quota 变动**（充值到账、返佣、消费）涉及 `users.quota` / `users.aff_quota`——这是 New-API 缓存的热点字段。必须走 New-API 的额度接口，否则缓存不一致导致用户余额显示错误或超额调用。

2. **New-API 的管理 API 需要鉴权**——LeapNode 调用时要持有 New-API 的 Access Token（root 或 admin 级别）。需要在 LeapNode 配置一个 New-API 的系统级 access_token（环境变量注入）。

3. **事务性丧失**——原来一个 DB 事务能保证"扣兑换码+加quota"原子性，改成跨进程 API 调用后要处理中间失败（补偿/幂等）。

## 执行进度（2026-09-15 更新）

### ✅ 第一阶段完成（user/quota 写操作解耦）
新增 `internal/services/newapi_client.go`，封装 New-API HTTP 调用：
- CreateUser / IncreaseQuota / SetUserQuota / UpdateUserPassword

已改造的写操作（直接写库 → 调 New-API）：
- ✅ user_service Register（创建用户）
- ✅ user_service ChangePassword（改密码）
- ✅ topup_service RedeemCode（兑换码加 quota）
- ✅ topup_service ConfirmCryptoOrderPaid（USDT 加 quota）
- ✅ admin_service AdjustUserQuota（管理员调余额）
- ✅ aff_service TransferAffQuota（aff_quota 扣本地 + quota 走 API）
- ✅ distributor_service AdjustUserQuota（分站调用户余额）

编译通过（go build ./... exit 0）。

### Token 缓存机制分析（关键发现，2026-09-15）
读 New-API 源码 model/token.go 后确认其 token 缓存行为：
- **鉴权读取** `GetTokenByKey`: 先查 Redis，**缓存 miss 时自动 fall through 读数据库**并回填缓存。
- **写入** Update/Delete: 写库前调 `invalidateTokenCacheForMutation(key)` 失效缓存。
- 缓存键格式 `token:HMAC(key)`（common.GenerateHMAC，需 New-API 的 HMAC 密钥才能构造）。
- `AddToken` 仅 `Insert()` 写库，**不初始化缓存**（靠鉴权时 miss 回源）。

### Token 处理最终策略
| 操作 | 方案 | 理由 |
|------|------|------|
| **建 Token** | ✅ 保留直接写库 | 新 key 缓存本就没有，New-API 鉴权 miss 自动回源，安全 |
| **删 Token** | 调 New-API DELETE /api/token/:id | 缓存 HMAC 加密，LeapNode 无法手动失效脏缓存 |
| **改 Token** | 调 New-API PUT /api/token/ | 同上，避免脏缓存导致旧 status/额度生效 |

### ⏳ 待改造（下一阶段）
- ⚠️ **token_service UpdateToken/DeleteToken**：需改成调 New-API API（建 Token 保留直接写库）。
  - New-API 的 token 接口需用户级 access_token。方案：LeapNode 用管理员 token 调，或为每个用户维护一个 New-API access_token。
- aff_quota 纯扣加（返佣发放）：保留直接写库（LeapNode 扩展列，New-API 不碰）✅ 合理

### ⚠️ 未联调
以上改造依赖 New-API 部署可用 + API 契约验证。当前 New-API 在 Railway 无法访问，改造代码尚未端到端测试。New-API 的 CreateUser/updateUser 实际字段名和响应格式需联调确认。

## 执行计划（待明天）

1. **先修好 New-API 的 Railway 部署**（当前无法访问，端口/健康检查问题）——这是前提
2. 在 LeapNode 加一个 `newapi_client`（HTTP 客户端封装 New-API 管理 API）
3. 逐个替换写操作：user → token → quota → channel
4. 配置 New-API 系统 access_token 到 LeapNode 环境变量
5. 建 Nginx 统一入口，对外单域名
6. 端到端测试：注册→充值→建Key→调模型→扣费→返佣

## 影响

- LeapNode 已完成的 111 个 API 端点**接口契约不变**，只改内部实现（service 层写操作）
- controller 层基本不动
- 工作量估计：2-3 天（含 New-API 部署修复 + Nginx 网关）
