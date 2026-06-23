package db

import (
	"gorm.io/gorm"
)

// SchoolScope 自动注入 school_id 多租户隔离条件。
// 所有 Content Service 数据查询必须通过该 Scope 限定 school_id，
// 防止跨学校数据泄露。属于项目的强制安全约束。
//
// 使用示例：
//   db.Scopes(SchoolScope(schoolID)).First(&post)
//
// 实现说明：
//   - 使用 GORM Scope 机制，应用到所有查询
//   - school_id = 0 表示无效学校，应返回空结果（防止误用）
func SchoolScope(schoolID int64) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// 防御性检查：schoolID 为 0 时返回空查询（WHERE 1=0）
		if schoolID <= 0 {
			return db.Where("1 = 0")
		}
		return db.Where("school_id = ?", schoolID)
	}
}

// SchoolIDFromContext 从 context 中提取 school_id（若存在）。
// 网关层在收到请求时，会将 JWT 中的 school_id 注入到 context，
// 业务层可通过该函数获取。如果 context 中不存在，返回 0。
//
// TODO: 与 pkg/contextx 集成（当 contextx 包就绪后）
func SchoolIDFromContext(schoolID int64) int64 {
	if schoolID <= 0 {
		return 0
	}
	return schoolID
}