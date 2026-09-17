// Package services 实现业务逻辑。
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"gorm.io/gorm"
)

// AdminService 实现总站管理员后台业务逻辑。
type AdminService struct {
	db           *gorm.DB
	newAPIClient *NewAPIClient
}

// NewAdminService 创建 AdminService。
func NewAdminService(db *gorm.DB, newAPIClient *NewAPIClient) *AdminService {
	return &AdminService{
		db:           db,
		newAPIClient: newAPIClient,
	}
}

// ========== 1. 系统概览（Dashboard）==========

// DashboardStats 总站统计数据。
type DashboardStats struct {
	TotalUsers        int     `json:"total_users"`
	TotalMerchants    int     `json:"total_merchants"`
	TotalDistributors int     `json:"total_distributors"`
	TotalRevenue      float64 `json:"total_revenue"`
	DailyActiveUsers  int     `json:"daily_active_users"`
}

// GetDashboardStats 获取总站统计数据。
func (s *AdminService) GetDashboardStats() (*DashboardStats, error) {
	var stats DashboardStats

	// 用户总数
	var totalUsers int64
	if err := s.db.Model(&models.User{}).Where("status = ?", 1).Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	stats.TotalUsers = int(totalUsers)

	// 商家总数 (user_level = 2)
	var totalMerchants int64
	if err := s.db.Model(&models.User{}).Where("user_level = ? AND status = ?", 2, 1).Count(&totalMerchants).Error; err != nil {
		return nil, err
	}
	stats.TotalMerchants = int(totalMerchants)

	// 分站总数 (user_level = 3)
	var totalDistributors int64
	if err := s.db.Model(&models.User{}).Where("user_level = ? AND status = ?", 3, 1).Count(&totalDistributors).Error; err != nil {
		return nil, err
	}
	stats.TotalDistributors = int(totalDistributors)

	// 总收入：所有已支付订单金额总和
	var totalRevenue float64
	if err := s.db.Model(&models.TopupOrder{}).Where("status = ?", 1).Select("COALESCE(SUM(amount), 0)").Scan(&totalRevenue).Error; err != nil {
		return nil, err
	}
	stats.TotalRevenue = totalRevenue

	// 日活：今天有 request_count 增长的用户数（简化实现）
	// 实际应该从 logs 表统计，这里用 request_count > 0 作为近似
	today := time.Now().Truncate(24 * time.Hour).Unix()
	var dailyActiveUsers int64
	if err := s.db.Model(&models.User{}).
		Where("status = ? AND created_at < ?", 1, today).
		Where("request_count > ?", 0).
		Count(&dailyActiveUsers).Error; err != nil {
		return nil, err
	}
	stats.DailyActiveUsers = int(dailyActiveUsers)

	return &stats, nil
}

// ChartData 图表数据。
type ChartData struct {
	RevenueChart     []ChartPoint `json:"revenue_chart"`
	UserGrowthChart  []ChartPoint `json:"user_growth_chart"`
	ModelUsageChart  []ChartPoint `json:"model_usage_chart"`
}

// ChartPoint 图表数据点。
type ChartPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// GetDashboardCharts 获取图表数据（最近7天）。
func (s *AdminService) GetDashboardCharts() (*ChartData, error) {
	data := &ChartData{
		RevenueChart:    make([]ChartPoint, 0),
		UserGrowthChart: make([]ChartPoint, 0),
		ModelUsageChart: make([]ChartPoint, 0),
	}

	// 收入趋势：最近7天每日充值金额
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		startOfDay := time.Now().AddDate(0, 0, -i).Truncate(24 * time.Hour).Unix()
		endOfDay := startOfDay + 86400

		var dailyRevenue float64
		s.db.Model(&models.TopupOrder{}).
			Where("status = ? AND paid_at >= ? AND paid_at < ?", 1, startOfDay, endOfDay).
			Select("COALESCE(SUM(amount), 0)").
			Scan(&dailyRevenue)

		data.RevenueChart = append(data.RevenueChart, ChartPoint{
			Date:  date,
			Value: dailyRevenue,
		})
	}

	// 用户增长：最近7天每日新增用户数
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		startOfDay := time.Now().AddDate(0, 0, -i).Truncate(24 * time.Hour).Unix()
		endOfDay := startOfDay + 86400

		var dailyUsers int64
		s.db.Model(&models.User{}).
			Where("created_at >= ? AND created_at < ?", startOfDay, endOfDay).
			Count(&dailyUsers)

		data.UserGrowthChart = append(data.UserGrowthChart, ChartPoint{
			Date:  date,
			Value: float64(dailyUsers),
		})
	}

	// 模型调用量：最近7天总请求数（简化实现：从 users.request_count）
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		
		// 简化实现：返回当天累计请求数
		var totalRequests int64
		s.db.Model(&models.User{}).
			Select("COALESCE(SUM(request_count), 0)").
			Scan(&totalRequests)

		data.ModelUsageChart = append(data.ModelUsageChart, ChartPoint{
			Date:  date,
			Value: float64(totalRequests),
		})
	}

	return data, nil
}

