package main

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	db, err := gorm.Open(sqlite.Open("./skills.db"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Println(err)
		return
	}
	result := db.Exec("DELETE FROM servers WHERE name = ?", "test-service")
	fmt.Printf("Deleted %d rows\n", result.RowsAffected)
}
