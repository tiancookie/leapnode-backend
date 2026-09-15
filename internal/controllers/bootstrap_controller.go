// Package controllers - bootstrap 一次性初始化端点。
package controllers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/tiancookie/leapnode-backend/internal/utils"
	"gorm.io/gorm"
)

// BootstrapController 处理一次性初始化操作（受 BOOTSTRAP_SECRET 保护）。
type BootstrapController struct {
	db *gorm.DB
}

// NewBootstrapController 创建 BootstrapController。
func NewBootstrapController(db *gorm.DB) *BootstrapController {
	return &BootstrapController{db: db}
}

// RegisterRoutes 注册 bootstrap 路由。
func (ctrl *BootstrapController) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/promote-root", ctrl.PromoteRoot)
}

// PromoteRoot 将指定用户升级为 root (role=100)。
// 受 BOOTSTRAP_SECRET 环境变量保护，用完应删除此端点。
// 请求: POST /bootstrap/promote-root?secret=xxx&username=leapnode_admin
func (ctrl *BootstrapController) PromoteRoot(c *gin.Context) {
	secret := c.Query("secret")
	expectedSecret := os.Getenv("BOOTSTRAP_SECRET")
	if expectedSecret == "" || secret != expectedSecret {
		utils.ErrorJSON(c, http.StatusForbidden, "invalid bootstrap secret")
		return
	}

	username := c.Query("username")
	if username == "" {
		username = "leapnode_admin"
	}

	result := ctrl.db.Exec("UPDATE users SET role = 100 WHERE username = ?", username)
	if result.Error != nil {
		utils.ErrorJSON(c, http.StatusInternalServerError, result.Error.Error())
		return
	}

	// 查询确认
	var user struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Role     int    `json:"role"`
		Quota    int64  `json:"quota"`
	}
	ctrl.db.Raw("SELECT id, username, role, quota FROM users WHERE username = ?", username).Scan(&user)

	utils.SuccessJSON(c, gin.H{
		"rows_affected": result.RowsAffected,
		"user":          user,
	})
}
