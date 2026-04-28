package main

import (
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 先备份数据库
	src, err := os.ReadFile("skills.db")
	if err != nil {
		panic(err)
	}
	os.WriteFile("skills.db.bak", src, 0644)
	fmt.Println("Backup created: skills.db.bak")

	// 尝试打开并修复
	db, err := gorm.Open(sqlite.Open("skills.db?mode=rwc&_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// 执行 integrity check
	var result string
	db.Raw("PRAGMA integrity_check").Scan(&result)
	fmt.Println("Integrity check:", result)

	// 重建数据库
	db.Exec("VACUUM")
	fmt.Println("VACUUM completed")
}
