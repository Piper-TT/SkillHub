package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type UserAPIKey struct {
	ID        uint   `gorm:"primarykey"`
	UserID    string `gorm:"column:user_id"`
	Provider  string `gorm:"column:provider"`
	BaseModel string `gorm:"column:base_model"`
}

func (UserAPIKey) TableName() string {
	return "user_api_keys"
}

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db"), &gorm.Config{})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	var keys []UserAPIKey
	db.Table("user_api_keys").Find(&keys)

	fmt.Println("API Keys in database:")
	for _, k := range keys {
		fmt.Printf("  UserID: %q, Provider: %s, Model: %s\n", k.UserID, k.Provider, k.BaseModel)
	}

	if len(keys) == 0 {
		fmt.Println("  (No API keys found)")
	}
}
