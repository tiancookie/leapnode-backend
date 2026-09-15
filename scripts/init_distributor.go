package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_URL") // Railway 自动注入
	if dsn == "" {
		dsn = os.Getenv("LEAPNODE_PG_DSN") // 备用
	}
	if dsn == "" {
		log.Fatal("DATABASE_URL or LEAPNODE_PG_DSN not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	userID := 24 // dist_owner_b7

	// 插入分站站点
	var siteID int
	err = db.QueryRow(`
		INSERT INTO distributor_sites (site_name, site_domain, user_id, status, commission_rate)
		VALUES ('批7测试站', 'b7test.leapnode.com', $1, 1, 30)
		ON CONFLICT (user_id) DO UPDATE SET site_name='批7测试站'
		RETURNING id
	`, userID).Scan(&siteID)
	
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✅ 分站创建成功: site_id=%d, owner_user_id=%d\n", siteID, userID)
	fmt.Println("可以用 dist_owner_b7 / Test@123456 登录测试")
}
