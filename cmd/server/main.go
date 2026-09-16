package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tiancookie/leapnode-backend/internal/controllers"
	"github.com/tiancookie/leapnode-backend/internal/middleware"
	"github.com/tiancookie/leapnode-backend/internal/services"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// LeapNode 后端服务入口
//
// 职责:
//   - 四层用户管理 / 商家管理 / 模型审批 / 套餐订阅 / 返佣计算 / 充值 / 订阅共享
//   - 与 New-API 共享 PostgreSQL, 通过扩展字段与 18 张扩展表协作
//
// 环境变量 (可被 config/config.yaml 覆盖):
//   LEAPNODE_HTTP_ADDR    默认 :8080
//   LEAPNODE_PG_DSN       PostgreSQL DSN
//   LEAPNODE_REDIS_ADDR   默认 127.0.0.1:6379

func main() {
	httpAddr := getenv("LEAPNODE_HTTP_ADDR", ":8080")
	pgDSN := getenv("LEAPNODE_PG_DSN", "host=127.0.0.1 user=leapnode password=leapnode dbname=leapnode port=5432 sslmode=disable TimeZone=UTC")
	redisAddr := getenv("LEAPNODE_REDIS_ADDR", "127.0.0.1:6379")

	// --- PostgreSQL (GORM) ---
	db, err := gorm.Open(postgres.Open(pgDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to ping postgres: %v", err)
	}
	log.Println("postgres connected")

	// --- Auto Migration: 执行 DDL ---
	if err := runMigrations(sqlDB); err != nil {
		log.Printf("warning: migration failed (may already exist): %v", err)
	} else {
		log.Println("migrations completed")
	}

	// --- Redis ---
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("warning: redis ping failed: %v", err)
	} else {
		log.Println("redis connected")
	}

	// --- HTTP (Gin) ---
	router := gin.Default()

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "leapnode-backend"})
	})

	// --- New-API HTTP 客户端 (用于调用 New-API 管理接口，避免直接写 users 表) ---
	newAPIClient := services.NewNewAPIClient()

	// /api/dist/* 契约需与 SubRouter 前端 100% 兼容, 在 internal/controllers 中逐步实现
	siteService := services.NewSiteService(db)
	siteController := controllers.NewSiteController(siteService)

	userService := services.NewUserService(db, newAPIClient)
	userController := controllers.NewUserController(userService)

	tokenService := services.NewTokenService(db)
	tokenController := controllers.NewTokenController(tokenService, siteService)

	topupService := services.NewTopupService(db, newAPIClient)
	topupController := controllers.NewTopupController(topupService)

	affService := services.NewAffService(db, newAPIClient)
	affController := controllers.NewAffController(affService)

	adminService := services.NewAdminService(db, newAPIClient)
	adminController := controllers.NewAdminController(adminService)

	merchantService := services.NewMerchantService(db, newAPIClient)
	merchantController := controllers.NewMerchantController(merchantService)

	distributorService := services.NewDistributorService(db, newAPIClient)
	distributorController := controllers.NewDistributorController(distributorService)

	packageService := services.NewPackageService(db)
	packageController := controllers.NewPackageController(packageService)

	channelGroupService := services.NewChannelGroupService(db)
	channelGroupController := controllers.NewChannelGroupController(channelGroupService)

	dist := router.Group("/api/dist")
	{
		siteGroup := dist.Group("/site")
		siteController.RegisterRoutes(siteGroup)

		// 用户认证: 公开路由 (注册/登录/登出)
		userGroup := dist.Group("/user")
		userController.RegisterPublicRoutes(userGroup)

		// 用户账户: 受保护路由 (self/password/language)
		// 走 AuthRequired: New-Api-User 头 + Session Cookie 双重校验
		userProtected := dist.Group("/user")
		userProtected.Use(middleware.AuthRequired(db))
		userController.RegisterProtectedRoutes(userProtected)

		// 令牌管理: 受保护路由 (list/create/update/delete/models)
		// 全部走 AuthRequired, 服务层强制 WHERE user_id = 当前用户 防越权。
		tokenProtected := dist.Group("/token")
		tokenProtected.Use(middleware.AuthRequired(db))
		tokenController.RegisterProtectedRoutes(tokenProtected)

		// 充值管理: 公开路由 (info) + 受保护路由 (redeem/history)
		topupGroup := dist.Group("/topup")
		topupController.RegisterPublicRoutes(topupGroup)

		topupProtected := dist.Group("/topup")
		topupProtected.Use(middleware.AuthRequired(db))
		topupController.RegisterProtectedRoutes(topupProtected)

		// 返佣系统: 受保护路由 (aff/kol)
		affProtected := dist.Group("")
		affProtected.Use(middleware.AuthRequired(db))
		{
			affProtected.GET("/aff", affController.GetAffCode)
			affProtected.GET("/aff_earnings", affController.GetAffEarnings)
			affProtected.POST("/aff_transfer", affController.TransferAffQuota)
			affProtected.POST("/aff_withdraw", affController.RequestWithdraw)
			affProtected.GET("/aff_payouts", affController.GetAffPayouts)
			affProtected.GET("/aff/invitees", affController.GetInvitees)
			affProtected.POST("/kol_apply", affController.ApplyKOL)
			affProtected.GET("/kol_status", affController.GetKOLStatus)
		}

		// 套餐管理: 公开路由 (site/packages) + 受保护路由 (subscribe/subscription/active)
		packageController.RegisterPublicRoutes(siteGroup)

		packageProtected := dist.Group("")
		packageProtected.Use(middleware.AuthRequired(db))
		packageController.RegisterProtectedRoutes(packageProtected)

		// 套餐管理后台: 受保护路由 (admin/packages CRUD)
		adminPackages := dist.Group("/admin")
		adminPackages.Use(middleware.AuthRequired(db))
		packageController.RegisterAdminRoutes(adminPackages)

		// 渠道分组管理: 受保护路由 (admin/channel-groups CRUD)
		channelGroupController.RegisterRoutes(adminPackages)
	}

	// 管理员后台: /api/admin/* (需要 AuthRequired + AdminAuth)
	admin := router.Group("/api/admin")
	admin.Use(middleware.AuthRequired(db))
	admin.Use(middleware.AdminAuth())
	{
		adminController.RegisterRoutes(admin)
	}

	// 商家中心: /api/merchant/* (需要 AuthRequired + MerchantAuth)
	merchant := router.Group("/api/merchant")
	merchant.Use(middleware.AuthRequired(db))
	merchant.Use(middleware.MerchantAuth())
	{
		merchantController.RegisterRoutes(merchant)
	}

	// 分站管理: /api/distributor/* (需要 AuthRequired + DistributorAuth)
	distributor := router.Group("/api/distributor")
	distributor.Use(middleware.AuthRequired(db))
	distributor.Use(middleware.DistributorAuth())
	{
		distributorController.RegisterRoutes(distributor)
	}

	srv := &http.Server{
		Addr:    httpAddr,
		Handler: router,
	}

	// 临时：初始化分站（POST /bootstrap/init-dist?user_id=24）
	router.POST("/bootstrap/init-dist", func(c *gin.Context) {
		userID := c.Query("user_id")
		if userID == "" {
			c.JSON(400, gin.H{"error": "user_id required"})
			return
		}
		
		// 1. 升级用户为分站站长（user_level=3）
		_, err := sqlDB.Exec(`UPDATE users SET user_level = 3 WHERE id = $1`, userID)
		if err != nil {
			c.JSON(500, gin.H{"error": "update user_level failed: " + err.Error()})
			return
		}
		
		// 2. 查询是否已存在分站
		var existingSiteID int
		err = sqlDB.QueryRow(`SELECT id FROM distributor_sites WHERE owner_id = $1`, userID).Scan(&existingSiteID)
		if err == nil {
			c.JSON(200, gin.H{"success": true, "site_id": existingSiteID, "user_id": userID, "message": "已存在，user_level已更新为3"})
			return
		}
		
		// 3. 不存在则创建分站（slug 带 user_id 保证唯一）
		var siteID int
		slug := "site" + userID
		siteName := "测试分站" + userID
		err = sqlDB.QueryRow(`
			INSERT INTO distributor_sites (owner_id, slug, name, status, global_markup_ratio)
			VALUES ($1, $2, $3, 1, 30.00)
			RETURNING id
		`, userID, slug, siteName).Scan(&siteID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"success": true, "site_id": siteID, "user_id": userID, "user_level": 3})
	})

	// 临时：初始化商家（POST /bootstrap/init-merchant?user_id=31）
	router.POST("/bootstrap/init-merchant", func(c *gin.Context) {
		userID := c.Query("user_id")
		if userID == "" {
			c.JSON(400, gin.H{"error": "user_id required"})
			return
		}
		// 升级 user_level=2
		if _, err := sqlDB.Exec(`UPDATE users SET user_level = 2 WHERE id = $1`, userID); err != nil {
			c.JSON(500, gin.H{"error": "update user_level failed: " + err.Error()})
			return
		}
		// 已存在则返回
		var existingID int
		if err := sqlDB.QueryRow(`SELECT id FROM merchants WHERE user_id = $1`, userID).Scan(&existingID); err == nil {
			c.JSON(200, gin.H{"success": true, "merchant_id": existingID, "user_id": userID, "message": "已存在，user_level已更新为2"})
			return
		}
		// 建 merchant 记录
		var mid int
		handle := "m" + userID
		mname := "测试商家" + userID
		err := sqlDB.QueryRow(`
			INSERT INTO merchants (user_id, merchant_name, merchant_handle, merchant_level, commission_rate, status, created_at)
			VALUES ($1, $2, $3, 'gold', 0.30, 1, NOW())
			RETURNING id
		`, userID, mname, handle).Scan(&mid)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// 回写 users.merchant_id
		_, _ = sqlDB.Exec(`UPDATE users SET merchant_id = $1 WHERE id = $2`, mid, userID)
		c.JSON(200, gin.H{"success": true, "merchant_id": mid, "user_id": userID, "user_level": 2})
	})

	go func() {
		log.Printf("leapnode-backend listening on %s", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// --- 优雅关闭 ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	_ = rdb.Close()
	_ = sqlDB.Close()
	fmt.Println("server exited")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// runMigrations 执行数据库迁移
func runMigrations(db *sql.DB) error {
	sqlBytes, err := os.ReadFile("migrations/001_initial_schema.sql")
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}
	
	_, err = db.Exec(string(sqlBytes))
	if err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}
	
	// 补充 description 列（旧表可能缺失）
	_, err = db.Exec("ALTER TABLE packages ADD COLUMN IF NOT EXISTS description TEXT;")
	if err != nil {
		log.Printf("warning: failed to add description column (may already exist): %v", err)
	}
	
	return nil
}
