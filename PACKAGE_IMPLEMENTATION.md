# LeapNode 套餐管理系统实现文档

## 概述

LeapNode 第 5 批实现：套餐管理（分站 CRUD + 用户订阅）

完成日期：2026-09-15  
实现文件：
- `internal/services/package_service.go` (业务逻辑)
- `internal/controllers/package_controller.go` (路由处理)
- `migrations/001_initial_schema.sql` (数据库 schema 扩展)

## 数据库变更

### 1. packages 表补充 description 列

```sql
ALTER TABLE packages ADD COLUMN IF NOT EXISTS description TEXT;
```

**变更原因**：前端 `Packages.jsx` 第 280 行明确需要 `description` 字段用于显示套餐描述。

### 2. 表结构完整定义

```sql
CREATE TABLE IF NOT EXISTS packages (
    id             SERIAL PRIMARY KEY,
    distributor_id INT DEFAULT 0,                     -- 0=总站套餐, >0=分站套餐
    name           VARCHAR(100),
    description    TEXT,                              -- 套餐描述
    price          DECIMAL(10,2),                     -- 套餐价格（人民币）
    original_price DECIMAL(10,2),                     -- 原价（划线价）
    quota_amount   BIGINT,                            -- 套餐包含的 quota (500000=$1)
    duration_days  INT,                               -- 有效期天数
    reset_period   VARCHAR(20),                       -- none/daily/weekly/monthly（周期性重置）
    enabled        BOOLEAN DEFAULT TRUE,
    created_at     TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id            SERIAL PRIMARY KEY,
    user_id       INT REFERENCES users(id),
    package_id    INT REFERENCES packages(id),
    status        INT DEFAULT 1,                      -- 1=有效 0=过期 2=取消
    start_at      TIMESTAMP,
    expire_at     TIMESTAMP,
    remain_quota  BIGINT,                             -- 剩余 quota
    next_reset_at TIMESTAMP,                          -- 下次重置时间（周期性套餐用）
    created_at    TIMESTAMP DEFAULT NOW()
);
```

## API 接口

### 分站管理套餐（需鉴权）

#### 1. 创建套餐
```http
POST /api/dist/admin/packages
Authorization: AuthRequired(db)
Content-Type: application/json

{
  "name": "基础套餐",
  "description": "适合轻度用户",
  "price": 9.90,
  "original_price": 19.90,
  "quota_amount": 5000000,
  "duration_days": 30,
  "reset_period": "none",
  "enabled": true
}

Response:
{
  "success": true,
  "data": {
    "id": 1,
    "distributor_id": 123,  // 当前登录用户 ID
    "name": "基础套餐",
    "description": "适合轻度用户",
    "price": 9.90,
    ...
  }
}
```

**业务逻辑**：
- `distributor_id` 自动设置为当前登录用户 ID（从 `middleware.CurrentUserID(c)` 获取）
- 校验必填字段：`name`, `price`, `quota_amount`, `duration_days`, `reset_period`
- `enabled` 默认 `true`

#### 2. 更新套餐
```http
PUT /api/dist/admin/packages/:id
Authorization: AuthRequired(db)
Content-Type: application/json

{
  "price": 8.90,
  "enabled": false
}
```

**安全校验**：
- 必须满足 `WHERE id=:id AND distributor_id=当前用户id`，防止跨分站篡改
- 使用 GORM 的 `Updates(map[string]interface{})` 支持部分更新

#### 3. 删除套餐
```http
DELETE /api/dist/admin/packages/:id
```

**删除策略**：硬删除（因为外键约束需谨慎）  
**前置校验**：
1. 检查归属：`WHERE id=:id AND distributor_id=当前用户id`
2. 检查活跃订阅：若存在 `status=1` 的订阅记录，拒绝删除并返回错误信息

#### 4. 查询本分站套餐
```http
GET /api/dist/admin/packages

Response:
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "基础套餐",
      "description": "适合轻度用户",
      "price": 9.90,
      "original_price": 19.90,
      "quota_amount": 5000000,
      "duration": 30,                   // 字段名映射
      "quota_reset_period": "none",     // 字段名映射
      "enabled": true
    }
  ]
}
```

**查询逻辑**：`WHERE distributor_id=当前用户id ORDER BY created_at DESC`

### 用户查询+订阅套餐

