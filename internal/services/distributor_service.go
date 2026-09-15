// Package services 实现业务逻辑。
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"gorm.io/gorm"
)

// GetSiteIDByOwner 按站长 user.ID 查 distributor_sites.id（分站主键）。
// 用于把 user.ID 映射到 distributor_id（外键指向 distributor_sites.id）。
func (s *DistributorService) GetSiteIDByOwner(ownerID int) (int, error) {
	var site models.DistributorSite
	if err := s.db.Where("owner_id = ?", ownerID).First(&site).Error; err != nil {
		return 0, err
	}
	return int(site.ID), nil
}

// DistributorService 实现分站管理后台业务逻辑。
type DistributorService struct {
	db           *gorm.DB
	newAPIClient *NewAPIClient
}

// NewDistributorService 创建 DistributorService。
func NewDistributorService(db *gorm.DB, newAPIClient *NewAPIClient) *DistributorService {
	return &DistributorService{
		db:           db,
		newAPIClient: newAPIClient,
	}
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
	var listings []models.DistributorModelListing
	var total int64

	q := s.db.Model(&models.DistributorModelListing{}).Where("distributor_id = ?", distributorID)
	q.Count(&total)

	offset := (page - 1) * pageSize
	if err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&listings).Error; err != nil {
		return nil, 0, err
	}

	out := make([]DistributorModel, 0, len(listings))
	for _, l := range listings {
		// 从绝对售价反推加价（以 input 价为基准展示）。
		markupType := "fixed"
		markupValue := l.SellingInputPrice - l.MerchantInputPrice
		if l.MerchantInputPrice > 0 {
			markupType = "percentage"
			markupValue = (l.SellingInputPrice/l.MerchantInputPrice - 1) * 100
		}
		out = append(out, DistributorModel{
			ID:           int(l.ID),
			ModelName:    l.ModelName,
			MerchantCost: l.MerchantInputPrice,
			MarkupType:   markupType,
			MarkupValue:  markupValue,
			FinalPrice:   l.SellingInputPrice,
			Status:       l.Status,
			CreatedAt:    l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return out, total, nil
}

// EnableModelInput 启用模型输入。
type EnableModelInput struct {
	ModelName    string  `json:"model_name"`
	ChannelID    int     `json:"channel_id"`
	MerchantID   int     `json:"merchant_id"`
	MerchantCost float64 `json:"merchant_cost"`  // 商家 input 成本
	MerchantOutputCost float64 `json:"merchant_output_cost"` // 商家 output 成本
	MarkupType   string  `json:"markup_type"`   // fixed/percentage
	MarkupValue  float64 `json:"markup_value"`
}

// calcSellingPrice 按加价策略计算售价。
func calcSellingPrice(cost float64, markupType string, markupValue float64) float64 {
	if markupType == "fixed" {
		return cost + markupValue
	}
	// percentage: markupValue 是百分比 (如 300 表示加价 300%)
	return cost * (1 + markupValue/100)
}

// EnableModel 启用模型到分站（从商家模型池选品 + 设置加价）。
func (s *DistributorService) EnableModel(distributorID int, in EnableModelInput) error {
	if in.MarkupType != "fixed" && in.MarkupType != "percentage" {
		return errors.New("invalid markup_type, must be fixed or percentage")
	}
	if in.ModelName == "" {
		return errors.New("model_name required")
	}

	sellingInput := calcSellingPrice(in.MerchantCost, in.MarkupType, in.MarkupValue)
	sellingOutput := calcSellingPrice(in.MerchantOutputCost, in.MarkupType, in.MarkupValue)

	listing := models.DistributorModelListing{
		DistributorID:       distributorID,
		ModelName:           in.ModelName,
		ChannelID:           in.ChannelID,
		MerchantID:          in.MerchantID,
		MerchantInputPrice:  in.MerchantCost,
		MerchantOutputPrice: in.MerchantOutputCost,
		SellingInputPrice:   sellingInput,
		SellingOutputPrice:  sellingOutput,
		Status:              1,
		CreatedAt:           time.Now(),
	}
	return s.db.Create(&listing).Error
}

// UpdateModelPricing 更新模型定价（重新计算售价）。
func (s *DistributorService) UpdateModelPricing(distributorID, modelID int, markupType string, markupValue float64) error {
	if markupType != "fixed" && markupType != "percentage" {
		return errors.New("invalid markup_type, must be fixed or percentage")
	}

	var listing models.DistributorModelListing
	if err := s.db.Where("id = ? AND distributor_id = ?", modelID, distributorID).First(&listing).Error; err != nil {
		return fmt.Errorf("model listing not found: %w", err)
	}

	sellingInput := calcSellingPrice(listing.MerchantInputPrice, markupType, markupValue)
	sellingOutput := calcSellingPrice(listing.MerchantOutputPrice, markupType, markupValue)

	return s.db.Model(&models.DistributorModelListing{}).
		Where("id = ? AND distributor_id = ?", modelID, distributorID).
		Updates(map[string]interface{}{
			"selling_input_price":  sellingInput,
			"selling_output_price": sellingOutput,
		}).Error
}

// DisableModel 下架模型（status=0）。
func (s *DistributorService) DisableModel(distributorID, modelID int) error {
	result := s.db.Model(&models.DistributorModelListing{}).
		Where("id = ? AND distributor_id = ?", modelID, distributorID).
		Update("status", 0)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("model listing not found")
	}
	return nil
}

// GetModelStats 获取模型销售统计。
// TODO(联调后): 从 New-API logs 表按 model+分站用户统计真实调用量/收益。
// 当前 logs 归 New-API 管，需按 distributor 的用户集合聚合，留待日志聚合专项。
func (s *DistributorService) GetModelStats(distributorID, modelID int) (map[string]interface{}, error) {
	var listing models.DistributorModelListing
	if err := s.db.Where("id = ? AND distributor_id = ?", modelID, distributorID).First(&listing).Error; err != nil {
		return nil, fmt.Errorf("model listing not found: %w", err)
	}
	return map[string]interface{}{
		"model_name":     listing.ModelName,
		"selling_price":  listing.SellingInputPrice,
		"merchant_cost":  listing.MerchantInputPrice,
		"margin":         listing.SellingInputPrice - listing.MerchantInputPrice,
		"total_calls":    0,   // TODO: 从 logs 聚合
		"total_revenue":  0.0, // TODO: 从 logs 聚合
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

// RedeemCodeListItem 兑换码列表项（对齐前端契约）
type RedeemCodeListItem struct {
	ID          int    `json:"id"`           // 批次虚拟ID
	Code        string `json:"code"`         // 展示用（批量时取第一个）
	Name        string `json:"name"`         // batch_name
	Amount      int64  `json:"amount"`       // quota
	Count       int    `json:"count"`        // 该批次的码数量
	UsedCount   int    `json:"used_count"`   // 已使用数量
	Status      int    `json:"status"`       // 0=未用 1=已用 2=作废 3=过期
	ExpireTime  int64  `json:"expire_time"`  // Unix 秒
	CreatedTime int64  `json:"created_time"` // Unix 秒
}

// GetDistributorRedeemCodes 获取分站兑换码列表（按 batch_name 分组）
func (s *DistributorService) GetDistributorRedeemCodes(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// 按 batch_name 分组统计
	type BatchStats struct {
		BatchName   string
		FirstCode   string
		Quota       int64
		TotalCount  int
		UsedCount   int
		ExpireAt    *time.Time
		CreatedAt   time.Time
	}

	var batches []BatchStats
	err := s.db.Raw(`
		SELECT 
			batch_name,
			MIN(code) as first_code,
			MAX(quota) as quota,
			COUNT(*) as total_count,
			SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) as used_count,
			MAX(expire_at) as expire_at,
			MIN(created_at) as created_at
		FROM redeem_codes
		WHERE distributor_id = ?
		GROUP BY batch_name
		ORDER BY created_at DESC
	`, distributorID).Scan(&batches).Error

	if err != nil {
		return nil, 0, err
	}

	result := make([]map[string]interface{}, 0, len(batches))
	for i, batch := range batches {
		status := 0
		if batch.UsedCount == batch.TotalCount {
			status = 1 // 全部已用
		} else if batch.ExpireAt != nil && batch.ExpireAt.Before(time.Now()) {
			status = 3 // 已过期
		}

		expireTime := int64(0)
		if batch.ExpireAt != nil {
			expireTime = batch.ExpireAt.Unix()
		}

		result = append(result, map[string]interface{}{
			"id":           i + 1,
			"code":         batch.FirstCode,
			"name":         batch.BatchName,
			"amount":       batch.Quota,
			"count":        batch.TotalCount,
			"used_count":   batch.UsedCount,
			"status":       status,
			"expire_time":  expireTime,
			"created_time": batch.CreatedAt.Unix(),
		})
	}

	return result, int64(len(result)), nil
}

// GenerateDistributorRedeemCodes 生成兑换码（分站自费）
func (s *DistributorService) GenerateDistributorRedeemCodes(distributorID int, name string, quota int64, count int, expiredTime int64) ([]string, error) {
	if count < 1 || count > 1000 {
		return nil, errors.New("count must be between 1 and 1000")
	}

	expireAt := time.Unix(expiredTime, 0)
	codes := make([]string, 0, count)
	redeemCodes := make([]models.RedeemCode, 0, count)

	// 生成唯一码（带重试逻辑）
	for len(codes) < count {
		code := generateUniqueRedeemCode()
		
		// 检查数据库唯一性
		var existing models.RedeemCode
		if err := s.db.Where("code = ?", code).First(&existing).Error; err == gorm.ErrRecordNotFound {
			codes = append(codes, code)
			redeemCodes = append(redeemCodes, models.RedeemCode{
				Code:          code,
				DistributorID: distributorID,
				Quota:         quota,
				BatchName:     name,
				Status:        0, // 未使用
				ExpireAt:      &expireAt,
				CreatedAt:     time.Now(),
			})
		}
	}

	// 批量插入（事务）
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&redeemCodes).Error
	}); err != nil {
		return nil, fmt.Errorf("批量插入兑换码失败: %w", err)
	}

	return codes, nil
}

