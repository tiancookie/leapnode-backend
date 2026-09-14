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

// UserController 处理 /api/dist/user/* 认证与账户接口。
type UserController struct {
	userService *services.UserService
}

// NewUserController 创建 UserController。
func NewUserController(userService *services.UserService) *UserController {
	return &UserController{userService: userService}
}

// RegisterPublicRoutes 注册无需认证的路由 (注册/登录/登出)。
// 路由前缀为 /api/dist/user。
func (ctrl *UserController) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.POST("/register", ctrl.Register)
	group.POST("/login", ctrl.Login)
	group.POST("/logout", ctrl.Logout)
}

// RegisterProtectedRoutes 注册需要认证的路由 (self/password/language)。
// 调用方需已对 group 应用 middleware.AuthRequired。
func (ctrl *UserController) RegisterProtectedRoutes(group *gin.RouterGroup) {
	group.GET("/self", ctrl.Self)
	group.PUT("/password", ctrl.ChangePassword)
	group.PUT("/language", ctrl.UpdateLanguage)
}

// Register POST /api/dist/user/register
//
// 前端: Register.jsx handleSubmit -> api.register(data)
// 参数: username / password / email? / verification_code? / aff_code?
// 注意: 注册成功不建立 session (AuthContext.register 注释: needsLogin)。
func (ctrl *UserController) Register(c *gin.Context) {
	var in services.RegisterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := ctrl.userService.Register(in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrValidation):
			utils.ErrorJSON(c, http.StatusBadRequest, "invalid username, email, or password (password must be 8-20 chars)")
		case errors.Is(err, services.ErrUsernameTaken):
			utils.ErrorJSON(c, http.StatusConflict, "username already exists")
		case errors.Is(err, services.ErrEmailTaken):
			utils.ErrorJSON(c, http.StatusConflict, "email already registered")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "registration failed")
		}
		return
	}
	// 与前端契约一致: 仅返回成功, 用户需另行登录。
	utils.SuccessJSON(c, ctrl.userService.ToResponse(user))
}

// loginRequest 登录请求体 (对齐 Login.jsx form)。
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login POST /api/dist/user/login
//
// 前端: AuthContext.login -> api.login({username,password})
// 成功后: set session cookie + 返回 user 对象 (含 id, 前端存 dist_user_id)。
func (ctrl *UserController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := ctrl.userService.Login(req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredential):
			utils.ErrorJSON(c, http.StatusUnauthorized, "invalid username or password")
		case errors.Is(err, services.ErrUserDisabled):
			utils.ErrorJSON(c, http.StatusForbidden, "account disabled")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "login failed")
		}
		return
	}
	utils.SetSession(c, user.ID)
	utils.SuccessJSON(c, ctrl.userService.ToResponse(user))
}

// Logout POST /api/dist/user/logout
//
// 前端: AuthContext.logout -> api.logout() (无认证要求, 幂等清 session)。
func (ctrl *UserController) Logout(c *gin.Context) {
	utils.ClearSession(c)
	utils.SuccessJSON(c, gin.H{})
}

// Self GET /api/dist/user/self (受保护)
//
// 前端: AuthContext.refreshUser / 挂载时恢复会话 -> api.getUserSelf()
func (ctrl *UserController) Self(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	utils.SuccessJSON(c, ctrl.userService.ToResponse(user))
}

// changePasswordRequest 对齐 Account.jsx updateUserPassword({original_password, password})。
type changePasswordRequest struct {
	OriginalPassword string `json:"original_password"`
	Password         string `json:"password"`
}

// ChangePassword PUT /api/dist/user/password (受保护)
func (ctrl *UserController) ChangePassword(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}
	err := ctrl.userService.ChangePassword(userID, req.OriginalPassword, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrValidation):
			utils.ErrorJSON(c, http.StatusBadRequest, "new password must be 8-20 characters")
		case errors.Is(err, services.ErrWrongPassword):
			utils.ErrorJSON(c, http.StatusBadRequest, "current password is incorrect")
		case errors.Is(err, services.ErrUserNotFound):
			utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		default:
			utils.ErrorJSON(c, http.StatusInternalServerError, "failed to update password")
		}
		return
	}
	utils.SuccessJSON(c, gin.H{})
}

// updateLanguageRequest 对齐 api.updateUserLanguage({language})。
type updateLanguageRequest struct {
	Language string `json:"language"`
}

// UpdateLanguage PUT /api/dist/user/language (受保护)
func (ctrl *UserController) UpdateLanguage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		utils.ErrorJSON(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req updateLanguageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorJSON(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := ctrl.userService.UpdateLanguage(userID, req.Language); err != nil {
		if errors.Is(err, services.ErrValidation) {
			utils.ErrorJSON(c, http.StatusBadRequest, "invalid language")
			return
		}
		utils.ErrorJSON(c, http.StatusInternalServerError, "failed to update language")
		return
	}
	utils.SuccessJSON(c, gin.H{})
}
