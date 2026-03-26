package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Task struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	UserID   string `gorm:"column:user_id" json:"user_id"`
	FileName string `gorm:"column:file_name" json:"file_name"`
	Status   string `gorm:"column:status" json:"status"`
}

func (Task) TableName() string {
	return "analysis_tasks"
}

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db"), &gorm.Config{})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	var tasks []Task
	db.Table("analysis_tasks").Order("id desc").Limit(10).Find(&tasks)

	fmt.Println("Recent tasks:")
	for _, t := range tasks {
		fmt.Printf("  ID: %d, UserID: %q, File: %s, Status: %s\n", t.ID, t.UserID, t.FileName, t.Status)
	}
}
