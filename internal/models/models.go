// Package models 定义 LeapNode 的 GORM 数据模型。
//
// 每个 struct 对应 migrations/001_initial_schema.sql 中的一张扩展表。
// 此处先定义核心模型骨架, 后续按模块补全字段与关联关系。
package models

import (
	"time"

	"gorm.io/gorm"
)

// Merchant 商家 (KOL / 渠道供应商) -> merchants
type Merchant struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `json:"user_id"`
	MerchantName   string    `json:"merchant_name"`
	MerchantHandle string    `gorm:"uniqueIndex" json:"merchant_handle"`
	MerchantLevel  string    `json:"merchant_level"` // gold/diamond
	DepositAmount  float64   `json:"deposit_amount"`
	CommissionRate float64   `gorm:"default:0.30" json:"commission_rate"`
	Balance        float64   `json:"balance"`
	TotalRevenue   float64   `json:"total_revenue"`
	TotalRequests  int64     `json:"total_requests"`
	Status         int       `gorm:"default:1" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// DistributorSite 分站配置 -> distributor_sites
type DistributorSite struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	OwnerID           uint      `json:"owner_id"`
	Slug              string    `gorm:"uniqueIndex" json:"slug"`
	Name              string    `json:"name"`
	CustomDomain      string    `json:"custom_domain"`
	LogoURL           string    `json:"logo_url"`
	Theme             string    `gorm:"default:default" json:"theme"`
	MarkupMode        string    `gorm:"default:global" json:"markup_mode"`
	GlobalMarkupRatio float64   `json:"global_markup_ratio"`
	Status            int       `gorm:"default:1" json:"status"`
	ExpireAt          time.Time `json:"expire_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// Package 套餐 -> packages
type Package struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	DistributorID int       `gorm:"default:0" json:"distributor_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	OriginalPrice float64   `json:"original_price"`
	QuotaAmount   int64     `json:"quota_amount"`
	DurationDays  int       `json:"duration_days"`
	ResetPeriod   string    `json:"reset_period"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
}

// Subscription 订阅 -> subscriptions
type Subscription struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `json:"user_id"`
	PackageID   uint      `json:"package_id"`
	Status      int       `gorm:"default:1" json:"status"`
	StartAt     time.Time `json:"start_at"`
	ExpireAt    time.Time `json:"expire_at"`
	RemainQuota int64     `json:"remain_quota"`
	NextResetAt time.Time `json:"next_reset_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// KeyGroup 密钥分组 -> key_groups
type KeyGroup struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	DistributorID   int       `gorm:"default:0" json:"distributor_id"`
	Name            string    `json:"name"`
	VendorCategory  string    `json:"vendor_category"`
	DiscountRatio   float64   `json:"discount_ratio"`
	PriceDiscount   float64   `json:"price_discount"`
	RmbPerUsd       float64   `json:"rmb_per_usd"`
	DiscountLabel   string    `json:"discount_label"`
	Description     string    `json:"description"`
	Tags            string    `json:"tags"`
	IsRecommended   bool      `json:"is_recommended"`
	IsUnavailable   bool      `json:"is_unavailable"`
	LeoThreshold    int64     `json:"leo_threshold"`
	LockedUserCount int       `gorm:"default:0" json:"locked_user_count"`
	IncludedModels  []string  `gorm:"type:text[]" json:"included_models"`
	Status          int       `gorm:"default:1" json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// DistributorModelListing 分站模型上架表 -> distributor_model_listings
type DistributorModelListing struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	DistributorID       int       `json:"distributor_id"`
	ModelName           string    `json:"model_name"`
	ChannelID           int       `json:"channel_id"`
	MerchantID          int       `json:"merchant_id"`
	MerchantInputPrice  float64   `json:"merchant_input_price"`
	MerchantOutputPrice float64   `json:"merchant_output_price"`
	SellingInputPrice   float64   `json:"selling_input_price"`
	SellingOutputPrice  float64   `json:"selling_output_price"`
	Status              int       `gorm:"default:1" json:"status"`
	CreatedAt           time.Time `json:"created_at"`
}

