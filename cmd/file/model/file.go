package model

import (
	"time"

	"gorm.io/gorm"
)

// FileCategory 文件业务分类
type FileCategory string

const (
	FileCategoryAvatar FileCategory = "avatar" // 用户头像
	FileCategoryPost   FileCategory = "post"   // 帖子图片
	FileCategoryTask   FileCategory = "task"   // 任务图片
	FileCategoryOther  FileCategory = "other"  // 其他
)

// File 文件元数据。
// 存储在独立数据库 campus_file 的 files 表。
type File struct {
	ID          int64          `gorm:"primaryKey;autoIncrement:false"          json:"id"`
	SchoolID    int64          `gorm:"column:school_id;index;not null"         json:"school_id"`
	UploaderID  int64          `gorm:"column:uploader_id;not null"            json:"uploader_id"`
	Category    string         `gorm:"size:32;default:'other'"                json:"category"`
	StorageKey  string         `gorm:"column:storage_key;size:255;not null"   json:"storage_key"`
	URL         string         `gorm:"size:512;not null"                      json:"url"`
	ContentType string         `gorm:"column:content_type;size:64;not null"   json:"content_type"`
	SizeBytes   int64          `gorm:"column:size_bytes;not null"             json:"size_bytes"`
	SHA256      string         `gorm:"column:sha256;size:64;uniqueIndex;not null" json:"sha256"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index:idx_cleanup"                      json:"-"`
}

func (File) TableName() string { return "files" }