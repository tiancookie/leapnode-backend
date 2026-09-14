// Package models 定义 LeapNode 的 GORM 数据模型。
//
// 每个 struct 对应 migrations/001_initial_schema.sql 中的一张扩展表。
// 此处先定义核心模型骨架, 后续按模块补全字段与关联关系。
package models

import "time"

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
	ID               uint      `gorm:"primaryKey" json:"id"`
	DistributorID    int       `gorm:"default:0" json:"distributor_id"`
	Name             string    `json:"name"`
	VendorCategory   string    `json:"vendor_category"`
	DiscountRatio    float64   `json:"discount_ratio"`
	PriceDiscount    float64   `json:"price_discount"`
	RmbPerUsd        float64   `json:"rmb_per_usd"`
	DiscountLabel    string    `json:"discount_label"`
	Description      string    `json:"description"`
	Tags             string    `json:"tags"`
	IsRecommended    bool      `json:"is_recommended"`
	IsUnavailable    bool      `json:"is_unavailable"`
	LeoThreshold     int64     `json:"leo_threshold"`
	LockedUserCount  int       `gorm:"default:0" json:"locked_user_count"`
	IncludedModels   []string  `gorm:"type:text[]" json:"included_models"`
	Status           int       `gorm:"default:1" json:"status"`
	CreatedAt        time.Time `json:"created_at"`
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
