// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// SiteController 处理 /api/dist/site/* 公开接口
type SiteController struct {
	siteService *services.SiteService
}

// NewSiteController 创建 SiteController
func NewSiteController(siteService *services.SiteService) *SiteController {
	return &SiteController{siteService: siteService}
}

// RegisterRoutes 注册路由到 /api/dist/site group
func (ctrl *SiteController) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/info", ctrl.GetSiteInfo)
	group.GET("/models", ctrl.GetSiteModels)
	group.GET("/pricing", ctrl.GetSitePricing)
	// /packages 已迁移到 PackageController
	group.GET("/official-channels", ctrl.GetSiteOfficialChannels)
	group.GET("/official-channels/:id/availability", ctrl.GetOfficialChannelAvailability)
	group.GET("/key-groups", ctrl.GetSiteKeyGroups)
	group.GET("/key-groups/:id/pricing", ctrl.GetSiteKeyGroupPricing)
}

// GetSiteInfo GET /api/dist/site/info
func (ctrl *SiteController) GetSiteInfo(c *gin.Context) {
	info, err := ctrl.siteService.GetSiteInfo()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load site info")
		return
	}
	utils.SuccessJSON(c, info)
}

// GetSiteModels GET /api/dist/site/models
func (ctrl *SiteController) GetSiteModels(c *gin.Context) {
	models, err := ctrl.siteService.GetSiteModels()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load models")
		return
	}
	utils.SuccessJSON(c, models)
}

// GetSitePricing GET /api/dist/site/pricing
func (ctrl *SiteController) GetSitePricing(c *gin.Context) {
	pricing, err := ctrl.siteService.GetSitePricing()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load pricing")
		return
	}
	utils.SuccessJSON(c, pricing)
}

// GetSiteOfficialChannels GET /api/dist/site/official-channels
func (ctrl *SiteController) GetSiteOfficialChannels(c *gin.Context) {
	var channelID *int
	if v := c.Query("channel_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			channelID = &id
		}
	}
	channels, err := ctrl.siteService.GetSiteOfficialChannels(channelID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load official channels")
		return
	}
	utils.SuccessJSON(c, channels)
}

// GetOfficialChannelAvailability GET /api/dist/site/official-channels/:id/availability
func (ctrl *SiteController) GetOfficialChannelAvailability(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid channel id")
		return
	}

	period := c.DefaultQuery("period", "24h")

	var modelID *int
	if v := c.Query("model_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			modelID = &id
		}
	}

	availability, err := ctrl.siteService.GetOfficialChannelAvailability(channelID, modelID, period)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load availability")
		return
	}
	utils.SuccessJSON(c, availability)
}

// GetSiteKeyGroups GET /api/dist/site/key-groups
func (ctrl *SiteController) GetSiteKeyGroups(c *gin.Context) {
	groups, err := ctrl.siteService.GetSiteKeyGroups()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load key groups")
		return
	}
	utils.SuccessJSON(c, groups)
}

// GetSiteKeyGroupPricing GET /api/dist/site/key-groups/:id/pricing
func (ctrl *SiteController) GetSiteKeyGroupPricing(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid group id")
		return
	}

	pricing, err := ctrl.siteService.GetSiteKeyGroupPricing(groupID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusNotFound, "key group not found")
		return
	}
	utils.SuccessJSON(c, pricing)
}
