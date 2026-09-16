// Package services 实现业务逻辑层。
package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/utils"
	"gorm.io/gorm"
)

// MerchantService 商家中心业务逻辑。
type MerchantService struct {
	db           *gorm.DB
	newAPIClient *NewAPIClient
}

// NewMerchantService 创建 MerchantService。
func NewMerchantService(db *gorm.DB, newAPIClient *NewAPIClient) *MerchantService {
	return &MerchantService{db: db, newAPIClient: newAPIClient}
}

// MerchantDashboardStats 商家首页统计数据。
type MerchantDashboardStats struct {
	TotalRevenue   float64 `json:"total_revenue"`    // 总收益（美元）
	PendingRevenue float64 `json:"pending_revenue"`  // 待结算收益
	SettledRevenue float64 `json:"settled_revenue"`  // 已结算收益
	TotalModels    int     `json:"total_models"`     // 模型数量
	TotalRequests  int64   `json:"total_requests"`   // 总调用量
	TotalOrders    int     `json:"total_orders"`     // 订单数
}

// GetDashboardStats 获取商家首页统计数据。
func (s *MerchantService) GetDashboardStats(userID int) (*MerchantDashboardStats, error) {
	// 获取商家信息
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	stats := &MerchantDashboardStats{}

	// 总收益 = merchant.total_revenue
	stats.TotalRevenue = merchant.TotalRevenue
	stats.TotalRequests = merchant.TotalRequests

	// 计算待结算和已结算收益
	// 这里简化处理：假设 total_revenue 是累计收益，待结算 = 可提现余额
	stats.SettledRevenue = stats.TotalRevenue - merchant.Balance
	stats.PendingRevenue = merchant.Balance

	// 统计模型数量（已通过审核的）
	var modelCount int64
	s.db.Model(&models.MerchantModel{}).
		Where("merchant_id = ? AND status = ?", merchant.ID, 1).
		Count(&modelCount)
	stats.TotalModels = int(modelCount)

	// 统计订单数（从收益记录表）
	var orderCount int64
	s.db.Model(&models.MerchantRevenueRecord{}).
		Where("merchant_id = ?", merchant.ID).
		Count(&orderCount)
	stats.TotalOrders = int(orderCount)

	return stats, nil
}

// RevenueTrendItem 收益趋势单日数据。
type RevenueTrendItem struct {
	Date    string  `json:"date"`    // YYYY-MM-DD
	Revenue float64 `json:"revenue"` // 当日收益
}

// GetRevenueTrend 获取收益趋势（近30天）。
func (s *MerchantService) GetRevenueTrend(userID int) ([]RevenueTrendItem, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	// 查询近30天的收益记录
	type DailyRevenue struct {
		Date    time.Time
		Revenue float64
	}

	var dailyRevenues []DailyRevenue
	err := s.db.Model(&models.MerchantRevenueRecord{}).
		Select("DATE(request_date) as date, SUM(revenue) as revenue").
		Where("merchant_id = ? AND request_date >= ?", merchant.ID, time.Now().AddDate(0, 0, -30)).
		Group("DATE(request_date)").
		Order("date ASC").
		Scan(&dailyRevenues).Error

	if err != nil {
		return nil, err
	}

	// 转换为前端格式
	result := make([]RevenueTrendItem, len(dailyRevenues))
	for i, dr := range dailyRevenues {
		result[i] = RevenueTrendItem{
			Date:    dr.Date.Format("2006-01-02"),
			Revenue: dr.Revenue,
		}
	}

	return result, nil
}

// GetModels 获取商家的模型列表（分页）。
func (s *MerchantService) GetModels(userID int, page, pageSize int) ([]models.MerchantModel, int64, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, 0, fmt.Errorf("merchant not found: %w", err)
	}

	var merchantModels []models.MerchantModel
	var total int64

	query := s.db.Model(&models.MerchantModel{}).Where("merchant_id = ?", merchant.ID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&merchantModels).Error

	return merchantModels, total, err
}

