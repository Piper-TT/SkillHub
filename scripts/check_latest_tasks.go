package main

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Task struct {
	ID           uint      `gorm:"primaryKey"`
	FileName     string    `gorm:"column:file_name"`
	Status       string    `gorm:"column:status"`
	Progress     int       `gorm:"column:progress"`
	FileSize     int64     `gorm:"column:file_size"`
	ErrorMessage string    `gorm:"column:error_message"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (Task) TableName() string { return "analysis_tasks" }

func main() {
	db, err := gorm.Open(sqlite.Open("./skills.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	var tasks []Task
	db.Order("id desc").Limit(5).Find(&tasks)
	for _, t := range tasks {
		errMsg := t.ErrorMessage
		if len(errMsg) > 60 {
			errMsg = errMsg[:60] + "..."
		}
		fmt.Printf("ID:%d  File:%s  Status:%s  Progress:%d%%  Size:%d  Created:%s  Updated:%s  Err:%s\n",
			t.ID, t.FileName, t.Status, t.Progress, t.FileSize,
			t.CreatedAt.Format("01-02 15:04:05"), t.UpdatedAt.Format("01-04:05"), errMsg)
	}
}