#### 5. 公开查询套餐列表
```http
GET /api/dist/site/packages

Response:
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "基础套餐",
      "description": "适合轻度用户",
      "price": 9.90,
      "original_price": 19.90,
      "quota_amount": 5000000,
      "duration": 30,
      "quota_reset_period": "none",
      "enabled": true
    }
  ]
}
```

**查询逻辑**：`WHERE enabled=true ORDER BY distributor_id ASC, price ASC`  
**排序说明**：总站套餐（distributor_id=0）优先，价格升序

#### 6. 订阅套餐
```http
POST /api/dist/package/:id/subscribe
Authorization: AuthRequired(db)
Content-Type: application/json

{}  // 空请求体，套餐 ID 在 URL 里

Response:
{
  "success": true,
  "message": "订阅成功",
  "data": {
    "subscription_id": 123,
    "remain_quota": 5000000
  }
}
```

**业务流程（事务保证原子性）**：

1. **查询套餐**：`WHERE id=:id AND enabled=true`
2. **锁定用户行**：`SELECT ... FOR UPDATE` 防止并发扣款
3. **计算套餐价格对应的 quota**：
   ```go
   const usdRate = 7.14  // 1 USD = 7.14 CNY
   quotaCost := int64(pkg.Price / usdRate * 500000)
   ```
   **汇率换算说明**：
   - 前端 `Packages.jsx` 396 行：`const pkgQuotaCost = Math.trunc((pkgPrice / usdRate) * Q);`
   - 与前端保持一致：`price_cny / 7.14 * 500000`
   - 示例：9.90 CNY 套餐 = 9.90/7.14 * 500000 ≈ 693277 quota

4. **检查余额**：`user.quota >= quotaCost`
5. **扣款**：`UPDATE users SET quota = quota - :quotaCost WHERE id = :userID`
6. **创建订阅记录**：
   ```go
   subscription := &Subscription{
       UserID:      userID,
       PackageID:   packageID,
       Status:      1,
       StartAt:     NOW(),
       ExpireAt:    NOW() + duration_days,
       RemainQuota: pkg.QuotaAmount,
       NextResetAt: calculateNextReset(pkg.ResetPeriod, NOW()),
   }
   ```

#### 7. 查询活跃订阅
```http
GET /api/dist/subscription/active
Authorization: AuthRequired(db)

Response:
{
  "success": true,
  "data": [
    {
      "id": 123,
      "package_id": 1,
      "amount_total": 5000000,          // 总额度
      "amount_used": 1200000,           // 已用额度 (total - remain)
      "end_time": 1726483200,           // Unix 时间戳（秒）
      "next_reset_time": 1726396800     // 下次重置时间（秒，0=不重置）
    }
  ],
  "total": 1
}
```

**查询逻辑**：`WHERE user_id=:userID AND status=1 ORDER BY created_at DESC`

## 周期重置计算

### calculateNextReset 逻辑

```go
func calculateNextReset(resetPeriod string, now time.Time) time.Time {
    switch resetPeriod {
    case "daily":
        return now.AddDate(0, 0, 1)     // 明天同时刻
    case "weekly":
        return now.AddDate(0, 0, 7)     // 下周同日同时刻
    case "monthly":
        return now.AddDate(0, 1, 0)     // 下月同日同时刻
    case "none":
        fallthrough
    default:
        return time.Time{}              // zero time（前端会转成 0）
    }
}
```

**前端显示**：
- `next_reset_time=0`：不重置，显示为 "不重置"
- `next_reset_time>0`：显示剩余时间或具体日期

**实际重置逻辑**：
- 本实现未包含定时任务自动重置逻辑
- 建议后续通过 cron job 或消费队列实现：
  1. 每小时扫描 `subscriptions` 表 `WHERE status=1 AND next_reset_at <= NOW()`
  2. 重置 `remain_quota = quota_amount`（从 packages 表读取）
  3. 更新 `next_reset_at` 为下一个周期

## 字段名映射

### 数据库列名 → JSON 字段名

| 数据库列名      | JSON 字段名           | 说明                           |
|----------------|-----------------------|--------------------------------|
| `duration_days`| `duration`            | 前端需要更短的字段名            |
| `reset_period` | `quota_reset_period`  | 前端字段命名习惯                |

