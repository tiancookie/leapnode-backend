// Package services 承载业务逻辑层。
package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"gorm.io/gorm"
)

// SiteService 公开站点信息服务
type SiteService struct {
	db *gorm.DB
}

// NewSiteService 创建 SiteService
func NewSiteService(db *gorm.DB) *SiteService {
	return &SiteService{db: db}
}

// SiteInfoResponse /api/dist/site/info 响应结构 (与前端 api.js previewResponse 对齐)
type SiteInfoResponse struct {
	Name                   string           `json:"name"`
	Logo                   string           `json:"logo,omitempty"`
	Favicon                string           `json:"favicon,omitempty"`
	ThemeTemplate          string           `json:"theme_template"`
	EnableTopup            bool             `json:"enable_topup"`
	TopUpLink              string           `json:"top_up_link,omitempty"`
	TopUpLinkName          string           `json:"top_up_link_name,omitempty"`
	AllowSubDist           bool             `json:"allow_sub_dist"`
	ShowAppMarket          bool             `json:"show_app_market"`
	ShowOfficialChannels   bool             `json:"show_official_channels"`
	HasOfficialChannels    bool             `json:"has_official_channels"`
	ShowSharedSubscriptions bool            `json:"show_shared_subscriptions"`
	CanViewProviders       bool             `json:"can_view_providers"`
	Currency               CurrencyInfo     `json:"currency"`
	Announcement           *AnnouncementInfo `json:"announcement,omitempty"`
	Footer                 string           `json:"footer,omitempty"`
}

// CurrencyInfo 货币配置
type CurrencyInfo struct {
	Code            string  `json:"code"`
	Symbol          string  `json:"symbol"`
	ExchangeRate    float64 `json:"exchange_rate"`
	UsdExchangeRate float64 `json:"usd_exchange_rate"`
}

// AnnouncementInfo 公告信息
type AnnouncementInfo struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

// GetSiteInfo 获取分站信息
func (s *SiteService) GetSiteInfo() (*SiteInfoResponse, error) {
	// 当前实现: 从环境变量 + 硬编码默认值返回
	// TODO: 从 distributor_sites 表读取真实配置
	info := &SiteInfoResponse{
		Name:                   getEnv("SITE_NAME", "LeapNode API Gateway"),
		Logo:                   getEnv("SITE_LOGO", ""),
		Favicon:                getEnv("SITE_FAVICON", ""),
		ThemeTemplate:          getEnv("SITE_THEME", "default"),
		EnableTopup:            getEnvBool("SITE_ENABLE_TOPUP", true),
		TopUpLink:              getEnv("SITE_TOPUP_LINK", ""),
		TopUpLinkName:          getEnv("SITE_TOPUP_LINK_NAME", ""),
		AllowSubDist:           getEnvBool("SITE_ALLOW_SUB_DIST", false),
		ShowAppMarket:          getEnvBool("SITE_SHOW_APP_MARKET", false),
		ShowOfficialChannels:   getEnvBool("SITE_SHOW_OFFICIAL_CHANNELS", false),
		HasOfficialChannels:    getEnvBool("SITE_HAS_OFFICIAL_CHANNELS", false),
		ShowSharedSubscriptions: getEnvBool("SITE_SHOW_SHARED_SUBSCRIPTIONS", false),
		CanViewProviders:       getEnvBool("SITE_CAN_VIEW_PROVIDERS", false),
		Currency: CurrencyInfo{
			Code:            getEnv("SITE_CURRENCY_CODE", "CNY"),
			Symbol:          getEnv("SITE_CURRENCY_SYMBOL", "¥"),
			ExchangeRate:    getEnvFloat("SITE_CURRENCY_RATE", 7.0),
			UsdExchangeRate: getEnvFloat("SITE_CURRENCY_USD_RATE", 7.0),
		},
		Footer: getEnv("SITE_FOOTER", ""),
	}

	// 查询最新公告 (status=1, target_users='all')
	var announcement models.Announcement
	err := s.db.Where("status = ? AND target_users = ?", 1, "all").
		Order("published_at DESC").
		First(&announcement).Error
	if err == nil && announcement.ID > 0 {
		info.Announcement = &AnnouncementInfo{
			Title:   announcement.Title,
			Content: announcement.Content,
			Type:    announcement.AnnouncementType,
		}
	}

	return info, nil
}

