// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/middleware"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// TokenController 处理 /api/dist/token/* 令牌管理接口 (全部需 AuthRequired)。
type TokenController struct {
	tokenService *services.TokenService
	siteService  *services.SiteService
}

// NewTokenController 创建 TokenController。
// siteService 用于 GetTokenSupportedModels 在未限制模型时返回站点全量模型。
func NewTokenController(tokenService *services.TokenService, siteService *services.SiteService) *TokenController {
	return &TokenController{tokenService: tokenService, siteService: siteService}
}

// RegisterProtectedRoutes 注册需要认证的令牌路由。
// 调用方需已对 group 应用 middleware.AuthRequired, group 前缀为 /api/dist/token。
func (ctrl *TokenController) RegisterProtectedRoutes(group *gin.RouterGroup) {
	group.GET("/list", ctrl.List)
	group.POST("/create", ctrl.Create)
	group.PUT("/:id", ctrl.Update)
	group.DELETE("/:id", ctrl.Delete)
	group.GET("/:id/models", ctrl.SupportedModels)
}

// List GET /api/dist/token/list (受保护)
//
// 前端: Tokens.jsx load() -> api.getTokens(); 消费 res.data.data 为 token 数组。
func (ctrl *TokenController) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokens, err := ctrl.tokenService.ListTokens(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load tokens")
		return
	}
	utils.SuccessJSON(c, tokens)
}

// Create POST /api/dist/token/create (受保护)
//
// 前端: Tokens.jsx handleCreate -> api.createToken(payload);
// 成功后读取 res.data.data.key 展示新 key (CreatedKeyResultModal)。
func (ctrl *TokenController) Create(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var in services.CreateTokenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}
	token, err := ctrl.tokenService.CreateToken(userID, in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTokenNameReq):
			utils.ErrorJSON(c, http.StatusBadRequest, "token name required")
		case errors.Is(err, services.ErrKeyGenFailed):
			utils.ErrorJSON(c, http.StatusInternalServerError, "failed to generate token key")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "failed to create token")
		}
		return
	}
	utils.SuccessJSON(c, token)
}

// Update PUT /api/dist/token/:id (受保护)
//
// 前端: handleToggle (只传 status) / handleEditSave (传编辑表单)。
// 归属校验: 不属于当前用户返回 404。
func (ctrl *TokenController) Update(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid token id")
		return
	}
	var in services.UpdateTokenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}
	token, err := ctrl.tokenService.UpdateToken(userID, tokenID, in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTokenNotFound):
			utils.ErrorJSON(c, http.StatusNotFound, "token not found")
		case errors.Is(err, services.ErrTokenNameReq):
			utils.ErrorJSON(c, http.StatusBadRequest, "token name required")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "failed to update token")
		}
		return
	}
	utils.SuccessJSON(c, token)
}

// Delete DELETE /api/dist/token/:id (受保护)
//
// 前端: handleDelete -> api.deleteToken(id) (软删除)。
// 归属校验: 不属于当前用户返回 404。
func (ctrl *TokenController) Delete(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid token id")
		return
	}
	if err := ctrl.tokenService.DeleteToken(userID, tokenID); err != nil {
		if errors.Is(err, services.ErrTokenNotFound) {
			utils.ErrorJSON(c, http.StatusNotFound, "token not found")
			return
		}
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to delete token")
		return
	}
	utils.SuccessJSON(c, gin.H{})
}

// SupportedModels GET /api/dist/token/:id/models (受保护)
//
// 前端: handleToggleSupportedModels -> api.getTokenSupportedModels(id);
// 消费 data.models/count/provider_names/restricted_by_providers/restricted_by_models。
func (ctrl *TokenController) SupportedModels(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid token id")
		return
	}
	resp, err := ctrl.tokenService.GetTokenSupportedModels(userID, tokenID, ctrl.siteService)
	if err != nil {
		if errors.Is(err, services.ErrTokenNotFound) {
			utils.ErrorJSON(c, http.StatusNotFound, "token not found")
			return
		}
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load supported models")
		return
	}
	utils.SuccessJSON(c, resp)
}