// CreateModel 添加新模型（创建审批记录）。
func (s *MerchantService) CreateModel(userID int, modelName string, channelID int, inputPrice, outputPrice float64) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	// 创建审批记录
	approval := models.MerchantModel{
		MerchantID:   merchant.ID,
		ModelName:    modelName,
		ChannelID:    channelID,
		InputPrice:   inputPrice,
		OutputPrice:  outputPrice,
		ApprovalType: "create",
		Status:       0, // 待审核
		SubmittedAt:  time.Now(),
	}

	return s.db.Create(&approval).Error
}

// UpdateModel 更新模型（创建调价审批记录）。
func (s *MerchantService) UpdateModel(userID int, modelID uint, inputPrice, outputPrice float64) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	// 查询原模型
	var oldModel models.MerchantModel
	if err := s.db.Where("id = ? AND merchant_id = ?", modelID, merchant.ID).First(&oldModel).Error; err != nil {
		return fmt.Errorf("model not found: %w", err)
	}

	// 创建更新审批记录
	approval := models.MerchantModel{
		MerchantID:   merchant.ID,
		ModelName:    oldModel.ModelName,
		ChannelID:    oldModel.ChannelID,
		InputPrice:   inputPrice,
		OutputPrice:  outputPrice,
		ApprovalType: "update",
		Status:       0, // 待审核
		SubmittedAt:  time.Now(),
	}

	return s.db.Create(&approval).Error
}

// DeleteModel 下架模型（软删除：设置状态为2-拒绝）。
func (s *MerchantService) DeleteModel(userID int, modelID uint) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	// 只能下架自己的模型
	result := s.db.Model(&models.MerchantModel{}).
		Where("id = ? AND merchant_id = ?", modelID, merchant.ID).
		Update("status", 2)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("model not found")
	}

	return nil
}

// ModelStats 单个模型统计数据。
type ModelStats struct {
	TotalRequests int64   `json:"total_requests"`
	TotalRevenue  float64 `json:"total_revenue"`
	TotalCost     float64 `json:"total_cost"`
	TotalProfit   float64 `json:"total_profit"`
}

// GetModelStats 获取单个模型统计。
func (s *MerchantService) GetModelStats(userID int, modelID uint) (*ModelStats, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	// 查询模型
	var model models.MerchantModel
	if err := s.db.Where("id = ? AND merchant_id = ?", modelID, merchant.ID).First(&model).Error; err != nil {
		return nil, fmt.Errorf("model not found: %w", err)
	}

	// 统计该模型的收益
	var stats ModelStats
	err := s.db.Model(&models.MerchantRevenueRecord{}).
		Select("COUNT(*) as total_requests, SUM(revenue) as total_revenue, SUM(cost) as total_cost, SUM(profit) as total_profit").
		Where("merchant_id = ? AND model_name = ?", merchant.ID, model.ModelName).
		Scan(&stats).Error

	return &stats, err
}

// GetChannels 获取商家的上游渠道列表。
func (s *MerchantService) GetChannels(userID int) ([]models.Channel, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	var channels []models.Channel
	err := s.db.Where("merchant_id = ?", merchant.ID).Find(&channels).Error
	return channels, err
}

