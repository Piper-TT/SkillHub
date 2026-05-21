package main

import (
	"fmt"
	"skillhub/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("./skills.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	result := db.Model(&models.Agent{}).Where("slug = ?", "crash-dump-analyzer").Update("redirect_url", "http://10.50.6.49:9090/")
	if result.Error != nil {
		panic(result.Error)
	}
	fmt.Printf("Updated %d row(s)\n", result.RowsAffected)
}
