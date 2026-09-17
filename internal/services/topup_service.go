// Package services 实现业务逻辑。
package services

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrRedeemCodeRequired 兑换码为空
	ErrRedeemCodeRequired = errors.New("redeem code required")
	// ErrRedeemCodeInvalid 兑换码无效（不存在）
	ErrRedeemCodeInvalid = errors.New("invalid redeem code")
	// ErrRedeemCodeUsed 兑换码已使用
	ErrRedeemCodeUsed = errors.New("redeem code already used")
	// ErrRedeemCodeDisabled 兑换码已禁用
	ErrRedeemCodeDisabled = errors.New("redeem code disabled")
	// ErrRedeemCodeExpired 兑换码已过期
	ErrRedeemCodeExpired = errors.New("redeem code expired")

	// --- Crypto 充值相关错误 ---

	// ErrCryptoDisabled 未启用加密货币充值
	ErrCryptoDisabled = errors.New("crypto topup disabled")
	// ErrCryptoInvalidAmount 金额非法（<=0 或 < 最小充值）
	ErrCryptoInvalidAmount = errors.New("invalid crypto amount")
	// ErrCryptoInvalidChain 链不支持（未配置收款地址）
	ErrCryptoInvalidChain = errors.New("invalid or unconfigured chain")
	// ErrCryptoInvalidToken 代币不支持
	ErrCryptoInvalidToken = errors.New("invalid crypto token")
	// ErrCryptoOrderNotFound 订单不存在
	ErrCryptoOrderNotFound = errors.New("crypto order not found")
	// ErrCryptoTradeNoRequired trade_no 为空
	ErrCryptoTradeNoRequired = errors.New("trade_no required")
	// ErrCryptoTxHashRequired tx_hash 为空
	ErrCryptoTxHashRequired = errors.New("tx_hash required")
	// ErrCryptoAmountUnique 动态金额生成失败（重试耗尽仍撞金额）
	ErrCryptoAmountUnique = errors.New("failed to allocate unique pay amount")
)

// 支持的代币（前端 CRYPTO_TOKEN_OPTIONS）。
var supportedCryptoTokens = map[string]bool{"usdt": true, "usdc": true}

const (
	// cryptoExpiryMinutesDefault 默认订单过期时间（分钟），与前端轮询自动停止一致。
	cryptoExpiryMinutesDefault = 30
	// cryptoAmountMaxRetries 动态金额去重最大重试次数。
	cryptoAmountMaxRetries = 40
	// cryptoTradeNoMaxRetries trade_no 唯一性最大重试次数。
	cryptoTradeNoMaxRetries = 8
)

// TopupService 充值相关业务逻辑。
type TopupService struct {
	db           *gorm.DB
	newAPIClient *NewAPIClient
}

// NewTopupService 创建 TopupService。
func NewTopupService(db *gorm.DB, newAPIClient *NewAPIClient) *TopupService {
	return &TopupService{
		db:           db,
		newAPIClient: newAPIClient,
	}
}

// RedeemCodeInput 兑换充值码入参。
type RedeemCodeInput struct {
	Key string `json:"key" binding:"required"`
}

// RedeemCodeResult 兑换充值码结果。
type RedeemCodeResult struct {
	Quota   int    `json:"quota"`   // 本次兑换增加的 quota
	Message string `json:"message"` // 成功提示消息
}

