// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// AdminWithdrawalController 处理 /api/admin/withdrawals/* 提现审批接口（批10）。
type AdminWithdrawalController struct {
	affService *services.AffService
}

// NewAdminWithdrawalController 创建 AdminWithdrawalController。
func NewAdminWithdrawalController(affService *services.AffService) *AdminWithdrawalController {
	return &AdminWithdrawalController{affService: affService}
}

// RegisterRoutes 注册到 adminProtected 路由组（前缀 /api/admin）。
func (ctrl *AdminWithdrawalController) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/withdrawals", ctrl.ListWithdrawals)
	group.PUT("/withdrawals/:id/approve", ctrl.ApproveWithdrawal)
	group.PUT("/withdrawals/:id/reject", ctrl.RejectWithdrawal)
}

// ListWithdrawals GET /api/admin/withdrawals - 提现申请列表
//
// 查询参数:
//   status  可选，过滤状态 (0=pending 1=approved 2=rejected)
//   limit   可选，分页大小（默认 20）
//   offset  可选，分页偏移（默认 0）
func (ctrl *AdminWithdrawalController) ListWithdrawals(c *gin.Context) {
	// 解析 status 过滤（可选）
	var statusFilter *int
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			statusFilter = &s
		}
	}

	// 解析分页
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	result, err := ctrl.affService.ListWithdrawals(statusFilter, limit, offset)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 响应契约: {"success": true, "data": [...], "total": N}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result["data"],
		"total":   result["total"],
	})
}

// approveRequest 审批通过请求体。
type approveRequest struct {
	AdminRemark string `json:"admin_remark"`
}

// ApproveWithdrawal PUT /api/admin/withdrawals/:id/approve - 审批通过
func (ctrl *AdminWithdrawalController) ApproveWithdrawal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid withdrawal id")
		return
	}

	var req approveRequest
	// admin_remark 可选，不强制 body
	_ = c.ShouldBindJSON(&req)

	if err := ctrl.affService.ApproveWithdrawal(id, req.AdminRemark); err != nil {
		switch {
		case errors.Is(err, services.ErrWithdrawalNotFound):
			utils.ErrorJSON(c, http.StatusNotFound, "提现申请不存在")
		case errors.Is(err, services.ErrWithdrawalNotPending):
			utils.ErrorJSON(c, http.StatusBadRequest, "该提现申请已处理，无法重复审批")
		case errors.Is(err, services.ErrInsufficientAffQuota):
			utils.ErrorJSON(c, http.StatusBadRequest, "用户返佣余额不足，无法审批")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "审批失败: "+err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "提现审批通过",
	})
}

// rejectRequest 审批拒绝请求体。
type rejectRequest struct {
	AdminRemark string `json:"admin_remark" binding:"required"`
}

// RejectWithdrawal PUT /api/admin/withdrawals/:id/reject - 审批拒绝
func (ctrl *AdminWithdrawalController) RejectWithdrawal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid withdrawal id")
		return
	}

	var req rejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "admin_remark 为必填项")
		return
	}

	if err := ctrl.affService.RejectWithdrawal(id, req.AdminRemark); err != nil {
		switch {
		case errors.Is(err, services.ErrWithdrawalNotFound):
			utils.ErrorJSON(c, http.StatusNotFound, "提现申请不存在")
		case errors.Is(err, services.ErrWithdrawalNotPending):
			utils.ErrorJSON(c, http.StatusBadRequest, "该提现申请已处理，无法重复审批")
		case errors.Is(err, services.ErrRemarkRequired):
			utils.ErrorJSON(c, http.StatusBadRequest, "admin_remark 为必填项")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "拒绝失败: "+err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已拒绝提现申请",
	})
}
