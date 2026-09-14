// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/middleware"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// MerchantController 处理 /api/merchant/* 商家中心接口。
type MerchantController struct {
	merchantService *services.MerchantService
}

// NewMerchantController 创建 MerchantController。
func NewMerchantController(merchantService *services.MerchantService) *MerchantController {
	return &MerchantController{merchantService: merchantService}
}

// RegisterRoutes 注册商家中心路由。
// 调用方需已对 group 应用 middleware.AuthRequired + middleware.MerchantAuth。
func (ctrl *MerchantController) RegisterRoutes(group *gin.RouterGroup) {
	// 1. 商家首页（Dashboard）
	group.GET("/dashboard/stats", ctrl.GetDashboardStats)
	group.GET("/dashboard/revenue-trend", ctrl.GetRevenueTrend)

	// 2. 我的模型
	group.GET("/models", ctrl.GetModels)
	group.POST("/models", ctrl.CreateModel)
	group.PUT("/models/:id", ctrl.UpdateModel)
	group.DELETE("/models/:id", ctrl.DeleteModel)
	group.GET("/models/:id/stats", ctrl.GetModelStats)

	// 3. 官方渠道（商家的上游渠道）
	group.GET("/channels", ctrl.GetChannels)
	group.POST("/channels", ctrl.CreateChannel)
	group.PUT("/channels/:id", ctrl.UpdateChannel)
	group.DELETE("/channels/:id", ctrl.DeleteChannel)
	group.GET("/channels/:id/test", ctrl.TestChannel)

	// 4. 收益管理
	group.GET("/revenue/summary", ctrl.GetRevenueSummary)
	group.GET("/revenue/records", ctrl.GetRevenueRecords)
	group.GET("/revenue/by-model", ctrl.GetRevenueByModel)

	// 5. 提现管理
	group.GET("/withdrawals", ctrl.GetWithdrawals)
	group.POST("/withdrawals", ctrl.CreateWithdrawal)
	group.GET("/withdrawals/:id", ctrl.GetWithdrawalByID)

	// 6. 兑换码
	group.GET("/redeem-codes", ctrl.GetRedeemCodes)
	group.POST("/redeem-codes/generate", ctrl.GenerateRedeemCodes)
	group.GET("/redeem-codes/:code/usage", ctrl.GetRedeemCodeUsage)

	// 7. 问题工单
	group.GET("/tickets", ctrl.GetTickets)
	group.POST("/tickets", ctrl.CreateTicket)
	group.GET("/tickets/:id", ctrl.GetTicketByID)
	group.PUT("/tickets/:id/reply", ctrl.ReplyTicket)

	// 8. 公告管理
	group.GET("/announcements", ctrl.GetAnnouncements)
	group.GET("/announcements/:id", ctrl.GetAnnouncementByID)
	group.PUT("/announcements/:id/read", ctrl.MarkAnnouncementAsRead)
}

// ========== 1. 商家首页 ==========

// GetDashboardStats 获取商家首页统计数据。
// GET /api/merchant/dashboard/stats
func (ctrl *MerchantController) GetDashboardStats(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	stats, err := ctrl.merchantService.GetDashboardStats(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, stats)
}

// GetRevenueTrend 获取收益趋势（近30天）。
// GET /api/merchant/dashboard/revenue-trend
func (ctrl *MerchantController) GetRevenueTrend(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	trend, err := ctrl.merchantService.GetRevenueTrend(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, trend)
}

// ========== 2. 我的模型 ==========

// GetModels 获取商家的模型列表（分页）。
// GET /api/merchant/models?page=1&page_size=20
func (ctrl *MerchantController) GetModels(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	models, total, err := ctrl.merchantService.GetModels(userID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"models": models,
		"total":  total,
		"page":   page,
		"page_size": pageSize,
	})
}

// CreateModel 添加新模型（上架审批）。
// POST /api/merchant/models
// Body: {"model_name": "gpt-4", "channel_id": 1, "input_price": 0.03, "output_price": 0.06}
func (ctrl *MerchantController) CreateModel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ModelName   string  `json:"model_name" binding:"required"`
		ChannelID   int     `json:"channel_id" binding:"required"`
		InputPrice  float64 `json:"input_price" binding:"required"`
		OutputPrice float64 `json:"output_price" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	err := ctrl.merchantService.CreateModel(userID, req.ModelName, req.ChannelID, req.InputPrice, req.OutputPrice)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "model submitted for approval"})
}

// UpdateModel 更新模型（调价审批）。
// PUT /api/merchant/models/:id
// Body: {"input_price": 0.03, "output_price": 0.06}
func (ctrl *MerchantController) UpdateModel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	modelID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid model id")
		return
	}

	var req struct {
		InputPrice  float64 `json:"input_price" binding:"required"`
		OutputPrice float64 `json:"output_price" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	err = ctrl.merchantService.UpdateModel(userID, uint(modelID), req.InputPrice, req.OutputPrice)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "price update submitted for approval"})
}