// RedeemCode 兑换充值码（事务安全）。
//
// 核心逻辑:
//  1. 查询充值码（加行锁 SELECT FOR UPDATE）
//  2. 校验状态（status=1 未使用、未过期）
//  3. 事务内更新充值码（status=3 已使用、记录使用时间和用户ID）
//  4. 事务内给用户增加 quota
//  5. 提交事务
//
// 并发安全: 使用 gorm Transaction + clause.Locking 防止重复兑换。
func (s *TopupService) RedeemCode(userID int, key string) (*RedeemCodeResult, error) {
	if key == "" {
		return nil, ErrRedeemCodeRequired
	}

	var result *RedeemCodeResult

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询充值码并加行锁（SELECT FOR UPDATE）
		var redemption models.Redemption
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("key = ?", key).
			First(&redemption).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRedeemCodeInvalid
			}
			return err
		}

		// 2. 校验状态
		// Status: 1=未使用 2=已禁用 3=已使用
		switch redemption.Status {
		case 2:
			return ErrRedeemCodeDisabled
		case 3:
			return ErrRedeemCodeUsed
		case 1:
			// 继续
		default:
			return ErrRedeemCodeInvalid
		}

		// 3. 校验过期时间（0 表示不过期）
		now := time.Now().Unix()
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < now {
			return ErrRedeemCodeExpired
		}

		// 4. 更新充值码状态（防并发重复兑换：WHERE status=1）
		updateResult := tx.Model(&models.Redemption{}).
			Where("id = ? AND status = ?", redemption.Id, 1).
			Updates(map[string]interface{}{
				"status":        3,
				"redeemed_time": now,
				"used_user_id":  userID,
			})

		if updateResult.Error != nil {
			return updateResult.Error
		}

		// 并发情况下，另一个事务可能已经更新了状态，RowsAffected = 0
		if updateResult.RowsAffected == 0 {
			return ErrRedeemCodeUsed
		}

		// 5. 调用 New-API 增加 quota（通过 HTTP API，不直接写 users 表）
		if err := s.newAPIClient.IncreaseQuota(userID, redemption.Quota); err != nil {
			return fmt.Errorf("failed to increase quota via New-API: %w", err)
		}

		// 构造返回结果
		result = &RedeemCodeResult{
			Quota:   redemption.Quota,
			Message: "充值成功",
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetTopupHistory 获取当前用户的充值历史。
//
// 查询 topup_orders 表，WHERE user_id = 当前用户，按创建时间倒序。
func (s *TopupService) GetTopupHistory(userID int) ([]models.TopupOrder, error) {
	var orders []models.TopupOrder
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&orders).Error

	if err != nil {
		return nil, err
	}

	return orders, nil
}

// =============================================================================
// USDT / 加密货币充值 (固定收款地址 + 动态金额尾数匹配模式)
// =============================================================================

// CryptoConfig 加密货币充值配置 (从环境变量读取, 收款地址不硬编码)。
//
// 收款地址按链读取环境变量:
//
//	CRYPTO_WALLET_TRON / CRYPTO_WALLET_ETH / CRYPTO_WALLET_BSC /
//	CRYPTO_WALLET_POLYGON / CRYPTO_WALLET_SOLANA
//
// key 必须与前端 CRYPTO_NETWORKS 的 key 一致 (tron/eth/bsc/polygon/solana),
// 否则前端 availableChains 过滤不到对应钱包, 该链不会展示。
type CryptoConfig struct {
	Enabled       bool              // 是否启用 (ENABLE_CRYPTO_TOPUP=1)
	Wallets       map[string]string // 链 -> 收款地址 (仅含已配置的链)
	ExpiryMinutes int               // 订单过期分钟数 (CRYPTO_EXPIRY_MINUTES, 默认 30)
	ExchangeRate  float64           // 法币汇率 1 USD = ? CNY (TOPUP_EXCHANGE_RATE, 默认 7.0)
	MinTopup      float64           // 最小充值金额 (CRYPTO_MIN_TOPUP, 默认 1.0, 单位同 currency)
}

// cryptoChainEnvKeys 链 key -> 环境变量名 (key 与前端 CRYPTO_NETWORKS 对齐)。
var cryptoChainEnvKeys = map[string]string{
	"tron":    "CRYPTO_WALLET_TRON",
	"eth":     "CRYPTO_WALLET_ETH",
	"bsc":     "CRYPTO_WALLET_BSC",
	"polygon": "CRYPTO_WALLET_POLYGON",
	"solana":  "CRYPTO_WALLET_SOLANA",
}

// GetCryptoConfig 从环境变量组装加密货币充值配置。
//
// TODO(parent): 收款地址/开关/汇率后续可迁移到 config.yaml 或 DB 的分站配置表,
// 以支持每个 distributor 各自配置 (当前 USDT 总站/分站共用总站钱包)。
func (s *TopupService) GetCryptoConfig() CryptoConfig {
	wallets := make(map[string]string)
	for chain, env := range cryptoChainEnvKeys {
		if addr := strings.TrimSpace(os.Getenv(env)); addr != "" {
			wallets[chain] = addr
		}
	}

	expiry := cryptoExpiryMinutesDefault
	if v := os.Getenv("CRYPTO_EXPIRY_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			expiry = n
		}
	}

	rate := 7.0
	if v := os.Getenv("TOPUP_EXCHANGE_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			rate = f
		}
	}

	minTopup := 1.0
	if v := os.Getenv("CRYPTO_MIN_TOPUP"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			minTopup = f
		}
	}

	// enable_crypto_topup: 显式开关, 且至少配置了一个收款地址才算真正可用。
	enabled := os.Getenv("ENABLE_CRYPTO_TOPUP") == "1" && len(wallets) > 0

	return CryptoConfig{
		Enabled:       enabled,
		Wallets:       wallets,
		ExpiryMinutes: expiry,
		ExchangeRate:  rate,
		MinTopup:      minTopup,
	}
}