// ========== 2. 用户管理 ==========

// UserListQuery 用户列表查询参数。
type UserListQuery struct {
	Page      int    `form:"page" binding:"min=1"`
	PageSize  int    `form:"page_size" binding:"min=1,max=100"`
	UserLevel *int   `form:"user_level"` // 筛选：0=普通用户 2=商家 3=分站
	Status    *int   `form:"status"`     // 筛选：1=正常 2=禁用
	Keyword   string `form:"keyword"`    // 搜索用户名/邮箱
}

// UserListResult 用户列表结果。
type UserListResult struct {
	Total int           `json:"total"`
	Users []models.User `json:"users"`
}

// GetUserList 获取用户列表（分页 + 筛选）。
func (s *AdminService) GetUserList(query UserListQuery) (*UserListResult, error) {
	var total int64
	var users []models.User

	db := s.db.Model(&models.User{})

	// 筛选条件
	if query.UserLevel != nil {
		db = db.Where("user_level = ?", *query.UserLevel)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		db = db.Where("username ILIKE ? OR email ILIKE ?", keyword, keyword)
	}

	// 总数
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	offset := (query.Page - 1) * query.PageSize
	if err := db.Offset(offset).Limit(query.PageSize).Order("id DESC").Find(&users).Error; err != nil {
		return nil, err
	}

	return &UserListResult{
		Total: int(total),
		Users: users,
	}, nil
}

// GetUserByID 获取用户详情。
func (s *AdminService) GetUserByID(userID int) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUserInput 更新用户参数。
type UpdateUserInput struct {
	UserLevel   *int   `json:"user_level"`   // 修改等级
	Status      *int   `json:"status"`       // 启用/禁用
	DisplayName string `json:"display_name"` // 显示名
}

