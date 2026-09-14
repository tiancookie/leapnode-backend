# 商家中心后台 API 文档

## 认证要求
所有 `/api/merchant/*` 路由需要：
1. `middleware.AuthRequired` - 用户已登录
2. `middleware.MerchantAuth` - `user_level = 2`（商家）

## API 端点清单（共 28 个）

### 1. 商家首页（Dashboard）- 2 个端点
- `GET /api/merchant/dashboard/stats` - 商家统计数据（收益/模型数/调用量/订单数）
- `GET /api/merchant/dashboard/revenue-trend` - 收益趋势图（近30天）

### 2. 我的模型 - 5 个端点
- `GET /api/merchant/models` - 商家的模型列表（分页）
- `POST /api/merchant/models` - 添加新模型（上架审批）
- `PUT /api/merchant/models/:id` - 更新模型（调价审批）
- `DELETE /api/merchant/models/:id` - 下架模型
- `GET /api/merchant/models/:id/stats` - 单个模型统计（调用量/收益）

### 3. 官方渠道（商家的上游渠道）- 5 个端点
- `GET /api/merchant/channels` - 商家的渠道列表
- `POST /api/merchant/channels` - 添加上游渠道
- `PUT /api/merchant/channels/:id` - 更新渠道
- `DELETE /api/merchant/channels/:id` - 删除渠道
- `GET /api/merchant/channels/:id/test` - 测试渠道连通性

### 4. 收益管理 - 3 个端点
- `GET /api/merchant/revenue/summary` - 收益汇总（总收益/待结算/已结算）
- `GET /api/merchant/revenue/records` - 收益明细（分页，按日/周/月）
- `GET /api/merchant/revenue/by-model` - 按模型分组收益

### 5. 提现管理 - 3 个端点
- `GET /api/merchant/withdrawals` - 提现记录列表
- `POST /api/merchant/withdrawals` - 申请提现
- `GET /api/merchant/withdrawals/:id` - 提现详情

### 6. 兑换码（商家可生成兑换码）- 3 个端点
- `GET /api/merchant/redeem-codes` - 商家的兑换码列表
- `POST /api/merchant/redeem-codes/generate` - 生成兑换码（商家自费）
- `GET /api/merchant/redeem-codes/:id/usage` - 兑换码使用记录

### 7. 问题工单 - 4 个端点
- `GET /api/merchant/tickets` - 工单列表（分页）
- `POST /api/merchant/tickets` - 创建工单
- `GET /api/merchant/tickets/:id` - 工单详情
- `PUT /api/merchant/tickets/:id/reply` - 回复工单

### 8. 公告管理（商家查看平台公告）- 3 个端点
- `GET /api/merchant/announcements` - 平台公告列表
- `GET /api/merchant/announcements/:id` - 公告详情
- `PUT /api/merchant/announcements/:id/read` - 标记已读

## 数据隔离原则
所有查询必须加 `WHERE user_id = ?` 或 `merchant_id = ?` 过滤，确保商家只能访问自己的数据。

## 关键业务逻辑

### 模型上架流程
1. 商家 `POST /api/merchant/models`（status = 0 待审核）
2. 创建 `model_approvals` 记录（待总站审批）
3. 总站批准后，status → 1 通过，模型可见

### 收益计算规则
- 商家收益 = 订单金额 × 70%
- 记录到 `merchant_revenue_records` 表
- 可提现余额 = 总收益 - 已提现

### 提现流程
1. 商家 `POST /api/merchant/withdrawals`（amount, payment_method）
2. 检查可提现余额是否足够
3. 创建 `withdrawal_requests`（status = 0 待审核）
4. 总站审批后，status → 1 通过，余额扣减

## 技术实现

### 文件结构
```
internal/
├── middleware/merchant_auth.go     # 商家认证中间件
├── services/merchant_service.go    # 商家业务逻辑（20+ 方法）
└── controllers/merchant_controller.go  # 商家 HTTP 处理器（28 个端点）
```

### 路由注册
```go
merchant := router.Group("/api/merchant")
merchant.Use(middleware.AuthRequired(db))
merchant.Use(middleware.MerchantAuth())
{
    merchantController.RegisterRoutes(merchant)
}
```

## 数据库表
使用已有的扩展表：
- `merchants` - 商家基本信息
- `channels` - 上游渠道（扩展 merchant_id 字段）
- `model_approvals` - 模型审批记录
- `merchant_revenue_records` - 收益记录
- `affiliate_payouts` - 提现申请
- `redeem_codes` - 兑换码
- `tickets` - 工单
- `announcements` - 公告
- `announcement_reads` - 公告已读记录

## 验证状态
✅ 编译成功  
✅ 28 个 API 端点已实现  
✅ 数据隔离已加固  
✅ 统一响应格式（success/data/message）  
✅ 分页支持（page/page_size）
