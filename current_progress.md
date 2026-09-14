# LeapNode 当前进度

## ✅ 已完成（1-5批）

### 第 1 批：6个公开接口
- GET /api/dist/site/info
- GET /api/dist/site/models
- GET /api/dist/site/pricing
- GET /api/dist/site/packages
- GET /api/dist/site/official-channels
- GET /api/dist/site/key-groups

### 第 2 批：认证系统
- POST /api/dist/user/register
- POST /api/dist/user/login
- POST /api/dist/user/logout

### 第 3 批：Token 管理
- GET /api/dist/token/list
- POST /api/dist/token/create
- PUT /api/dist/token/{id}
- DELETE /api/dist/token/{id}

### 第 4 批：充值码兑换
- POST /api/dist/topup/redeem
- GET /api/dist/topup/info (基础版)
- GET /api/dist/topup/history

### 第 5 批：USDT 充值
- POST /api/dist/topup/crypto/pay
- GET /api/dist/topup/crypto/status
- POST /api/dist/topup/crypto/claim
- POST /api/dist/topup/crypto/reconcile
- 扩展 GetTopupInfo (crypto配置)

**小计**: 约 23 个 API 端点已实现

---

## 🚧 待完成（按优先级）

### P0: 核心业务闭环（Week 5-7）

#### 支付渠道（充值系统）
- [ ] POST /api/dist/topup/epay（支付宝/微信，易支付对接）
- [ ] POST /api/dist/topup/stripe（Stripe 对接）

#### 套餐订阅
- [ ] POST /api/dist/package/{id}/subscribe
- [ ] GET /api/dist/subscription/active
- [ ] POST /api/dist/subscription/activate

#### 返佣系统（7 个 API）
- [ ] GET /api/dist/aff/code（获取邀请码）
- [ ] GET /api/dist/aff/earnings（收益查询）
- [ ] POST /api/dist/aff/transfer（收益转余额）
- [ ] POST /api/dist/aff/withdraw（提现申请）
- [ ] GET /api/dist/aff/invitees（邀请列表）
- [ ] POST /api/dist/kol/apply（商家/分站申请）
- [ ] GET /api/dist/kol/status（申请状态）

#### 用户管理扩展（10 个 API）
- [ ] GET /api/dist/user/profile
- [ ] PUT /api/dist/user/profile
- [ ] POST /api/dist/user/change-password
- [ ] GET /api/dist/user/logs
- [ ] POST /api/dist/user/2fa/enable
- [ ] POST /api/dist/user/2fa/verify
- [ ] GET /api/dist/user/quota
- [ ] GET /api/dist/user/notifications

### P1: 市场功能（Week 7-8，33 个 API）
- [ ] Marketplace 模型搜索/筛选
- [ ] 订阅共享商家列表
- [ ] 订阅共享进货/激活
- [ ] 官方渠道购买

### P2: 管理后台（Week 9）
- [ ] 审核中心（KOL 申请/模型上架/提现审批）
- [ ] 用户/商家/分站管理
- [ ] 财务管理（对账/报表）
- [ ] 工单系统

### P3: 辅助功能
- [ ] OAuth 登录（GitHub/Google）
- [ ] 发票管理
- [ ] 日志统计 API（3个）

---

## 📊 进度统计
- 已实现：23/109 API（21%）
- 核心闭环（P0）：约 30 个待实现
- 市场功能（P1）：33 个待实现
- 管理后台（P2）：后端基础设施 + 前端页面
- 辅助功能（P3）：约 15 个待实现

---

## 🎯 下一批建议（第 6 批）

**目标**: 完成核心业务闭环 - 返佣系统（7 个 API）

**理由**:
1. 返佣是 LeapNode 差异化核心功能（区别于 SubRouter）
2. 用户增长飞轮：注册奖励 → 邀请返佣 → 首充奖励
3. 为商家/分站申请（KOL）打基础
4. 不依赖外部支付渠道，可独立测试

**包含**:
- 邀请码生成/查询
- 收益计算引擎（5% 默认 + 二级无上限）
- 收益转余额
- 提现申请（状态机：pending → reviewing → success/rejected）
- 邀请列表（一级/二级邀请人）
- KOL 申请（升级商家/分站）
- 申请状态查询

**前置依赖**: 
- aff_history 表（已有）
- referral_config 表（已有）
- users.aff_code/inviter_id（已有）