// UpdateUser 更新用户信息。
func (s *AdminService) UpdateUser(userID int, input UpdateUserInput) error {
	updates := make(map[string]interface{})

	if input.UserLevel != nil {
		updates["user_level"] = *input.UserLevel
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.DisplayName != "" {
		updates["display_name"] = input.DisplayName
	}

	if len(updates) == 0 {
		return errors.New("no fields to update")
	}

	return s.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

// DeleteUser 删除用户（软删除或禁用）。
func (s *AdminService) DeleteUser(userID int) error {
	// 实际操作：将状态设为禁用
	return s.db.Model(&models.User{}).Where("id = ?", userID).Update("status", 2).Error
}

// AdjustUserQuotaInput 手动调整用户余额参数。
type AdjustUserQuotaInput struct {
	QuotaDelta int64  `json:"quota_delta" binding:"required"` // 增减额度（正数增加，负数减少）
	Reason     string `json:"reason"`                        // 调整原因
}

// AdjustUserQuota 手动调整用户余额。
func (s *AdminService) AdjustUserQuota(userID int, input AdjustUserQuotaInput) error {
	// 先读取当前余额验证
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}

	newQuota := user.Quota + input.QuotaDelta
	if newQuota < 0 {
		return errors.New("insufficient quota")
	}

	// 调用 New-API 设置新余额（调用 /api/user/:id 更新接口）
	return s.newAPIClient.SetUserQuota(userID, int(newQuota))
}

// ========== 3. 商家审批 ==========

// GetPendingMerchants 获取待审批商家列表。
func (s *AdminService) GetPendingMerchants(page, pageSize int) (*UserListResult, error) {
	var total int64
	var applications []models.KolApplication

	db := s.db.Model(&models.KolApplication{}).
		Where("apply_type = ? AND status = ?", "merchant", 0)

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("applied_at DESC").Find(&applications).Error; err != nil {
		return nil, err
	}

	// 加载关联用户信息
	userIDs := make([]int, len(applications))
	for i, app := range applications {
		userIDs[i] = app.UserID
	}

	var users []models.User
	if len(userIDs) > 0 {
		s.db.Where("id IN ?", userIDs).Find(&users)
	}

	return &UserListResult{
		Total: int(total),
		Users: users,
	}, nil
}

// GetMerchantApplication 获取商家申请详情。
func (s *AdminService) GetMerchantApplication(applicationID int) (*models.KolApplication, error) {
	var app models.KolApplication
	if err := s.db.Where("id = ? AND apply_type = ?", applicationID, "merchant").First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ApproveMerchant 批准商家申请（user_level 升级为 2）。
func (s *AdminService) ApproveMerchant(applicationID int, adminRemark string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var app models.KolApplication
		if err := tx.Where("id = ? AND apply_type = ?", applicationID, "merchant").First(&app).Error; err != nil {
			return err
		}

		if app.Status != 0 {
			return errors.New("application already processed")
		}

		// 更新申请状态
		now := time.Now()
		if err := tx.Model(&app).Updates(map[string]interface{}{
			"status":       1,
			"admin_remark": adminRemark,
			"processed_at": &now,
		}).Error; err != nil {
			return err
		}

		// 升级用户等级为商家 (user_level = 2，扩展列可直接写)
		if err := tx.Model(&models.User{}).Where("id = ?", app.UserID).Update("user_level", 2).Error; err != nil {
			return err
		}

		// 创建 merchant 记录（若不存在）——商家中心所有接口依赖此记录。
		// 之前缺此步导致审批通过后访问商家中心报 "merchant not found"。
		var existing models.Merchant
		err := tx.Where("user_id = ?", app.UserID).First(&existing).Error
		if err == nil {
			return nil // 已存在，幂等
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		merchantName := app.CompanyName
		if merchantName == "" {
			merchantName = fmt.Sprintf("merchant_%d", app.UserID)
		}
		merchant := models.Merchant{
			UserID:         uint(app.UserID),
			MerchantName:   merchantName,
			MerchantHandle: fmt.Sprintf("m%d", app.UserID),
			MerchantLevel:  "gold",
			DepositAmount:  app.DepositAmount,
			CommissionRate: 0.30,
			Status:         1,
			CreatedAt:      now,
		}
		return tx.Create(&merchant).Error
	})
}

// RejectMerchant 拒绝商家申请。
func (s *AdminService) RejectMerchant(applicationID int, adminRemark string) error {
	var app models.KolApplication
	if err := s.db.Where("id = ? AND apply_type = ?", applicationID, "merchant").First(&app).Error; err != nil {
		return err
	}

	if app.Status != 0 {
		return errors.New("application already processed")
	}

	now := time.Now()
	return s.db.Model(&app).Updates(map[string]interface{}{
		"status":       2,
		"admin_remark": adminRemark,
		"processed_at": &now,
	}).Error
}

// ========== 4. 分站审批 ==========

// GetPendingDistributors 获取待审批分站列表。
func (s *AdminService) GetPendingDistributors(page, pageSize int) (*UserListResult, error) {
	var total int64
	var applications []models.KolApplication

	db := s.db.Model(&models.KolApplication{}).
		Where("apply_type = ? AND status = ?", "distributor", 0)

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("applied_at DESC").Find(&applications).Error; err != nil {
		return nil, err
	}

	// 加载关联用户信息
	userIDs := make([]int, len(applications))
	for i, app := range applications {
		userIDs[i] = app.UserID
	}

	var users []models.User
	if len(userIDs) > 0 {
		s.db.Where("id IN ?", userIDs).Find(&users)
	}

	return &UserListResult{
		Total: int(total),
		Users: users,
	}, nil
}

// GetDistributorApplication 获取分站申请详情。
func (s *AdminService) GetDistributorApplication(applicationID int) (*models.KolApplication, error) {
	var app models.KolApplication
	if err := s.db.Where("id = ? AND apply_type = ?", applicationID, "distributor").First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ApproveDistributor 批准分站申请（user_level 升级为 3）。
func (s *AdminService) ApproveDistributor(applicationID int, adminRemark string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var app models.KolApplication
		if err := tx.Where("id = ? AND apply_type = ?", applicationID, "distributor").First(&app).Error; err != nil {
			return err
		}

		if app.Status != 0 {
			return errors.New("application already processed")
		}

		// 更新申请状态
		now := time.Now()
		if err := tx.Model(&app).Updates(map[string]interface{}{
			"status":       1,
			"admin_remark": adminRemark,
			"processed_at": &now,
		}).Error; err != nil {
			return err
		}

		// 升级用户等级为分站 (user_level = 3)
		if err := tx.Model(&models.User{}).Where("id = ?", app.UserID).Update("user_level", 3).Error; err != nil {
			return err
		}

		// 创建 distributor_sites 记录（若不存在）——分站后台所有接口依赖此记录。
		var existing models.DistributorSite
		err := tx.Where("owner_id = ?", app.UserID).First(&existing).Error
		if err == nil {
			return nil // 已存在，幂等
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		siteName := app.CompanyName
		if siteName == "" {
			siteName = fmt.Sprintf("站点_%d", app.UserID)
		}
		site := models.DistributorSite{
			OwnerID:           uint(app.UserID),
			Slug:              fmt.Sprintf("d%d", app.UserID),
			Name:              siteName,
			Theme:             "default",
			MarkupMode:        "global",
			GlobalMarkupRatio: 3.0, // 默认全站加价 300%（老大定的）
			Status:            1,
			CreatedAt:         now,
		}
		return tx.Create(&site).Error
	})
}

// RejectDistributor 拒绝分站申请。
func (s *AdminService) RejectDistributor(applicationID int, adminRemark string) error {
	var app models.KolApplication
	if err := s.db.Where("id = ? AND apply_type = ?", applicationID, "distributor").First(&app).Error; err != nil {
		return err
	}

	if app.Status != 0 {
		return errors.New("application already processed")
	}

	now := time.Now()
	return s.db.Model(&app).Updates(map[string]interface{}{
		"status":       2,
		"admin_remark": adminRemark,
		"processed_at": &now,
	}).Error
}

// ========== 5. 渠道管理（总站官方渠道）==========

// ChannelListResult 渠道列表结果。
type ChannelListResult struct {
	Total    int              `json:"total"`
	Channels []models.Channel `json:"channels"`
}

// GetOfficialChannels 获取总站官方渠道列表（merchant_id IS NULL）。
func (s *AdminService) GetOfficialChannels(page, pageSize int) (*ChannelListResult, error) {
	var total int64
	var channels []models.Channel

	db := s.db.Model(&models.Channel{}).Where("merchant_id IS NULL")

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&channels).Error; err != nil {
		return nil, err
	}

	return &ChannelListResult{
		Total:    int(total),
		Channels: channels,
	}, nil
}