// CreateChannel 添加上游渠道。
//
// ⚠️⚠️ 架构债 (ADR-001, 批3待专项打通): 此处直接写 channels 表, 但 New-API 的
// AI 调用路由依赖 abilities 表 (channels + abilities 一起才生效)。New-API 自己
// 建 channel 时会自动 AddAbilities() 建路由记录并刷新 channel 缓存; LeapNode 直接
// 写 channels 跳过了这步, 导致【商家建的渠道对 AI 调用完全隐形——用户调用 AI 时
// New-API 不会路由到这些渠道】。
// 正确做法: 走 New-API POST /api/channel (它同时处理 channels+abilities+缓存)。
// 当前先保留直接写库 (管理后台能看到渠道列表), AI 实际路由打通留批 channel 专项。
// 判据: 商家渠道要真正驱动 AI 调用, 必须改成调 New-API channel API。
func (s *MerchantService) CreateChannel(userID int, channelType int, key, name, group, modelList string, inputPrice, outputPrice float64) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}
	merchantID := int(merchant.ID)

	if group == "" {
		group = "default"
	}

	// 1. 通过 New-API 建 channel（它会同建 channels + abilities，驱动 AI 路由）。
	//    base_url 走 New-API 默认（OpenAI 兼容渠道由 New-API 按 type 填默认），
	//    这里商家渠道默认空 base_url 用官方端点；如需自定义上游可扩展入参。
	if s.newAPIClient == nil {
		return fmt.Errorf("newAPIClient 未注入，无法创建渠道")
	}
	channelID, err := s.newAPIClient.CreateChannel(name, key, "", modelList, group, channelType)
	if err != nil {
		return fmt.Errorf("New-API 建渠道失败: %w", err)
	}

	// 2. 回写 LeapNode 扩展字段到同一行（merchant_id / 成本价）。
	//    channels 由 New-API 独占写，这里只 UPDATE 扩展列，不碰 New-API 管的字段。
	if err := s.db.Model(&models.Channel{}).
		Where("id = ?", channelID).
		Updates(map[string]interface{}{
			"merchant_id":  merchantID,
			"input_price":  inputPrice,
			"output_price": outputPrice,
		}).Error; err != nil {
		// 扩展字段回写失败不影响渠道可用（AI 路由已生效），仅记录
		return fmt.Errorf("渠道已建(id=%d)但回写商家扩展字段失败: %w", channelID, err)
	}

	return nil
}

// UpdateChannel 更新渠道。
func (s *MerchantService) UpdateChannel(userID int, channelID int, key, name, group, modelList string, inputPrice, outputPrice float64) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	updates := map[string]interface{}{
		"key":          key,
		"name":         name,
		"group":        group,
		"models":       modelList,
		"input_price":  inputPrice,
		"output_price": outputPrice,
	}

	result := s.db.Model(&models.Channel{}).
		Where("id = ? AND merchant_id = ?", channelID, merchant.ID).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("channel not found")
	}

	return nil
}

// DeleteChannel 删除渠道（走 New-API，同步删 channels+abilities+刷新缓存）。
func (s *MerchantService) DeleteChannel(userID int, channelID int) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	// 归属校验：该 channel 必须属于当前商家（防越权删别人的渠道）
	var cnt int64
	if err := s.db.Model(&models.Channel{}).
		Where("id = ? AND merchant_id = ?", channelID, merchant.ID).
		Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return fmt.Errorf("channel not found")
	}

	if s.newAPIClient == nil {
		return fmt.Errorf("newAPIClient 未注入，无法删除渠道")
	}
	// New-API 删 channel 会级联删 abilities 并刷新路由缓存
	if err := s.newAPIClient.DeleteChannel(channelID); err != nil {
		return fmt.Errorf("New-API 删渠道失败: %w", err)
	}
	return nil
}

// TestChannel 测试渠道连通性（简化实现：返回成功）。
func (s *MerchantService) TestChannel(userID int, channelID int) (bool, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return false, fmt.Errorf("merchant not found: %w", err)
	}

	// 查询渠道是否存在
	var channel models.Channel
	if err := s.db.Where("id = ? AND merchant_id = ?", channelID, merchant.ID).First(&channel).Error; err != nil {
		return false, fmt.Errorf("channel not found: %w", err)
	}

	// 简化实现：直接返回成功
	// 实际生产环境需要调用上游 API 测试
	return true, nil
}

// RevenueSummary 收益汇总。
type RevenueSummary struct {
	TotalRevenue     float64 `json:"total_revenue"`      // 总收益
	WithdrawableAmount float64 `json:"withdrawable_amount"` // 可提现余额
	WithdrawnAmount  float64 `json:"withdrawn_amount"`   // 已提现
}

// GetRevenueSummary 获取收益汇总。
func (s *MerchantService) GetRevenueSummary(userID int) (*RevenueSummary, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	summary := &RevenueSummary{
		TotalRevenue:       merchant.TotalRevenue,
		WithdrawableAmount: merchant.Balance,
		WithdrawnAmount:    merchant.TotalRevenue - merchant.Balance,
	}

	return summary, nil
}

