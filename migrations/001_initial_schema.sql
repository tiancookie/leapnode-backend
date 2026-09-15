-- =============================================================================
-- LeapNode 数据库初始化脚本
-- Migration: 001_initial_schema
-- Database:  PostgreSQL 14+
-- 说明:
--   1. 扩展 New-API 原生表字段 (users / channels / logs / tokens)
--   2. 创建 18 张 LeapNode 扩展表 (含索引与外键)
--   Quota 体系: 500,000 = $1.00
-- =============================================================================

BEGIN;

-- =============================================================================
-- PART 1: New-API 原生表扩展字段 (ALTER TABLE)
-- 使用 IF NOT EXISTS 保证幂等; 若原生表尚未创建, 请先部署 New-API 后再执行本段。
-- =============================================================================

-- users 表: 四层用户体系
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS user_level    INT DEFAULT 0; -- 0=普通用户 1=分站用户 2=商家(KOL) 3=分站站长 9=总站管理员
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS merchant_id   INT;           -- 关联 merchants.id
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS distributor_id INT;          -- 关联 distributor_sites.id
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS language      VARCHAR(16);    -- 用户界面语言 (PUT /api/dist/user/language)

-- channels 表: 商家渠道 + 定价
ALTER TABLE IF EXISTS channels ADD COLUMN IF NOT EXISTS merchant_id  INT;
ALTER TABLE IF EXISTS channels ADD COLUMN IF NOT EXISTS input_price  DECIMAL(10,6);
ALTER TABLE IF EXISTS channels ADD COLUMN IF NOT EXISTS output_price DECIMAL(10,6);

-- logs 表: 收益跟踪字段
ALTER TABLE IF EXISTS logs ADD COLUMN IF NOT EXISTS merchant_id       INT;
ALTER TABLE IF EXISTS logs ADD COLUMN IF NOT EXISTS channel_id        INT;
ALTER TABLE IF EXISTS logs ADD COLUMN IF NOT EXISTS merchant_cost     BIGINT;  -- 商家成本 (Quota)
ALTER TABLE IF EXISTS logs ADD COLUMN IF NOT EXISTS distributor_price BIGINT;  -- 分站售价 (Quota)
ALTER TABLE IF EXISTS logs ADD COLUMN IF NOT EXISTS distributor_profit BIGINT; -- 分站利润 (Quota)

-- tokens 表: token 类型
ALTER TABLE IF EXISTS tokens ADD COLUMN IF NOT EXISTS token_type VARCHAR(20) DEFAULT 'normal'; -- normal/subscription/shared

-- 扩展字段索引
CREATE INDEX IF NOT EXISTS idx_users_merchant_id    ON users(merchant_id);
CREATE INDEX IF NOT EXISTS idx_users_distributor_id ON users(distributor_id);
CREATE INDEX IF NOT EXISTS idx_channels_merchant_id ON channels(merchant_id);
CREATE INDEX IF NOT EXISTS idx_logs_merchant_id     ON logs(merchant_id);
CREATE INDEX IF NOT EXISTS idx_logs_channel_id      ON logs(channel_id);

