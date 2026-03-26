package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db"), &gorm.Config{})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// 更新所有 API Keys 的 user_id 为 default
	result := db.Exec("UPDATE user_api_keys SET user_id = 'default'")
	if result.Error != nil {
		fmt.Println("Error updating API keys:", result.Error)
		return
	}
	fmt.Printf("Updated %d API keys to 'default' user\n", result.RowsAffected)

	// 显示更新后的结果
	var keys []struct {
		UserID   string
		Provider string
	}
	db.Raw("SELECT user_id, provider FROM user_api_keys").Scan(&keys)
	fmt.Println("\nCurrent API keys:")
	for _, k := range keys {
		fmt.Printf("  UserID: %q, Provider: %s\n", k.UserID, k.Provider)
	}
}
