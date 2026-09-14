package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SessionCookieName 是签名会话 Cookie 的名称。
// 与 New-API 自身的 "session" 分开命名, 避免同域下互相覆盖。
const SessionCookieName = "leapnode_session"

// sessionTTL 会话有效期。
const sessionTTL = 30 * 24 * time.Hour

// sessionSecret 返回用于签名会话 Cookie 的密钥。
//
// 生产环境必须通过 LEAPNODE_SESSION_SECRET 注入; 未配置时回退到一个
// 开发用弱密钥并打印告警 (绝不可用于生产)。
func sessionSecret() []byte {
	if s := os.Getenv("LEAPNODE_SESSION_SECRET"); s != "" {
		return []byte(s)
	}
	log.Println("warning: LEAPNODE_SESSION_SECRET not set, using insecure dev fallback — DO NOT use in production")
	return []byte("leapnode-dev-insecure-session-secret")
}

// isSecureCookie 判断是否应设置 Secure 标志。
// 默认开启; 通过 LEAPNODE_COOKIE_INSECURE=1 可在本地 http 环境下关闭。
func isSecureCookie() bool {
	return os.Getenv("LEAPNODE_COOKIE_INSECURE") != "1"
}

// signSession 生成 "<userID>.<expiryUnix>.<base64hmac>" 形式的签名令牌。
func signSession(userID int, expiry time.Time) string {
	payload := fmt.Sprintf("%d.%d", userID, expiry.Unix())
	mac := hmac.New(sha256.New, sessionSecret())
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// SetSession 在响应中写入签名会话 Cookie。
func SetSession(c *gin.Context, userID int) {
	expiry := time.Now().Add(sessionTTL)
	token := signSession(userID, expiry)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		SessionCookieName,
		token,
		int(sessionTTL.Seconds()),
		"/",
		"", // domain: 交由浏览器按当前 host 处理
		isSecureCookie(),
		true, // HttpOnly
	)
}

// ClearSession 清除会话 Cookie。
func ClearSession(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", isSecureCookie(), true)
}

// ParseSession 校验会话 Cookie 并返回其中的 userID。
// 返回 (userID, true) 表示会话有效且未过期。
func ParseSession(c *gin.Context) (int, bool) {
	raw, err := c.Cookie(SessionCookieName)
	if err != nil || raw == "" {
		return 0, false
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return 0, false
	}
	userID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	expUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	// 校验签名 (恒定时间比较)
	payload := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, sessionSecret())
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(parts[2])) != 1 {
		return 0, false
	}
	// 校验过期
	if time.Now().Unix() > expUnix {
		return 0, false
	}
	return userID, true
}
