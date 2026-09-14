# LeapNode 总站管理员后台 API - Week 10.1 交付

## 实现概览

已完成 LeapNode 总站管理员后台的 9 个核心模块，共 33 个 API 端点。

## 交付文件

### 1. 核心文件（新增 3 个）

- **internal/controllers/admin_controller.go** (609 行)
  - AdminController 实现所有管理员后台 HTTP 处理器
  - 33 个端点处理函数，统一响应格式

- **internal/services/admin_service.go** (896 行)
  - AdminService 实现所有业务逻辑
  - 数据查询、统计、审批、管理功能

- **internal/middleware/admin_auth.go** (48 行)
  - AdminAuth 中间件：验证 user_level = 10
  - 必须在 AuthRequired 之后使用

### 2. 路由注册（已更新）

- **cmd/server/main.go**
  - 注册 /api/admin/* 路由组
  - 应用 AuthRequired + AdminAuth 双重认证

## 9 个核心模块 API 列表

### 1. 系统概览（Dashboard）
- `GET /api/admin/dashboard/stats` - 总站统计数据
- `GET /api/admin/dashboard/charts` - 图表数据（收入/用户/模型调用量）

### 2. 用户管理
- `GET /api/admin/users` - 用户列表（分页/筛选）
- `GET /api/admin/users/:id` - 用户详情
- `PUT /api/admin/users/:id` - 更新用户（等级/状态）
- `DELETE /api/admin/users/:id` - 删除用户（禁用）
- `POST /api/admin/users/:id/quota` - 手动调整余额

### 3. 商家审批
- `GET /api/admin/merchants/pending` - 待审批商家列表
- `GET /api/admin/merchants/:id` - 商家详情
- `POST /api/admin/merchants/:id/approve` - 批准商家（user_level → 2）
- `POST /api/admin/merchants/:id/reject` - 拒绝商家

### 4. 分站审批
- `GET /api/admin/distributors/pending` - 待审批分站列表
- `GET /api/admin/distributors/:id` - 分站详情
- `POST /api/admin/distributors/:id/approve` - 批准分站（user_level → 3）
- `POST /api/admin/distributors/:id/reject` - 拒绝分站

### 5. 渠道管理（总站官方渠道）
- `GET /api/admin/channels` - 总站官方渠道列表
- `POST /api/admin/channels` - 添加官方渠道
- `PUT /api/admin/channels/:id` - 更新渠道
- `DELETE /api/admin/channels/:id` - 删除渠道

### 6. 套餐管理（总站套餐）
- `GET /api/admin/packages` - 套餐列表
- `POST /api/admin/packages` - 创建套餐
- `PUT /api/admin/packages/:id` - 更新套餐
- `DELETE /api/admin/packages/:id` - 删除套餐

### 7. 兑换码管理
- `GET /api/admin/redeem-codes` - 兑换码列表
- `POST /api/admin/redeem-codes/generate` - 批量生成兑换码
- `DELETE /api/admin/redeem-codes/:id` - 作废兑换码

### 8. 返佣管理
- `GET /api/admin/referrals` - 返佣记录列表（分页）
- `GET /api/admin/referrals/stats` - 返佣统计
- `POST /api/admin/referrals/:id/settle` - 手动结算返佣

### 9. 系统配置
- `GET /api/admin/config` - 获取系统配置
- `PUT /api/admin/config` - 更新系统配置

## 认证机制

所有 `/api/admin/*` 路由需要双重认证：

1. **AuthRequired** - 基础用户认证
   - New-Api-User 头 + Session Cookie 验证
   - 确保用户已登录且状态正常

2. **AdminAuth** - 管理员权限验证
   - 检查 user_level = 10（总站管理员）
   - 非管理员返回 403 Forbidden

## 技术实现要点

### 数据库操作
- 使用 GORM 进行所有数据库操作
- 支持事务处理（审批、余额调整）
- 分页查询（默认 page=1, page_size=20）

### 响应格式
```json
{
  "success": true,
  "data": {...},
  "message": ""
}
```

### 错误处理
- 400 Bad Request - 参数错误
- 403 Forbidden - 权限不足
- 404 Not Found - 资源不存在
- 500 Internal Server Error - 服务器错误

### 代码规范
- 所有注释使用中文
- 统一使用 utils.SuccessJSON / ErrorJSON
- 服务层强制业务逻辑校验

## 编译验证

```bash
cd ~/projects/leapnode-backend
go mod tidy
go build -o leapnode cmd/server/main.go
```

✅ 编译成功，生成可执行文件：leapnode (19MB)

## 数据库依赖

使用已有的扩展表：
- `users` - 用户表（扩展 user_level 字段）
- `user_applications` - 商家/分站申请表
- `channels` - 渠道表
- `packages` - 套餐表
- `redemptions` - 兑换码表（New-API 原生）
- `affiliate_earnings` - 返佣记录表
- `referral_config` - 返佣配置表
- `topup_orders` - 充值订单表

## 部署说明

服务器需要设置环境变量：
- `LEAPNODE_PG_DSN` - PostgreSQL 连接字符串

管理员账户要求：
- 数据库中至少一个用户的 `user_level = 10`
- 该用户可登录后访问所有管理员后台 API

## 下一步

Week 10.2 可能的方向：
1. 管理员前端界面（React Dashboard）
2. 更细粒度的权限控制
3. 操作日志记录
4. 数据导出功能
5. 实时监控与告警

---

**Week 10.1 完成时间**: 2026-09-14  
**开发者**: tiancookie  
**项目**: leapnode-backend
