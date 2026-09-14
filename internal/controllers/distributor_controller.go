// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// DistributorController 处理 /api/distributor/* 分站管理接口。
type DistributorController struct {
	distributorService *services.DistributorService
}

// NewDistributorController 创建 DistributorController。
func NewDistributorController(distributorService *services.DistributorService) *DistributorController {
	return &DistributorController{distributorService: distributorService}
}

// getDistributorID 从上下文获取分站ID。
func (ctrl *DistributorController) getDistributorID(c *gin.Context) (int, bool) {
	userVal, exists := c.Get("auth_user")
	if !exists {
		return 0, false
	}
	user, ok := userVal.(*models.User)
	if !ok {
		return 0, false
	}
	// 分站ID = user.ID (分站站长的用户ID)
	return user.ID, true
}

// RegisterRoutes 注册分站管理路由。
func (ctrl *DistributorController) RegisterRoutes(group *gin.RouterGroup) {
	// 1. 经营概览（Dashboard）
	group.GET("/dashboard/stats", ctrl.GetDashboardStats)
	group.GET("/dashboard/revenue-trend", ctrl.GetRevenueTrend)
	group.GET("/dashboard/top-models", ctrl.GetTopModels)
	group.GET("/dashboard/recent-orders", ctrl.GetRecentOrders)

	// 2. 商品配置 - 模型管理
	group.GET("/models", ctrl.GetModels)
	group.POST("/models/enable", ctrl.EnableModel)
	group.PUT("/models/:id/pricing", ctrl.UpdateModelPricing)
	group.DELETE("/models/:id", ctrl.DisableModel)
	group.GET("/models/:id/stats", ctrl.GetModelStats)

	// 3. 商品配置 - 官方渠道
	group.GET("/official-channels", ctrl.GetOfficialChannels)
	group.POST("/official-channels/:id/enable", ctrl.EnableOfficialChannel)
	group.PUT("/official-channels/:id/pricing", ctrl.UpdateChannelPricing)
	group.DELETE("/official-channels/:id/disable", ctrl.DisableOfficialChannel)

	// 4. 商品配置 - 订阅共享
	group.GET("/shared-subscriptions", ctrl.GetSharedSubscriptions)
	group.POST("/shared-subscriptions/:id/enable", ctrl.EnableSharedSubscription)
	group.PUT("/shared-subscriptions/:id/pricing", ctrl.UpdateSubscriptionPricing)
	group.DELETE("/shared-subscriptions/:id/disable", ctrl.DisableSharedSubscription)

	// 5. 商品配置 - 套餐管理
	group.GET("/packages", ctrl.GetPackages)
	group.POST("/packages", ctrl.CreatePackage)
	group.PUT("/packages/:id", ctrl.UpdatePackage)
	group.DELETE("/packages/:id", ctrl.DeletePackage)
	group.GET("/packages/:id/sales", ctrl.GetPackageSales)

	// 6. 商品配置 - 密钥分组
	group.GET("/key-groups", ctrl.GetKeyGroups)
	group.POST("/key-groups", ctrl.CreateKeyGroup)
	group.PUT("/key-groups/:id", ctrl.UpdateKeyGroup)
	group.DELETE("/key-groups/:id", ctrl.DeleteKeyGroup)

	// 7. 商品配置 - 兑换码
	group.GET("/redeem-codes", ctrl.GetRedeemCodes)
	group.POST("/redeem-codes/generate", ctrl.GenerateRedeemCodes)
	group.GET("/redeem-codes/:id/usage", ctrl.GetRedeemCodeUsage)
	group.DELETE("/redeem-codes/:id", ctrl.InvalidateRedeemCode)

	// 8. 客户运营 - 用户管理
	group.GET("/users", ctrl.GetUsers)
	group.GET("/users/:id", ctrl.GetUserDetail)
	group.PUT("/users/:id/quota", ctrl.AdjustUserQuota)
	group.GET("/users/:id/orders", ctrl.GetUserOrders)
	group.GET("/users/:id/tokens", ctrl.GetUserTokens)

	// 9. 客户运营 - 令牌管理
	group.GET("/tokens", ctrl.GetTokens)
	group.PUT("/tokens/:id/status", ctrl.UpdateTokenStatus)
	group.GET("/tokens/:id/usage", ctrl.GetTokenUsage)

	// 10. 财务管理
	group.GET("/finance/summary", ctrl.GetFinanceSummary)
	group.GET("/finance/revenue-records", ctrl.GetRevenueRecords)
	group.GET("/finance/withdrawal-records", ctrl.GetWithdrawalRecords)
	group.POST("/finance/withdraw", ctrl.RequestWithdrawal)
	group.GET("/finance/recharge-records", ctrl.GetRechargeRecords)

	// 11. 站点与开发 - 站点设置
	group.GET("/site/settings", ctrl.GetSiteSettings)
	group.PUT("/site/settings", ctrl.UpdateSiteSettings)
	group.GET("/site/payment-methods", ctrl.GetPaymentMethods)
	group.PUT("/site/payment-methods", ctrl.UpdatePaymentMethods)

	// 12. 站点与开发 - API密钥
	group.GET("/site/api-keys", ctrl.GetAPIKeys)
	group.POST("/site/api-keys/generate", ctrl.GenerateAPIKey)
	group.DELETE("/site/api-keys/:id", ctrl.DeleteAPIKey)
}

