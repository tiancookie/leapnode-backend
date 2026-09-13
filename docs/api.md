# LeapNode API 文档

所有分站接口以 `/api/dist/*` 为前缀，需与 SubRouter 前端 **100% 兼容**（共 109 个端点）。本文档按模块列出关键端点，实现进度以 checkbox 标记。

约定：
- 认证：与 New-API 共享 session / JWT。
- 金额单位：Quota（`500,000 = $1.00`）。
- 响应包裹：`{ "success": bool, "message": string, "data": any }`（对齐 New-API 风格）。

## 模块与数量

| 模块 | API 数量 | 说明 |
|------|---------|------|
| 公开接口 | 8 | 站点信息 / 模型列表 / 套餐 / 定价 |
| 用户认证 | 5 | 注册 / 登录 / OAuth |
| 用户管理 | 10 | 个人信息 / 日志 / 2FA |
| Token 管理 | 6 | CRUD + 模型支持 |
| 充值系统 | 3 | 充值历史 / 兑换码 |
| 返佣系统 | 7 | 邀请码 / 收益 / 提现 / KOL 申请 |
| 套餐订阅 | 3 | 套餐列表 / 订阅 / 激活订阅 |
| 市场 / 共享 | 33 | Marketplace + 订阅共享 |
| 日志统计 | 3 | 调用日志 / 统计 |
| 其他 | 31 | 支付 / 发票 / 工单等 |
| **总计** | **109** | |

## 公开接口

- [ ] `GET  /api/dist/site/info`
- [ ] `GET  /api/dist/site/models`
- [ ] `GET  /api/dist/site/pricing`
- [ ] `GET  /api/dist/site/packages`
- [ ] `GET  /api/dist/site/official-channels`
- [ ] `GET  /api/dist/site/key-groups`

## 用户认证

- [ ] `POST /api/dist/user/register`
- [ ] `POST /api/dist/user/login`
- [ ] `POST /api/dist/user/logout`
- [ ] `GET  /api/dist/oauth/{provider}/callback`

## Token 管理

- [ ] `GET    /api/dist/token/list`
- [ ] `POST   /api/dist/token/create`
- [ ] `PUT    /api/dist/token/{id}`
- [ ] `DELETE /api/dist/token/{id}`

## 充值系统

- [ ] `GET  /api/dist/topup/info`
- [ ] `POST /api/dist/topup/epay`      支付宝 / 微信
- [ ] `POST /api/dist/topup/stripe`
- [ ] `POST /api/dist/topup/crypto`    USDT
- [ ] `POST /api/dist/topup/redeem`    兑换码
- [ ] `GET  /api/dist/topup/history`

## 返佣系统

- [ ] `GET  /api/dist/aff/code`
- [ ] `GET  /api/dist/aff/earnings`
- [ ] `POST /api/dist/aff/transfer`
- [ ] `POST /api/dist/aff/withdraw`
- [ ] `POST /api/dist/kol/apply`
- [ ] `GET  /api/dist/kol/status`

## 套餐订阅

- [ ] `POST /api/dist/package/{id}/subscribe`
- [ ] `GET  /api/dist/subscription/active`

## 市场 / 共享（33 个）

订阅共享进货 (Marketplace) 与拼车相关端点，待补全完整列表。

## 总站管理员后台

审核中心（商家申请 / 模型上架 / 调价审批）、用户/商家/分站管理、财务管理、工单管理。在 New-API Web 基础上扩展。
