package main

import (
	"fmt"
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Skill struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string
	Slug       string
	FileName   string
	Downloads  int
	Description string
	Category   string
	Icon       string
}

func (Skill) TableName() string {
	return "skills"
}

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	var total int64
	db.Model(&Skill{}).Count(&total)
	fmt.Printf("Total skills: %d\n", total)

	var localCount int64
	db.Model(&Skill{}).Where("file_name != '' AND file_name IS NOT NULL").Count(&localCount)
	fmt.Printf("Local uploads (file_name not empty): %d\n", localCount)

	var clawhubCount int64
	db.Model(&Skill{}).Where("file_name = '' OR file_name IS NULL").Count(&clawhubCount)
	fmt.Printf("ClawHub crawled (file_name empty/null): %d\n", clawhubCount)

	fmt.Println("\n--- Sample rows (name, slug, file_name, downloads) ---")
	var samples []Skill
	db.Limit(5).Find(&samples)
	for i, s := range samples {
		fn := s.FileName
		if fn == "" {
			fn = "(empty)"
		}
		fmt.Printf("%d. %-40s  %-30s  file=%-20s  downloads=%d\n", i+1, s.Name, s.Slug, fn, s.Downloads)
	}

	fmt.Println("\n--- Sample local uploads ---")
	var locals []Skill
	db.Where("file_name != '' AND file_name IS NOT NULL").Limit(5).Find(&locals)
	for i, s := range locals {
		fmt.Printf("%d. %-40s  file=%s\n", i+1, s.Name, s.FileName)
	}
}
