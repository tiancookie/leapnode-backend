// Package middleware 提供 Gin 中间件。
package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/utils"
	"gorm.io/gorm"
)

// gin.Context 中注入的键名。
const (
	CtxUserIDKey = "auth_user_id"
	CtxUserKey   = "auth_user"
)

// AuthRequired 返回一个认证中间件。
//
// 双重认证 (与 New-API + SubRouter 前端契约一致):
//  1. 请求头 New-Api-User: <userId> (前端 api.js 拦截器注入)
//  2. 签名 Session Cookie (withCredentials 自动携带)
//
// 两者都必须存在、都指向同一个 userID, 且该用户存在且状态正常。
// 校验通过后把 userID 与完整 User 注入 gin.Context, 供处理器复用。
// 任一环节失败均返回 401, 前端拦截器据此清除 dist_user_id 并跳登录。
func AuthRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. New-Api-User 头
		headerVal := c.GetHeader("New-Api-User")
		if headerVal == "" {
			utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		headerUserID, err := strconv.Atoi(headerVal)
		if err != nil {
			utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		// 2. Session Cookie
		sessionUserID, ok := utils.ParseSession(c)
		if !ok {
			utils.ErrorJSON(c, http.StatusUnauthorized, "session expired")
			c.Abort()
			return
		}

		// 3. 两者必须一致 (防止拿别人的 header 冒充)
		if headerUserID != sessionUserID {
			utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		// 4. 用户必须存在且未被禁用
		var user models.User
		if err := db.Where("id = ?", sessionUserID).First(&user).Error; err != nil {
			utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		if user.Status != 1 {
			utils.ErrorJSON(c, http.StatusUnauthorized, "account disabled")
			c.Abort()
			return
		}

		c.Set(CtxUserIDKey, user.ID)
		c.Set(CtxUserKey, &user)
		c.Next()
	}
}

// CurrentUserID 从 gin.Context 取出已认证的 userID。
func CurrentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get(CtxUserIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

// CurrentUser 从 gin.Context 取出已认证的 User。
func CurrentUser(c *gin.Context) (*models.User, bool) {
	v, ok := c.Get(CtxUserKey)
	if !ok {
		return nil, false
	}
	u, ok := v.(*models.User)
	return u, ok
}