// ========== 1. 经营概览（Dashboard）==========

// GetDashboardStats GET /api/distributor/dashboard/stats - 分站统计数据
func (ctrl *DistributorController) GetDashboardStats(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	stats, err := ctrl.distributorService.GetDistributorStats(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, stats)
}

// GetRevenueTrend GET /api/distributor/dashboard/revenue-trend - 收益趋势图
func (ctrl *DistributorController) GetRevenueTrend(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	trend, err := ctrl.distributorService.GetRevenueTrend(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"trend": trend})
}

// GetTopModels GET /api/distributor/dashboard/top-models - 热门模型 TOP 10
func (ctrl *DistributorController) GetTopModels(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	models, err := ctrl.distributorService.GetTopModels(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"models": models})
}

// GetRecentOrders GET /api/distributor/dashboard/recent-orders - 最近订单
func (ctrl *DistributorController) GetRecentOrders(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	orders, err := ctrl.distributorService.GetRecentOrders(distributorID, limit)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"orders": orders})
}

// ========== 2. 商品配置 - 模型管理 ==========

// GetModels GET /api/distributor/models - 模型列表
func (ctrl *DistributorController) GetModels(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	models, total, err := ctrl.distributorService.GetModels(distributorID, page, pageSize)
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

// EnableModel POST /api/distributor/models/enable - 启用模型
func (ctrl *DistributorController) EnableModel(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req services.EnableModelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.EnableModel(distributorID, req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "model enabled"})
}