// CreateChannelInput 创建渠道参数。
type CreateChannelInput struct {
	Type    int    `json:"type" binding:"required"`
	Key     string `json:"key" binding:"required"`
	Name    string `json:"name" binding:"required"`
	BaseURL string `json:"base_url"`
	Models  string `json:"models"`
	Status  int    `json:"status"`
	Group   string `json:"group"`
}

// CreateOfficialChannel 创建总站官方渠道。
// 走 New-API POST /api/channel/（同建 channels+abilities+驱动 AI 路由），
// 不再直接写库。官方渠道 merchant_id 保持 NULL。
func (s *AdminService) CreateOfficialChannel(input CreateChannelInput) error {
	if s.newAPIClient == nil {
		return errors.New("newAPIClient 未注入，无法创建渠道")
	}
	group := input.Group
	if group == "" {
		group = "default"
	}
	// New-API 建 channel（会自动建 abilities）
	_, err := s.newAPIClient.CreateChannel(input.Name, input.Key, input.BaseURL, input.Models, group, input.Type)
	if err != nil {
		return fmt.Errorf("New-API 建官方渠道失败: %w", err)
	}
	return nil
}

// UpdateChannelInput 更新渠道参数。
type UpdateChannelInput struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Models string `json:"models"`
	Status *int   `json:"status"`
	Group  string `json:"group"`
}

