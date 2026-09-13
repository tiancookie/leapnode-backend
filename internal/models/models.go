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
	ID             uint      `gorm:"primaryKey" json:"id"`
	DistributorID  int       `gorm:"default:0" json:"distributor_id"`
	Name           string    `json:"name"`
	Price          float64   `json:"price"`
	OriginalPrice  float64   `json:"original_price"`
	QuotaAmount    int64     `json:"quota_amount"`
	DurationDays   int       `json:"duration_days"`
	ResetPeriod    string    `json:"reset_period"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
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

// TODO: 补全其余 14 张扩展表模型
//   ModelApproval, DistributorModelListing, UserApplication, TopupOrder,
//   RedeemCode, AffiliateEarning, AffiliatePayout, KeyGroup,
//   SharedSubscriptionProduct, Invoice, SubDistributorRelation,
//   Ticket, Announcement, ReferralConfig