// CreateCryptoOrderInput 创建加密货币充值订单入参 (对齐前端 createCryptoOrder)。
type CreateCryptoOrderInput struct {
	Amount    float64 `json:"amount" binding:"required"`
	Chain     string  `json:"chain" binding:"required"`
	Token     string  `json:"token" binding:"required"`
	Currency  string  `json:"currency"`
	TierIndex *int    `json:"tier_index"`
}

// CryptoOrderView 返回给前端的订单视图 (对齐前端 cryptoOrder.* 消费字段)。
type CryptoOrderView struct {
	TradeNo string  `json:"trade_no"`
	Chain   string  `json:"chain"`
	Token   string  `json:"token"`
	Wallet  string  `json:"wallet"` // 收款地址
	Amount  float64 `json:"amount"` // 精确应付金额 (含随机尾数)
	Status  string  `json:"status"` // pending/reviewing/success/expired/failed
}

// CreateCryptoOrder 创建 USDT 充值订单。
//
// 流程:
//  1. 校验开关/金额/链(收款地址)/代币
//  2. currency 换算: CNY 先按 exchange_rate 转 USD, 再 DollarsToQuota 得 quota
//  3. 生成唯一 trade_no
//  4. 动态金额: pay_amount = base + 随机尾数, 保证同链同地址 pending 唯一
//  5. 落库, status=pending, 记录 expire_at
func (s *TopupService) CreateCryptoOrder(userID int, in CreateCryptoOrderInput) (*CryptoOrderView, error) {
	cfg := s.GetCryptoConfig()
	if !cfg.Enabled {
		return nil, ErrCryptoDisabled
	}

	chain := strings.ToLower(strings.TrimSpace(in.Chain))
	token := strings.ToLower(strings.TrimSpace(in.Token))
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "USD"
	}

	if in.Amount <= 0 || in.Amount < cfg.MinTopup {
		return nil, ErrCryptoInvalidAmount
	}
	if !supportedCryptoTokens[token] {
		return nil, ErrCryptoInvalidToken
	}
	wallet, ok := cfg.Wallets[chain]
	if !ok || wallet == "" {
		return nil, ErrCryptoInvalidChain
	}

	// currency 换算成美元, 再换算成 quota (Q=500000/$)。
	usdAmount := in.Amount
	if currency == "CNY" {
		usdAmount = in.Amount / cfg.ExchangeRate
	}
	quota := utils.DollarsToQuota(usdAmount)

	tierIndex := -1
	if in.TierIndex != nil {
		tierIndex = *in.TierIndex
	}

	now := time.Now()
	expireAt := now.Add(time.Duration(cfg.ExpiryMinutes) * time.Minute)

	// 生成唯一 trade_no。
	tradeNo, err := s.generateUniqueTradeNo()
	if err != nil {
		return nil, err
	}

	// 动态金额: base + 随机尾数, 保证同链同地址 pending 订单金额唯一。
	payAmount, err := s.allocateUniquePayAmount(chain, wallet, in.Amount)
	if err != nil {
		return nil, err
	}

	order := models.CryptoTopupOrder{
		TradeNo:       tradeNo,
		UserID:        userID,
		DistributorID: 0, // TODO(parent): 分站上下文接入后按当前站点填充
		Chain:         chain,
		Token:         token,
		Currency:      currency,
		BaseAmount:    in.Amount,
		PayAmount:     payAmount,
		Quota:         quota,
		WalletAddress: wallet,
		Status:        "pending",
		TierIndex:     tierIndex,
		CreatedAt:     now,
		ExpireAt:      expireAt,
	}

	if err := s.db.Create(&order).Error; err != nil {
		return nil, err
	}

	return &CryptoOrderView{
		TradeNo: order.TradeNo,
		Chain:   order.Chain,
		Token:   order.Token,
		Wallet:  order.WalletAddress,
		Amount:  order.PayAmount,
		Status:  order.Status,
	}, nil
}

