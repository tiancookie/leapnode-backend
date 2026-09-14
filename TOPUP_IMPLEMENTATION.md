# 充值码(兑换码)系统实现文档

## 概述
为 LeapNode 后端实现充值码(兑换码)系统，与 SubRouter 前端完全兼容。本批只做兑换码，真实支付网关（支付宝/Stripe/USDT）留待下一批接入。

## 实现的接口

### 1. POST /api/dist/topup/redeem (需认证)
**功能**: 兑换充值码

**前端契约**: `api.js` L616 `redeemCode(key)` -> POST `/api/dist/topup/redeem` 发送 `{key}`

**请求体**:
```json
{
  "key": "充值码字符串"
}
```

**成功响应**:
```json
{
  "success": true,
  "data": {
    "quota": 500000,
    "message": "充值成功"
  },
  "message": ""
}
```

**错误响应**:
- `"充值码不能为空"` (400)
- `"充值码不存在"` (400)
- `"充值码已被使用"` (400)
- `"充值码已被禁用"` (400)
- `"充值码已过期"` (400)
- `"兑换失败，请重试"` (500)

### 2. GET /api/dist/topup/info (公开)
**功能**: 获取充值配置

**前端契约**: `api.js` L623 `getTopupInfo()` -> GET `/api/dist/topup/info`

**响应字段**:
```json
{
  "success": true,
  "data": {
    "enable_topup": true,
    "enable_online_topup": false,  // TODO: 下一批接入易支付
    "enable_stripe_topup": false,  // TODO: 下一批接入 Stripe
    "enable_creem_topup": false,   // TODO: 下一批接入 Creem
    "enable_crypto_topup": false,  // TODO: 下一批接入 USDT
    "currency": "CNY",
    "exchange_rate": 7.0,          // TODO: 从配置读取
    "min_topup": 1.0,              // TODO: 从配置读取
    "pay_methods": [],             // TODO: 下一批添加支付方式
    "top_up_link": "",             // TODO: 从配置读取
    "top_up_link_name": ""         // TODO: 从配置读取
  },
  "message": ""
}
```

### 3. GET /api/dist/topup/history (需认证)
**功能**: 获取当前用户充值历史

**前端契约**: `api.js` L642 `getTopupHistory(params)` -> GET `/api/dist/topup/history`

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "order_id",
      "user_id": 1,
      "amount": 10.0,
      "quota": 5000000,
      "payment_method": "redeem",
      "trade_no": "",
      "status": 1,
      "paid_at": 1673000000000,
      "created_at": 1673000000000
    }
  ],
  "message": ""
}
```

## 核心实现

### Redemption 模型 (internal/models/models.go)
严格对齐 New-API `redemptions` 表的真实列名：

| 字段 | GORM Column | 类型 | 说明 |
|------|------------|------|------|
| Id | id | int | 主键 |
| UserId | user_id | int | 创建者 ID |
| Key | key | string(32) | 充值码（唯一索引）|
| Status | status | int | 1=未使用 2=已禁用 3=已使用 |
| Name | name | string | 批次名称（索引）|
| Quota | quota | int | 充值额度（直接加到 user.quota）|
| CreatedTime | created_time | int64 | 创建时间（Unix 时间戳）|
| RedeemedTime | redeemed_time | int64 | 兑换时间（Unix 时间戳）|
| UsedUserId | used_user_id | int | 使用者 ID |
| DeletedAt | - | gorm.DeletedAt | 软删除（GORM 原生）|
| ExpiredTime | expired_time | int64 | 过期时间（0=不过期）|

### TopupOrder 模型 (internal/models/models.go)
对接 `topup_orders` 表：

| 字段 | GORM Column | 类型 | 说明 |
|------|------------|------|------|
| ID | id | string(32) | 订单ID（主键）|
| UserID | user_id | int | 用户 ID |
| Amount | amount | float64 | 充值金额 |
| Quota | quota | int64 | 充值 quota |
| PaymentMethod | payment_method | string(20) | 支付方式 |
| TradeNo | trade_no | string(100) | 交易号 |
| Status | status | int | 0=待支付 1=已支付 2=失败 |
| PaidAt | paid_at | *int64 | 支付时间 |
| CreatedAt | created_at | int64 | 创建时间 |

### 兑换事务实现 (internal/services/topup_service.go)
**并发安全设计**:
1. 使用 `gorm.Transaction` 包裹整个操作
2. 使用 `clause.Locking{Strength: "UPDATE"}` 实现 `SELECT FOR UPDATE` 行锁
3. 更新充值码时使用 `WHERE id = ? AND status = 1` CAS（Compare-And-Swap）防止并发重复兑换
4. 检查 `RowsAffected` 确认更新成功

**核心流程**:
```go
err := s.db.Transaction(func(tx *gorm.DB) error {
    // 1. 查询充值码并加行锁
    var redemption models.Redemption
    err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
        Where("key = ?", key).
        First(&redemption).Error
    
    // 2. 校验状态（status=1 未使用）
    if redemption.Status != 1 {
        return ErrRedeemCodeUsed/ErrRedeemCodeDisabled
    }
    
    // 3. 校验过期时间（expired_time=0 表示不过期）
    if redemption.ExpiredTime != 0 && redemption.ExpiredTime < now {
        return ErrRedeemCodeExpired
    }
    
    // 4. 更新充值码（CAS 防并发）
    result := tx.Model(&models.Redemption{}).
        Where("id = ? AND status = ?", redemption.Id, 1).
        Updates(map[string]interface{}{
            "status": 3,
            "redeemed_time": now,
            "used_user_id": userID,
        })
    
    if result.RowsAffected == 0 {
        return ErrRedeemCodeUsed
    }
    
    // 5. 给用户增加 quota
    err = tx.Model(&models.User{}).
        Where("id = ?", userID).
        Update("quota", gorm.Expr("quota + ?", redemption.Quota)).
        Error
    
    return nil
})
```

## 修改的文件

### 新增文件
1. `internal/services/topup_service.go` - 充值业务逻辑
2. `internal/controllers/topup_controller.go` - 充值 HTTP 处理器

### 修改文件
1. `cmd/server/main.go` - 注册充值路由
2. `internal/models/models.go` - 新增 Redemption 和 TopupOrder 模型

## 路由注册
```go
// 充值管理: 公开路由 (info) + 受保护路由 (redeem/history)
topupGroup := dist.Group("/topup")
topupController.RegisterPublicRoutes(topupGroup)

