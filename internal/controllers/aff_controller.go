// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/middleware"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// AffController 返佣控制器
type AffController struct {
	affService *services.AffService
}

// NewAffController 创建返佣控制器实例
func NewAffController(affService *services.AffService) *AffController {
	return &AffController{affService: affService}
}

// GetAffCode GET /api/dist/aff - 获取邀请码
func (ctrl *AffController) GetAffCode(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	data, err := ctrl.affService.GetAffCode(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// GetAffEarnings GET /api/dist/aff_earnings - 查询返佣收益
func (ctrl *AffController) GetAffEarnings(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	data, err := ctrl.affService.GetAffEarnings(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// TransferAffQuota POST /api/dist/aff_transfer - 收益转余额
func (ctrl *AffController) TransferAffQuota(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid amount")
		return
	}

	data, err := ctrl.affService.TransferAffQuota(userID, req.Amount)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// RequestWithdraw POST /api/dist/aff_withdraw - 提现申请
func (ctrl *AffController) RequestWithdraw(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Amount      float64 `json:"amount" binding:"required,gt=0"`
		Method      string  `json:"method" binding:"required"`
		AccountInfo string  `json:"account_info" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	data, err := ctrl.affService.RequestWithdraw(userID, req.Amount, req.Method, req.AccountInfo)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// GetAffPayouts GET /api/dist/aff_payouts - 查询提现记录（前端契约要求）
func (ctrl *AffController) GetAffPayouts(c *gin.Context) {
	_, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// TODO: 实现提现记录查询（当前返回空列表）
	utils.SuccessJSON(c, gin.H{
		"payouts": []interface{}{},
		"total":   0,
	})
}

// GetInvitees GET /api/dist/aff/invitees - 邀请列表（注意：前端是 /api/dist/aff 但老大要求改成独立端点）
// 根据前端 api.js，实际应该是 GET /api/dist/aff?type=invitees 或独立路由
func (ctrl *AffController) GetInvitees(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	data, err := ctrl.affService.GetInvitees(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// ApplyKOL POST /api/dist/kol_apply - 商家/分站申请
func (ctrl *AffController) ApplyKOL(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Type        string `json:"type" binding:"required"`        // merchant | distributor
		Reason      string `json:"reason" binding:"required"`
		ContactInfo string `json:"contact_info" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request")
		return
	}

	data, err := ctrl.affService.ApplyKOL(userID, req.Type, req.Reason, req.ContactInfo)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// GetKOLStatus GET /api/dist/kol_status - 查询申请状态
func (ctrl *AffController) GetKOLStatus(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	data, err := ctrl.affService.GetKOLStatus(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// GetAffStats GET /api/dist/aff/stats - 用户/分站自己的返佣统计（批10）
func (ctrl *AffController) GetAffStats(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	data, err := ctrl.affService.GetAffStats(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}

// GetAffOverview GET /api/admin/aff/overview - 平台级返佣大盘（批10，admin）
func (ctrl *AffController) GetAffOverview(c *gin.Context) {
	data, err := ctrl.affService.GetAffOverview()
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessJSON(c, data)
}
