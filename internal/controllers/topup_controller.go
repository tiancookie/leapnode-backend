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

	// USDT / 加密货币充值 (固定收款地址 + 动态金额尾数匹配)
	group.POST("/crypto/pay", ctrl.CreateCryptoOrder)
	group.GET("/crypto/status", ctrl.GetCryptoOrderStatus)
	group.POST("/crypto/reconcile", ctrl.ReconcileCryptoOrder)
	group.POST("/crypto/claim", ctrl.ClaimCryptoOrder)
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
	// 加密货币充值配置从环境变量读取（收款地址不硬编码）。
	cryptoCfg := ctrl.topupService.GetCryptoConfig()

	info := gin.H{
		"enable_topup":          true,                    // 启用充值（兑换码可用）
		"enable_online_topup":   false,                   // TODO: 下一批接入易支付/支付宝
		"enable_stripe_topup":   false,                   // TODO: 下一批接入 Stripe
		"enable_creem_topup":    false,                   // TODO: 下一批接入 Creem（如需要）
		"enable_crypto_topup":   cryptoCfg.Enabled,       // USDT 开关（ENABLE_CRYPTO_TOPUP=1 且已配置钱包）
		"crypto_wallets":        cryptoCfg.Wallets,       // map 链->收款地址（tron/eth/bsc/polygon/solana）
		"crypto_expiry_minutes": cryptoCfg.ExpiryMinutes, // 订单过期分钟（默认 30）
		"currency":              "CNY",                   // 默认人民币
		"exchange_rate":         cryptoCfg.ExchangeRate,  // 汇率 1 USD = ? CNY
		"min_topup":             cryptoCfg.MinTopup,      // 最小充值金额
		"pay_methods":           []gin.H{},               // 暂无易支付/Stripe 支付方式（TODO: 下一批添加）
		"top_up_link":           "",                      // 外部充值商店链接（TODO: 从配置读取）
		"top_up_link_name":      "",                      // 外部充值商店名称（TODO: 从配置读取）
	}

	utils.SuccessJSON(c, info)
}

// successMessageJSON 返回 {success:true, data, message:"success"}。
//
// 前端 createCryptoOrder / 对账 modal 严格判断 res.data.message === "success",
// 而 utils.SuccessJSON 的 message 为空串, 故 crypto 接口用此助手对齐契约。
func successMessageJSON(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, utils.Response{
		Success: true,
		Data:    data,
		Message: "success",
	})
}

// CreateCryptoOrder POST /api/dist/topup/crypto/pay (受保护)
//
// 前端: Topup.jsx handleCryptoPay -> createCryptoOrder({amount,chain,token,currency,tier_index?})
// 成功响应: {success:true, message:"success", data:{trade_no,chain,token,wallet,amount,status}}
func (ctrl *TopupController) CreateCryptoOrder(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input services.CreateCryptoOrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "请求参数不合法")
		return
	}

	view, err := ctrl.topupService.CreateCryptoOrder(userID, input)
	if err != nil {
		ctrl.handleCryptoError(c, err)
		return
	}

	successMessageJSON(c, view)
}

// GetCryptoOrderStatus GET /api/dist/topup/crypto/status?trade_no=xxx (受保护)
//
// 前端每 5 秒轮询: getCryptoOrderStatus(tradeNo) -> data.status ∈ {pending,success,expired,...}
// 过期自动标记: pending 且 now > expire_at -> expired。
func (ctrl *TopupController) GetCryptoOrderStatus(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	tradeNo := c.Query("trade_no")
	status, err := ctrl.topupService.GetCryptoOrderStatus(userID, tradeNo)
	if err != nil {
		ctrl.handleCryptoError(c, err)
		return
	}

	successMessageJSON(c, gin.H{"status": status})
}

// ReconcileCryptoOrder POST /api/dist/topup/crypto/reconcile (受保护)
//
// 前端: CryptoTopupReconcileModal reconcile() -> reconcileCryptoOrder(trade_no)
// 响应 data 含 phase (success/challenge/candidates/其他), 前端据此渲染。
// 本批仅回显订单状态, 真实链上核对留 TODO。
func (ctrl *TopupController) ReconcileCryptoOrder(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		TradeNo string `json:"trade_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "请求参数不合法")
		return
	}

	result, err := ctrl.topupService.ReconcileCryptoOrder(userID, body.TradeNo)
	if err != nil {
		ctrl.handleCryptoError(c, err)
		return
	}

	successMessageJSON(c, result)
}

// ClaimCryptoOrder POST /api/dist/topup/crypto/claim (受保护)
//
// 前端: CryptoTopupReconcileModal startClaim(txHash) -> claimCryptoOrderTransfer(trade_no, tx_hash)
// 记录 tx_hash, 状态转 reviewing 等待对账确认。真实链上验证留 TODO。
func (ctrl *TopupController) ClaimCryptoOrder(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input services.ClaimCryptoInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "请求参数不合法")
		return
	}

	result, err := ctrl.topupService.ClaimCryptoOrderTransfer(userID, input)
	if err != nil {
		ctrl.handleCryptoError(c, err)
		return
	}

	successMessageJSON(c, result)
}

// handleCryptoError 将 crypto 服务层错误映射为用户可读的中文错误响应。
func (ctrl *TopupController) handleCryptoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrCryptoDisabled):
		utils.ErrorJSON(c, http.StatusBadRequest, "加密货币充值未启用")
	case errors.Is(err, services.ErrCryptoInvalidAmount):
		utils.ErrorJSON(c, http.StatusBadRequest, "充值金额不合法")
	case errors.Is(err, services.ErrCryptoInvalidChain):
		utils.ErrorJSON(c, http.StatusBadRequest, "不支持的链或未配置收款地址")
	case errors.Is(err, services.ErrCryptoInvalidToken):
		utils.ErrorJSON(c, http.StatusBadRequest, "不支持的代币")
	case errors.Is(err, services.ErrCryptoTradeNoRequired):
		utils.ErrorJSON(c, http.StatusBadRequest, "订单号不能为空")
	case errors.Is(err, services.ErrCryptoTxHashRequired):
		utils.ErrorJSON(c, http.StatusBadRequest, "交易哈希不能为空")
	case errors.Is(err, services.ErrCryptoOrderNotFound):
		utils.ErrorJSON(c, http.StatusNotFound, "订单不存在")
	case errors.Is(err, services.ErrCryptoAmountUnique):
		utils.ErrorJSON(c, http.StatusServiceUnavailable, "系统繁忙，请稍后重试")
	default:
		utils.ErrorJSON(c, http.StatusInternalServerError, "操作失败，请重试")
	}
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