// generateUniqueRedeemCode 生成格式为 LEAP-{8位随机大写字母数字} 的兑换码
func generateUniqueRedeemCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return "LEAP-" + string(b)
}

// GetRedeemCodeUsage 获取兑换码使用记录（按 batch_name 查所有码）
func (s *DistributorService) GetRedeemCodeUsage(distributorID, codeID int) ([]map[string]interface{}, error) {
	// codeID 在前端是批次虚拟ID，需转换为 batch_name
	// 简化：直接返回该分站所有已使用的码
	type UsageRow struct {
		Code     string
		UsedBy   *int
		UsedAt   *time.Time
		Quota    int64
		Username string
	}

	var rows []UsageRow
	err := s.db.Raw(`
		SELECT 
			rc.code,
			rc.used_by,
			rc.used_at,
			rc.quota,
			COALESCE(u.username, '') as username
		FROM redeem_codes rc
		LEFT JOIN users u ON rc.used_by = u.id
		WHERE rc.distributor_id = ? AND rc.status = 1
		ORDER BY rc.used_at DESC
		LIMIT 100
	`, distributorID).Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		if row.UsedBy != nil && row.UsedAt != nil {
			result = append(result, map[string]interface{}{
				"code":       row.Code,
				"user_id":    *row.UsedBy,
				"username":   row.Username,
				"used_at":    row.UsedAt.Unix(),
				"quota_used": row.Quota,
			})
		}
	}

	return result, nil
}

