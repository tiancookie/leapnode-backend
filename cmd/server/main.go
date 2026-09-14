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

	// /api/dist/* 契约需与 SubRouter 前端 100% 兼容, 在 internal/controllers 中逐步实现
	dist := router.Group("/api/dist")
	{
		dist.GET("/site/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "not implemented"})
		})
	}

	srv := &http.Server{
		Addr:    httpAddr,
		Handler: router,
	}

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
	
	return nil
}
