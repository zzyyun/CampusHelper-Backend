package user_database

import (
	"errors"
	"log"
	"strings"

	"go_projects/praProject1/cmd/user/model"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/snowflake"

	"gorm.io/gorm"
)

// mustUserDB 返回 user 服务的 *gorm.DB，若未初始化则记录 fatal。
// user 服务的 main.go 必须在调用 DAO 之前执行 db.InitUserDB()。
func mustUserDB() *gorm.DB {
	d, err := db.GetUserDB()
	if err != nil {
		log.Fatalf("[user-dao] 未初始化 user db: %v", err)
	}
	return d
}

// ─── User DAO ────────────────────────────────────────────────────────────────

// GetOrCreateByOpenID finds a user by WeChat openid; creates one if absent.
// Returns (user, isNewUser, error).
func GetOrCreateByOpenID(openID, unionID, nickname, avatarURL string) (*model.User, bool, error) {
	var u model.User
	err := mustUserDB().Where("wx_openid = ?", openID).First(&u).Error
	if err == nil {
		return &u, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	// 使用雪花算法生成用户ID
	userID := snowflake.GenerateID()

	u = model.User{
		ID:        userID,
		WxOpenID:  openID,
		WxUnionID: unionID,
		Nickname:  nickname,
		AvatarURL: avatarURL,
		Role:      model.RoleStudent,
		Status:    model.StatusNormal,
	}
	if err = mustUserDB().Create(&u).Error; err != nil {
		return nil, false, err
	}
	return &u, true, nil
}

// GetByID returns a user by primary key.
func GetByID(id int64) (*model.User, error) {
	var u model.User
	if err := mustUserDB().First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// BindSchool sets the school for a user.
func BindSchool(userID, schoolID int64) error {
	return mustUserDB().Model(&model.User{}).Where("id = ?", userID).
		Update("school_id", schoolID).Error
}

// GetSchoolByID returns a school by id.
func GetSchoolByID(id int64) (*model.School, error) {
	var s model.School
	if err := mustUserDB().First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// SearchSchools returns schools matching the name keyword.
func SearchSchools(keyword string, limit int) ([]model.School, error) {
	var schools []model.School
	err := mustUserDB().Where("name LIKE ?", "%"+keyword+"%").
		Limit(limit).Find(&schools).Error
	return schools, err
}

// ListSchools returns all schools.
func ListSchools() ([]model.School, error) {
	var schools []model.School
	err := mustUserDB().Order("id asc").Find(&schools).Error
	return schools, err
}

// UpdateUserInfo 更新用户昵称和头像
func UpdateUserInfo(userID int64, nickname, avatarURL string) error {
	return mustUserDB().Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"nickname":   nickname,
			"avatar_url": avatarURL,
		}).Error
}

// SetUserStatus 更新用户状态（封禁/解封/删除）。
func SetUserStatus(userID int64, status model.UserStatus) error {
	return mustUserDB().Model(&model.User{}).Where("id = ?", userID).
		Update("status", status).Error
}

// SearchUsers 按条件筛选用户列表，支持游标分页。
// schoolID=0 时不过滤学校；role=0 时不过滤角色；status=0 时不过滤状态。
func SearchUsers(schoolID int64, role, status int, keyword string, cursor int64, pageSize int) ([]model.User, bool, error) {
	query := mustUserDB().Model(&model.User{})

	if schoolID > 0 {
		query = query.Where("school_id = ?", schoolID)
	}
	if role > 0 {
		query = query.Where("role = ?", role)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		query = query.Where("nickname LIKE ?", "%"+keyword+"%")
	}
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}

	query = query.Order("id DESC").Limit(pageSize + 1) // 多取一条判断 has_more

	var users []model.User
	if err := query.Find(&users).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(users) > pageSize
	if hasMore {
		users = users[:pageSize]
	}

	return users, hasMore, nil
}

// GetUserCountBySchool 统计学校用户数（可选，供后续 Dashboard 使用）。
func GetUserCountBySchool(schoolID int64) (int64, error) {
	var count int64
	err := mustUserDB().Model(&model.User{}).Where("school_id = ?", schoolID).Count(&count).Error
	return count, err
}