// InvalidateDistributorRedeemCode 作废兑换码（按 codeID 关联的 batch 整批作废）
func (s *DistributorService) InvalidateDistributorRedeemCode(distributorID, codeID int) error {
	// 简化实现：codeID 暂不映射 batch，直接作废所有未用码
	result := s.db.Model(&models.RedeemCode{}).
		Where("distributor_id = ? AND status = 0", distributorID).
		Update("status", 2) // 2=作废

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("no redeemable codes to invalidate")
	}

	return nil
}

// ========== 8. 客户运营 - 用户管理 ==========

// GetDistributorUsers 获取分站用户列表（对齐前端契约）
func (s *DistributorService) GetDistributorUsers(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 查询总数
	var total int64
	if err := s.db.Model(&models.User{}).
		Where("distributor_id = ?", distributorID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询用户列表（关联订阅数和最后请求时间）
	type UserRow struct {
		ID              int
		Username        string
		Email           string
		Quota           int64
		UsedQuota       int64
		CreatedTime     int64
		SubCount        int
		LastRequestTime int64
	}

	var rows []UserRow
	err := s.db.Raw(`
		SELECT 
			u.id,
			u.username,
			COALESCE(u.email, '') as email,
			u.quota,
			u.used_quota,
			u.created_at as created_time,
			COALESCE(COUNT(s.id) FILTER (WHERE s.status = 1 AND s.expire_at > NOW()), 0) as sub_count,
			COALESCE(MAX(t.accessed_time), 0) as last_request_time
		FROM users u
		LEFT JOIN subscriptions s ON u.id = s.user_id
		LEFT JOIN tokens t ON u.id = t.user_id
		WHERE u.distributor_id = ?
		GROUP BY u.id
		ORDER BY u.created_at DESC
		LIMIT ? OFFSET ?
	`, distributorID, pageSize, offset).Scan(&rows).Error

	if err != nil {
		return nil, 0, err
	}

	result := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]interface{}{
			"id":                 row.ID,
			"username":           row.Username,
			"email":              row.Email,
			"quota":              row.Quota,
			"used_quota":         row.UsedQuota,
			"register_time":      row.CreatedTime,
			"subscription_count": row.SubCount,
			"last_request_time":  row.LastRequestTime,
		})
	}

	return result, total, nil
}

