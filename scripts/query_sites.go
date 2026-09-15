package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("LEAPNODE_PG_DSN")
	if dsn == "" {
		log.Fatal("LEAPNODE_PG_DSN not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, site_name, user_id, status FROM distributor_sites ORDER BY id LIMIT 5")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("已有分站站点:")
	hasData := false
	for rows.Next() {
		hasData = true
		var id, userID, status int
		var siteName string
		if err := rows.Scan(&id, &siteName, &userID, &status); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  site_id=%d, site_name=%s, owner_user_id=%d, status=%d\n", id, siteName, userID, status)

		// 查 owner 用户名
		var username string
		err := db.QueryRow("SELECT username FROM users WHERE id=$1", userID).Scan(&username)
		if err == nil {
			fmt.Printf("    → owner username: %s\n", username)
		}
	}

	if !hasData {
		fmt.Println("  (无分站站点，需先创建)")
	}
}
