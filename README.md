# LeapNode 后端

LeapNode 是基于 [New-API](https://github.com/QuantumNous/new-api) 二次开发的 AI 网关分销平台后端。前端复用 SubRouter 分站界面（109 个 `/api/dist/*` 接口，需 100% 兼容），后端由 New-API（AI 调用 / Token 鉴权 / 基础计费）与本项目（四层用户体系 / 商家管理 / 审批流程 / 套餐订阅 / 返佣 / 充值 / 订阅共享）协同组成。

## 项目介绍

- **定位**：SubRouter 分站体系的自建替代，去除第三方 30% 抽成。
- **四层用户体系**：总站管理员 → 商家(KOL) / 分站站长 → 普通用户 / 下级分站用户。
- **Quota 体系**：`500,000 Quota = $1.00`。
- **与 New-API 关系**：共享同一 PostgreSQL 库；New-API 复用约 60%，扩展约 10% 字段；LeapNode 承担约 40% 新逻辑。

## 技术栈

| 组件 | 选型 |
|------|------|
| 语言 | Go 1.21+ |
| Web 框架 | Gin v1.9.1 |
| ORM | GORM v1.25.5 (postgres driver v1.5.4) |
| 数据库 | PostgreSQL 14+ |
| 缓存 / 队列 | Redis (go-redis v9.3.0) |
| 部署 | Railway (API + DB) + Upstash Redis |

## 目录结构

```
leapnode-backend/
├── cmd/
│   └── server/
│       └── main.go          # 服务入口 (Gin + GORM + Redis 初始化, 优雅关闭)
├── internal/
│   ├── models/              # GORM 数据模型 (对应 18 张扩展表)
│   ├── controllers/         # HTTP 处理器 (/api/dist/* 契约)
│   ├── services/            # 业务逻辑 (审批/订阅/返佣/结算引擎)
│   ├── middleware/          # 认证 / 四层权限 / 分站域名解析 / 限流
│   └── utils/               # 工具函数 (Quota 换算等)
├── migrations/
│   └── 001_initial_schema.sql  # 扩展字段 + 18 张扩展表 DDL
├── config/
│   └── config.yaml          # 配置文件 (环境变量可覆盖)
├── docs/
│   └── api.md               # API 文档
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

## 开发环境配置

### 依赖

- Go 1.21+
- PostgreSQL 14+
- Redis 6+

### 拉取依赖

```bash
cd leapnode-backend
go mod tidy      # 生成 go.sum, 下载依赖
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `LEAPNODE_HTTP_ADDR` | HTTP 监听地址 | `:8080` |
| `LEAPNODE_PG_DSN` | PostgreSQL DSN | 见 `main.go` |
| `LEAPNODE_REDIS_ADDR` | Redis 地址 | `127.0.0.1:6379` |

`config/config.yaml` 提供完整默认配置，环境变量优先级更高。**切勿将支付密钥、共享密钥等明文提交到仓库**（`.env` 已被忽略）。

### 运行

```bash
go run ./cmd/server        # 开发
go build -o bin/server ./cmd/server && ./bin/server   # 构建
```

健康检查：`GET /healthz`。

## 数据库初始化步骤

1. **先部署 New-API** 并让其自动建表（`users` / `channels` / `logs` / `tokens` / `abilities`），LeapNode 与其共享同一数据库。
2. 创建数据库与账号：

   ```bash
   createdb leapnode
   ```

3. 执行迁移脚本（扩展原生表字段 + 创建 18 张扩展表）：

   ```bash
   psql -d leapnode -f migrations/001_initial_schema.sql
   ```

   脚本特性：
   - `ALTER TABLE ... IF EXISTS / ADD COLUMN IF NOT EXISTS`，对已部署的 New-API 幂等安全。
   - 全部 `CREATE TABLE IF NOT EXISTS`，附带索引与外键。
   - 内置 `referral_config` 种子数据（注册奖励 $0.10、首充奖励 $1.00）。

4. 验证：

   ```bash
   psql -d leapnode -c "\dt"
   ```

## 开发路线（10 周）

Week 1-2 基础设施 + New-API 扩展 → Week 3-4 商家/审批 → Week 5-6 充值/套餐 → Week 7-8 返佣/订阅共享 → Week 9 总站管理员后台 → Week 10 测试上线。详见 `~/projects/leapnode-complete-dev-plan.md`。