// Announcement 公告表 -> announcements
type Announcement struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	AnnouncementType string    `json:"announcement_type"`
	TargetUsers      string    `json:"target_users"`
	Priority         string    `json:"priority"`
	Status           int       `gorm:"default:1" json:"status"`
	PublishedAt      time.Time `json:"published_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// User New-API 原生用户表 (含 LeapNode 扩展字段) -> users
//
// 字段命名严格对齐 New-API 的 users 表列名, 以保证与 New-API AutoMigrate
// 建出的表 100% 兼容 (不新增/不改动 New-API 已有列)。
// LeapNode 扩展列 (user_level/merchant_id/distributor_id/language) 由
// migrations/001_initial_schema.sql 以幂等 ALTER 追加。
type User struct {
	ID              int    `gorm:"primaryKey;column:id" json:"id"`
	Username        string `gorm:"column:username" json:"username"`
	Password        string `gorm:"column:password" json:"-"` // 绝不序列化到响应
	DisplayName     string `gorm:"column:display_name" json:"display_name"`
	Role            int    `gorm:"column:role" json:"role"`     // 1=普通用户 10=管理员 100=超管 (New-API 语义)
	Status          int    `gorm:"column:status" json:"status"` // 1=正常 2=禁用
	Email           string `gorm:"column:email" json:"email"`
	Quota           int64  `gorm:"column:quota" json:"quota"`
	UsedQuota       int64  `gorm:"column:used_quota" json:"used_quota"`
	RequestCount    int    `gorm:"column:request_count" json:"request_count"`
	Group           string `gorm:"column:group" json:"group"`
	AffCode         string `gorm:"column:aff_code" json:"aff_code"`
	AffCount        int    `gorm:"column:aff_count" json:"aff_count"`
	AffQuota        int64  `gorm:"column:aff_quota" json:"aff_quota"`
	AffHistoryQuota int64  `gorm:"column:aff_history" json:"aff_history_quota"`
	InviterID       int    `gorm:"column:inviter_id" json:"inviter_id"`
	CreatedTime     int64  `gorm:"autoCreateTime;column:created_at" json:"created_time"` // New-API 列名 created_at, 前端契约字段 created_time

	// --- LeapNode 扩展列 ---
	UserLevel     int    `gorm:"column:user_level;default:0" json:"user_level"`
	MerchantID    *int   `gorm:"column:merchant_id" json:"merchant_id,omitempty"`
	DistributorID *int   `gorm:"column:distributor_id" json:"distributor_id,omitempty"`
	Language      string `gorm:"column:language" json:"language,omitempty"`
}

func (User) TableName() string {
	return "users"
}

// --- New-API native tables (read-only for these endpoints) ---

// Ability New-API 原生模型能力表 (只读)
type Ability struct {
	ChannelID int     `gorm:"column:channel_id" json:"channel_id"`
	Group     string  `gorm:"column:group" json:"group"`
	Model     string  `gorm:"column:model" json:"model"`
	Priority  int     `gorm:"column:priority" json:"priority"`
	Enabled   bool    `gorm:"column:enabled" json:"enabled"`
	Weight    int     `gorm:"column:weight" json:"weight"`
	GroupID   *string `gorm:"column:group_id" json:"group_id"`
}

func (Ability) TableName() string {
	return "abilities"
}

// Channel New-API 原生渠道表 (已扩展字段)
type Channel struct {
	ID          int       `gorm:"primaryKey;column:id" json:"id"`
	Type        int       `gorm:"column:type" json:"type"`
	Key         string    `gorm:"column:key" json:"key"`
	Status      int       `gorm:"column:status" json:"status"`
	Name        string    `gorm:"column:name" json:"name"`
	Group       string    `gorm:"column:group" json:"group"`
	Models      string    `gorm:"column:models" json:"models"`
	MerchantID  *int      `gorm:"column:merchant_id" json:"merchant_id"`
	InputPrice  *float64  `gorm:"column:input_price" json:"input_price"`
	OutputPrice *float64  `gorm:"column:output_price" json:"output_price"`
	CreatedAt   time.Time `gorm:"column:created_time" json:"created_at"`
}

func (Channel) TableName() string {
	return "channels"
}

// Token New-API 原生令牌表 -> tokens
//
// 字段命名严格对齐 New-API 的 tokens 表列 (~/Downloads/new-api_code/.../model/token.go),
// 以保证与 New-API AutoMigrate 建出的表 100% 兼容 (不新增/不改动 New-API 已有列)。
//
// 注意: tokens 表由 New-API AutoMigrate 建立, 已存在。LeapNode 只做 CRUD,
// 绝不 AutoMigrate 或改列。此处仅复用其原生列 (AutoGroups 前端不消费, 故不映射)。
type Token struct {
	Id                 int            `json:"id" gorm:"primaryKey;column:id"`
	UserId             int            `json:"user_id" gorm:"column:user_id;index"`
	Key                string         `json:"key" gorm:"column:key;type:varchar(128);uniqueIndex"`
	Status             int            `json:"status" gorm:"column:status;default:1"`
	Name               string         `json:"name" gorm:"column:name;index"`
	CreatedTime        int64          `json:"created_time" gorm:"column:created_time;bigint"`
	AccessedTime       int64          `json:"accessed_time" gorm:"column:accessed_time;bigint"`
	ExpiredTime        int64          `json:"expired_time" gorm:"column:expired_time;bigint;default:-1"` // -1 表永不过期
	RemainQuota        int            `json:"remain_quota" gorm:"column:remain_quota;default:0"`
	UnlimitedQuota     bool           `json:"unlimited_quota" gorm:"column:unlimited_quota"`
	ModelLimitsEnabled bool           `json:"model_limits_enabled" gorm:"column:model_limits_enabled"`
	ModelLimits        string         `json:"model_limits" gorm:"column:model_limits;type:text"`
	AllowIps           *string        `json:"allow_ips" gorm:"column:allow_ips;default:''"`
	UsedQuota          int            `json:"used_quota" gorm:"column:used_quota;default:0"`
	Group              string         `json:"group" gorm:"column:group;default:''"`
	CrossGroupRetry    bool           `json:"cross_group_retry" gorm:"column:cross_group_retry"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"` // 软删除 (New-API 原生列)
}

