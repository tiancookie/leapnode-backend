// Package controllers 实现 HTTP 处理器。
package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/middleware"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// TopupController 处理 /api/dist/topup/* 充值接口。
type TopupController struct {
	topupService *services.TopupService
}

// NewTopupController 创建 TopupController。
func NewTopupController(topupService *services.TopupService) *TopupController {
	return &TopupController{topupService: topupService}
}

// RegisterPublicRoutes 注册公开充值路由（不需认证）。
// group 前缀为 /api/dist/topup。
func (ctrl *TopupController) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.GET("/info", ctrl.GetTopupInfo)
}

// RegisterProtectedRoutes 注册需要认证的充值路由。
// 调用方需已对 group 应用 middleware.AuthRequired, group 前缀为 /api/dist/topup。
func (ctrl *TopupController) RegisterProtectedRoutes(group *gin.RouterGroup) {
	group.POST("/redeem", ctrl.RedeemCode)
	group.GET("/history", ctrl.GetTopupHistory)
}

// RedeemCode POST /api/dist/topup/redeem (受保护)
//
// 前端: Topup.jsx handleRedeem -> api.redeemCode(key) -> POST /api/dist/topup/redeem {key}
// 请求体: {key: string}
// 成功响应: {success:true, data:{quota:新增额度, message:"充值成功"}, message:""}
func (ctrl *TopupController) RedeemCode(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input services.RedeemCodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := ctrl.topupService.RedeemCode(userID, input.Key)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrRedeemCodeRequired):
			utils.ErrorJSON(c, http.StatusBadRequest, "充值码不能为空")
		case errors.Is(err, services.ErrRedeemCodeInvalid):
			utils.ErrorJSON(c, http.StatusBadRequest, "充值码不存在")
		case errors.Is(err, services.ErrRedeemCodeUsed):
			utils.ErrorJSON(c, http.StatusBadRequest, "充值码已被使用")
		case errors.Is(err, services.ErrRedeemCodeDisabled):
			utils.ErrorJSON(c, http.StatusBadRequest, "充值码已被禁用")
		case errors.Is(err, services.ErrRedeemCodeExpired):
			utils.ErrorJSON(c, http.StatusBadRequest, "充值码已过期")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "兑换失败，请重试")
		}
		return
	}

	utils.SuccessJSON(c, result)
}

// GetTopupInfo GET /api/dist/topup/info (公开)
//
// 前端: Topup.jsx loadData -> api.getTopupInfo() -> GET /api/dist/topup/info
// 返回充值配置，前端消费字段：
//   - enable_topup: 是否启用充值
//   - enable_online_topup: 是否启用在线支付（本批返回 false，TODO 下一批接入真实支付网关）
//   - enable_stripe_topup: 是否启用 Stripe（本批返回 false，TODO）
//   - enable_crypto_topup: 是否启用 USDT（本批返回 false，TODO）
//   - currency: 充值货币（CNY/USD）
//   - exchange_rate: 汇率
//   - min_topup: 最小充值金额
func (ctrl *TopupController) GetTopupInfo(c *gin.Context) {
	// 本批只做兑换码，真实支付网关未接入，所有支付方式开关返回 false
	info := gin.H{
		"enable_topup":         true,  // 启用充值（兑换码可用）
		"enable_online_topup":  false, // TODO: 下一批接入易支付/支付宝
		"enable_stripe_topup":  false, // TODO: 下一批接入 Stripe
		"enable_creem_topup":   false, // TODO: 下一批接入 Creem（如需要）
		"enable_crypto_topup":  false, // TODO: 下一批接入 USDT
		"currency":             "CNY", // 默认人民币
		"exchange_rate":        7.0,   // 默认汇率 1 USD = 7 CNY（TODO: 从配置读取）
		"min_topup":            1.0,   // 最小充值 1 元（TODO: 从配置读取）
		"pay_methods":          []gin.H{}, // 暂无支付方式（TODO: 下一批添加）
		"top_up_link":          "",    // 外部充值商店链接（TODO: 从配置读取）
		"top_up_link_name":     "",    // 外部充值商店名称（TODO: 从配置读取）
	}

	utils.SuccessJSON(c, info)
}

// GetTopupHistory GET /api/dist/topup/history (受保护)
//
// 前端: Topup.jsx loadHistory -> api.getTopupHistory(params) -> GET /api/dist/topup/history
// 返回当前用户的充值历史（topup_orders 表）。
// 成功响应: {success:true, data:[{id,user_id,amount,quota,payment_method,status,created_at,...}], message:""}
func (ctrl *TopupController) GetTopupHistory(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	orders, err := ctrl.topupService.GetTopupHistory(userID)
	if err != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to load history")
		return
	}

	utils.SuccessJSON(c, orders)
}
