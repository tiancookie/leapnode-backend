// Package middleware 提供 Gin 中间件。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/utils"
)

// DistributorAuth 返回一个分站认证中间件。
//
// 要求:
//  1. 用户必须已通过基础认证
//  2. user_level 必须为 3 (分站站长)
//
// 非分站访问返回 403 Forbidden。
// 校验通过后允许请求继续。
//
// 注意：此中间件应在 AuthRequired 之后使用。
func DistributorAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查用户等级
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

		// 验证分站权限 (user_level = 3)
		if user.UserLevel != 3 {
			utils.ErrorJSON(c, http.StatusForbidden, "distributor access required")
			c.Abort()
			return
		}

		c.Next()
	}
}
