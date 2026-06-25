package model

import (
	"time"

	"gorm.io/gorm"
)

// TaskStatus 任务状态
type TaskStatus int8

const (
	TaskStatusOpen       TaskStatus = 1 // 待接单
	TaskStatusInProgress TaskStatus = 2 // 进行中
	TaskStatusCompleted  TaskStatus = 3 // 已完成
	TaskStatusCancelled  TaskStatus = 4 // 已取消
	TaskStatusExpired    TaskStatus = 5 // 已过期
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeDelivery TaskType = "delivery" // 跑腿
	TaskTypeCarpool  TaskType = "carpool"  // 拼车
	TaskTypeBounty   TaskType = "bounty"   // 悬赏
)

// Task 任务数据模型。
// 对应 campus_task 数据库的 tasks 表。
type Task struct {
	ID              int64          `gorm:"primaryKey;autoIncrement:false"                 json:"id"`
	SchoolID        int64          `gorm:"column:school_id;index;not null"                json:"school_id"`
	UserID          int64          `gorm:"column:user_id;index;not null"                  json:"user_id"`     // 发布者
	ClaimantID      int64          `gorm:"column:claimant_id;default:0"                   json:"claimant_id"` // 接单者（0=未接单）
	TaskType        string         `gorm:"column:task_type;size:32;not null"              json:"task_type"`
	Title           string         `gorm:"size:128;not null"                              json:"title"`
	Description     string         `gorm:"size:2000;default:''"                          json:"description"`
	Location        string         `gorm:"size:256;default:''"                           json:"location"`
	RewardDesc      string         `gorm:"column:reward_desc;size:256;default:''"         json:"reward_desc"`
	Contact         string         `gorm:"size:256;default:''"                            json:"contact"`          // 发布者联系方式
	Note            string         `gorm:"size:500;default:''"                            json:"note"`             // 发布者留言
	ClaimantContact string         `gorm:"column:claimant_contact;size:256;default:''"    json:"claimant_contact"` // 接单者联系方式
	ClaimantMsg     string         `gorm:"column:claimant_msg;size:500;default:''"        json:"claimant_msg"`     // 接单者留言
	Status          TaskStatus     `gorm:"default:1"                                      json:"status"`
	ExpiredAt       time.Time      `gorm:"column:expired_at"                              json:"expired_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index"                                          json:"-"`
}

func (Task) TableName() string { return "tasks" }

// AllowedTransitions 定义任务状态机的合法转移。
// key: 当前状态，value: 可转移到的状态集合。
var AllowedTransitions = map[TaskStatus]map[TaskStatus]bool{
	TaskStatusOpen: {
		TaskStatusInProgress: true, // 接单
		TaskStatusCancelled:  true, // 发布者取消
		TaskStatusExpired:    true, // 自动过期
	},
	TaskStatusInProgress: {
		TaskStatusCompleted: true, // 接单者完成
		TaskStatusCancelled: true, // 发布者/接单者取消
	},
}

// CanTransitionTo 检查是否可以转移到目标状态。
func (s TaskStatus) CanTransitionTo(target TaskStatus) bool {
	if targets, ok := AllowedTransitions[s]; ok {
		return targets[target]
	}
	return false
}
