package repo

import (
	"errors"
	"fmt"
	"log"
	"time"

	"go_projects/praProject1/cmd/file/model"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/snowflake"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("记录不存在")
var ErrForbidden = errors.New("无权操作")

func mustFileDB() *gorm.DB {
	d, err := db.GetFileDB()
	if err != nil {
		log.Fatalf("[file-repo] 未初始化 file db: %v", err)
	}
	return d
}

// ─── CRUD ──────────────────────────────────────────────────────────────────

// Create 创建文件记录（不去重，调用方先检查 SHA-256）。
func Create(f *model.File) error {
	f.ID = snowflake.GenerateID()
	return mustFileDB().Create(f).Error
}

// FindBySHA256 通过 SHA-256 查找已存在的文件（用于去重）。
func FindBySHA256(sha256 string) (*model.File, error) {
	var f model.File
	err := mustFileDB().Where("sha256 = ?", sha256).First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &f, nil
}

// GetByID 根据 ID 获取文件。
func GetByID(id int64) (*model.File, error) {
	var f model.File
	err := mustFileDB().First(&f, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &f, nil
}

// SoftDelete 软删除文件（仅上传者可操作）。
func SoftDelete(id, uploaderID int64) error {
	result := mustFileDB().Where("id = ? AND uploader_id = ?", id, uploaderID).
		Delete(&model.File{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrForbidden
	}
	return nil
}

// CleanupBefore 物理删除 30 天前已软删除的文件。
func CleanupBefore(before time.Time) (int64, error) {
	result := mustFileDB().Unscoped().
		Where("deleted_at IS NOT NULL AND created_at < ?", before).
		Delete(&model.File{})
	if result.Error != nil {
		return 0, fmt.Errorf("清理文件: %w", result.Error)
	}
	return result.RowsAffected, nil
}