// ModelResponse 模型响应 (对齐前端 previewModels 字段)
type ModelResponse struct {
	ID                 interface{} `json:"id"`
	ModelName          string      `json:"model_name"`
	DisplayName        string      `json:"display_name"`
	Enabled            bool        `json:"enabled"`
	Category           string      `json:"category"`
	Status             string      `json:"status"`
	InputPrice         float64     `json:"input_price"`
	OutputPrice        float64     `json:"output_price"`
	CacheReadPrice     *float64    `json:"cache_read_price,omitempty"`
	CacheCreationPrice *float64    `json:"cache_creation_price,omitempty"`
}

// GetSiteModels 获取可用模型列表
func (s *SiteService) GetSiteModels() ([]ModelResponse, error) {
	// 从 abilities + channels 表聚合模型列表
	var abilities []models.Ability
	err := s.db.Where("enabled = ?", true).Find(&abilities).Error
	if err != nil {
		return []ModelResponse{}, nil // 空数组, 不报错
	}

	modelMap := make(map[string]*ModelResponse)
	for _, ab := range abilities {
		if _, exists := modelMap[ab.Model]; !exists {
			modelMap[ab.Model] = &ModelResponse{
				ID:          fmt.Sprintf("model-%s", ab.Model),
				ModelName:   ab.Model,
				DisplayName: ab.Model, // 默认与 model_name 相同
				Enabled:     true,
				Category:    ab.Group,
				Status:      "healthy",
				InputPrice:  0.0,
				OutputPrice: 0.0,
			}
		}
	}

	// 尝试从 channels 表补充价格
	var channels []models.Channel
	s.db.Where("status = ?", 1).Find(&channels)
	for _, ch := range channels {
		if ch.Models != "" {
			modelNames := strings.Split(ch.Models, ",")
			for _, mn := range modelNames {
				mn = strings.TrimSpace(mn)
				if m, exists := modelMap[mn]; exists {
					if ch.InputPrice != nil && *ch.InputPrice > 0 {
						m.InputPrice = *ch.InputPrice
					}
					if ch.OutputPrice != nil && *ch.OutputPrice > 0 {
						m.OutputPrice = *ch.OutputPrice
					}
				}
			}
		}
	}

	result := make([]ModelResponse, 0, len(modelMap))
	for _, m := range modelMap {
		result = append(result, *m)
	}
	return result, nil
}

// GetSitePricing 获取模型定价表 (与 GetSiteModels 相同, 前端可能用不同接口)
func (s *SiteService) GetSitePricing() ([]ModelResponse, error) {
	return s.GetSiteModels()
}

// PackageResponse 套餐响应 (对齐前端 previewPackages)
type PackageResponse struct {
	ID              interface{} `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description,omitempty"`
	Price           float64     `json:"price"`
	OriginalPrice   float64     `json:"original_price"`
	Duration        int         `json:"duration"`
	QuotaAmount     int64       `json:"quota_amount"`
	QuotaResetPeriod string     `json:"quota_reset_period"`
	Enabled         bool        `json:"enabled"`
}

// GetSitePackages 已迁移到 PackageService

// OfficialChannelResponse 官方渠道响应 (对齐 previewOfficialChannels)
type OfficialChannelResponse struct {
	OfficialChannelID       int                       `json:"official_channel_id"`
	Name                    string                    `json:"name"`
	Description             string                    `json:"description"`
	MaxFinalDiscount        float64                   `json:"max_final_discount"`
	MinAllowedFinalDiscount float64                   `json:"min_allowed_final_discount"`
	MinFinalPriceDiscount   float64                   `json:"min_final_price_discount"`
	UsableModelCount        int                       `json:"usable_model_count"`
	AvailableKeyCount       int                       `json:"available_key_count"`
	AvailableProviderCount  int                       `json:"available_provider_count"`
	KeyAvailability         float64                   `json:"key_availability"`
	ModelAvailability       float64                   `json:"model_availability"`
	Models                  []OfficialChannelModel    `json:"models"`
}

