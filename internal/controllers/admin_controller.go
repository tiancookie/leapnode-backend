// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// AdminController 处理 /api/admin/* 总站管理员后台接口。
type AdminController struct {
	adminService *services.AdminService
}

// NewAdminController 创建 AdminController。
func NewAdminController(adminService *services.AdminService) *AdminController {
	return &AdminController{adminService: adminService}
}

// RegisterRoutes 注册管理员后台路由。
// 调用方需已对 group 应用 middleware.AuthRequired + middleware.AdminAuth。
func (ctrl *AdminController) RegisterRoutes(group *gin.RouterGroup) {
	// 1. 系统概览（Dashboard）
	group.GET("/dashboard/stats", ctrl.GetDashboardStats)
	group.GET("/dashboard/charts", ctrl.GetDashboardCharts)

	// 2. 用户管理
	group.GET("/users", ctrl.GetUserList)
	group.GET("/users/:id", ctrl.GetUserByID)
	group.PUT("/users/:id", ctrl.UpdateUser)
	group.DELETE("/users/:id", ctrl.DeleteUser)
	group.POST("/users/:id/quota", ctrl.AdjustUserQuota)

	// 3. 商家审批
	group.GET("/merchants/pending", ctrl.GetPendingMerchants)
	group.GET("/merchants/:id", ctrl.GetMerchantApplication)
	group.POST("/merchants/:id/approve", ctrl.ApproveMerchant)
	group.POST("/merchants/:id/reject", ctrl.RejectMerchant)

	// 4. 分站审批
	group.GET("/distributors/pending", ctrl.GetPendingDistributors)
	group.GET("/distributors/:id", ctrl.GetDistributorApplication)
	group.POST("/distributors/:id/approve", ctrl.ApproveDistributor)
	group.POST("/distributors/:id/reject", ctrl.RejectDistributor)

	// 5. 渠道管理（总站官方渠道）
	group.GET("/channels", ctrl.GetOfficialChannels)
	group.POST("/channels", ctrl.CreateOfficialChannel)
	group.PUT("/channels/:id", ctrl.UpdateChannel)
	group.DELETE("/channels/:id", ctrl.DeleteChannel)

	// 6. 套餐管理（总站套餐）
	group.GET("/packages", ctrl.GetOfficialPackages)
	group.POST("/packages", ctrl.CreatePackage)
	group.PUT("/packages/:id", ctrl.UpdatePackage)
	group.DELETE("/packages/:id", ctrl.DeletePackage)

	// 7. 兑换码管理
	group.GET("/redeem-codes", ctrl.GetRedeemCodes)
	group.POST("/redeem-codes/generate", ctrl.GenerateRedeemCodes)
	group.DELETE("/redeem-codes/:id", ctrl.InvalidateRedeemCode)

	// 8. 返佣管理
	group.GET("/referrals", ctrl.GetReferrals)
	group.GET("/referrals/stats", ctrl.GetReferralStats)
	group.POST("/referrals/:id/settle", ctrl.SettleReferral)

	// 9. 系统配置
	group.GET("/config", ctrl.GetSystemConfig)
	group.PUT("/config", ctrl.UpdateSystemConfig)
}

// ========== 1. 系统概览（Dashboard）==========

// GetDashboardStats GET /api/admin/dashboard/stats - 总站统计数据
func (ctrl *AdminController) GetDashboardStats(c *gin.Context) {
	stats, err := ctrl.adminService.GetDashboardStats()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, stats)
}

// GetDashboardCharts GET /api/admin/dashboard/charts - 图表数据
func (ctrl *AdminController) GetDashboardCharts(c *gin.Context) {
	charts, err := ctrl.adminService.GetDashboardCharts()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, charts)
}

// ========== 2. 用户管理 ==========

// GetUserList GET /api/admin/users - 用户列表（分页 + 筛选）
func (ctrl *AdminController) GetUserList(c *gin.Context) {
	var query services.UserListQuery

	// 设置默认分页参数
	query.Page = 1
	query.PageSize = 20

	if err := c.ShouldBindQuery(&query); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid query parameters")
		return
	}

	result, err := ctrl.adminService.GetUserList(query)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, result)
}