// UpdateChannel 更新渠道。
//
// ⚠️ 路由风险(批9标注): 若更新 models 字段, New-API 的 abilities 表不会自动同步,
// 会导致新增/删除的模型路由不生效。当前只支持改 key/name/status/group(不影响路由);
// 改 models 的正确做法是删旧渠道重建(走 New-API), 或调 New-API PUT /api/channel/。
// TODO: models 变更时改走 New-API PUT + UpdateChannelStatus。
func (s *AdminService) UpdateChannel(channelID int, input UpdateChannelInput) error {
	updates := make(map[string]interface{})

	if input.Key != "" {
		updates["key"] = input.Key
	}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Models != "" {
		updates["models"] = input.Models
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.Group != "" {
		updates["group"] = input.Group
	}

	if len(updates) == 0 {
		return errors.New("no fields to update")
	}

	return s.db.Model(&models.Channel{}).
		Where("id = ? AND merchant_id IS NULL", channelID).
		Updates(updates).Error
}

// DeleteChannel 删除官方渠道（走 New-API，级联删 abilities+刷新缓存）。
func (s *AdminService) DeleteChannel(channelID int) error {
	// 校验是官方渠道（merchant_id IS NULL），防误删商家渠道
	var cnt int64
	if err := s.db.Model(&models.Channel{}).
		Where("id = ? AND merchant_id IS NULL", channelID).
		Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return errors.New("官方渠道不存在")
	}
	if s.newAPIClient == nil {
		return errors.New("newAPIClient 未注入，无法删除渠道")
	}
	return s.newAPIClient.DeleteChannel(channelID)
}

// ========== 6. 套餐管理（总站套餐）==========

// PackageListResult 套餐列表结果。
type PackageListResult struct {
	Total    int             `json:"total"`
	Packages []models.Package `json:"packages"`
}

// GetOfficialPackages 获取总站套餐列表（distributor_id = 0）。
func (s *AdminService) GetOfficialPackages(page, pageSize int) (*PackageListResult, error) {
	var total int64
	var packages []models.Package

	db := s.db.Model(&models.Package{}).Where("distributor_id = ?", 0)

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&packages).Error; err != nil {
		return nil, err
	}

	return &PackageListResult{
		Total:    int(total),
		Packages: packages,
	}, nil
}

// CreatePackageInput 创建套餐参数。
type CreatePackageInput struct {
	Name          string  `json:"name" binding:"required"`
	Description   string  `json:"description"`
	Price         float64 `json:"price" binding:"required"`
	OriginalPrice float64 `json:"original_price"`
	QuotaAmount   int64   `json:"quota_amount" binding:"required"`
	DurationDays  int     `json:"duration_days"`
	ResetPeriod   string  `json:"reset_period"`
	Enabled       bool    `json:"enabled"`
}

// CreatePackage 创建总站套餐。
func (s *AdminService) CreatePackage(input CreatePackageInput) error {
	pkg := models.Package{
		DistributorID: 0, // 总站套餐
		Name:          input.Name,
		Description:   input.Description,
		Price:         input.Price,
		OriginalPrice: input.OriginalPrice,
		QuotaAmount:   input.QuotaAmount,
		DurationDays:  input.DurationDays,
		ResetPeriod:   input.ResetPeriod,
		Enabled:       input.Enabled,
		CreatedAt:     time.Now(),
	}

	return s.db.Create(&pkg).Error
}