func (Token) TableName() string {
	return "tokens"
}

// Redemption New-API 原生兑换码表 -> redemptions
//
// 字段命名严格对齐 New-API 的 redemptions 表列 (~/Downloads/new-api_code/.../model/redemption.go),
// 以保证与 New-API AutoMigrate 建出的表 100% 兼容。
//
// 注意: redemptions 表由 New-API AutoMigrate 建立, 已存在。LeapNode 只做兑换 CRUD,
// 绝不 AutoMigrate 或改列。此处仅复用其原生列。
// Status 语义: 1=未使用(enabled) 2=已禁用(disabled) 3=已使用(used)
type Redemption struct {
	Id           int            `json:"id" gorm:"primaryKey;column:id"`
	UserId       int            `json:"user_id" gorm:"column:user_id"`
	Key          string         `json:"key" gorm:"column:key;type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"column:status;default:1"`
	Name         string         `json:"name" gorm:"column:name;index"`
	Quota        int            `json:"quota" gorm:"column:quota;default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"column:created_time;bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"column:redeemed_time;bigint"`
	UsedUserId   int            `json:"used_user_id" gorm:"column:used_user_id"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"column:expired_time;bigint"` // 0 表示不过期
}

func (Redemption) TableName() string {
	return "redemptions"
}

// TopupOrder 充值订单表 -> topup_orders
type TopupOrder struct {
	ID            string  `gorm:"primaryKey;column:id" json:"id"`
	UserID        int     `gorm:"column:user_id" json:"user_id"`
	Amount        float64 `gorm:"column:amount" json:"amount"`
	Quota         int64   `gorm:"column:quota" json:"quota"`
	PaymentMethod string  `gorm:"column:payment_method" json:"payment_method"`
	TradeNo       string  `gorm:"column:trade_no" json:"trade_no"`
	Status        int     `gorm:"column:status;default:0" json:"status"` // 0=待支付 1=已支付 2=失败
	PaidAt        *int64  `gorm:"column:paid_at" json:"paid_at,omitempty"`
	CreatedAt     int64   `gorm:"column:created_at;autoCreateTime:milli" json:"created_at"`
}

func (TopupOrder) TableName() string {
	return "topup_orders"
}