// OfficialChannelModel 官方渠道模型
type OfficialChannelModel struct {
	ID                  string  `json:"id"`
	ModelName           string  `json:"model_name"`
	Category            string  `json:"category"`
	PriceCurrency       string  `json:"price_currency"`
	OfficialInputPrice  float64 `json:"official_input_price"`
	OfficialOutputPrice float64 `json:"official_output_price"`
	FinalInputPrice     float64 `json:"final_input_price"`
	FinalOutputPrice    float64 `json:"final_output_price"`
	FinalPriceDiscount  float64 `json:"final_price_discount"`
	KeyCount            int     `json:"key_count"`
	AvailableKeyCount   int     `json:"available_key_count"`
	KeyAvailability     float64 `json:"key_availability"`
}

// GetSiteOfficialChannels 获取官方渠道列表
func (s *SiteService) GetSiteOfficialChannels(channelID *int) ([]OfficialChannelResponse, error) {
	// TODO: 当前返回空数组 (官方渠道功能未启用)
	// 后续从 key_groups + channels 聚合
	return []OfficialChannelResponse{}, nil
}

// AvailabilityResponse 可用性响应 (对齐 previewResponse)
type AvailabilityResponse struct {
	OfficialChannelID int                    `json:"official_channel_id"`
	OfficialModelID   int                    `json:"official_model_id"`
	Period            string                 `json:"period"`
	Availability      float64                `json:"availability"`
	Providers         []ProviderAvailability `json:"providers"`
	Keys              []KeyAvailability      `json:"keys"`
	Buckets           []AvailabilityBucket   `json:"buckets"`
}

type ProviderAvailability struct {
	ProviderIndex   int     `json:"provider_index"`
	KeyCount        int     `json:"key_count"`
	Availability    float64 `json:"availability"`
	PriceDiscount   float64 `json:"price_discount"`
	Buckets         []AvailabilityBucket `json:"buckets"`
}

type KeyAvailability struct {
	KeyIndex        int     `json:"key_index"`
	ProviderKeyIndex int    `json:"provider_key_index"`
	ProviderIndex   int     `json:"provider_index"`
	Availability    float64 `json:"availability"`
	PriceDiscount   float64 `json:"price_discount"`
	FixedPrice      float64 `json:"fixed_price"`
	PriceCurrency   string  `json:"price_currency"`
	ProbeTotal      int     `json:"probe_total"`
	ProbeSuccesses  int     `json:"probe_successes"`
	Buckets         []AvailabilityBucket `json:"buckets"`
}

type AvailabilityBucket struct {
	BucketTime   int     `json:"bucket_time"`
	Total        int     `json:"total"`
	Successes    int     `json:"successes"`
	Availability float64 `json:"availability"`
}

// GetOfficialChannelAvailability 获取官方渠道可用性
func (s *SiteService) GetOfficialChannelAvailability(channelID int, modelID *int, period string) (*AvailabilityResponse, error) {
	// TODO: 当前返回模拟数据 (可用性监控未实现)
	resp := &AvailabilityResponse{
		OfficialChannelID: channelID,
		OfficialModelID:   0,
		Period:            period,
		Availability:      100.0,
		Providers:         []ProviderAvailability{},
		Keys:              []KeyAvailability{},
		Buckets:           []AvailabilityBucket{},
	}
	if modelID != nil {
		resp.OfficialModelID = *modelID
	}
	return resp, nil
}

// KeyGroupResponse 密钥分组响应 (对齐 Tokens.jsx 消费字段)
type KeyGroupResponse struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	VendorCategory string  `json:"vendor_category,omitempty"`
	PriceDiscount  float64 `json:"price_discount,omitempty"`
	RmbPerUsd      float64 `json:"rmb_per_usd,omitempty"`
	DiscountLabel  string  `json:"discount_label,omitempty"`
	Description    string  `json:"description,omitempty"`
	Tags           string  `json:"tags,omitempty"`
	IsRecommended  bool    `json:"is_recommended,omitempty"`
	IsUnavailable  bool    `json:"is_unavailable,omitempty"`
}

