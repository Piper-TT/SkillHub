package main

import (
	"fmt"
	"log"
	"gorm.io/gorm"
	"github.com/glebarez/sqlite"
)

type AnalysisTask struct {
	ID           uint   `gorm:"primaryKey"`
	UserID       string `gorm:"column:user_id"`
	FileName     string `gorm:"column:file_name"`
	Status       string `gorm:"column:status"`
	ErrorMessage string `gorm:"column:error_message"`
}

func (AnalysisTask) TableName() string {
	return "analysis_tasks"
}

func main() {
	db, err := gorm.Open(sqlite.Open("./skills.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	var tasks []AnalysisTask
	if err := db.Order("id desc").Limit(10).Find(&tasks).Error; err != nil {
		log.Fatal(err)
	}

	fmt.Println("Recent analysis tasks:")
	fmt.Println("ID\tUserID\t\t\tStatus\t\tFileName\t\tErrorMsg")
	fmt.Println("================================================================================")
	for _, t := range tasks {
		errMsg := t.ErrorMessage
		if len(errMsg) > 30 {
			errMsg = errMsg[:30] + "..."
		}
		fmt.Printf("%d\t%s\t%s\t%s\t%s\n", t.ID, t.UserID, t.Status, t.FileName, errMsg)
	}
}
