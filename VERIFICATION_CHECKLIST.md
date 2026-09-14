# LeapNode 功能验证清单

## 待验证功能（按批次）

### 第 1-4 批（已部分验证）
- [x] 用户注册/登录
- [x] Token 创建/列表/删除
- [x] 充值码兑换
- [ ] **GetTopupInfo 完整字段验证**（tier_list/套餐/邀请奖励）

### 第 5 批：USDT 加密货币充值（刚完成）

#### 基础功能验证
- [x] GetTopupInfo 返回 crypto 配置（enable_crypto_topup/crypto_wallets/crypto_expiry_minutes）
- [x] 创建订单：动态金额尾数生成（100 → 100.003662）
- [x] 查询状态：pending 状态正确
- [x] 提交 tx_hash：pending → reviewing 状态转换
- [x] 手动对账接口：reconcile 调用成功

#### 待真实环境验证（需真实收款地址 + 链上 RPC）
- [ ] **真实收款地址配置**（替换 PLACEHOLDER）
  - [ ] BSC 收款地址
  - [ ] Tron 收款地址
  
- [ ] **链上 RPC 验证**（需接入 Infura/TronGrid）
  - [ ] BSC: 查询 tx_hash 的 to/amount/token 是否匹配订单
  - [ ] Tron: 查询 tx_hash 的 to/amount/token 是否匹配订单
  - [ ] 验证通过后自动调用 ConfirmCryptoOrderPaid 入账
  
- [ ] **完整充值流程**
  - [ ] 用户创建订单（真实金额）
  - [ ] 用户转账到收款地址（真实 USDT）
  - [ ] 提交 tx_hash
  - [ ] 系统验证链上交易
  - [ ] 订单状态 reviewing → success
  - [ ] user.quota 正确增加（100 USDT → 50,000,000 quota）
  
- [ ] **过期机制**
  - [ ] 30 分钟未支付订单自动 expired
  - [ ] expired 订单不能再 claim
  
- [ ] **防碰撞验证**
  - [ ] 同一时间创建多个相同金额订单，pay_amount 不重复
  - [ ] 40 次重试机制触发测试
  
- [ ] **多租户隔离**（未来分站功能时验证）
  - [ ] distributor_id=0（总站）订单独立
  - [ ] distributor_id>0（分站）订单独立
  - [ ] 分站配置独立收款地址

#### 已知 TODO（代码已预留框架）
- [ ] 自动监听链上转账（Webhook/定时扫描）
- [ ] 收款配置迁移到 config.yaml 或 DB 分站表

---

## 验证优先级

**P0（必须立即验证）**：基础功能验证 ✅ 已完成

**P1（需真实环境，Week 10 前）**：
- 真实收款地址配置
- 链上 RPC 验证接入
- 完整充值流程（小额真实测试）

**P2（压力测试/边界场景）**：
- 防碰撞机制
- 过期机制
- 并发下单

**P3（多租户/长期）**：
- 分站独立配置（等分站管理功能完成后）

---

## 验证方法模板

### USDT 充值完整流程验证
```bash
# 1. 配置真实收款地址
railway variables --service leapnode-backend \
  --set "CRYPTO_WALLET_BSC=0x真实地址" \
  --set "CRYPTO_WALLET_TRON=T真实地址"

# 2. 创建订单
curl -X POST $BASE/api/dist/topup/crypto/pay \
  -H "Content-Type: application/json" \
  -d '{"amount":10,"chain":"bsc","token":"usdt","currency":"USD"}'
# 记录 trade_no 和 pay_amount

# 3. 真实转账（MetaMask/TronLink）
# 转账到返回的 wallet 地址，金额必须精确匹配 pay_amount

# 4. 提交 tx_hash
curl -X POST $BASE/api/dist/topup/crypto/claim \
  -d '{"trade_no":"CT...", "tx_hash":"0x真实hash"}'

# 5. 等待对账（或手动触发）
curl -X POST $BASE/api/dist/topup/crypto/reconcile \
  -d '{"trade_no":"CT..."}'

# 6. 验证结果
# - 订单状态 success
# - user.quota 增加 5,000,000（10 USD × 500,000）
# - crypto_topup_orders.paid_at 有时间戳
```

---

**更新时间**: 2026-09-14 18:40
**负责人**: 萃萃