// UpdateModelPricing PUT /api/distributor/models/:id/pricing - 更新模型定价
func (ctrl *DistributorController) UpdateModelPricing(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	modelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid model id")
		return
	}

	var req struct {
		MarkupType  string  `json:"markup_type"`
		MarkupValue float64 `json:"markup_value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.UpdateModelPricing(distributorID, modelID, req.MarkupType, req.MarkupValue); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "pricing updated"})
}

// DisableModel DELETE /api/distributor/models/:id - 下架模型
func (ctrl *DistributorController) DisableModel(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	modelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid model id")
		return
	}

	if err := ctrl.distributorService.DisableModel(distributorID, modelID); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "model disabled"})
}

// GetModelStats GET /api/distributor/models/:id/stats - 模型销售统计
func (ctrl *DistributorController) GetModelStats(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	modelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid model id")
		return
	}

	stats, err := ctrl.distributorService.GetModelStats(distributorID, modelID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, stats)
}

// ========== 3-12. 其他模块（简化实现，结构相同）==========

// GetOfficialChannels GET /api/distributor/official-channels
func (ctrl *DistributorController) GetOfficialChannels(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	channels, err := ctrl.distributorService.GetOfficialChannels(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"channels": channels})
}

func (ctrl *DistributorController) EnableOfficialChannel(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	channelID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Markup float64 `json:"markup"`
	}
	c.ShouldBindJSON(&req)

	if err := ctrl.distributorService.EnableOfficialChannel(distributorID, channelID, req.Markup); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel enabled"})
}

func (ctrl *DistributorController) UpdateChannelPricing(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	channelID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Markup float64 `json:"markup"`
	}
	c.ShouldBindJSON(&req)

	if err := ctrl.distributorService.UpdateChannelPricing(distributorID, channelID, req.Markup); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "pricing updated"})
}

func (ctrl *DistributorController) DisableOfficialChannel(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	channelID, _ := strconv.Atoi(c.Param("id"))
	if err := ctrl.distributorService.DisableOfficialChannel(distributorID, channelID); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel disabled"})
}

// 订阅共享
func (ctrl *DistributorController) GetSharedSubscriptions(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	subs, err := ctrl.distributorService.GetSharedSubscriptions(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"subscriptions": subs})
}

func (ctrl *DistributorController) EnableSharedSubscription(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	subID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Multiplier float64 `json:"multiplier"`
	}
	c.ShouldBindJSON(&req)

	if err := ctrl.distributorService.EnableSharedSubscription(distributorID, subID, req.Multiplier); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "subscription enabled"})
}

func (ctrl *DistributorController) UpdateSubscriptionPricing(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	subID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Multiplier float64 `json:"multiplier"`
	}
	c.ShouldBindJSON(&req)

	if err := ctrl.distributorService.UpdateSubscriptionPricing(distributorID, subID, req.Multiplier); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "pricing updated"})
}

func (ctrl *DistributorController) DisableSharedSubscription(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	subID, _ := strconv.Atoi(c.Param("id"))
	if err := ctrl.distributorService.DisableSharedSubscription(distributorID, subID); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "subscription disabled"})
}

// 套餐管理
func (ctrl *DistributorController) GetPackages(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	packages, err := ctrl.distributorService.GetPackages(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"packages": packages})
}

func (ctrl *DistributorController) CreatePackage(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req services.CreateDistributorPackageInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.CreatePackage(distributorID, req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "package created"})
}

func (ctrl *DistributorController) UpdatePackage(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	packageID, _ := strconv.Atoi(c.Param("id"))
	var req services.CreateDistributorPackageInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.UpdatePackage(distributorID, packageID, req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "package updated"})
}

func (ctrl *DistributorController) DeletePackage(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	packageID, _ := strconv.Atoi(c.Param("id"))
	if err := ctrl.distributorService.DeletePackage(distributorID, packageID); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "package deleted"})
}

func (ctrl *DistributorController) GetPackageSales(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	packageID, _ := strconv.Atoi(c.Param("id"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	sales, total, err := ctrl.distributorService.GetPackageSales(distributorID, packageID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"sales": sales,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

// 密钥分组
func (ctrl *DistributorController) GetKeyGroups(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	groups, err := ctrl.distributorService.GetKeyGroups(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"groups": groups})
}

func (ctrl *DistributorController) CreateKeyGroup(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req services.CreateKeyGroupInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.CreateKeyGroup(distributorID, req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "key group created"})
}

func (ctrl *DistributorController) UpdateKeyGroup(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	groupID, _ := strconv.Atoi(c.Param("id"))
	var req services.CreateKeyGroupInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.UpdateKeyGroup(distributorID, groupID, req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "key group updated"})
}

func (ctrl *DistributorController) DeleteKeyGroup(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	groupID, _ := strconv.Atoi(c.Param("id"))
	if err := ctrl.distributorService.DeleteKeyGroup(distributorID, groupID); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "key group deleted"})
}

// 兑换码
func (ctrl *DistributorController) GetRedeemCodes(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	codes, total, err := ctrl.distributorService.GetDistributorRedeemCodes(distributorID, page, pageSize)
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

func (ctrl *DistributorController) GenerateRedeemCodes(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Quota       int64  `json:"quota"`
		Count       int    `json:"count"`
		ExpiredTime int64  `json:"expired_time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	codes, err := ctrl.distributorService.GenerateDistributorRedeemCodes(distributorID, req.Name, req.Quota, req.Count, req.ExpiredTime)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"codes": codes})
}

func (ctrl *DistributorController) GetRedeemCodeUsage(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	codeID, _ := strconv.Atoi(c.Param("id"))
	usage, err := ctrl.distributorService.GetRedeemCodeUsage(distributorID, codeID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"usage": usage})
}

func (ctrl *DistributorController) InvalidateRedeemCode(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	codeID, _ := strconv.Atoi(c.Param("id"))
	if err := ctrl.distributorService.InvalidateDistributorRedeemCode(distributorID, codeID); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "redeem code invalidated"})
}