-- =============================================================================
-- PART 2: LeapNode 扩展表 (18 张)
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. 商家表 (KOL / 渠道供应商)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS merchants (
    id              SERIAL PRIMARY KEY,
    user_id         INT REFERENCES users(id),
    merchant_name   VARCHAR(100),
    merchant_handle VARCHAR(50) UNIQUE,
    merchant_level  VARCHAR(20),                    -- gold/diamond
    deposit_amount  DECIMAL(10,2),
    commission_rate DECIMAL(5,4) DEFAULT 0.30,
    balance         DECIMAL(10,2) DEFAULT 0,
    total_revenue   DECIMAL(10,2) DEFAULT 0,
    total_requests  BIGINT DEFAULT 0,
    status          INT DEFAULT 1,                  -- 1=正常 0=禁用
    created_at      TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_merchants_user_id ON merchants(user_id);
CREATE INDEX IF NOT EXISTS idx_merchants_status  ON merchants(status);

-- -----------------------------------------------------------------------------
-- 2. 分站配置表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS distributor_sites (
    id                  SERIAL PRIMARY KEY,
    owner_id            INT REFERENCES users(id),
    slug                VARCHAR(50) UNIQUE,
    name                VARCHAR(100),
    custom_domain       VARCHAR(100),
    logo_url            VARCHAR(255),
    theme               VARCHAR(50) DEFAULT 'default',
    markup_mode         VARCHAR(20) DEFAULT 'global', -- global/per_model
    global_markup_ratio DECIMAL(5,2),
    status              INT DEFAULT 1,
    expire_at           TIMESTAMP,
    created_at          TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_distributor_sites_owner_id      ON distributor_sites(owner_id);
CREATE INDEX IF NOT EXISTS idx_distributor_sites_custom_domain ON distributor_sites(custom_domain);
CREATE INDEX IF NOT EXISTS idx_distributor_sites_status        ON distributor_sites(status);

-- -----------------------------------------------------------------------------
-- 3. 分站模型上架表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS distributor_model_listings (
    id                    SERIAL PRIMARY KEY,
    distributor_id        INT REFERENCES distributor_sites(id),
    model_name            VARCHAR(100),
    channel_id            INT,                        -- 关联 New-API channels
    merchant_id           INT REFERENCES merchants(id),
    merchant_input_price  DECIMAL(10,6),
    merchant_output_price DECIMAL(10,6),
    selling_input_price   DECIMAL(10,6),
    selling_output_price  DECIMAL(10,6),
    status                INT DEFAULT 1,
    created_at            TIMESTAMP DEFAULT NOW(),
    UNIQUE(distributor_id, model_name, channel_id)
);
CREATE INDEX IF NOT EXISTS idx_dml_distributor_id ON distributor_model_listings(distributor_id);
CREATE INDEX IF NOT EXISTS idx_dml_merchant_id    ON distributor_model_listings(merchant_id);
CREATE INDEX IF NOT EXISTS idx_dml_channel_id     ON distributor_model_listings(channel_id);

-- -----------------------------------------------------------------------------
-- 4. 模型审批表 (核心: 商家上架/调价需总站审批)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS model_approvals (
    id            SERIAL PRIMARY KEY,
    merchant_id   INT REFERENCES merchants(id),
    model_name    VARCHAR(100),
    channel_id    INT,
    input_price   DECIMAL(10,6),
    output_price  DECIMAL(10,6),
    approval_type VARCHAR(20),                        -- create/update/delete
    status        INT DEFAULT 0,                      -- 0=待审核 1=通过 2=拒绝
    admin_id      INT,
    admin_remark  TEXT,
    submitted_at  TIMESTAMP DEFAULT NOW(),
    processed_at  TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_model_approvals_merchant_id ON model_approvals(merchant_id);
CREATE INDEX IF NOT EXISTS idx_model_approvals_status      ON model_approvals(status);

-- -----------------------------------------------------------------------------
-- 5. KOL / 商家 / 分站申请表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_applications (
    id             SERIAL PRIMARY KEY,
    user_id        INT REFERENCES users(id),
    apply_type     VARCHAR(20),                       -- merchant/distributor
    company_name   VARCHAR(200),
    contact_info   TEXT,
    deposit_amount DECIMAL(10,2),
    status         INT DEFAULT 0,                     -- 0=待审核 1=通过 2=拒绝
    admin_remark   TEXT,
    applied_at     TIMESTAMP DEFAULT NOW(),
    processed_at   TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_user_applications_user_id ON user_applications(user_id);
CREATE INDEX IF NOT EXISTS idx_user_applications_status  ON user_applications(status);

-- -----------------------------------------------------------------------------
-- 6. 套餐表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS packages (
    id             SERIAL PRIMARY KEY,
    distributor_id INT DEFAULT 0,                     -- 0=总站套餐
    name           VARCHAR(100),
    description    TEXT,                              -- 套餐描述
    price          DECIMAL(10,2),
    original_price DECIMAL(10,2),
    quota_amount   BIGINT,                            -- Quota 体系 (500000=$1)
    duration_days  INT,
    reset_period   VARCHAR(20),                       -- none/daily/weekly/monthly
    enabled        BOOLEAN DEFAULT TRUE,
    created_at     TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_packages_distributor_id ON packages(distributor_id);
CREATE INDEX IF NOT EXISTS idx_packages_enabled        ON packages(enabled);

-- -----------------------------------------------------------------------------
-- 7. 订阅表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS subscriptions (
    id            SERIAL PRIMARY KEY,
    user_id       INT REFERENCES users(id),
    package_id    INT REFERENCES packages(id),
    status        INT DEFAULT 1,                      -- 1=有效 0=过期 2=取消
    start_at      TIMESTAMP,
    expire_at     TIMESTAMP,
    remain_quota  BIGINT,
    next_reset_at TIMESTAMP,
    created_at    TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id    ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_package_id ON subscriptions(package_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status     ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_expire_at  ON subscriptions(expire_at);

-- -----------------------------------------------------------------------------
-- 8. 充值订单表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS topup_orders (
    id             VARCHAR(32) PRIMARY KEY,
    user_id        INT REFERENCES users(id),
    amount         DECIMAL(10,2),
    quota          BIGINT,
    payment_method VARCHAR(20),                        -- epay/stripe/crypto/redeem
    trade_no       VARCHAR(100),
    status         INT DEFAULT 0,                      -- 0=待支付 1=已支付 2=失败
    paid_at        TIMESTAMP,
    created_at     TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_topup_orders_user_id  ON topup_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_topup_orders_status   ON topup_orders(status);
CREATE INDEX IF NOT EXISTS idx_topup_orders_trade_no ON topup_orders(trade_no);

-- -----------------------------------------------------------------------------
-- 9. 兑换码表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS redeem_codes (
    code           VARCHAR(32) PRIMARY KEY,
    distributor_id INT DEFAULT 0,
    quota          BIGINT,
    batch_name     VARCHAR(100),
    used_by        INT,
    used_at        TIMESTAMP,
    status         INT DEFAULT 0,                      -- 0=未使用 1=已使用 2=作废
    expire_at      TIMESTAMP,
    created_at     TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_redeem_codes_distributor_id ON redeem_codes(distributor_id);
CREATE INDEX IF NOT EXISTS idx_redeem_codes_status         ON redeem_codes(status);
CREATE INDEX IF NOT EXISTS idx_redeem_codes_batch_name     ON redeem_codes(batch_name);

-- -----------------------------------------------------------------------------
-- 10. 返佣记录表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS affiliate_earnings (
    id              BIGSERIAL PRIMARY KEY,
    user_id         INT REFERENCES users(id),        -- 邀请人
    invitee_id      INT REFERENCES users(id),        -- 被邀请人
    order_id        VARCHAR(32),
    quota           BIGINT,
    commission_rate DECIMAL(5,4),
    created_at      TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_affiliate_earnings_user_id    ON affiliate_earnings(user_id);
CREATE INDEX IF NOT EXISTS idx_affiliate_earnings_invitee_id ON affiliate_earnings(invitee_id);
CREATE INDEX IF NOT EXISTS idx_affiliate_earnings_order_id   ON affiliate_earnings(order_id);

-- -----------------------------------------------------------------------------
-- 11. 提现申请表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS affiliate_payouts (
    id             BIGSERIAL PRIMARY KEY,
    user_id        INT REFERENCES users(id),
    amount         DECIMAL(10,2),
    payment_method VARCHAR(50),
    account_info   TEXT,
    status         INT DEFAULT 0,                      -- 0=待审核 1=通过 2=拒绝 3=已打款
    admin_remark   TEXT,
    processed_at   TIMESTAMP,
    created_at     TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_affiliate_payouts_user_id ON affiliate_payouts(user_id);
CREATE INDEX IF NOT EXISTS idx_affiliate_payouts_status  ON affiliate_payouts(status);

-- -----------------------------------------------------------------------------
-- 12. 密钥分组表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS key_groups (
    id                SERIAL PRIMARY KEY,
    distributor_id    INT DEFAULT 0,
    name              VARCHAR(100),
    discount_ratio    DECIMAL(4,2),
    leo_threshold     BIGINT,
    locked_user_count INT DEFAULT 0,
    included_models   TEXT[],
    status            INT DEFAULT 1,
    created_at        TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_key_groups_distributor_id ON key_groups(distributor_id);
CREATE INDEX IF NOT EXISTS idx_key_groups_status         ON key_groups(status);

-- -----------------------------------------------------------------------------
-- 13. 订阅共享商品表 (拼车进货)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS shared_subscription_products (
    id          SERIAL PRIMARY KEY,
    merchant_id INT REFERENCES merchants(id),
    name        VARCHAR(100),
    provider    VARCHAR(50),                          -- openai/anthropic/...
    plan        VARCHAR(50),
    price_ratio DECIMAL(4,2),
    model_count INT DEFAULT 0,
    status      INT DEFAULT 1,
    created_at  TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ssp_merchant_id ON shared_subscription_products(merchant_id);
CREATE INDEX IF NOT EXISTS idx_ssp_status      ON shared_subscription_products(status);

-- -----------------------------------------------------------------------------
-- 14. 发票表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS invoices (
    id             VARCHAR(32) PRIMARY KEY,
    user_id        INT REFERENCES users(id),
    distributor_id INT DEFAULT 0,
    order_ids      JSONB,
    invoice_type   INT,                                -- 1=个人 2=企业
    company_name   VARCHAR(200),
    amount_usd     DECIMAL(10,2),
    status         INT DEFAULT 0,                      -- 0=待处理 1=已开具 2=拒绝
    processed_at   TIMESTAMP,
    created_at     TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_invoices_user_id        ON invoices(user_id);
CREATE INDEX IF NOT EXISTS idx_invoices_distributor_id ON invoices(distributor_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status         ON invoices(status);

-- -----------------------------------------------------------------------------
-- 15. 下级分站关系表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sub_distributor_relations (
    id                    SERIAL PRIMARY KEY,
    parent_distributor_id INT,
    child_distributor_id  INT,
    commission_rate       DECIMAL(5,4),
    status                INT DEFAULT 1,
    created_at            TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sdr_parent ON sub_distributor_relations(parent_distributor_id);
CREATE INDEX IF NOT EXISTS idx_sdr_child  ON sub_distributor_relations(child_distributor_id);

-- -----------------------------------------------------------------------------
-- 16. 工单表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tickets (
    id          SERIAL PRIMARY KEY,
    user_id     INT REFERENCES users(id),
    ticket_type VARCHAR(50),
    title       VARCHAR(200),
    content     TEXT,
    priority    VARCHAR(20),                           -- low/normal/high/urgent
    status      INT DEFAULT 0,                         -- 0=待处理 1=处理中 2=已解决 3=关闭
    admin_reply TEXT,
    created_at  TIMESTAMP DEFAULT NOW(),
    resolved_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status  ON tickets(status);

-- -----------------------------------------------------------------------------
-- 17. 公告表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS announcements (
    id                SERIAL PRIMARY KEY,
    title             VARCHAR(200),
    content           TEXT,
    announcement_type VARCHAR(20),                     -- info/warning/promo
    target_users      VARCHAR(20),                     -- all/user/merchant/distributor
    priority          VARCHAR(20),
    status            INT DEFAULT 1,                   -- 1=发布 0=下线
    published_at      TIMESTAMP,
    created_at        TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_announcements_status       ON announcements(status);
CREATE INDEX IF NOT EXISTS idx_announcements_target_users ON announcements(target_users);

-- -----------------------------------------------------------------------------
-- 18. 邀请奖励配置表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS referral_config (
    id                SERIAL PRIMARY KEY,
    reward_type       VARCHAR(20),                     -- register/first_topup
    reward_amount     DECIMAL(10,2),
    is_platform_funded BOOLEAN DEFAULT TRUE,
    enabled           BOOLEAN DEFAULT TRUE,
    created_at        TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_referral_config_reward_type ON referral_config(reward_type);

-- =============================================================================
-- PART 3: 初始化种子数据 (邀请奖励默认配置)
--   注册奖励 $0.10 = 50,000 Quota; 首充奖励 $1.00 = 500,000 Quota
-- =============================================================================
INSERT INTO referral_config (reward_type, reward_amount, is_platform_funded, enabled)
VALUES
    ('register',     0.10, TRUE, TRUE),
    ('first_topup',  1.00, TRUE, TRUE)
ON CONFLICT DO NOTHING;

-- =============================================================================
-- PART 4: key_groups 补列 (前端 Tokens.jsx 消费字段, 幂等)
--   原始建表仅含 discount_ratio/leo_threshold; 以下为展示层所需的扩展列
-- =============================================================================
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS vendor_category VARCHAR(50);
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS price_discount  DECIMAL(6,4);
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS rmb_per_usd     DECIMAL(10,4);
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS discount_label  VARCHAR(100);
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS description     TEXT;
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS tags            VARCHAR(255);
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS is_recommended  BOOLEAN DEFAULT FALSE;
ALTER TABLE IF EXISTS key_groups ADD COLUMN IF NOT EXISTS is_unavailable  BOOLEAN DEFAULT FALSE;

-- =============================================================================
-- PART 5: USDT/加密货币充值订单表 (固定收款地址 + 动态金额尾数匹配模式)
--   固定平台钱包收款, 靠"每单唯一 pay_amount"匹配到具体订单。
--   distributor_id 预留多租户隔离 (0=总站); USDT 总站/分站可共用总站配置,
--   支付宝/Stripe 各自配置(本批不做, 但字段结构预留归属区分)。
-- =============================================================================
CREATE TABLE IF NOT EXISTS crypto_topup_orders (
    id             BIGSERIAL PRIMARY KEY,
    trade_no       VARCHAR(64)  NOT NULL UNIQUE,       -- 唯一订单号 (业务主键)
    user_id        INT          NOT NULL,             -- 下单用户 (users.id)
    distributor_id INT          NOT NULL DEFAULT 0,    -- 0=总站, >0=分站, 多租户隔离
    chain          VARCHAR(20)  NOT NULL,             -- tron/eth/bsc/polygon/solana (前端 CRYPTO_NETWORKS key)
    token          VARCHAR(20)  NOT NULL,             -- usdt/usdc
    currency       VARCHAR(10)  NOT NULL DEFAULT 'USD', -- 用户下单法币 USD/CNY
    base_amount    DECIMAL(20,6) NOT NULL,            -- 用户输入的原始金额(法币/USDT)
    pay_amount     DECIMAL(20,6) NOT NULL,            -- 精确应付金额 = base + 随机尾数 (同链同地址唯一)
    quota          BIGINT       NOT NULL DEFAULT 0,    -- 到账后应加的 quota (Q=500000/$)
    wallet_address VARCHAR(128) NOT NULL,             -- 该链平台收款地址(固定, 可配置)
    tx_hash        VARCHAR(128) NOT NULL DEFAULT '',   -- 用户提交/对账命中的链上交易 hash
    status         VARCHAR(20)  NOT NULL DEFAULT 'pending', -- pending/reviewing/success/expired/failed
    tier_index     INT          NOT NULL DEFAULT -1,   -- 前端预设档位下标(可选, -1=无)
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expire_at      TIMESTAMPTZ  NOT NULL,             -- 过期时间 (created + crypto_expiry_minutes)
    paid_at        TIMESTAMPTZ                         -- 确认到账时间 (nullable)
);
CREATE INDEX IF NOT EXISTS idx_crypto_topup_user       ON crypto_topup_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_crypto_topup_status     ON crypto_topup_orders(status);
CREATE INDEX IF NOT EXISTS idx_crypto_topup_dist       ON crypto_topup_orders(distributor_id);
-- 动态金额匹配的核心索引: 同链+同收款地址+状态 上按 pay_amount 查重/匹配
CREATE INDEX IF NOT EXISTS idx_crypto_topup_match      ON crypto_topup_orders(chain, wallet_address, status, pay_amount);

-- =============================================================================
-- PART 6: 分站支付配置表 (Week 10.3)
--   分站在"站点与开发 > 站点设置"配置自己的支付渠道:
--     - usdt:   分站勾选启用, 复用总站钱包配置 (config 可留空)
--     - alipay: 分站配置自己的易支付账号 (config: pid/key/api_url)
--     - stripe: 分站配置自己的 Stripe Key (config: publishable_key/secret_key)
--   一个分站每种支付类型仅一条记录 (UNIQUE)。
-- =============================================================================
CREATE TABLE IF NOT EXISTS distributor_payment_configs (
    id             SERIAL PRIMARY KEY,
    distributor_id INT          NOT NULL,
    payment_type   VARCHAR(20)  NOT NULL,              -- 'usdt' / 'alipay' / 'stripe'
    enabled        BOOLEAN      DEFAULT FALSE,
    config         JSONB,                              -- 易支付账号 / Stripe Key 等
    created_at     TIMESTAMP    DEFAULT NOW(),
    updated_at     TIMESTAMP    DEFAULT NOW(),
    UNIQUE(distributor_id, payment_type)
);
CREATE INDEX IF NOT EXISTS idx_dpc_distributor_id ON distributor_payment_configs(distributor_id);

-- =============================================================================
-- PART 7: 分站 API 密钥表 (Week 10.3)
--   "站点与开发 > API密钥"页面: 分站可生成/删除对接自家系统的 API Key。
-- =============================================================================
CREATE TABLE IF NOT EXISTS distributor_api_keys (
    id             SERIAL PRIMARY KEY,
    distributor_id INT          NOT NULL,
    name           VARCHAR(100),
    api_key        VARCHAR(128) NOT NULL UNIQUE,
    status         INT          DEFAULT 1,             -- 1=启用 0=禁用
    last_used_at   TIMESTAMP,
    created_at     TIMESTAMP    DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_dak_distributor_id ON distributor_api_keys(distributor_id);

-- =============================================================================
-- PART 8: 分站站点设置补列 (Week 10.3)
--   distributor_sites 增加 SEO 字段 (前端站点设置页消费)。
-- =============================================================================
ALTER TABLE IF EXISTS distributor_sites ADD COLUMN IF NOT EXISTS seo_title       VARCHAR(200);
ALTER TABLE IF EXISTS distributor_sites ADD COLUMN IF NOT EXISTS seo_keywords    VARCHAR(255);
ALTER TABLE IF EXISTS distributor_sites ADD COLUMN IF NOT EXISTS seo_description TEXT;
ALTER TABLE IF EXISTS distributor_sites ADD COLUMN IF NOT EXISTS balance         DECIMAL(12,2) DEFAULT 0;
ALTER TABLE IF EXISTS distributor_sites ADD COLUMN IF NOT EXISTS total_revenue   DECIMAL(12,2) DEFAULT 0;

-- =============================================================================
-- PART 9: 渠道分组表 (Batch 6)
--   分站可创建渠道分组（如"高优先级"、"备用池"），关联多个 channels，
--   用于灵活的流量路由和负载均衡。
-- =============================================================================

-- 渠道分组表
CREATE TABLE IF NOT EXISTS channel_groups (
    id             SERIAL PRIMARY KEY,
    distributor_id INT NOT NULL,                      -- 所属分站（0=总站）
    name           VARCHAR(100) NOT NULL,             -- 分组名称
    description    TEXT,                              -- 分组描述
    priority       INT DEFAULT 0,                     -- 优先级（数字越大越高）
    enabled        BOOLEAN DEFAULT TRUE,              -- 是否启用
    created_at     TIMESTAMP DEFAULT NOW(),
    updated_at     TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_channel_groups_distributor ON channel_groups(distributor_id);
CREATE INDEX IF NOT EXISTS idx_channel_groups_enabled     ON channel_groups(enabled);

-- 渠道分组与渠道关联表
CREATE TABLE IF NOT EXISTS channel_group_relations (
    id         SERIAL PRIMARY KEY,
    group_id   INT NOT NULL,                          -- 关联 channel_groups.id
    channel_id INT NOT NULL,                          -- 关联 channels.id
    weight     INT DEFAULT 1,                         -- 权重（用于负载均衡）
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_channel_group_relations_group   ON channel_group_relations(group_id);
CREATE INDEX IF NOT EXISTS idx_channel_group_relations_channel ON channel_group_relations(channel_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_group_relations_unique ON channel_group_relations(group_id, channel_id);

COMMIT;
