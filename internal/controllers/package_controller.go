// Package controllers 提供套餐管理路由处理。
package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/middleware"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// PackageController 套餐控制器
type PackageController struct {
	packageService *services.PackageService
}

// NewPackageController 创建套餐控制器实例
func NewPackageController(packageService *services.PackageService) *PackageController {
	return &PackageController{
		packageService: packageService,
	}
}

// RegisterPublicRoutes 注册公开路由
func (ctrl *PackageController) RegisterPublicRoutes(r *gin.RouterGroup) {
	// GET /api/dist/site/packages — 公开查询套餐列表
	r.GET("/packages", ctrl.GetPublicPackages)
}

// RegisterProtectedRoutes 注册受保护路由（需要认证）
func (ctrl *PackageController) RegisterProtectedRoutes(r *gin.RouterGroup) {
	// POST /api/dist/package/:id/subscribe — 订阅套餐
	r.POST("/package/:id/subscribe", ctrl.SubscribePackage)
	
	// GET /api/dist/subscription/active — 查询活跃订阅
	r.GET("/subscription/active", ctrl.GetActiveSubscriptions)
}

// RegisterAdminRoutes 注册管理员路由（分站管理套餐）
func (ctrl *PackageController) RegisterAdminRoutes(r *gin.RouterGroup) {
	// POST /api/dist/admin/packages — 创建套餐
	r.POST("/packages", ctrl.CreatePackage)
	
	// GET /api/dist/admin/packages — 查询本分站套餐列表
	r.GET("/packages", ctrl.GetDistributorPackages)
	
	// PUT /api/dist/admin/packages/:id — 编辑套餐
	r.PUT("/packages/:id", ctrl.UpdatePackage)
	
	// DELETE /api/dist/admin/packages/:id — 删除套餐
	r.DELETE("/packages/:id", ctrl.DeletePackage)
}

// CreatePackage 创建套餐（分站站长）
// POST /api/dist/admin/packages
func (ctrl *PackageController) CreatePackage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input services.PackageCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid input: "+err.Error())
		return
	}

	pkg, err := ctrl.packageService.CreatePackage(userID, &input)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, pkg)
}

// UpdatePackage 更新套餐
// PUT /api/dist/admin/packages/:id
func (ctrl *PackageController) UpdatePackage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid package id")
		return
	}

	var input services.PackageUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid input: "+err.Error())
		return
	}

	pkg, err := ctrl.packageService.UpdatePackage(id, userID, &input)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, pkg)
}

// DeletePackage 删除套餐
// DELETE /api/dist/admin/packages/:id
func (ctrl *PackageController) DeletePackage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid package id")
		return
	}

	if err := ctrl.packageService.DeletePackage(id, userID); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "package deleted"})
}

// GetDistributorPackages 查询本分站的套餐列表
// GET /api/dist/admin/packages
func (ctrl *PackageController) GetDistributorPackages(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	packages, err := ctrl.packageService.GetDistributorPackages(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, packages)
}

// GetPublicPackages 查询公开套餐列表
// GET /api/dist/site/packages
func (ctrl *PackageController) GetPublicPackages(c *gin.Context) {
	packages, err := ctrl.packageService.GetPublicPackages()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, packages)
}

// SubscribePackage 订阅套餐
// POST /api/dist/package/:id/subscribe
func (ctrl *PackageController) SubscribePackage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := c.Param("id")
	packageID, err := strconv.Atoi(idStr)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid package id")
		return
	}

	result, err := ctrl.packageService.SubscribePackage(userID, packageID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订阅成功",
		"data":    result,
	})
}

// GetActiveSubscriptions 查询活跃订阅
// GET /api/dist/subscription/active
func (ctrl *PackageController) GetActiveSubscriptions(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	subscriptions, err := ctrl.packageService.GetActiveSubscriptions(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    subscriptions,
		"total":   len(subscriptions),
	})
}
