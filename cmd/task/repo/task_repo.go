package repo

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"go_projects/praProject1/cmd/task/model"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/snowflake"

	"gorm.io/gorm"
)

func mustTaskDB() *gorm.DB {
	d, err := db.GetTaskDB()
	if err != nil {
		log.Fatalf("[task-repo] 未初始化 task db: %v", err)
	}
	return d
}

var ErrNotFound = errors.New("记录不存在")
var ErrForbidden = errors.New("无权操作")
var ErrInvalidStatus = errors.New("当前状态不允许该操作")

// ─── 游标分页 ──────────────────────────────────────────────────────────────

type Cursor struct {
	ID        int64 `json:"id"`
	CreatedAt int64 `json:"created_at"`
}

func EncodeCursor(c Cursor) string {
	if c.ID == 0 {
		return ""
	}
	data, _ := json.Marshal(c)
	return base64.StdEncoding.EncodeToString(data)
}

func DecodeCursor(encoded string) (Cursor, error) {
	if encoded == "" {
		return Cursor{}, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("游标解码失败: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return Cursor{}, fmt.Errorf("游标 JSON 解析失败: %w", err)
	}
	return c, nil
}

// ─── CRUD ──────────────────────────────────────────────────────────────────

func Create(task *model.Task) error {
	task.ID = snowflake.GenerateID()
	return mustTaskDB().Create(task).Error
}

func GetByID(schoolID, taskID int64) (*model.Task, error) {
	var t model.Task
	err := mustTaskDB().Where("school_id = ? AND id = ?", schoolID, taskID).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func List(schoolID int64, cursorStr string, pageSize int, taskType, status string) ([]model.Task, bool, string, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	dbQuery := mustTaskDB().Where("school_id = ?", schoolID)

	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	} else {
		dbQuery = dbQuery.Where("status = ?", model.TaskStatusOpen) // 默认仅显示待接单
	}
	if taskType != "" {
		dbQuery = dbQuery.Where("task_type = ?", taskType)
	}

	cursor, err := DecodeCursor(cursorStr)
	if err != nil {
		return nil, false, "", fmt.Errorf("无效游标: %w", err)
	}
	if cursor.ID > 0 {
		dbQuery = dbQuery.Where("(created_at < ? OR (created_at = ? AND id < ?))",
			time.Unix(cursor.CreatedAt, 0), time.Unix(cursor.CreatedAt, 0), cursor.ID)
	}

	var tasks []model.Task
	if err := dbQuery.Order("CASE WHEN status = 1 THEN 0 ELSE 1 END, expired_at ASC, created_at DESC").
		Limit(pageSize + 1).Find(&tasks).Error; err != nil {
		return nil, false, "", err
	}

	hasMore := len(tasks) > pageSize
	if hasMore {
		tasks = tasks[:pageSize]
	}

	nextCursor := ""
	if hasMore && len(tasks) > 0 {
		last := tasks[len(tasks)-1]
		nextCursor = EncodeCursor(Cursor{ID: last.ID, CreatedAt: last.CreatedAt.Unix()})
	}

	return tasks, hasMore, nextCursor, nil
}

func Update(schoolID, userID, taskID int64, updates map[string]interface{}) error {
	result := mustTaskDB().Model(&model.Task{}).
		Where("school_id = ? AND id = ? AND user_id = ?", schoolID, taskID, userID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func SoftDelete(schoolID, userID, taskID int64) error {
	result := mustTaskDB().Where("school_id = ? AND id = ? AND user_id = ?", schoolID, taskID, userID).
		Delete(&model.Task{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ─── 接单操作 ──────────────────────────────────────────────────────────────

// Claim 接单（先到先得，CAS 乐观锁）。
// 仅 open 状态、非本人可接单。
func Claim(schoolID, taskID, claimantID int64, contact, message string) (claimantUserID int64, err error) {
	err = mustTaskDB().Transaction(func(tx *gorm.DB) error {
		var t model.Task
		if err := tx.Where("school_id = ? AND id = ?", schoolID, taskID).
			First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if t.UserID == claimantID {
			return ErrForbidden
		}
		if t.Status != model.TaskStatusOpen {
			return ErrInvalidStatus
		}

		// CAS: 仅 claimant_id=0 时可接单
		result := tx.Model(&model.Task{}).
			Where("id = ? AND claimant_id = 0 AND status = ?", taskID, model.TaskStatusOpen).
			Updates(map[string]interface{}{
				"claimant_id":      claimantID,
				"claimant_contact": contact,
				"claimant_msg":     message,
				"status":           model.TaskStatusInProgress,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrInvalidStatus // 已被他人接单或状态变更
		}

		claimantUserID = t.UserID // 返回发布者 user_id（用于后续查询联系方式）
		return nil
	})
	return claimantUserID, err
}

// Complete 完成任务（仅接单者，in_progress → completed）。
func Complete(schoolID, taskID, userID int64) error {
	return transitionStatus(schoolID, taskID, userID, model.TaskStatusInProgress, model.TaskStatusCompleted)
}

// Cancel 取消任务。
// 发布者可取消 open 或 in_progress；接单者可取消 in_progress。
func Cancel(schoolID, taskID, userID int64) error {
	return mustTaskDB().Transaction(func(tx *gorm.DB) error {
		var t model.Task
		if err := tx.Where("school_id = ? AND id = ?", schoolID, taskID).
			First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		// 校验操作者身份
		if t.UserID != userID && t.ClaimantID != userID {
			return ErrForbidden
		}

		// 发布者可 cancel open 或 in_progress；接单者仅可 cancel in_progress
		if t.UserID == userID {
			if t.Status != model.TaskStatusOpen && t.Status != model.TaskStatusInProgress {
				return ErrInvalidStatus
			}
		} else {
			if t.Status != model.TaskStatusInProgress {
				return ErrInvalidStatus
			}
		}

		return tx.Model(&model.Task{}).
			Where("id = ?", taskID).
			Update("status", model.TaskStatusCancelled).Error
	})
}

// ExpireOpenTasks 将所有已过期的 open 任务标记为 expired。
func ExpireOpenTasks() (int64, error) {
	result := mustTaskDB().Model(&model.Task{}).
		Where("status = ? AND expired_at < NOW()", model.TaskStatusOpen).
		Update("status", model.TaskStatusExpired)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// transitionStatus 原子状态转移（权限 + 状态校验）。
func transitionStatus(schoolID, taskID, userID int64, from, to model.TaskStatus) error {
	result := mustTaskDB().Model(&model.Task{}).
		Where("school_id = ? AND id = ? AND claimant_id = ? AND status = ?",
			schoolID, taskID, userID, from).
		Update("status", to)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
