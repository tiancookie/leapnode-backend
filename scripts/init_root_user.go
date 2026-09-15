package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	// 升级 leapnode_admin 为 root (role=100)
	result := db.Exec("UPDATE users SET role = 100 WHERE username = 'leapnode_admin'")
	if result.Error != nil {
		log.Fatal("Update failed:", result.Error)
	}

	fmt.Printf("✅ Updated %d rows. leapnode_admin is now root (role=100)\n", result.RowsAffected)

	// 查询确认
	var user struct {
		ID       int
		Username string
		Role     int
	}
	db.Raw("SELECT id, username, role FROM users WHERE username = 'leapnode_admin'").Scan(&user)
	fmt.Printf("Confirmed: ID=%d Username=%s Role=%d\n", user.ID, user.Username, user.Role)
}