// 用户管理
func (ctrl *DistributorController) GetUsers(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := ctrl.distributorService.GetDistributorUsers(distributorID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"users": users,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

func (ctrl *DistributorController) GetUserDetail(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	userID, _ := strconv.Atoi(c.Param("id"))
	detail, err := ctrl.distributorService.GetDistributorUserDetail(distributorID, userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, detail)
}

func (ctrl *DistributorController) AdjustUserQuota(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	userID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Amount int64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.AdjustDistributorUserQuota(distributorID, userID, req.Amount); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "quota adjusted"})
}

func (ctrl *DistributorController) GetUserOrders(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	userID, _ := strconv.Atoi(c.Param("id"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	orders, total, err := ctrl.distributorService.GetUserOrders(distributorID, userID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"orders": orders,
		"total":  total,
		"page":   page,
		"page_size": pageSize,
	})
}

func (ctrl *DistributorController) GetUserTokens(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	userID, _ := strconv.Atoi(c.Param("id"))
	tokens, err := ctrl.distributorService.GetUserTokens(distributorID, userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"tokens": tokens})
}

// 令牌管理
func (ctrl *DistributorController) GetTokens(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	tokens, total, err := ctrl.distributorService.GetDistributorTokens(distributorID, page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"tokens": tokens,
		"total":  total,
		"page":   page,
		"page_size": pageSize,
	})
}

func (ctrl *DistributorController) UpdateTokenStatus(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	tokenID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Status int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.UpdateTokenStatus(distributorID, tokenID, req.Status); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "token status updated"})
}

func (ctrl *DistributorController) GetTokenUsage(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	tokenID, _ := strconv.Atoi(c.Param("id"))
	usage, err := ctrl.distributorService.GetTokenUsage(distributorID, tokenID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, usage)
}

// 财务管理
func (ctrl *DistributorController) GetFinanceSummary(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	summary, err := ctrl.distributorService.GetFinanceSummary(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, summary)
}

func (ctrl *DistributorController) GetRevenueRecords(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	records, total, err := ctrl.distributorService.GetRevenueRecords(distributorID, page, pageSize)
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

func (ctrl *DistributorController) GetWithdrawalRecords(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	records, total, err := ctrl.distributorService.GetWithdrawalRecords(distributorID, page, pageSize)
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

func (ctrl *DistributorController) RequestWithdrawal(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		Amount        float64 `json:"amount"`
		PaymentMethod string  `json:"payment_method"`
		AccountInfo   string  `json:"account_info"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	requestID, err := ctrl.distributorService.RequestWithdrawal(distributorID, req.Amount, req.PaymentMethod, req.AccountInfo)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{
		"request_id": requestID,
		"status":     "pending",
	})
}

func (ctrl *DistributorController) GetRechargeRecords(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	records, total, err := ctrl.distributorService.GetRechargeRecords(distributorID, page, pageSize)
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

// 站点设置
func (ctrl *DistributorController) GetSiteSettings(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	settings, err := ctrl.distributorService.GetSiteSettings(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, settings)
}

func (ctrl *DistributorController) UpdateSiteSettings(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req services.SiteSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.UpdateSiteSettings(distributorID, &req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "settings updated"})
}

func (ctrl *DistributorController) GetPaymentMethods(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	methods, err := ctrl.distributorService.GetPaymentMethods(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"payment_methods": methods})
}

func (ctrl *DistributorController) UpdatePaymentMethods(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req []services.PaymentMethodConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := ctrl.distributorService.UpdatePaymentMethods(distributorID, req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "payment methods updated"})
}

// API密钥
func (ctrl *DistributorController) GetAPIKeys(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	keys, err := ctrl.distributorService.GetAPIKeys(distributorID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"api_keys": keys})
}

func (ctrl *DistributorController) GenerateAPIKey(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		KeyName string `json:"key_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	key, err := ctrl.distributorService.GenerateAPIKey(distributorID, req.KeyName)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, key)
}

func (ctrl *DistributorController) DeleteAPIKey(c *gin.Context) {
	distributorID, ok := ctrl.getDistributorID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
		return
	}

	keyID, _ := strconv.Atoi(c.Param("id"))
	if err := ctrl.distributorService.DeleteAPIKey(distributorID, keyID); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "api key deleted"})
}
