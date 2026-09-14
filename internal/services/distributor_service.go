// Package services 实现业务逻辑。
package services

import (
	"errors"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"gorm.io/gorm"
)

// DistributorService 实现分站管理后台业务逻辑。
type DistributorService struct {
	db *gorm.DB
}

// NewDistributorService 创建 DistributorService。
func NewDistributorService(db *gorm.DB) *DistributorService {
	return &DistributorService{db: db}
}

// ========== 1. 经营概览（Dashboard）==========

// DistributorStats 分站统计数据。
type DistributorStats struct {
	TotalRevenue   float64 `json:"total_revenue"`
	TotalUsers     int     `json:"total_users"`
	TotalOrders    int     `json:"total_orders"`
	EnabledModels  int     `json:"enabled_models"`
	PendingOrders  int     `json:"pending_orders"`
	TodayRevenue   float64 `json:"today_revenue"`
	MonthlyRevenue float64 `json:"monthly_revenue"`
}

// GetDistributorStats 获取分站统计数据。
func (s *DistributorService) GetDistributorStats(distributorID int) (*DistributorStats, error) {
	var stats DistributorStats

	// 分站用户数
	var userCount int64
	if err := s.db.Model(&models.User{}).Where("distributor_id = ?", distributorID).Count(&userCount).Error; err != nil {
		return nil, err
	}
	stats.TotalUsers = int(userCount)

	// TODO: 订单数和收益（需要 orders 表）
	stats.TotalOrders = 0
	stats.TotalRevenue = 0.0
	stats.TodayRevenue = 0.0
	stats.MonthlyRevenue = 0.0

	// TODO: 启用的模型数（需要 distributor_models 表）
	stats.EnabledModels = 0

	return &stats, nil
}

// RevenueTrendPoint 收益趋势数据点。
type RevenueTrendPoint struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
}

// GetRevenueTrend 获取收益趋势（近30天）。
func (s *DistributorService) GetRevenueTrend(distributorID int) ([]RevenueTrendPoint, error) {
	// TODO: 实现收益趋势查询
	// 从 distributor_revenue_records 表按日汇总
	return []RevenueTrendPoint{}, nil
}

// TopModel 热门模型。
type TopModel struct {
	ModelName  string  `json:"model_name"`
	CallCount  int     `json:"call_count"`
	Revenue    float64 `json:"revenue"`
	GrowthRate float64 `json:"growth_rate"`
}

// GetTopModels 获取热门模型 TOP 10。
func (s *DistributorService) GetTopModels(distributorID int) ([]TopModel, error) {
	// TODO: 实现热门模型统计
	// 从 orders 表按 model 分组统计
	return []TopModel{}, nil
}

// RecentOrder 最近订单。
type RecentOrder struct {
	OrderID   string  `json:"order_id"`
	Username  string  `json:"username"`
	ModelName string  `json:"model_name"`
	Amount    float64 `json:"amount"`
	Status    int     `json:"status"`
	CreatedAt string  `json:"created_at"`
}

// GetRecentOrders 获取最近订单列表。
func (s *DistributorService) GetRecentOrders(distributorID int, limit int) ([]RecentOrder, error) {
	// TODO: 实现最近订单查询
	// 从 orders 表查询，WHERE distributor_id = ?
	return []RecentOrder{}, nil
}

// ========== 2. 商品配置 - 模型管理 ==========

// DistributorModel 分站模型。
type DistributorModel struct {
	ID           int     `json:"id"`
	ModelName    string  `json:"model_name"`
	MerchantCost float64 `json:"merchant_cost"` // 商家成本
	MarkupType   string  `json:"markup_type"`   // fixed/percentage
	MarkupValue  float64 `json:"markup_value"`
	FinalPrice   float64 `json:"final_price"` // 分站售价
	Status       int     `json:"status"`      // 1=enabled 0=disabled
	CreatedAt    string  `json:"created_at"`
}

// GetModels 获取分站可售模型列表。
func (s *DistributorService) GetModels(distributorID int, page, pageSize int) ([]DistributorModel, int64, error) {
	// TODO: 实现模型列表查询
	// 从 distributor_models 表查询，WHERE distributor_id = ?
	return []DistributorModel{}, 0, nil
}

