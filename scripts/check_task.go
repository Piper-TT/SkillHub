package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db"), &gorm.Config{})
	if err != nil {
		fmt.Printf("连接数据库失败: %v\n", err)
		return
	}

	var tasks []struct {
		ID        uint
		FileName  string
		UserID    string
		Status    string
		ReportPDF string
	}

	db.Raw(`SELECT id, file_name, user_id, status, report_pdf
		FROM analysis_tasks
		ORDER BY id DESC
		LIMIT 10`).Scan(&tasks)

	fmt.Println("分析任务:")
	fmt.Println("==========================================")
	for _, t := range tasks {
		fmt.Printf("ID: %d, User: %s\n", t.ID, t.UserID)
		fmt.Printf("文件: %s, 状态: %s\n", t.FileName, t.Status)
		fmt.Printf("PDF: %s\n", t.ReportPDF)
		fmt.Printf("PDF是否为空: %v\n", t.ReportPDF == "")
		fmt.Println("------------------------------------------")
	}
}