topupProtected := dist.Group("/topup")
topupProtected.Use(middleware.AuthRequired(db))
topupController.RegisterProtectedRoutes(topupProtected)
```

## TODO 占位字段（下一批接入）

### GetTopupInfo 接口
- `enable_online_topup`: false → 接入易支付/支付宝
- `enable_stripe_topup`: false → 接入 Stripe
- `enable_creem_topup`: false → 接入 Creem
- `enable_crypto_topup`: false → 接入 USDT
- `exchange_rate`: 7.0 → 从配置/数据库读取
- `min_topup`: 1.0 → 从配置/数据库读取
- `pay_methods`: [] → 添加真实支付方式列表
- `top_up_link`: "" → 外部充值商店链接
- `top_up_link_name`: "" → 外部充值商店名称

## 编译验证
```bash
cd ~/projects/leapnode-backend
GOTOOLCHAIN=local go build ./...
# exit 0 ✓
```

## Git 提交
```bash
git add -A
git commit -m "feat: 实现充值码(兑换码)系统，与 SubRouter 前端兼容"
# commit hash: b25db4c
```

## 需要确认的事项

1. **quota 体系**: 确认 `redemption.quota` 直接加到 `user.quota` 是否正确（都是 Q=500000 体系）
2. **topup_orders 集成**: 是否需要在兑换成功后插入一条 `topup_orders` 记录（payment_method="redeem"）？
3. **充值配置来源**: `GetTopupInfo` 接口的配置（汇率、最小充值等）应该从哪里读取？
   - 选项 A: 数据库配置表
   - 选项 B: 环境变量
   - 选项 C: config.yaml
4. **响应 message 语言**: 当前错误提示是中文，是否需要支持多语言（i18n）？
5. **充值历史范围**: `GetTopupHistory` 是否需要分页参数？当前返回全部历史记录。
6. **兑换日志**: 是否需要记录到 New-API 的 `logs` 表（type=topup）？

## 测试建议

### 手动测试
1. 创建测试充值码（使用 New-API 管理端或直接插入数据库）
2. 调用 `POST /api/dist/topup/redeem` 测试兑换流程
3. 验证用户 quota 是否正确增加
4. 测试并发兑换（同时提交同一个充值码）
5. 测试各种错误场景（已使用/已禁用/已过期/不存在）

### 集成测试
1. 前端 Topup.jsx 页面测试
2. 验证充值历史展示
3. 验证充值配置读取

## 下一批工作（真实支付网关接入）
1. 易支付（Epay）集成
2. Stripe 集成
3. USDT/加密货币支付集成
4. 完善 `pay_methods` 配置
5. 实现充值订单创建和回调处理