// EnableModelInput 启用模型输入。
type EnableModelInput struct {
	ModelName    string  `json:"model_name"`
	MerchantCost float64 `json:"merchant_cost"`
	MarkupType   string  `json:"markup_type"`   // fixed/percentage
	MarkupValue  float64 `json:"markup_value"`
}

// EnableModel 启用模型到分站。
func (s *DistributorService) EnableModel(distributorID int, in EnableModelInput) error {
	// 验证加价类型
	if in.MarkupType != "fixed" && in.MarkupType != "percentage" {
		return errors.New("invalid markup_type, must be fixed or percentage")
	}

	// TODO: 插入到 distributor_models 表
	// 计算 final_price = merchant_cost + markup (fixed) 或 merchant_cost * (1 + markup) (percentage)
	return errors.New("not implemented")
}

// UpdateModelPricing 更新模型定价。
func (s *DistributorService) UpdateModelPricing(distributorID, modelID int, markupType string, markupValue float64) error {
	// TODO: 更新 distributor_models 表
	// WHERE id = ? AND distributor_id = ?
	return errors.New("not implemented")
}

// DisableModel 下架模型。
func (s *DistributorService) DisableModel(distributorID, modelID int) error {
	// TODO: 更新 distributor_models 表，status = 0
	return errors.New("not implemented")
}

// GetModelStats 获取模型销售统计。
func (s *DistributorService) GetModelStats(distributorID, modelID int) (map[string]interface{}, error) {
	// TODO: 从 orders 表统计该模型的调用量和收益
	return map[string]interface{}{
		"total_calls":   0,
		"total_revenue": 0.0,
		"avg_price":     0.0,
	}, nil
}

// ========== 3. 商品配置 - 官方渠道 ==========

// OfficialChannel 官方渠道。
type OfficialChannel struct {
	ID          int     `json:"id"`
	ChannelName string  `json:"channel_name"`
	ChannelType string  `json:"channel_type"`
	BaseCost    float64 `json:"base_cost"`
	Enabled     bool    `json:"enabled"`
	Markup      float64 `json:"markup"`
}

// GetOfficialChannels 获取平台官方渠道列表（只读）。
func (s *DistributorService) GetOfficialChannels(distributorID int) ([]OfficialChannel, error) {
	// TODO: 从 official_channels 表查询
	// LEFT JOIN distributor_channel_config 获取分站是否启用
	return []OfficialChannel{}, nil
}

// EnableOfficialChannel 启用官方渠道到分站。
func (s *DistributorService) EnableOfficialChannel(distributorID, channelID int, markup float64) error {
	// TODO: 插入 distributor_channel_config 表
	return errors.New("not implemented")
}

// UpdateChannelPricing 更新渠道加价。
func (s *DistributorService) UpdateChannelPricing(distributorID, channelID int, markup float64) error {
	// TODO: 更新 distributor_channel_config 表
	return errors.New("not implemented")
}

// DisableOfficialChannel 禁用渠道。
func (s *DistributorService) DisableOfficialChannel(distributorID, channelID int) error {
	// TODO: 从 distributor_channel_config 表删除或标记
	return errors.New("not implemented")
}

// ========== 4. 商品配置 - 订阅共享 ==========

// SharedSubscription 订阅共享。
type SharedSubscription struct {
	ID           int     `json:"id"`
	MerchantName string  `json:"merchant_name"`
	ServiceType  string  `json:"service_type"`
	BaseCost     float64 `json:"base_cost"`
	Enabled      bool    `json:"enabled"`
	Multiplier   float64 `json:"multiplier"` // 倍率加价
}

// GetSharedSubscriptions 获取订阅共享池。
func (s *DistributorService) GetSharedSubscriptions(distributorID int) ([]SharedSubscription, error) {
	// TODO: 从 shared_subscription_pool 表查询
	return []SharedSubscription{}, nil
}

// EnableSharedSubscription 启用订阅共享。
func (s *DistributorService) EnableSharedSubscription(distributorID, subID int, multiplier float64) error {
	// TODO: 插入 distributor_shared_subscriptions 表
	return errors.New("not implemented")
}

// UpdateSubscriptionPricing 设置倍率加价。
func (s *DistributorService) UpdateSubscriptionPricing(distributorID, subID int, multiplier float64) error {
	// TODO: 更新 distributor_shared_subscriptions 表
	return errors.New("not implemented")
}

