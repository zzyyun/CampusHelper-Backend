package db

import (
	"gorm.io/gorm"
)

// SchoolScope 返回一个强制 school_id 隔离的 GORM Scope。
// 这是多租户隔离的核心：所有跨租户的查询都会被自动限制在调用方所属的 school_id 范围。
//
// 关键安全特性：
//   - schoolID <= 0 时返回 "WHERE 1=0"，让查询永远空集（避免误用全局查询）
//   - schoolID > 0 时返回 "WHERE school_id = ?"，自动注入到查询
//
// 使用示例：
//
//	db.GetContentDB().Scopes(db.SchoolScope(schoolID)).Where("status = ?", pb.POST_STATUS_PUBLISHED).Find(&posts)
func SchoolScope(schoolID int64) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if schoolID <= 0 {
			// 防御性兜底：未携带 school_id 的请求不允许查询任何数据
			return db.Where("1 = 0")
		}
		return db.Where("school_id = ?", schoolID)
	}
}

// TenantSafe 在事务中自动应用 SchoolScope，常用于 service 层的入口校验。
// 若 schoolID 无效则返回错误，调用方应中止本次业务。
func TenantSafe(tx *gorm.DB, schoolID int64) *gorm.DB {
	if schoolID <= 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where("school_id = ?", schoolID)
}