实现方式：在 `toPackageDTO()` 函数中手动映射

## 安全设计

### 1. 防越权访问
- **分站 CRUD**：所有操作强制 `WHERE distributor_id=当前用户id`
- **用户订阅**：通过 `middleware.CurrentUserID(c)` 获取真实用户 ID，无法伪造

### 2. 并发安全
- **订阅扣款**：使用 `SELECT ... FOR UPDATE` 行锁 + GORM Transaction
- **参考实现**：复用充值码兑换的事务模式（`internal/services/topup_service.go`）

### 3. 数据完整性
- **删除保护**：禁止删除有活跃订阅的套餐
- **外键约束**：`subscriptions.user_id` 和 `subscriptions.package_id` 均有外键

## 前端契约对齐

### 关键点

1. **description 字段**：前端 `Packages.jsx` 第 280 行需要，已补充到数据库
2. **duration 字段**：前端使用 `duration`，不是 `duration_days`
3. **quota_reset_period**：前端使用这个字段名，不是 `reset_period`
4. **订阅响应格式**：
   ```json
   {
     "success": true,
     "message": "订阅成功",
     "data": { "subscription_id": 123, "remain_quota": 5000000 }
   }
   ```
5. **活跃订阅列表**：返回 `total` 字段（数组长度）

## 测试建议

### 单元测试
1. **汇率换算**：9.90 CNY → 693277 quota
2. **周期计算**：daily/weekly/monthly 的 `next_reset_at`
3. **余额不足**：订阅时 `user.quota < quotaCost` 应返回错误

### 集成测试
1. **创建→订阅→查询**：完整流程
2. **并发订阅**：多个用户同时订阅同一套餐，验证余额扣减正确
3. **删除保护**：有活跃订阅时删除套餐应失败

### 数据验证
```sql
-- 验证 description 列已添加
SELECT column_name, data_type FROM information_schema.columns 
WHERE table_name = 'packages' AND column_name = 'description';

-- 验证套餐创建
SELECT * FROM packages WHERE distributor_id = 123;

-- 验证订阅记录
SELECT s.*, p.name FROM subscriptions s 
JOIN packages p ON s.package_id = p.id 
WHERE s.user_id = 456 AND s.status = 1;
```

## 已知限制

1. **定时重置未实现**：需要后续添加 cron job 或消费队列
2. **套餐历史记录**：未保存修改历史（若需审计，建议加 `package_history` 表）
3. **订阅退款**：未实现取消订阅+退款逻辑（需求未明确）
4. **多币种支持**：当前硬编码 CNY，汇率 7.14（可改为配置项）

## 文件清单

```
leapnode-backend/
├── migrations/
│   └── 001_initial_schema.sql          (✅ 补充 description 列)
├── internal/
│   ├── models/
│   │   └── models.go                   (✅ Package + Subscription 模型已存在)
│   ├── services/
│   │   └── package_service.go          (✅ 新增，7 个接口业务逻辑)
│   └── controllers/
│       └── package_controller.go       (✅ 新增，7 个路由处理)
├── cmd/server/
│   └── main.go                         (✅ 注册路由)
└── PACKAGE_IMPLEMENTATION.md           (✅ 本文档)
```

## Git Commit

```bash
git add migrations/001_initial_schema.sql \
        internal/services/package_service.go \
        internal/controllers/package_controller.go \
        cmd/server/main.go \
        PACKAGE_IMPLEMENTATION.md

git commit -m "feat: 实现套餐管理系统（分站 CRUD + 用户订阅）

- 补充 packages.description 列（前端需要）
- 实现 7 个接口：
  * 分站管理：POST/PUT/DELETE/GET /api/dist/admin/packages
  * 用户：GET /api/dist/site/packages
  * 订阅：POST /api/dist/package/:id/subscribe
  * 活跃订阅：GET /api/dist/subscription/active
- 订阅逻辑：汇率换算 (1 USD = 7.14 CNY) + 事务扣款 + 周期计算
- 安全：行锁防并发、归属校验防越权、活跃订阅保护删除
- 字段映射：duration_days→duration, reset_period→quota_reset_period"
```

---

**实现完成时间**：2026-09-15  
**测试状态**：编译通过，待集成测试  
**后续工作**：添加定时重置任务、完善错误处理、补充单元测试
