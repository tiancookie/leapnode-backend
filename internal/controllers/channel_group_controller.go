package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/middleware"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// ChannelGroupController 渠道分组控制器
type ChannelGroupController struct {
	channelGroupService *services.ChannelGroupService
}

// NewChannelGroupController 创建渠道分组控制器实例
func NewChannelGroupController(
	channelGroupService *services.ChannelGroupService,
) *ChannelGroupController {
	return &ChannelGroupController{
		channelGroupService: channelGroupService,
	}
}

// RegisterRoutes 注册路由（挂载在 /api/dist/admin 下，需要 AuthRequired）
func (ctrl *ChannelGroupController) RegisterRoutes(r *gin.RouterGroup) {
	// POST   /api/dist/admin/channel-groups         — 创建渠道分组
	// GET    /api/dist/admin/channel-groups         — 查询分组列表
	// PUT    /api/dist/admin/channel-groups/:id     — 编辑分组
	// DELETE /api/dist/admin/channel-groups/:id     — 删除分组
	// POST   /api/dist/admin/channel-groups/:id/channels — 分配渠道
	// GET    /api/dist/admin/channel-groups/:id/channels — 查询分组渠道

	r.POST("/channel-groups", ctrl.CreateChannelGroup)
	r.GET("/channel-groups", ctrl.GetChannelGroups)
	r.PUT("/channel-groups/:id", ctrl.UpdateChannelGroup)
	r.DELETE("/channel-groups/:id", ctrl.DeleteChannelGroup)
	r.POST("/channel-groups/:id/channels", ctrl.AssignChannels)
	r.GET("/channel-groups/:id/channels", ctrl.GetGroupChannels)
}

// CreateChannelGroup POST /api/dist/admin/channel-groups
//
// 创建渠道分组
func (ctrl *ChannelGroupController) CreateChannelGroup(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input services.ChannelGroupCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := ctrl.channelGroupService.CreateChannelGroup(userID, &input)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, group)
}

// GetChannelGroups GET /api/dist/admin/channel-groups
//
// 查询分站的渠道分组列表
func (ctrl *ChannelGroupController) GetChannelGroups(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groups, err := ctrl.channelGroupService.GetChannelGroups(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, groups)
}

// UpdateChannelGroup PUT /api/dist/admin/channel-groups/:id
//
// 更新渠道分组
func (ctrl *ChannelGroupController) UpdateChannelGroup(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid group id")
		return
	}

	var input services.ChannelGroupUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.channelGroupService.UpdateChannelGroup(userID, uint(groupID), &input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel group updated"})
}

// DeleteChannelGroup DELETE /api/dist/admin/channel-groups/:id
//
// 删除渠道分组
func (ctrl *ChannelGroupController) DeleteChannelGroup(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid group id")
		return
	}

	if err := ctrl.channelGroupService.DeleteChannelGroup(userID, uint(groupID)); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channel group deleted"})
}

// AssignChannels POST /api/dist/admin/channel-groups/:id/channels
//
// 为分组分配渠道
func (ctrl *ChannelGroupController) AssignChannels(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid group id")
		return
	}

	var input services.ChannelGroupRelationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ctrl.channelGroupService.AssignChannels(userID, uint(groupID), &input); err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"message": "channels assigned"})
}

// GetGroupChannels GET /api/dist/admin/channel-groups/:id/channels
//
// 查询分组关联的渠道列表
func (ctrl *ChannelGroupController) GetGroupChannels(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid group id")
		return
	}

	channelIDs, err := ctrl.channelGroupService.GetGroupChannels(userID, uint(groupID))
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, gin.H{"channel_ids": channelIDs})
}