// GetRevenueRecords 获取收益明细（分页）。
func (s *MerchantService) GetRevenueRecords(userID int, page, pageSize int) ([]models.MerchantRevenueRecord, int64, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, 0, fmt.Errorf("merchant not found: %w", err)
	}

	var records []models.MerchantRevenueRecord
	var total int64

	query := s.db.Model(&models.MerchantRevenueRecord{}).Where("merchant_id = ?", merchant.ID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&records).Error

	return records, total, err
}

// RevenueByModel 按模型分组的收益。
type RevenueByModel struct {
	ModelName string  `json:"model_name"`
	Revenue   float64 `json:"revenue"`
	Requests  int64   `json:"requests"`
}

// GetRevenueByModel 获取按模型分组的收益。
func (s *MerchantService) GetRevenueByModel(userID int) ([]RevenueByModel, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	var results []RevenueByModel
	err := s.db.Model(&models.MerchantRevenueRecord{}).
		Select("model_name, SUM(revenue) as revenue, COUNT(*) as requests").
		Where("merchant_id = ?", merchant.ID).
		Group("model_name").
		Order("revenue DESC").
		Scan(&results).Error

	return results, err
}

// GetWithdrawals 获取提现记录列表。
func (s *MerchantService) GetWithdrawals(userID int, page, pageSize int) ([]models.WithdrawalRequest, int64, error) {
	var total int64
	var withdrawals []models.WithdrawalRequest

	query := s.db.Model(&models.WithdrawalRequest{}).Where("user_id = ?", userID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&withdrawals).Error

	return withdrawals, total, err
}

// CreateWithdrawal 申请提现。
func (s *MerchantService) CreateWithdrawal(userID int, amount float64, paymentMethod, accountInfo string) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	// 检查可提现余额
	if merchant.Balance < amount {
		return fmt.Errorf("insufficient balance")
	}

	// 创建提现申请
	withdrawal := models.WithdrawalRequest{
		UserID:        userID,
		Amount:        amount,
		PaymentMethod: paymentMethod,
		AccountInfo:   accountInfo,
		Status:        0, // 待审核
		CreatedAt:     time.Now(),
	}

	return s.db.Create(&withdrawal).Error
}

// GetWithdrawalByID 获取提现详情。
func (s *MerchantService) GetWithdrawalByID(userID int, withdrawalID int64) (*models.WithdrawalRequest, error) {
	var withdrawal models.WithdrawalRequest
	err := s.db.Where("id = ? AND user_id = ?", withdrawalID, userID).First(&withdrawal).Error
	return &withdrawal, err
}

// GetRedeemCodes 获取商家的兑换码列表。
func (s *MerchantService) GetRedeemCodes(userID int, page, pageSize int) ([]models.RedeemCode, int64, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, 0, fmt.Errorf("merchant not found: %w", err)
	}

	merchantID := int(merchant.ID)
	var codes []models.RedeemCode
	var total int64

	query := s.db.Model(&models.RedeemCode{}).Where("merchant_id = ?", merchantID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&codes).Error

	return codes, total, err
}