// GetDistributorUserDetail 获取用户详情（对齐前端契约）
func (s *DistributorService) GetDistributorUserDetail(distributorID, userID int) (map[string]interface{}, error) {
	// 权限校验
	var user models.User
	if err := s.db.Where("id = ? AND distributor_id = ?", userID, distributorID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户不存在或无权访问")
		}
		return nil, err
	}

	// 查询订阅数
	var subCount int64
	s.db.Model(&models.Subscription{}).
		Where("user_id = ? AND status = 1 AND expire_at > NOW()", userID).
		Count(&subCount)

	// 查询 token 数
	var tokenCount int64
	s.db.Model(&models.Token{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&tokenCount)

	// 查询最后请求时间
	var lastAccessTime int64
	s.db.Model(&models.Token{}).
		Where("user_id = ?", userID).
		Select("MAX(accessed_time)").
		Scan(&lastAccessTime)

	return map[string]interface{}{
		"id":                 user.ID,
		"username":           user.Username,
		"email":              user.Email,
		"display_name":       user.DisplayName,
		"quota":              user.Quota,
		"used_quota":         user.UsedQuota,
		"register_time":      user.CreatedTime,
		"status":             user.Status,
		"subscription_count": int(subCount),
		"token_count":        int(tokenCount),
		"last_request_time":  lastAccessTime,
	}, nil
}