// DisableSharedSubscription 禁用订阅共享。
func (s *DistributorService) DisableSharedSubscription(distributorID, subID int) error {
	// TODO: 删除 distributor_shared_subscriptions 表记录
	return errors.New("not implemented")
}

// ========== 5. 商品配置 - 套餐管理 ==========

// DistributorPackage 分站套餐。
type DistributorPackage struct {
	ID           int     `json:"id"`
	PackageName  string  `json:"package_name"`
	BasePrice    float64 `json:"base_price"`
	FinalPrice   float64 `json:"final_price"`
	Quota        int64   `json:"quota"`
	ValidDays    int     `json:"valid_days"`
	Status       int     `json:"status"`
	TotalSales   int     `json:"total_sales"`
	CreatedAt    string  `json:"created_at"`
}

// GetPackages 获取分站套餐列表。
func (s *DistributorService) GetPackages(distributorID int) ([]DistributorPackage, error) {
	// TODO: 从 distributor_packages 表查询
	return []DistributorPackage{}, nil
}

// CreateDistributorPackageInput 创建套餐输入。
type CreateDistributorPackageInput struct {
	PackageName string  `json:"package_name"`
	BasePrice   float64 `json:"base_price"`
	Markup      float64 `json:"markup"`
	Quota       int64   `json:"quota"`
	ValidDays   int     `json:"valid_days"`
}

// CreatePackage 创建套餐。
func (s *DistributorService) CreatePackage(distributorID int, in CreateDistributorPackageInput) error {
	// TODO: 插入 distributor_packages 表
	// final_price = base_price + markup
	return errors.New("not implemented")
}

// UpdatePackage 更新套餐。
func (s *DistributorService) UpdatePackage(distributorID, packageID int, in CreateDistributorPackageInput) error {
	// TODO: 更新 distributor_packages 表
	return errors.New("not implemented")
}

// DeletePackage 删除套餐。
func (s *DistributorService) DeletePackage(distributorID, packageID int) error {
	// TODO: 删除 distributor_packages 表记录
	return errors.New("not implemented")
}

// GetPackageSales 获取套餐销售记录。
func (s *DistributorService) GetPackageSales(distributorID, packageID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 从 package_purchases 表查询
	return []map[string]interface{}{}, 0, nil
}

// ========== 6. 商品配置 - 密钥分组 ==========

// KeyGroup 密钥分组。
type KeyGroup struct {
	ID            int     `json:"id"`
	GroupName     string  `json:"group_name"`
	LEOThreshold  int     `json:"leo_threshold"`  // LEO 代币门槛
	DiscountRate  float64 `json:"discount_rate"`  // 折扣率
	TotalMembers  int     `json:"total_members"`
	CreatedAt     string  `json:"created_at"`
}

// GetKeyGroups 获取分站密钥分组列表。
func (s *DistributorService) GetKeyGroups(distributorID int) ([]KeyGroup, error) {
	// TODO: 从 key_groups 表查询，WHERE distributor_id = ?
	return []KeyGroup{}, nil
}

// CreateKeyGroupInput 创建密钥分组输入。
type CreateKeyGroupInput struct {
	GroupName     string  `json:"group_name"`
	LEOThreshold  int     `json:"leo_threshold"`
	DiscountRate  float64 `json:"discount_rate"`
}

// CreateKeyGroup 创建分组。
func (s *DistributorService) CreateKeyGroup(distributorID int, in CreateKeyGroupInput) error {
	// TODO: 插入 key_groups 表
	return errors.New("not implemented")
}

// UpdateKeyGroup 更新分组。
func (s *DistributorService) UpdateKeyGroup(distributorID, groupID int, in CreateKeyGroupInput) error {
	// TODO: 更新 key_groups 表
	return errors.New("not implemented")
}

// DeleteKeyGroup 删除分组。
func (s *DistributorService) DeleteKeyGroup(distributorID, groupID int) error {
	// TODO: 删除 key_groups 表记录
	return errors.New("not implemented")
}

// ========== 7. 商品配置 - 兑换码 ==========

// GetDistributorRedeemCodes 获取分站兑换码列表。
func (s *DistributorService) GetDistributorRedeemCodes(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 从 redeem_codes 表查询，WHERE distributor_id = ?
	return []map[string]interface{}{}, 0, nil
}