// DeleteModel 下架模型。
// DELETE /api/merchant/models/:id
func (ctrl *MerchantController) DeleteModel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	modelID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid model id")
		return
	}

	err = ctrl.merchantService.DeleteModel(userID, uint(modelID))
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "model deleted"})
}

// GetModelStats 获取单个模型统计。
// GET /api/merchant/models/:id/stats
func (ctrl *MerchantController) GetModelStats(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	modelID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid model id")
		return
	}

	stats, err := ctrl.merchantService.GetModelStats(userID, uint(modelID))
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, stats)
}

// ========== 3. 官方渠道 ==========

// GetChannels 获取商家的渠道列表。
// GET /api/merchant/channels
func (ctrl *MerchantController) GetChannels(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	channels, err := ctrl.merchantService.GetChannels(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, channels)
}

// CreateChannel 添加上游渠道。
// POST /api/merchant/channels
// Body: {"type": 1, "key": "sk-xxx", "name": "OpenAI", "group": "default", "models": "gpt-4,gpt-3.5-turbo", "input_price": 0.03, "output_price": 0.06}
func (ctrl *MerchantController) CreateChannel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Type        int     `json:"type" binding:"required"`
		Key         string  `json:"key" binding:"required"`
		Name        string  `json:"name" binding:"required"`
		Group       string  `json:"group"`
		Models      string  `json:"models" binding:"required"`
		InputPrice  float64 `json:"input_price" binding:"required"`
		OutputPrice float64 `json:"output_price" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	err := ctrl.merchantService.CreateChannel(userID, req.Type, req.Key, req.Name, req.Group, req.Models, req.InputPrice, req.OutputPrice)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel created"})
}

// UpdateChannel 更新渠道。
// PUT /api/merchant/channels/:id
func (ctrl *MerchantController) UpdateChannel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid channel id")
		return
	}

	var req struct {
		Key         string  `json:"key" binding:"required"`
		Name        string  `json:"name" binding:"required"`
		Group       string  `json:"group"`
		Models      string  `json:"models" binding:"required"`
		InputPrice  float64 `json:"input_price" binding:"required"`
		OutputPrice float64 `json:"output_price" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	err = ctrl.merchantService.UpdateChannel(userID, channelID, req.Key, req.Name, req.Group, req.Models, req.InputPrice, req.OutputPrice)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel updated"})
}

// DeleteChannel 删除渠道。
// DELETE /api/merchant/channels/:id
func (ctrl *MerchantController) DeleteChannel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid channel id")
		return
	}

	err = ctrl.merchantService.DeleteChannel(userID, channelID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel deleted"})
}

// TestChannel 测试渠道连通性。
// GET /api/merchant/channels/:id/test
func (ctrl *MerchantController) TestChannel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid channel id")
		return
	}

	success, err := ctrl.merchantService.TestChannel(userID, channelID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"success": success})
}

// ========== 4. 收益管理 ==========

// GetRevenueSummary 获取收益汇总。
// GET /api/merchant/revenue/summary
func (ctrl *MerchantController) GetRevenueSummary(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	summary, err := ctrl.merchantService.GetRevenueSummary(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, summary)
}

// GetRevenueRecords 获取收益明细（分页）。
// GET /api/merchant/revenue/records?page=1&page_size=20
func (ctrl *MerchantController) GetRevenueRecords(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	records, total, err := ctrl.merchantService.GetRevenueRecords(userID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"records": records,
		"total":   total,
		"page":    page,
		"page_size": pageSize,
	})
}

// GetRevenueByModel 获取按模型分组的收益。
// GET /api/merchant/revenue/by-model
func (ctrl *MerchantController) GetRevenueByModel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	results, err := ctrl.merchantService.GetRevenueByModel(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, results)
}

// ========== 5. 提现管理 ==========

// GetWithdrawals 获取提现记录列表。
// GET /api/merchant/withdrawals?page=1&page_size=20
func (ctrl *MerchantController) GetWithdrawals(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	withdrawals, total, err := ctrl.merchantService.GetWithdrawals(userID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"withdrawals": withdrawals,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
	})
}