// AdjustDistributorUserQuota 调整用户额度（事务 + New-API 同步）
func (s *DistributorService) AdjustDistributorUserQuota(distributorID, userID int, amount int64) error {
	var beforeQuota, afterQuota int64

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 权限校验 + 锁行
		var user models.User
		if err := tx.Raw("SELECT * FROM users WHERE id = ? AND distributor_id = ? FOR UPDATE", userID, distributorID).
			Scan(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("用户不存在或无权访问")
			}
			return err
		}

		// 2. 计算新额度
		beforeQuota = user.Quota
		afterQuota = user.Quota + amount

		if afterQuota < 0 {
			return fmt.Errorf("扣减后余额不能为负数")
		}

		// 3. 记录 quota_records
		record := models.QuotaRecord{
			UserID:        userID,
			DistributorID: distributorID,
			ChangeAmount:  amount,
			BeforeQuota:   beforeQuota,
			AfterQuota:    afterQuota,
			Reason:        "分站手动调整",
			CreatedAt:     time.Now(),
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("记录额度变化失败: %w", err)
		}

		// 4. 更新本地 users 表
		if err := tx.Model(&user).Update("quota", afterQuota).Error; err != nil {
			return fmt.Errorf("更新本地 quota 失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 5. 同步到 New-API（事务外，防阻塞）
	if amount > 0 {
		_ = s.newAPIClient.IncreaseQuota(userID, int(amount))
	} else if amount < 0 {
		_ = s.newAPIClient.DecreaseQuota(userID, int(-amount))
	}

	return nil
}

// GetUserOrders 获取用户订单记录（充值 + 套餐订阅）
func (s *DistributorService) GetUserOrders(distributorID, userID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// 权限校验
	var user models.User
	if err := s.db.Where("id = ? AND distributor_id = ?", userID, distributorID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, fmt.Errorf("用户不存在或无权访问")
		}
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 查询充值订单
	var topupOrders []models.TopupOrder
	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&topupOrders).Error; err != nil {
		return nil, 0, err
	}

	// 查询订阅订单
	var subscriptions []models.Subscription
	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&subscriptions).Error; err != nil {
		return nil, 0, err
	}

	// 合并结果
	result := make([]map[string]interface{}, 0)
	for _, order := range topupOrders {
		result = append(result, map[string]interface{}{
			"type":       "topup",
			"id":         order.ID,
			"amount":     order.Amount,
			"quota":      order.Quota,
			"status":     order.Status,
			"created_at": order.CreatedAt,
		})
	}
	for _, sub := range subscriptions {
		result = append(result, map[string]interface{}{
			"type":       "subscription",
			"id":         sub.ID,
			"package_id": sub.PackageID,
			"status":     sub.Status,
			"expire_at":  sub.ExpireAt.Unix(),
			"created_at": sub.CreatedAt.Unix(),
		})
	}

	var total int64
	s.db.Model(&models.TopupOrder{}).Where("user_id = ?", userID).Count(&total)

	return result, total, nil
}

// GetUserTokens 获取用户令牌列表
func (s *DistributorService) GetUserTokens(distributorID, userID int) ([]map[string]interface{}, error) {
	// 权限校验
	var user models.User
	if err := s.db.Where("id = ? AND distributor_id = ?", userID, distributorID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户不存在或无权访问")
		}
		return nil, err
	}

	var tokens []models.Token
	if err := s.db.Where("user_id = ?", userID).
		Order("created_time DESC").
		Find(&tokens).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, map[string]interface{}{
			"id":            t.Id,
			"name":          t.Name,
			"key":           t.Key,
			"status":        t.Status,
			"used_quota":    t.UsedQuota,
			"remain_quota":  t.RemainQuota,
			"created_time":  t.CreatedTime,
			"accessed_time": t.AccessedTime,
		})
	}

	return result, nil
}

// ========== 9. 客户运营 - 令牌管理 ==========

