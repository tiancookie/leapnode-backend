// Package middleware 提供 Gin 中间件。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// MerchantAuth 返回一个商家认证中间件。
//
// 要求:
//  1. 用户必须已通过基础认证
//  2. user_level 必须为 2 (商家)
//
// 非商家访问返回 403 Forbidden。
// 校验通过后允许请求继续。
//
// 注意：此中间件应在 AuthRequired 之后使用。
func MerchantAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查用户等级 (使用 auth.go 中定义的常量)
		userVal, exists := c.Get("auth_user")
		if !exists {
			utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}

		user, ok := userVal.(*models.User)
		if !ok {
			utils.ErrorJSON(c, http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}

		// 验证商家权限 (user_level = 2)
		if user.UserLevel != 2 {
			utils.ErrorJSON(c, http.StatusForbidden, "merchant access required")
			c.Abort()
			return
		}

		c.Next()
	}
}
