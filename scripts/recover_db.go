package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db?mode=ro"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// 尝试读取各表数据
	tables := []string{"agents", "skills", "mcp_servers", "sessions", "user_api_keys", "analysis_tasks"}
	for _, table := range tables {
		var count int64
		db.Table(table).Count(&count)
		fmt.Printf("%-20s: %d rows\n", table, count)
	}

	// 尝试读取 agents
	type Agent struct {
		ID   uint
		Name string
		Slug string
	}
	var agents []Agent
	if err := db.Table("agents").Where("deleted_at IS NULL").Find(&agents).Error; err != nil {
		fmt.Printf("Error reading agents: %v\n", err)
	} else {
		for _, a := range agents {
			fmt.Printf("  Agent: ID=%d, Name=%s, Slug=%s\n", a.ID, a.Name, a.Slug)
		}
	}
}