// generateUniqueTradeNo 生成唯一订单号: CT + 时间戳 + 随机后缀。
func (s *TopupService) generateUniqueTradeNo() (string, error) {
	for i := 0; i < cryptoTradeNoMaxRetries; i++ {
		suffix, err := randomDigits(6)
		if err != nil {
			return "", err
		}
		tradeNo := fmt.Sprintf("CT%d%s", time.Now().UnixNano(), suffix)

		var count int64
		if err := s.db.Model(&models.CryptoTopupOrder{}).
			Where("trade_no = ?", tradeNo).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return tradeNo, nil
		}
	}
	return "", ErrCryptoAmountUnique
}

// allocateUniquePayAmount 生成动态精确金额。
//
// 算法:
//
//	pay_amount = base_amount + 随机尾数
//	随机尾数 ∈ [0.000001, 0.009999] (小数点后 3-6 位, 步进 0.000001)
//	即在 base 之上加 1..9999 个百万分之一, 共约 1 万种取值。
//
// 生成后查 (同链 + 同收款地址 + status=pending) 是否已存在相同 pay_amount,
// 存在则重试; 重试耗尽 (cryptoAmountMaxRetries) 返回 ErrCryptoAmountUnique。
//
// 金额以 6 位小数存储 (DECIMAL(20,6)), 用整数微单位比较避免浮点误差。
func (s *TopupService) allocateUniquePayAmount(chain, wallet string, base float64) (float64, error) {
	// base 转微单位 (x1e6) 整数, 四舍五入避免浮点。
	baseMicro := int64(base*1e6 + 0.5)

	for i := 0; i < cryptoAmountMaxRetries; i++ {
		// 随机尾数 1..9999 微单位 (0.000001 .. 0.009999)。
		n, err := rand.Int(rand.Reader, big.NewInt(9999))
		if err != nil {
			return 0, err
		}
		tailMicro := n.Int64() + 1 // 1..9999
		payMicro := baseMicro + tailMicro
		payAmount := float64(payMicro) / 1e6

		// 查同链同地址 pending 订单是否已占用该精确金额。
		// 用 [pay-0.0000005, pay+0.0000005] 区间避免 DECIMAL 边界浮点比较问题。
		lo := float64(payMicro) - 0.5
		hi := float64(payMicro) + 0.5
		var count int64
		if err := s.db.Model(&models.CryptoTopupOrder{}).
			Where("chain = ? AND wallet_address = ? AND status = ?", chain, wallet, "pending").
			Where("pay_amount * 1000000 BETWEEN ? AND ?", lo, hi).
			Count(&count).Error; err != nil {
			return 0, err
		}
		if count == 0 {
			return payAmount, nil
		}
	}
	return 0, ErrCryptoAmountUnique
}

// randomDigits 生成 n 位随机数字字符串 (0-9)。
func randomDigits(n int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[idx.Int64()]
	}
	return string(b), nil
}

// GetCryptoOrderStatus 查询订单状态。
//
// 若订单仍 pending 且 now > expire_at, 自动标记 expired 后返回。
// 返回 status ∈ {pending, reviewing, success, expired, failed}。
func (s *TopupService) GetCryptoOrderStatus(userID int, tradeNo string) (string, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return "", ErrCryptoTradeNoRequired
	}

	var order models.CryptoTopupOrder
	err := s.db.Where("trade_no = ? AND user_id = ?", tradeNo, userID).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrCryptoOrderNotFound
		}
		return "", err
	}

	// 过期处理: pending 且已过期 -> expired。
	if order.Status == "pending" && time.Now().After(order.ExpireAt) {
		if err := s.db.Model(&models.CryptoTopupOrder{}).
			Where("id = ? AND status = ?", order.ID, "pending").
			Update("status", "expired").Error; err != nil {
			return "", err
		}
		return "expired", nil
	}

	return order.Status, nil
}

// CryptoReconcileResult 对账结果 (对齐前端 CryptoTopupReconcileModal 消费的 phase 字段)。
//
// phase 语义 (前端渲染分支):
//   - "success":    已到账, 前端弹成功
//   - "challenge":  需要验证原始钱包 (需 claim.challenge_amount/wallet/from_address 等)
//   - "candidates": 列出候选链上转账供用户认领 (candidates[].tx_hash/amount/...)
//   - 其他:         无可认领转账 (前端 else 分支)
//
// 本批仅做订单状态回显, 真实链上 RPC 查询留 TODO, 因此:
//   - success 订单 -> phase=success
//   - 其余 -> phase="" (前端显示"暂无可认领转账", 用户可提交 hash 走 claim)
type CryptoReconcileResult struct {
	Phase       string                `json:"phase"`
	CryptoChain string                `json:"crypto_chain"`
	CryptoToken string                `json:"crypto_token"`
	Candidates  []CryptoTxCandidate   `json:"candidates"`
	Claim       *CryptoReconcileClaim `json:"claim,omitempty"`
}