// GetUserByID GET /api/admin/users/:id - 用户详情
func (ctrl *AdminController) GetUserByID(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := ctrl.adminService.GetUserByID(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusNotFound, "user not found")
		return
	}

	utils.SuccessJSON(c, user)
}

// UpdateUser PUT /api/admin/users/:id - 更新用户
func (ctrl *AdminController) UpdateUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var input services.UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.UpdateUser(userID, input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "user updated"})
}

// DeleteUser DELETE /api/admin/users/:id - 删除用户
func (ctrl *AdminController) DeleteUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := ctrl.adminService.DeleteUser(userID); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "user deleted"})
}

// AdjustUserQuota POST /api/admin/users/:id/quota - 手动调整余额
func (ctrl *AdminController) AdjustUserQuota(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var input services.AdjustUserQuotaInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.AdjustUserQuota(userID, input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "quota adjusted"})
}

// ========== 3. 商家审批 ==========

// GetPendingMerchants GET /api/admin/merchants/pending - 待审批商家列表
func (ctrl *AdminController) GetPendingMerchants(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := ctrl.adminService.GetPendingMerchants(page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, result)
}

// GetMerchantApplication GET /api/admin/merchants/:id - 商家详情
func (ctrl *AdminController) GetMerchantApplication(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid application id")
		return
	}

	app, err := ctrl.adminService.GetMerchantApplication(appID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusNotFound, "application not found")
		return
	}

	utils.SuccessJSON(c, app)
}

// ApproveMerchant POST /api/admin/merchants/:id/approve - 批准商家
func (ctrl *AdminController) ApproveMerchant(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid application id")
		return
	}

	var input struct {
		AdminRemark string `json:"admin_remark"`
	}
	c.ShouldBindJSON(&input)

	if err := ctrl.adminService.ApproveMerchant(appID, input.AdminRemark); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "merchant approved"})
}

// RejectMerchant POST /api/admin/merchants/:id/reject - 拒绝商家
func (ctrl *AdminController) RejectMerchant(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid application id")
		return
	}

	var input struct {
		AdminRemark string `json:"admin_remark"`
	}
	c.ShouldBindJSON(&input)

	if err := ctrl.adminService.RejectMerchant(appID, input.AdminRemark); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "merchant rejected"})
}

// ========== 4. 分站审批 ==========

// GetPendingDistributors GET /api/admin/distributors/pending - 待审批分站列表
func (ctrl *AdminController) GetPendingDistributors(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := ctrl.adminService.GetPendingDistributors(page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, result)
}

// GetDistributorApplication GET /api/admin/distributors/:id - 分站详情
func (ctrl *AdminController) GetDistributorApplication(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid application id")
		return
	}

	app, err := ctrl.adminService.GetDistributorApplication(appID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusNotFound, "application not found")
		return
	}

	utils.SuccessJSON(c, app)
}

// ApproveDistributor POST /api/admin/distributors/:id/approve - 批准分站
func (ctrl *AdminController) ApproveDistributor(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid application id")
		return
	}

	var input struct {
		AdminRemark string `json:"admin_remark"`
	}
	c.ShouldBindJSON(&input)

	if err := ctrl.adminService.ApproveDistributor(appID, input.AdminRemark); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "distributor approved"})
}

// RejectDistributor POST /api/admin/distributors/:id/reject - 拒绝分站
func (ctrl *AdminController) RejectDistributor(c *gin.Context) {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid application id")
		return
	}

	var input struct {
		AdminRemark string `json:"admin_remark"`
	}
	c.ShouldBindJSON(&input)

	if err := ctrl.adminService.RejectDistributor(appID, input.AdminRemark); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "distributor rejected"})
}

// ========== 5. 渠道管理（总站官方渠道）==========

// GetOfficialChannels GET /api/admin/channels - 总站官方渠道列表
func (ctrl *AdminController) GetOfficialChannels(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := ctrl.adminService.GetOfficialChannels(page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, result)
}

