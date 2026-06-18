package model

import (
	"time"

	"gorm.io/gorm"
)

type Role int8

const (
	RoleStudent    Role = 1
	RoleAdmin      Role = 2
	RoleSuperAdmin Role = 3
)

func (r Role) String() string {
	switch r {
	case RoleAdmin:
		return "admin"
	case RoleSuperAdmin:
		return "super_admin"
	default:
		return "student"
	}
}

type UserStatus int8

const (
	StatusNormal  UserStatus = 1
	StatusBanned  UserStatus = 2
	StatusDeleted UserStatus = 3
)

// User maps to the `users` table.
type User struct {
	// ID 由雪花算法生成（见 pkg/snowflake），关闭数据库自增以避免应用层赋值被覆盖
	ID          int64          `gorm:"primaryKey;autoIncrement:false"                    json:"id"`
	WxOpenID    string         `gorm:"column:wx_openid;uniqueIndex;size:64;not null"     json:"wx_openid"`
	WxUnionID   string         `gorm:"column:wx_unionid;size:64;default:''"              json:"wx_unionid"`
	Nickname    string         `gorm:"size:64;default:''"                                json:"nickname"`
	AvatarURL   string         `gorm:"column:avatar_url;size:512;default:''"             json:"avatar_url"`
	SchoolID    int64          `gorm:"column:school_id;default:0;index"                  json:"school_id"`
	Role        Role           `gorm:"default:1"                                         json:"role"`
	Status      UserStatus     `gorm:"default:1"                                         json:"status"`
	IsVerified  bool           `gorm:"column:is_verified;default:false"                  json:"is_verified"`
	CreditScore int            `gorm:"column:credit_score;default:100"                   json:"credit_score"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"                                             json:"-"`
}

func (User) TableName() string { return "users" }

// School maps to the `schools` table.
type School struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:128;not null" json:"name"`
	City      string    `gorm:"size:64;default:''" json:"city"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (School) TableName() string { return "schools" }

// ─── RBAC ────────────────────────────────────────────────────────────────────

type Permission string

const (
	PermPostCreate   Permission = "post:create"
	PermPostModerate Permission = "post:moderate"
	PermTaskCreate   Permission = "task:create"
	PermTaskTake     Permission = "task:take"
	PermUserBan      Permission = "user:ban"
	PermUserManage   Permission = "user:manage"
	PermContentAudit Permission = "content:audit"
)

// roleMinLevel defines the minimum role required for each permission.
var roleMinLevel = map[Permission]Role{
	PermPostCreate:   RoleStudent,
	PermTaskCreate:   RoleStudent,
	PermTaskTake:     RoleStudent,
	PermPostModerate: RoleAdmin,
	PermContentAudit: RoleAdmin,
	PermUserBan:      RoleAdmin,
	PermUserManage:   RoleSuperAdmin,
}

// Can returns true if the role has the given permission.
func Can(role Role, perm Permission) bool {
	min, ok := roleMinLevel[perm]
	if !ok {
		return false
	}
	return role >= min
}