// GetDistributorTokens 获取分站所有令牌列表
func (s *DistributorService) GetDistributorTokens(distributorID int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 查询总数
	var total int64
	err := s.db.Raw(`
		SELECT COUNT(*)
		FROM tokens t
		INNER JOIN users u ON t.user_id = u.id
		WHERE u.distributor_id = ? AND t.deleted_at IS NULL
	`, distributorID).Scan(&total).Error

	if err != nil {
		return nil, 0, err
	}

	// 查询列表
	type TokenRow struct {
		ID           int
		UserID       int
		Username     string
		Name         string
		Key          string
		Status       int
		UsedQuota    int
		RemainQuota  int
		CreatedTime  int64
		AccessedTime int64
	}

	var rows []TokenRow
	err = s.db.Raw(`
		SELECT 
			t.id,
			t.user_id,
			u.username,
			t.name,
			t.key,
			t.status,
			t.used_quota,
			t.remain_quota,
			t.created_time,
			t.accessed_time
		FROM tokens t
		INNER JOIN users u ON t.user_id = u.id
		WHERE u.distributor_id = ? AND t.deleted_at IS NULL
		ORDER BY t.created_time DESC
		LIMIT ? OFFSET ?
	`, distributorID, pageSize, offset).Scan(&rows).Error

	if err != nil {
		return nil, 0, err
	}

	result := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]interface{}{
			"id":            row.ID,
			"user_id":       row.UserID,
			"username":      row.Username,
			"name":          row.Name,
			"key":           row.Key,
			"status":        row.Status,
			"used_quota":    row.UsedQuota,
			"remain_quota":  row.RemainQuota,
			"created_time":  row.CreatedTime,
			"accessed_time": row.AccessedTime,
		})
	}

	return result, total, nil
}

// UpdateTokenStatus 启用/禁用令牌
func (s *DistributorService) UpdateTokenStatus(distributorID, tokenID int, status int) error {
	// 权限校验：token 必须属于本分站用户
	var token models.Token
	err := s.db.Raw(`
		SELECT t.*
		FROM tokens t
		INNER JOIN users u ON t.user_id = u.id
		WHERE t.id = ? AND u.distributor_id = ? AND t.deleted_at IS NULL
	`, tokenID, distributorID).Scan(&token).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("Token 不存在或无权访问")
		}
		return err
	}

	// 直接更新 tokens 表（New-API 暂无 token 管理接口）
	// 风险：可能与 New-API 缓存不一致，需后续优化
	if err := s.db.Model(&models.Token{}).
		Where("id = ?", tokenID).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("更新 token 状态失败: %w", err)
	}

	return nil
}

// GetTokenUsage 获取令牌使用统计
func (s *DistributorService) GetTokenUsage(distributorID, tokenID int) (map[string]interface{}, error) {
	// 权限校验
	var token models.Token
	err := s.db.Raw(`
		SELECT t.*
		FROM tokens t
		INNER JOIN users u ON t.user_id = u.id
		WHERE t.id = ? AND u.distributor_id = ? AND t.deleted_at IS NULL
	`, tokenID, distributorID).Scan(&token).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Token 不存在或无权访问")
		}
		return nil, err
	}

	// 基础统计（从 token 本身）
	stats := map[string]interface{}{
		"token_id":      tokenID,
		"token_name":    token.Name,
		"total_quota":   token.UsedQuota,
		"total_calls":   0,
		"last_7day_calls": 0,
		"avg_response":  0.0,
	}

	// 尝试从 logs 表统计（如果存在）
	type LogStats struct {
		TotalCalls    int
		Last7DayCalls int
	}

	var logStats LogStats
	err = s.db.Raw(`
		SELECT 
			COUNT(*) as total_calls,
			COALESCE(COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '7 days'), 0) as last_7day_calls
		FROM logs
		WHERE token_id = ?
	`, tokenID).Scan(&logStats).Error

	if err == nil {
		stats["total_calls"] = logStats.TotalCalls
		stats["last_7day_calls"] = logStats.Last7DayCalls
	}

	return stats, nil
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