// CreateOfficialChannel POST /api/admin/channels - 添加官方渠道
func (ctrl *AdminController) CreateOfficialChannel(c *gin.Context) {
	var input services.CreateChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.CreateOfficialChannel(input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel created"})
}

// UpdateChannel PUT /api/admin/channels/:id - 更新渠道
func (ctrl *AdminController) UpdateChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid channel id")
		return
	}

	var input services.UpdateChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.UpdateChannel(channelID, input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel updated"})
}

// DeleteChannel DELETE /api/admin/channels/:id - 删除渠道
func (ctrl *AdminController) DeleteChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid channel id")
		return
	}

	if err := ctrl.adminService.DeleteChannel(channelID); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel deleted"})
}

// ========== 6. 套餐管理（总站套餐）==========

// GetOfficialPackages GET /api/admin/packages - 套餐列表
func (ctrl *AdminController) GetOfficialPackages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := ctrl.adminService.GetOfficialPackages(page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, result)
}

// CreatePackage POST /api/admin/packages - 创建套餐
func (ctrl *AdminController) CreatePackage(c *gin.Context) {
	var input services.CreatePackageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.CreatePackage(input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "package created"})
}

// UpdatePackage PUT /api/admin/packages/:id - 更新套餐
func (ctrl *AdminController) UpdatePackage(c *gin.Context) {
	packageID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid package id")
		return
	}

	var input services.UpdatePackageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.UpdatePackage(packageID, input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "package updated"})
}

// DeletePackage DELETE /api/admin/packages/:id - 删除套餐
func (ctrl *AdminController) DeletePackage(c *gin.Context) {
	packageID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid package id")
		return
	}

	if err := ctrl.adminService.DeletePackage(packageID); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "package deleted"})
}

// ========== 7. 兑换码管理 ==========

// GetRedeemCodes GET /api/admin/redeem-codes - 兑换码列表
func (ctrl *AdminController) GetRedeemCodes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := ctrl.adminService.GetRedeemCodes(page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, result)
}

// GenerateRedeemCodes POST /api/admin/redeem-codes/generate - 批量生成兑换码
func (ctrl *AdminController) GenerateRedeemCodes(c *gin.Context) {
	var input services.GenerateRedeemCodesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.GenerateRedeemCodes(input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "redeem codes generated"})
}

// InvalidateRedeemCode DELETE /api/admin/redeem-codes/:id - 作废兑换码
func (ctrl *AdminController) InvalidateRedeemCode(c *gin.Context) {
	codeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid code id")
		return
	}

	if err := ctrl.adminService.InvalidateRedeemCode(codeID); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "redeem code invalidated"})
}

// ========== 8. 返佣管理 ==========

// GetReferrals GET /api/admin/referrals - 返佣记录列表
func (ctrl *AdminController) GetReferrals(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := ctrl.adminService.GetReferrals(page, pageSize)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, result)
}

// GetReferralStats GET /api/admin/referrals/stats - 返佣统计
func (ctrl *AdminController) GetReferralStats(c *gin.Context) {
	stats, err := ctrl.adminService.GetReferralStats()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, stats)
}

// SettleReferral POST /api/admin/referrals/:id/settle - 手动结算返佣
func (ctrl *AdminController) SettleReferral(c *gin.Context) {
	referralID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid referral id")
		return
	}

	if err := ctrl.adminService.SettleReferral(referralID); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "referral settled"})
}

// ========== 9. 系统配置 ==========

// GetSystemConfig GET /api/admin/config - 获取系统配置
func (ctrl *AdminController) GetSystemConfig(c *gin.Context) {
	config, err := ctrl.adminService.GetSystemConfig()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, config)
}

// UpdateSystemConfig PUT /api/admin/config - 更新系统配置
func (ctrl *AdminController) UpdateSystemConfig(c *gin.Context) {
	var input services.UpdateSystemConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.adminService.UpdateSystemConfig(input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "system config updated"})
}