// CryptoTopupOrder USDT/加密货币充值订单 -> crypto_topup_orders
//
// 固定收款地址 + 动态金额尾数匹配模式:
//   - 平台配置固定钱包 (wallet_address), 不给每个用户生成独立地址
//   - 靠"每单唯一 pay_amount"在 (chain + wallet_address + 时间窗) 内匹配订单
//   - pay_amount = base_amount + 随机小数尾数, 保证同链同地址 pending 订单金额不撞
//
// distributor_id 预留多租户隔离 (0=总站): USDT 总站/分站可共用总站配置。
// Status 语义: pending(待支付) reviewing(已提交hash待对账) success(已到账)
//
//	expired(超时未付) failed(对账失败)。
type CryptoTopupOrder struct {
	ID            int64      `gorm:"primaryKey;column:id" json:"id"`
	TradeNo       string     `gorm:"column:trade_no;uniqueIndex" json:"trade_no"`
	UserID        int        `gorm:"column:user_id" json:"user_id"`
	DistributorID int        `gorm:"column:distributor_id;default:0" json:"distributor_id"`
	Chain         string     `gorm:"column:chain" json:"chain"` // tron/eth/bsc/polygon/solana
	Token         string     `gorm:"column:token" json:"token"` // usdt/usdc
	Currency      string     `gorm:"column:currency;default:USD" json:"currency"`
	BaseAmount    float64    `gorm:"column:base_amount" json:"base_amount"`
	PayAmount     float64    `gorm:"column:pay_amount" json:"pay_amount"`
	Quota         int64      `gorm:"column:quota;default:0" json:"quota"`
	WalletAddress string     `gorm:"column:wallet_address" json:"wallet_address"`
	TxHash        string     `gorm:"column:tx_hash;default:''" json:"tx_hash"`
	Status        string     `gorm:"column:status;default:pending" json:"status"`
	TierIndex     int        `gorm:"column:tier_index;default:-1" json:"tier_index"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	ExpireAt      time.Time  `gorm:"column:expire_at" json:"expire_at"`
	PaidAt        *time.Time `gorm:"column:paid_at" json:"paid_at,omitempty"`
}

func (CryptoTopupOrder) TableName() string {
	return "crypto_topup_orders"
}

// AffHistory 返佣记录表 -> affiliate_earnings
type AffHistory struct {
	ID             int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID         int       `gorm:"column:user_id" json:"user_id"`         // 邀请人
	InviteeID      int       `gorm:"column:invitee_id" json:"invitee_id"`   // 被邀请人
	OrderID        string    `gorm:"column:order_id" json:"order_id"`       // 关联订单号
	Quota          int64     `gorm:"column:quota" json:"quota"`             // 收益金额 (Quota单位)
	CommissionRate float64   `gorm:"column:commission_rate" json:"commission_rate"`
	EventType      string    `gorm:"column:event_type" json:"event_type"`   // register/first_topup/consumption/transfer
	Status         int       `gorm:"column:status;default:1" json:"status"` // 0=pending 1=completed 2=rejected
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AffHistory) TableName() string {
	return "affiliate_earnings"
}

// ReferralConfig 邀请奖励配置表 -> referral_config
type ReferralConfig struct {
	ID               int       `gorm:"primaryKey;column:id" json:"id"`
	RewardType       string    `gorm:"column:reward_type" json:"reward_type"` // register/first_topup/consumption_rate/level2_rate
	RewardAmount     float64   `gorm:"column:reward_amount" json:"reward_amount"`
	IsPlatformFunded bool      `gorm:"column:is_platform_funded" json:"is_platform_funded"`
	Enabled          bool      `gorm:"column:enabled" json:"enabled"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (ReferralConfig) TableName() string {
	return "referral_config"
}

// KolApplication 商家/分站申请表 -> user_applications
type KolApplication struct {
	ID            int       `gorm:"primaryKey;column:id" json:"id"`
	UserID        int       `gorm:"column:user_id" json:"user_id"`
	ApplyType     string    `gorm:"column:apply_type" json:"apply_type"` // merchant/distributor
	CompanyName   string    `gorm:"column:company_name" json:"company_name"`
	ContactInfo   string    `gorm:"column:contact_info" json:"contact_info"`
	DepositAmount float64   `gorm:"column:deposit_amount" json:"deposit_amount"`
	Status        int       `gorm:"column:status;default:0" json:"status"` // 0=pending 1=approved 2=rejected
	AdminRemark   string    `gorm:"column:admin_remark" json:"admin_remark"`
	AppliedAt     time.Time `gorm:"column:applied_at;autoCreateTime" json:"applied_at"`
	ProcessedAt   *time.Time `gorm:"column:processed_at" json:"processed_at,omitempty"`
}

func (KolApplication) TableName() string {
	return "user_applications"
}

// WithdrawalRequest 提现申请表 -> affiliate_payouts
type WithdrawalRequest struct {
	ID            int64      `gorm:"primaryKey;column:id" json:"id"`
	UserID        int        `gorm:"column:user_id" json:"user_id"`
	Amount        float64    `gorm:"column:amount" json:"amount"` // 美元金额
	PaymentMethod string     `gorm:"column:payment_method" json:"payment_method"`
	AccountInfo   string     `gorm:"column:account_info" json:"account_info"`
	Status        int        `gorm:"column:status;default:0" json:"status"` // 0=pending 1=approved 2=rejected 3=paid
	AdminRemark   string     `gorm:"column:admin_remark" json:"admin_remark"`
	ProcessedAt   *time.Time `gorm:"column:processed_at" json:"processed_at,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (WithdrawalRequest) TableName() string {
	return "affiliate_payouts"
}

// MerchantChannel 商家上游渠道 -> channels (扩展商家渠道查询)
// 使用 New-API 原生 Channel 类型，但只查询 merchant_id = 当前商家的记录
type MerchantChannel = Channel

// MerchantModel 商家模型（待审批/已上架）-> model_approvals
type MerchantModel struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	MerchantID   uint       `json:"merchant_id"`
	ModelName    string     `json:"model_name"`
	ChannelID    int        `json:"channel_id"`
	InputPrice   float64    `json:"input_price"`
	OutputPrice  float64    `json:"output_price"`
	ApprovalType string     `json:"approval_type"` // create/update/delete
	Status       int        `json:"status"`        // 0=待审核 1=通过 2=拒绝
	AdminID      *int       `json:"admin_id"`
	AdminRemark  string     `json:"admin_remark"`
	SubmittedAt  time.Time  `json:"submitted_at"`
	ProcessedAt  *time.Time `json:"processed_at"`
}

func (MerchantModel) TableName() string {
	return "model_approvals"
}

// MerchantRevenueRecord 商家收益记录 -> merchant_revenue_records
type MerchantRevenueRecord struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	MerchantID  int       `json:"merchant_id"`
	ModelName   string    `json:"model_name"`
	OrderID     string    `json:"order_id"`
	Revenue     float64   `json:"revenue"`     // 商家收益（美元）
	Cost        float64   `json:"cost"`        // 上游成本（美元）
	Profit      float64   `json:"profit"`      // 净利润（美元）
	RequestDate time.Time `json:"request_date"` // 请求日期（用于按日统计）
	CreatedAt   time.Time `json:"created_at"`
}

func (MerchantRevenueRecord) TableName() string {
	return "merchant_revenue_records"
}

// SupportTicket 工单 -> tickets
type SupportTicket struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     int        `json:"user_id"`
	TicketType string     `json:"ticket_type"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Priority   string     `json:"priority"` // low/normal/high/urgent
	Status     int        `json:"status"`   // 0=待处理 1=处理中 2=已解决 3=关闭
	AdminReply string     `json:"admin_reply"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

func (SupportTicket) TableName() string {
	return "tickets"
}

// AnnouncementRead 公告已读记录 -> announcement_reads
type AnnouncementRead struct {
	ID             int       `gorm:"primaryKey" json:"id"`
	UserID         int       `json:"user_id"`
	AnnouncementID int       `json:"announcement_id"`
	ReadAt         time.Time `json:"read_at"`
}

func (AnnouncementRead) TableName() string {
	return "announcement_reads"
}

// RedeemCode 兑换码 -> redeem_codes
type RedeemCode struct {
	Code           string     `gorm:"primaryKey" json:"code"`
	DistributorID  int        `json:"distributor_id"`
	Quota          int64      `json:"quota"`
	BatchName      string     `json:"batch_name"`
	UsedBy         *int       `json:"used_by"`
	UsedAt         *time.Time `json:"used_at"`
	Status         int        `json:"status"` // 0=未使用 1=已使用 2=作废
	ExpireAt       *time.Time `json:"expire_at"`
	CreatedAt      time.Time  `json:"created_at"`
	MerchantID     *int       `json:"merchant_id"` // 商家生成的兑换码
}

func (RedeemCode) TableName() string {
	return "redeem_codes"
}
