package models

import (
	"encoding/json"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Part holds OBS part number and ETag for multipart upload completion.
type Part struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

// UploadSession tracks an in-progress or completed multipart upload.
type UploadSession struct {
	gorm.Model
	FileID          string `gorm:"uniqueIndex;not null"`
	FileName        string `gorm:"not null"`
	FileSize        int64  `gorm:"not null"`
	TotalChunks     int    `gorm:"not null"`
	UploadedChunks  int    `gorm:"default:0"`
	ObjectKey       string `gorm:"not null"`
	OBSUploadID     string `gorm:"not null"` // OBS multipart upload ID
	PartsJSON       string `gorm:"type:text;default:'[]'"`
	Status          string `gorm:"default:'uploading'"` // uploading | completed | failed
	URL             string
}

func (s *UploadSession) GetParts() []Part {
	var parts []Part
	_ = json.Unmarshal([]byte(s.PartsJSON), &parts)
	return parts
}

func (s *UploadSession) SetParts(parts []Part) {
	b, _ := json.Marshal(parts)
	s.PartsJSON = string(b)
}

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("videos.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	if err := DB.AutoMigrate(&UploadSession{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	log.Println("database initialized: videos.db")
}