// CreateWithdrawal 申请提现。
// POST /api/merchant/withdrawals
// Body: {"amount": 100.0, "payment_method": "paypal", "account_info": "user@example.com"}
func (ctrl *MerchantController) CreateWithdrawal(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Amount        float64 `json:"amount" binding:"required"`
		PaymentMethod string  `json:"payment_method" binding:"required"`
		AccountInfo   string  `json:"account_info" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	err := ctrl.merchantService.CreateWithdrawal(userID, req.Amount, req.PaymentMethod, req.AccountInfo)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "withdrawal request submitted"})
}

// GetWithdrawalByID 获取提现详情。
// GET /api/merchant/withdrawals/:id
func (ctrl *MerchantController) GetWithdrawalByID(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	withdrawalID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid withdrawal id")
		return
	}

	withdrawal, err := ctrl.merchantService.GetWithdrawalByID(userID, withdrawalID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, withdrawal)
}

// ========== 6. 兑换码 ==========

// GetRedeemCodes 获取商家的兑换码列表。
// GET /api/merchant/redeem-codes?page=1&page_size=20
func (ctrl *MerchantController) GetRedeemCodes(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	codes, total, err := ctrl.merchantService.GetRedeemCodes(userID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"codes": codes,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

// GenerateRedeemCodes 生成兑换码（商家自费）。
// POST /api/merchant/redeem-codes/generate
// Body: {"quota": 500000, "batch_name": "2024-promotion", "count": 10}
func (ctrl *MerchantController) GenerateRedeemCodes(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Quota     int64  `json:"quota" binding:"required"`
		BatchName string `json:"batch_name" binding:"required"`
		Count     int    `json:"count" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Count <= 0 || req.Count > 100 {
		utils.ErrorJSON(c, http.StatusBadRequest, "count must be between 1 and 100")
		return
	}

	err := ctrl.merchantService.GenerateRedeemCode(userID, req.Quota, req.BatchName, req.Count)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "redeem codes generated"})
}

// GetRedeemCodeUsage 获取兑换码使用记录。
// GET /api/merchant/redeem-codes/:code/usage
func (ctrl *MerchantController) GetRedeemCodeUsage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	code := c.Param("code")
	usage, err := ctrl.merchantService.GetRedeemCodeUsage(userID, code)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, usage)
}

// ========== 7. 问题工单 ==========

// GetTickets 获取工单列表。
// GET /api/merchant/tickets?page=1&page_size=20
func (ctrl *MerchantController) GetTickets(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	tickets, total, err := ctrl.merchantService.GetTickets(userID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"tickets": tickets,
		"total":   total,
		"page":    page,
		"page_size": pageSize,
	})
}

// CreateTicket 创建工单。
// POST /api/merchant/tickets
// Body: {"ticket_type": "technical", "title": "问题标题", "content": "问题详情", "priority": "normal"}
func (ctrl *MerchantController) CreateTicket(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		TicketType string `json:"ticket_type" binding:"required"`
		Title      string `json:"title" binding:"required"`
		Content    string `json:"content" binding:"required"`
		Priority   string `json:"priority"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Priority == "" {
		req.Priority = "normal"
	}

	err := ctrl.merchantService.CreateTicket(userID, req.TicketType, req.Title, req.Content, req.Priority)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "ticket created"})
}

// GetTicketByID 获取工单详情。
// GET /api/merchant/tickets/:id
func (ctrl *MerchantController) GetTicketByID(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid ticket id")
		return
	}

	ticket, err := ctrl.merchantService.GetTicketByID(userID, uint(ticketID))
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, ticket)
}

// ReplyTicket 回复工单。
// PUT /api/merchant/tickets/:id/reply
// Body: {"reply": "回复内容"}
func (ctrl *MerchantController) ReplyTicket(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid ticket id")
		return
	}

	var req struct {
		Reply string `json:"reply" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	err = ctrl.merchantService.ReplyTicket(userID, uint(ticketID), req.Reply)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "reply added"})
}

// ========== 8. 公告管理 ==========

// GetAnnouncements 获取平台公告列表。
// GET /api/merchant/announcements?page=1&page_size=20
func (ctrl *MerchantController) GetAnnouncements(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	announcements, total, err := ctrl.merchantService.GetAnnouncements(userID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"announcements": announcements,
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
	})
}

// GetAnnouncementByID 获取公告详情。
// GET /api/merchant/announcements/:id
func (ctrl *MerchantController) GetAnnouncementByID(c *gin.Context) {
	announcementID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid announcement id")
		return
	}

	announcement, err := ctrl.merchantService.GetAnnouncementByID(uint(announcementID))
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, announcement)
}

// MarkAnnouncementAsRead 标记公告为已读。
// PUT /api/merchant/announcements/:id/read
func (ctrl *MerchantController) MarkAnnouncementAsRead(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	announcementID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid announcement id")
		return
	}

	err = ctrl.merchantService.MarkAnnouncementAsRead(userID, uint(announcementID))
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "announcement marked as read"})
}