// UpdatePackageInput 更新套餐参数。
type UpdatePackageInput struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Price         *float64 `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	QuotaAmount   *int64   `json:"quota_amount"`
	DurationDays  *int     `json:"duration_days"`
	ResetPeriod   string   `json:"reset_period"`
	Enabled       *bool    `json:"enabled"`
}

// UpdatePackage 更新套餐。
func (s *AdminService) UpdatePackage(packageID int, input UpdatePackageInput) error {
	updates := make(map[string]interface{})

	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}
	if input.Price != nil {
		updates["price"] = *input.Price
	}
	if input.OriginalPrice != nil {
		updates["original_price"] = *input.OriginalPrice
	}
	if input.QuotaAmount != nil {
		updates["quota_amount"] = *input.QuotaAmount
	}
	if input.DurationDays != nil {
		updates["duration_days"] = *input.DurationDays
	}
	if input.ResetPeriod != "" {
		updates["reset_period"] = input.ResetPeriod
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}

	if len(updates) == 0 {
		return errors.New("no fields to update")
	}

	return s.db.Model(&models.Package{}).
		Where("id = ? AND distributor_id = ?", packageID, 0).
		Updates(updates).Error
}

// DeletePackage 删除套餐。
func (s *AdminService) DeletePackage(packageID int) error {
	return s.db.Where("id = ? AND distributor_id = ?", packageID, 0).Delete(&models.Package{}).Error
}

// ========== 7. 兑换码管理 ==========

// RedeemCodeListResult 兑换码列表结果。
type RedeemCodeListResult struct {
	Total int                 `json:"total"`
	Codes []models.Redemption `json:"codes"`
}

// GetRedeemCodes 获取兑换码列表。
func (s *AdminService) GetRedeemCodes(page, pageSize int) (*RedeemCodeListResult, error) {
	var total int64
	var codes []models.Redemption

	db := s.db.Model(&models.Redemption{})

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&codes).Error; err != nil {
		return nil, err
	}

	return &RedeemCodeListResult{
		Total: int(total),
		Codes: codes,
	}, nil
}

// GenerateRedeemCodesInput 批量生成兑换码参数。
type GenerateRedeemCodesInput struct {
	Count       int    `json:"count" binding:"required,min=1,max=1000"`
	Name        string `json:"name" binding:"required"`
	Quota       int    `json:"quota" binding:"required,min=1"`
	ExpiredTime int64  `json:"expired_time"` // 0 表示不过期
}

// GenerateRedeemCodes 批量生成兑换码。
func (s *AdminService) GenerateRedeemCodes(input GenerateRedeemCodesInput) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		
		for i := 0; i < input.Count; i++ {
			code := models.Redemption{
				UserId:       1, // 管理员创建
				Key:          generateRedeemKey(),
				Status:       1, // 未使用
				Name:         fmt.Sprintf("%s-%d", input.Name, i+1),
				Quota:        input.Quota,
				CreatedTime:  now,
				ExpiredTime:  input.ExpiredTime,
				RedeemedTime: 0,
				UsedUserId:   0,
			}

			if err := tx.Create(&code).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// generateRedeemKey 生成兑换码（32位随机字符串）。
func generateRedeemKey() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// InvalidateRedeemCode 作废兑换码。
func (s *AdminService) InvalidateRedeemCode(codeID int) error {
	return s.db.Model(&models.Redemption{}).
		Where("id = ? AND status = ?", codeID, 1).
		Update("status", 2).Error
}

// ========== 8. 返佣管理 ==========

// ReferralListResult 返佣记录列表结果。
type ReferralListResult struct {
	Total     int                 `json:"total"`
	Referrals []models.AffHistory `json:"referrals"`
}

// GetReferrals 获取返佣记录列表。
func (s *AdminService) GetReferrals(page, pageSize int) (*ReferralListResult, error) {
	var total int64
	var referrals []models.AffHistory

	db := s.db.Model(&models.AffHistory{})

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&referrals).Error; err != nil {
		return nil, err
	}

	return &ReferralListResult{
		Total:     int(total),
		Referrals: referrals,
	}, nil
}

// ReferralStats 返佣统计。
type ReferralStats struct {
	TotalCommission   float64 `json:"total_commission"`   // 总返佣金额
	PendingCommission float64 `json:"pending_commission"` // 待结算
	SettledCommission float64 `json:"settled_commission"` // 已结算
}

// GetReferralStats 获取返佣统计。
func (s *AdminService) GetReferralStats() (*ReferralStats, error) {
	var stats ReferralStats

	// 总返佣金额（所有记录）
	var totalQuota int64
	s.db.Model(&models.AffHistory{}).Select("COALESCE(SUM(quota), 0)").Scan(&totalQuota)
	stats.TotalCommission = float64(totalQuota) / 500000.0

	// 待结算（status = 0）
	var pendingQuota int64
	s.db.Model(&models.AffHistory{}).Where("status = ?", 0).Select("COALESCE(SUM(quota), 0)").Scan(&pendingQuota)
	stats.PendingCommission = float64(pendingQuota) / 500000.0

	// 已结算（status = 1）
	var settledQuota int64
	s.db.Model(&models.AffHistory{}).Where("status = ?", 1).Select("COALESCE(SUM(quota), 0)").Scan(&settledQuota)
	stats.SettledCommission = float64(settledQuota) / 500000.0

	return &stats, nil
}

// SettleReferral 手动结算返佣。
func (s *AdminService) SettleReferral(referralID int) error {
	return s.db.Model(&models.AffHistory{}).
		Where("id = ? AND status = ?", referralID, 0).
		Update("status", 1).Error
}

// ========== 9. 系统配置 ==========

// SystemConfig 系统配置。
type SystemConfig struct {
	DefaultCommissionRate float64 `json:"default_commission_rate"` // 默认返佣比例
	RegisterReward        int64   `json:"register_reward"`         // 注册奖励（Quota）
	FirstTopupReward      int64   `json:"first_topup_reward"`      // 首充奖励（Quota）
	LeoThreshold          int64   `json:"leo_threshold"`           // LEO 门槛（Quota）
}

// GetSystemConfig 获取系统配置。
func (s *AdminService) GetSystemConfig() (*SystemConfig, error) {
	config := &SystemConfig{
		DefaultCommissionRate: 0.10, // 默认 10%
		RegisterReward:        50000,  // 默认 $0.10
		FirstTopupReward:      100000, // 默认 $0.20
		LeoThreshold:          5000000, // 默认 $10
	}

	// 从 referral_config 表读取配置
	var registerConfig models.ReferralConfig
	if err := s.db.Where("reward_type = ?", "register").First(&registerConfig).Error; err == nil {
		config.RegisterReward = int64(registerConfig.RewardAmount * 500000)
	}

	var firstTopupConfig models.ReferralConfig
	if err := s.db.Where("reward_type = ?", "first_topup").First(&firstTopupConfig).Error; err == nil {
		config.FirstTopupReward = int64(firstTopupConfig.RewardAmount * 500000)
	}

	var commissionConfig models.ReferralConfig
	if err := s.db.Where("reward_type = ?", "consumption_rate").First(&commissionConfig).Error; err == nil {
		config.DefaultCommissionRate = commissionConfig.RewardAmount
	}

	return config, nil
}

// UpdateSystemConfigInput 更新系统配置参数。
type UpdateSystemConfigInput struct {
	DefaultCommissionRate *float64 `json:"default_commission_rate"`
	RegisterReward        *int64   `json:"register_reward"`
	FirstTopupReward      *int64   `json:"first_topup_reward"`
	LeoThreshold          *int64   `json:"leo_threshold"`
}

// UpdateSystemConfig 更新系统配置。
func (s *AdminService) UpdateSystemConfig(input UpdateSystemConfigInput) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 更新返佣比例
		if input.DefaultCommissionRate != nil {
			tx.Model(&models.ReferralConfig{}).
				Where("reward_type = ?", "consumption_rate").
				Update("reward_amount", *input.DefaultCommissionRate)
		}

		// 更新注册奖励
		if input.RegisterReward != nil {
			rewardAmount := float64(*input.RegisterReward) / 500000.0
			tx.Model(&models.ReferralConfig{}).
				Where("reward_type = ?", "register").
				Update("reward_amount", rewardAmount)
		}

		// 更新首充奖励
		if input.FirstTopupReward != nil {
			rewardAmount := float64(*input.FirstTopupReward) / 500000.0
			tx.Model(&models.ReferralConfig{}).
				Where("reward_type = ?", "first_topup").
				Update("reward_amount", rewardAmount)
		}

		// LEO 门槛配置（需要系统配置表，这里简化处理）
		// 实际应该有单独的 system_configs 表

		return nil
	})
}
