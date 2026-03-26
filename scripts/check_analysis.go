package main

import (
	"fmt"
	"strings"

	"skillhub/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Printf("Failed to connect database: %v\n", err)
		return
	}

	var task models.AnalysisTask
	if err := db.First(&task, 19).Error; err != nil {
		fmt.Printf("Failed to find task: %v\n", err)
		return
	}

	fmt.Println("Task 19 Details:")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Status: %s\n", task.Status)
	fmt.Printf("Progress: %d%%\n", task.Progress)
	fmt.Printf("Report PDF: %q\n", task.ReportPDF)
	fmt.Printf("Report MD length: %d\n", len(task.ReportMD))
	fmt.Printf("Error: %s\n", task.ErrorMessage)
}
