// Package utils 提供通用工具函数。
package utils

import "github.com/gin-gonic/gin"

// QuotaPerDollar Quota 体系换算基准: 500,000 Quota = $1.00
const QuotaPerDollar = 500000

// DollarsToQuota 美元金额转 Quota。
func DollarsToQuota(dollars float64) int64 {
	return int64(dollars * QuotaPerDollar)
}

// QuotaToDollars Quota 转美元金额。
func QuotaToDollars(quota int64) float64 {
	return float64(quota) / QuotaPerDollar
}

// Response 统一响应结构 (与前端 api.js 契约对齐)
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// SuccessJSON 返回成功响应
func SuccessJSON(c *gin.Context, data interface{}) {
	c.JSON(200, Response{
		Success: true,
		Data:    data,
		Message: "",
	})
}

// ErrorJSON 返回失败响应
func ErrorJSON(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, Response{
		Success: false,
		Data:    nil,
		Message: message,
	})
}