// CryptoTxCandidate 候选链上转账 (真实数据来自链上 RPC, 本批为空)。
type CryptoTxCandidate struct {
	TxHash      string  `json:"tx_hash"`
	Amount      float64 `json:"amount"`
	FromAddress string  `json:"from_address"`
	Timestamp   int64   `json:"timestamp"`
}

// CryptoReconcileClaim challenge 阶段返回的验证信息 (本批未启用)。
type CryptoReconcileClaim struct {
	Chain           string  `json:"chain"`
	Token           string  `json:"token"`
	Wallet          string  `json:"wallet"`
	ChallengeAmount float64 `json:"challenge_amount"`
	FromAddress     string  `json:"from_address"`
	ExpiresAt       int64   `json:"expires_at"`
}

// ReconcileCryptoOrder 手动触发对账。
//
// 本批实现: 检查订单当前状态并回显。
//   - success:  phase=success (已到账)
//   - expired:  自动确认过期; phase="" (无可认领)
//   - pending/reviewing: phase="" + 空 candidates (前端提示暂无可认领转账)
//
// TODO(parent): 真实对账需接链上 RPC:
//  1. 按 (chain + wallet_address + pay_amount + [created_at, expire_at] 时间窗) 查链上入账
//  2. 命中则事务确认到账 (confirmCryptoOrderPaid), 未命中列出候选转账 (candidates)
//  3. 金额尾数不符时进入 challenge 验证原始钱包
func (s *TopupService) ReconcileCryptoOrder(userID int, tradeNo string) (*CryptoReconcileResult, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, ErrCryptoTradeNoRequired
	}

	var order models.CryptoTopupOrder
	err := s.db.Where("trade_no = ? AND user_id = ?", tradeNo, userID).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCryptoOrderNotFound
		}
		return nil, err
	}

	// 过期处理: pending 且已过期 -> expired。
	if order.Status == "pending" && time.Now().After(order.ExpireAt) {
		_ = s.db.Model(&models.CryptoTopupOrder{}).
			Where("id = ? AND status = ?", order.ID, "pending").
			Update("status", "expired").Error
		order.Status = "expired"
	}

	result := &CryptoReconcileResult{
		CryptoChain: order.Chain,
		CryptoToken: order.Token,
		Candidates:  []CryptoTxCandidate{},
	}
	if order.Status == "success" {
		result.Phase = "success"
	} else {
		// TODO(parent): 接链上 RPC 后, 此处根据入账查询结果返回
		// phase=success / candidates / challenge。当前无链上数据源, 返回空。
		result.Phase = ""
	}

	return result, nil
}

// ClaimCryptoInput 用户提交交易 hash 入参。
type ClaimCryptoInput struct {
	TradeNo string `json:"trade_no" binding:"required"`
	TxHash  string `json:"tx_hash" binding:"required"`
}

// ClaimCryptoOrderTransfer 用户提交链上交易 hash, 状态转为 reviewing 等待对账确认。
//
// 本批实现: 记录 tx_hash, pending/expired -> reviewing。
// TODO(parent): 真实链上验证 tx_hash 的金额/收款地址/确认数是否匹配订单,
//
//	匹配则调 confirmCryptoOrderPaid 给用户加 quota; 不匹配则标记 failed 或退回。
func (s *TopupService) ClaimCryptoOrderTransfer(userID int, in ClaimCryptoInput) (*CryptoReconcileResult, error) {
	tradeNo := strings.TrimSpace(in.TradeNo)
	txHash := strings.TrimSpace(in.TxHash)
	if tradeNo == "" {
		return nil, ErrCryptoTradeNoRequired
	}
	if txHash == "" {
		return nil, ErrCryptoTxHashRequired
	}

	var order models.CryptoTopupOrder
	err := s.db.Where("trade_no = ? AND user_id = ?", tradeNo, userID).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCryptoOrderNotFound
		}
		return nil, err
	}

	// 已到账直接回显成功。
	if order.Status == "success" {
		return &CryptoReconcileResult{
			Phase:       "success",
			CryptoChain: order.Chain,
			CryptoToken: order.Token,
			Candidates:  []CryptoTxCandidate{},
		}, nil
	}

	// 记录 tx_hash 并转 reviewing (从 pending/expired/reviewing 均可提交)。
	if err := s.db.Model(&models.CryptoTopupOrder{}).
		Where("id = ?", order.ID).
		Updates(map[string]interface{}{
			"tx_hash": txHash,
			"status":  "reviewing",
		}).Error; err != nil {
		return nil, err
	}

	// TODO(parent): 触发链上验证。当前返回空 phase, 前端显示"提交成功待对账"。
	return &CryptoReconcileResult{
		Phase:       "",
		CryptoChain: order.Chain,
		CryptoToken: order.Token,
		Candidates:  []CryptoTxCandidate{},
	}, nil
}