// GetSiteKeyGroups 获取密钥分组列表
func (s *SiteService) GetSiteKeyGroups() ([]KeyGroupResponse, error) {
	var groups []models.KeyGroup
	err := s.db.Where("status = ? AND distributor_id = ?", 1, 0).Find(&groups).Error
	if err != nil {
		return []KeyGroupResponse{}, nil
	}

	result := make([]KeyGroupResponse, 0, len(groups))
	for _, g := range groups {
		result = append(result, KeyGroupResponse{
			ID:             int(g.ID),
			Name:           g.Name,
			VendorCategory: g.VendorCategory,
			PriceDiscount:  g.PriceDiscount,
			RmbPerUsd:      g.RmbPerUsd,
			DiscountLabel:  g.DiscountLabel,
			Description:    g.Description,
			Tags:           g.Tags,
			IsRecommended:  g.IsRecommended,
			IsUnavailable:  g.IsUnavailable,
		})
	}
	return result, nil
}

// KeyGroupPricingResponse 密钥分组定价响应
type KeyGroupPricingResponse struct {
	Group           KeyGroupResponse      `json:"group"`
	Items           []KeyGroupPricingItem `json:"items"`
	Summary         *KeyGroupPricingSummary `json:"summary,omitempty"`
	RegionRestricted bool                  `json:"region_restricted"`
}

// KeyGroupPricingItem 定价项 (对齐 Tokens.jsx 的 item.* 字段)
type KeyGroupPricingItem struct {
	ModelName                string  `json:"model_name"`
	DisplayName              string  `json:"display_name,omitempty"`
	Category                 string  `json:"category,omitempty"`
	BillingType              string  `json:"billing_type"` // per_token / per_call / tiered_expr
	Status                   string  `json:"status"`
	InputPriceMin            *float64 `json:"input_price_min,omitempty"`
	InputPriceMax            *float64 `json:"input_price_max,omitempty"`
	OutputPriceMin           *float64 `json:"output_price_min,omitempty"`
	OutputPriceMax           *float64 `json:"output_price_max,omitempty"`
	CacheReadPriceMin        *float64 `json:"cache_read_price_min,omitempty"`
	CacheReadPriceMax        *float64 `json:"cache_read_price_max,omitempty"`
	CacheCreationPriceMin    *float64 `json:"cache_creation_price_min,omitempty"`
	CacheCreationPriceMax    *float64 `json:"cache_creation_price_max,omitempty"`
	CacheCreationPrice1hMin  *float64 `json:"cache_creation_price_1h_min,omitempty"`
	CacheCreationPrice1hMax  *float64 `json:"cache_creation_price_1h_max,omitempty"`
	FixedPriceMin            *float64 `json:"fixed_price_min,omitempty"`
	FixedPriceMax            *float64 `json:"fixed_price_max,omitempty"`
	RouteCount               int     `json:"route_count"`
	HasRange                 bool    `json:"has_range"`
}

// KeyGroupPricingSummary 定价汇总
type KeyGroupPricingSummary struct {
	ProviderCount   int  `json:"provider_count"`
	ModelCount      int  `json:"model_count"`
	ProviderLimited bool `json:"provider_limited"`
	ModelLimited    bool `json:"model_limited"`
}

// GetSiteKeyGroupPricing 获取密钥分组定价
func (s *SiteService) GetSiteKeyGroupPricing(groupID int) (*KeyGroupPricingResponse, error) {
	var group models.KeyGroup
	err := s.db.First(&group, groupID).Error
	if err != nil {
		return nil, fmt.Errorf("key group not found: %w", err)
	}

	// 当前返回空 items (定价聚合逻辑待实现)
	resp := &KeyGroupPricingResponse{
		Group: KeyGroupResponse{
			ID:             int(group.ID),
			Name:           group.Name,
			VendorCategory: group.VendorCategory,
			PriceDiscount:  group.PriceDiscount,
			RmbPerUsd:      group.RmbPerUsd,
			DiscountLabel:  group.DiscountLabel,
			Description:    group.Description,
			Tags:           group.Tags,
			IsRecommended:  group.IsRecommended,
			IsUnavailable:  group.IsUnavailable,
		},
		Items:            []KeyGroupPricingItem{},
		Summary:          nil,
		RegionRestricted: false,
	}
	return resp, nil
}

// --- helpers ---

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1"
}

func getEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var f float64
	_, err := fmt.Sscanf(v, "%f", &f)
	if err != nil {
		return fallback
	}
	return f
}