// GenerateDistributorRedeemCodes 生成兑换码（分站自费）。
func (s *DistributorService) GenerateDistributorRedeemCodes(distributorID int, name string, quota int64, count int, expiredTime int64) ([]string, error) {
	// TODO: 插入 redeem_codes 表，distributor_id = ?
	// 生成 count 个兑换码，每个 quota 额度
	return []string{}, errors.New("not implemented")
}

// GetRedeemCodeUsage 获取兑换码使用记录。
func (s *DistributorService) GetRedeemCodeUsage(distributorID, codeID int) ([]map[string]interface{}, error) {
	// TODO: 从 redeem_code_usage 表查询
	return []map[string]interface{}{}, nil
}

// InvalidateDistributorRedeemCode 作废兑换码。
func (s *DistributorService) InvalidateDistributorRedeemCode(distributorID, codeID int) error {
	// TODO: 更新 redeem_codes 表，status = 2 (disabled)
	return errors.New("not implemented")
}

// ========== 8. 客户运营 - 用户管理 ==========

// GetDistributorUsers 获取分站用户列表。
func (s *DistributorService) GetDistributorUsers(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	var users []models.User
	var total int64

	offset := (page - 1) * pageSize
	if err := s.db.Model(&models.User{}).Where("distributor_id = ?", distributorID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := s.db.Where("distributor_id = ?", distributorID).
		Order("created_time DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	result := make([]map[string]interface{}, 0)
	for _, u := range users {
		result = append(result, map[string]interface{}{
			"id":           u.ID,
			"username":     u.Username,
			"display_name": u.DisplayName,
			"quota":        u.Quota,
			"used_quota":   u.UsedQuota,
			"status":       u.Status,
			"created_at":   time.Unix(u.CreatedTime, 0).Format("2006-01-02 15:04:05"),
		})
	}

	return result, total, nil
}

// GetDistributorUserDetail 获取用户详情。
func (s *DistributorService) GetDistributorUserDetail(distributorID, userID int) (map[string]interface{}, error) {
	var user models.User
	if err := s.db.Where("id = ? AND distributor_id = ?", userID, distributorID).First(&user).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"email":        user.Email,
		"quota":        user.Quota,
		"used_quota":   user.UsedQuota,
		"aff_quota":    user.AffQuota,
		"status":       user.Status,
		"user_level":   user.UserLevel,
		"created_at":   time.Unix(user.CreatedTime, 0).Format("2006-01-02 15:04:05"),
	}, nil
}

// AdjustDistributorUserQuota 调整用户额度。
func (s *DistributorService) AdjustDistributorUserQuota(distributorID, userID int, amount int64) error {
	// 验证用户归属
	var user models.User
	if err := s.db.Where("id = ? AND distributor_id = ?", userID, distributorID).First(&user).Error; err != nil {
		return err
	}

	// 调整额度
	return s.db.Model(&models.User{}).Where("id = ?", userID).
		Update("quota", gorm.Expr("quota + ?", amount)).Error
}

// GetUserOrders 获取用户订单记录。
func (s *DistributorService) GetUserOrders(distributorID, userID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 从 orders 表查询
	// WHERE user_id = ? AND distributor_id = ?
	return []map[string]interface{}{}, 0, nil
}

// GetUserTokens 获取用户令牌列表。
func (s *DistributorService) GetUserTokens(distributorID, userID int) ([]map[string]interface{}, error) {
	// TODO: 从 tokens 表查询
	// WHERE user_id = ? AND (distributor_id = ? OR distributor_id = 0)
	return []map[string]interface{}{}, nil
}

// ========== 9. 客户运营 - 令牌管理 ==========

// GetDistributorTokens 获取分站所有令牌列表。
func (s *DistributorService) GetDistributorTokens(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 从 tokens 表查询
	// WHERE distributor_id = ?
	return []map[string]interface{}{}, 0, nil
}

// UpdateTokenStatus 启用/禁用令牌。
func (s *DistributorService) UpdateTokenStatus(distributorID, tokenID int, status int) error {
	// TODO: 更新 tokens 表，WHERE id = ? AND distributor_id = ?
	return errors.New("not implemented")
}

// GetTokenUsage 获取令牌使用统计。
func (s *DistributorService) GetTokenUsage(distributorID, tokenID int) (map[string]interface{}, error) {
	// TODO: 从 token_usage_logs 表统计
	return map[string]interface{}{
		"total_calls":   0,
		"total_tokens":  0,
		"total_cost":    0.0,
		"last_used_at":  "",
	}, nil
}

// ========== 10. 财务管理 ==========

// FinanceSummary 财务汇总。
type FinanceSummary struct {
	TotalRevenue       float64 `json:"total_revenue"`
	TotalExpense       float64 `json:"total_expense"`
	AvailableBalance   float64 `json:"available_balance"`
	PendingWithdrawal  float64 `json:"pending_withdrawal"`
	TotalWithdrawn     float64 `json:"total_withdrawn"`
}

// GetFinanceSummary 获取财务汇总。
func (s *DistributorService) GetFinanceSummary(distributorID int) (*FinanceSummary, error) {
	// TODO: 实现财务汇总统计
	return &FinanceSummary{}, nil
}

// GetRevenueRecords 获取收益明细。
func (s *DistributorService) GetRevenueRecords(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 从 distributor_revenue_records 表查询
	return []map[string]interface{}{}, 0, nil
}

// GetWithdrawalRecords 获取提现记录。
func (s *DistributorService) GetWithdrawalRecords(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 从 withdrawal_requests 表查询
	// WHERE distributor_id = ?
	return []map[string]interface{}{}, 0, nil
}

// RequestWithdrawal 申请提现。
func (s *DistributorService) RequestWithdrawal(distributorID int, amount float64, paymentMethod, accountInfo string) (int64, error) {
	// TODO: 检查可提现余额
	// 创建 withdrawal_request 记录
	return 0, errors.New("not implemented")
}

// GetRechargeRecords 获取充值记录。
func (s *DistributorService) GetRechargeRecords(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 从 topup_orders 表查询
	// WHERE distributor_id = ?
	return []map[string]interface{}{}, 0, nil
}

// ========== 11. 站点与开发 - 站点设置 ==========

// SiteSettings 站点配置。
type SiteSettings struct {
	SiteName        string `json:"site_name"`
	SiteLogo        string `json:"site_logo"`
	CustomDomain    string `json:"custom_domain"`
	SEOTitle        string `json:"seo_title"`
	SEODescription  string `json:"seo_description"`
	SEOKeywords     string `json:"seo_keywords"`
}

// GetSiteSettings 获取站点配置。
func (s *DistributorService) GetSiteSettings(distributorID int) (*SiteSettings, error) {
	// TODO: 从 distributor_settings 表查询
	return &SiteSettings{}, nil
}

// UpdateSiteSettings 更新站点配置。
func (s *DistributorService) UpdateSiteSettings(distributorID int, settings *SiteSettings) error {
	// TODO: 更新 distributor_settings 表
	return errors.New("not implemented")
}

// PaymentMethodConfig 支付方式配置。
type PaymentMethodConfig struct {
	Type    string                 `json:"type"`    // usdt/alipay/stripe
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// GetPaymentMethods 获取支付方式配置。
func (s *DistributorService) GetPaymentMethods(distributorID int) ([]PaymentMethodConfig, error) {
	// TODO: 从 distributor_payment_configs 表查询
	return []PaymentMethodConfig{}, nil
}

// UpdatePaymentMethods 更新支付配置。
func (s *DistributorService) UpdatePaymentMethods(distributorID int, methods []PaymentMethodConfig) error {
	// TODO: 更新 distributor_payment_configs 表
	return errors.New("not implemented")
}

// ========== 12. 站点与开发 - API密钥 ==========

// APIKey 分站API密钥。
type APIKey struct {
	ID        int    `json:"id"`
	KeyName   string `json:"key_name"`
	KeyValue  string `json:"key_value"`
	CreatedAt string `json:"created_at"`
	LastUsed  string `json:"last_used"`
}

// GetAPIKeys 获取分站API密钥列表。
func (s *DistributorService) GetAPIKeys(distributorID int) ([]APIKey, error) {
	// TODO: 从 distributor_api_keys 表查询
	return []APIKey{}, nil
}

// GenerateAPIKey 生成新密钥。
func (s *DistributorService) GenerateAPIKey(distributorID int, keyName string) (*APIKey, error) {
	// TODO: 生成随机密钥，插入 distributor_api_keys 表
	return nil, errors.New("not implemented")
}

// DeleteAPIKey 删除密钥。
func (s *DistributorService) DeleteAPIKey(distributorID, keyID int) error {
	// TODO: 删除 distributor_api_keys 表记录
	return errors.New("not implemented")
}