// ConfirmCryptoOrderPaid 确认加密货币订单到账 (事务安全)。
//
// 供真实对账/后台审核调用: 事务内
//  1. 行锁查订单, 校验非终态 (未 success)
//  2. CAS 更新订单 status=success, paid_at, tx_hash
//  3. 给 user.quota 加 order.quota (gorm.Expr 原子自增)
//  4. 写一条 topup_orders 历史 (payment_method=crypto, status=1 已支付)
//
// 参考 RedeemCode 的事务写法。幂等: 已 success 的订单不重复加 quota。
//
// TODO(parent): 由真实链上对账或后台审核触发; 本批 4 接口暂不自动调用
// (无链上数据源), 保留为可复用的到账入账入口。
func (s *TopupService) ConfirmCryptoOrderPaid(tradeNo, txHash string) error {
	// isFirstTopup / topupUserID / topupAmount 在事务内确定，事务提交成功后再触发首充返佣。
	// 之所以在事务外触发：GrantFirstTopupReward 用 s.db 读写，需读到已提交的 topup_orders 行，
	// 否则会与未提交的事务产生可见性竞态。
	var isFirstTopup bool
	var topupUserID int
	var topupAmount float64

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.CryptoTopupOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCryptoOrderNotFound
			}
			return err
		}

		// 幂等: 已到账不重复处理。
		if order.Status == "success" {
			return nil
		}

		now := time.Now()
		finalTxHash := order.TxHash
		if strings.TrimSpace(txHash) != "" {
			finalTxHash = txHash
		}

		// CAS 更新: WHERE status != success 防并发重复入账。
		res := tx.Model(&models.CryptoTopupOrder{}).
			Where("id = ? AND status <> ?", order.ID, "success").
			Updates(map[string]interface{}{
				"status":  "success",
				"paid_at": now,
				"tx_hash": finalTxHash,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 并发下已被其它事务确认。
			return nil
		}

		// 给用户加 quota（调用 New-API HTTP API）。
		if err := s.newAPIClient.IncreaseQuota(order.UserID, int(order.Quota)); err != nil {
			return fmt.Errorf("failed to increase quota via New-API: %w", err)
		}

		// 写 topup_orders 历史 (统一充值记录)。
		paidTime := now
		history := models.TopupOrder{
			ID:            order.TradeNo,
			UserID:        order.UserID,
			Amount:        order.PayAmount,
			Quota:         order.Quota,
			PaymentMethod: "crypto",
			TradeNo:       order.TradeNo,
			Status:        1, // 1=已支付
			PaidAt:        &paidTime,
		}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}

		// --- 批10补充：检测首充（topup_orders 表中 status=1 的订单数）---
		var topupCount int64
		if err := tx.Model(&models.TopupOrder{}).
			Where("user_id = ? AND status = 1", order.UserID).
			Count(&topupCount).Error; err != nil {
			return err
		}
		// 刚写入的这条是唯一一条 => 首充
		if topupCount == 1 {
			isFirstTopup = true
			topupUserID = order.UserID
			topupAmount = order.PayAmount
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	// --- 批10补充：事务提交后触发首充返佣（防重复由 GrantFirstTopupReward 内部保证）---
	if isFirstTopup {
		affService := NewAffService(s.db, s.newAPIClient)
		if err := affService.GrantFirstTopupReward(topupUserID, topupAmount); err != nil {
			// 记录日志但不阻塞充值主流程（quota 已到账）
			fmt.Printf("Failed to grant first topup reward: user=%d, amount=%.2f, err=%v\n", topupUserID, topupAmount, err)
		}
	}

	return nil
}
