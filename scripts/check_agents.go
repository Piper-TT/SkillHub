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

	var count int64
	db.Table("agents").Where("deleted_at IS NULL").Count(&count)
	fmt.Printf("Agents count: %d\n", count)

	type Agent struct {
		ID   uint
		Name string
		Slug string
	}
	var agents []Agent
	db.Table("agents").Where("deleted_at IS NULL").Find(&agents)
	for _, a := range agents {
		fmt.Printf("  ID=%d, Name=%s, Slug=%s\n", a.ID, a.Name, a.Slug)
	}
}
