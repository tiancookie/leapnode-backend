// Package utils 提供通用工具函数。
package utils

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