// GenerateRedeemCode 生成兑换码（商家自费）。
func (s *MerchantService) GenerateRedeemCode(userID int, quota int64, batchName string, count int) error {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	// 计算总成本（Quota 转美元）
	totalCost := utils.QuotaToDollars(quota * int64(count))

	// 检查余额
	if merchant.Balance < totalCost {
		return fmt.Errorf("insufficient balance")
	}

	// 扣除余额
	if err := s.db.Model(&merchant).Update("balance", gorm.Expr("balance - ?", totalCost)).Error; err != nil {
		return err
	}

	// 生成兑换码
	merchantID := int(merchant.ID)
	for i := 0; i < count; i++ {
		code := generateRandomCode(16)
		redeemCode := models.RedeemCode{
			Code:          code,
			DistributorID: 0,
			Quota:         quota,
			BatchName:     batchName,
			Status:        0, // 未使用
			CreatedAt:     time.Now(),
			MerchantID:    &merchantID,
		}

		if err := s.db.Create(&redeemCode).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetRedeemCodeUsage 获取兑换码使用记录。
func (s *MerchantService) GetRedeemCodeUsage(userID int, code string) ([]map[string]interface{}, error) {
	var merchant models.Merchant
	if err := s.db.Where("user_id = ?", userID).First(&merchant).Error; err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	merchantID := int(merchant.ID)
	
	// 查询兑换码
	var redeemCode models.RedeemCode
	if err := s.db.Where("code = ? AND merchant_id = ?", code, merchantID).First(&redeemCode).Error; err != nil {
		return nil, fmt.Errorf("redeem code not found: %w", err)
	}

	// 如果已使用，返回使用者信息
	if redeemCode.Status == 1 && redeemCode.UsedBy != nil {
		var user models.User
		if err := s.db.Where("id = ?", *redeemCode.UsedBy).First(&user).Error; err == nil {
			return []map[string]interface{}{
				{
					"user_id":   user.ID,
					"username":  user.Username,
					"used_at":   redeemCode.UsedAt,
					"quota":     redeemCode.Quota,
				},
			}, nil
		}
	}

	return []map[string]interface{}{}, nil
}

// GetTickets 获取工单列表。
func (s *MerchantService) GetTickets(userID int, page, pageSize int) ([]models.SupportTicket, int64, error) {
	var total int64
	var tickets []models.SupportTicket

	query := s.db.Model(&models.SupportTicket{}).Where("user_id = ?", userID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&tickets).Error

	return tickets, total, err
}

// CreateTicket 创建工单。
func (s *MerchantService) CreateTicket(userID int, ticketType, title, content, priority string) error {
	ticket := models.SupportTicket{
		UserID:     userID,
		TicketType: ticketType,
		Title:      title,
		Content:    content,
		Priority:   priority,
		Status:     0, // 待处理
		CreatedAt:  time.Now(),
	}

	return s.db.Create(&ticket).Error
}

// GetTicketByID 获取工单详情。
func (s *MerchantService) GetTicketByID(userID int, ticketID uint) (*models.SupportTicket, error) {
	var ticket models.SupportTicket
	err := s.db.Where("id = ? AND user_id = ?", ticketID, userID).First(&ticket).Error
	return &ticket, err
}

// ReplyTicket 回复工单（商家追加内容）。
func (s *MerchantService) ReplyTicket(userID int, ticketID uint, reply string) error {
	// 查询工单
	var ticket models.SupportTicket
	if err := s.db.Where("id = ? AND user_id = ?", ticketID, userID).First(&ticket).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	// 追加回复到 content（简化实现）
	newContent := ticket.Content + "\n\n[商家回复 " + time.Now().Format("2006-01-02 15:04:05") + "]\n" + reply

	return s.db.Model(&ticket).Update("content", newContent).Error
}

// GetAnnouncements 获取平台公告列表（商家可见）。
func (s *MerchantService) GetAnnouncements(userID int, page, pageSize int) ([]models.Announcement, int64, error) {
	var total int64
	var announcements []models.Announcement

	// 查询针对商家或全部用户的公告
	query := s.db.Model(&models.Announcement{}).
		Where("status = ? AND (target_users = ? OR target_users = ?)", 1, "merchant", "all")
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("published_at DESC").Offset(offset).Limit(pageSize).Find(&announcements).Error

	return announcements, total, err
}

// GetAnnouncementByID 获取公告详情。
func (s *MerchantService) GetAnnouncementByID(announcementID uint) (*models.Announcement, error) {
	var announcement models.Announcement
	err := s.db.Where("id = ?", announcementID).First(&announcement).Error
	return &announcement, err
}

// MarkAnnouncementAsRead 标记公告为已读。
func (s *MerchantService) MarkAnnouncementAsRead(userID int, announcementID uint) error {
	// 检查是否已读
	var existing models.AnnouncementRead
	err := s.db.Where("user_id = ? AND announcement_id = ?", userID, announcementID).First(&existing).Error
	if err == nil {
		// 已存在，无需重复插入
		return nil
	}

	// 创建已读记录
	read := models.AnnouncementRead{
		UserID:         userID,
		AnnouncementID: int(announcementID),
		ReadAt:         time.Now(),
	}

	return s.db.Create(&read).Error
}

// generateRandomCode 生成随机兑换码。
func generateRandomCode(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}
