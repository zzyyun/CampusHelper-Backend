package user_database

import (
	"errors"

	"go_projects/praProject1/cmd/user/model"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/snowflake"

	"gorm.io/gorm"
)

// ─── User DAO ────────────────────────────────────────────────────────────────

// GetOrCreateByOpenID finds a user by WeChat openid; creates one if absent.
// Returns (user, isNewUser, error).
func GetOrCreateByOpenID(openID, unionID, nickname, avatarURL string) (*model.User, bool, error) {
	var u model.User
	err := db.DB.Where("wx_openid = ?", openID).First(&u).Error
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
	if err = db.DB.Create(&u).Error; err != nil {
		return nil, false, err
	}
	return &u, true, nil
}

// GetByID returns a user by primary key.
func GetByID(id int64) (*model.User, error) {
	var u model.User
	if err := db.DB.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// BindSchool sets the school for a user.
func BindSchool(userID, schoolID int64) error {
	return db.DB.Model(&model.User{}).Where("id = ?", userID).
		Update("school_id", schoolID).Error
}

// GetSchoolByID returns a school by id.
func GetSchoolByID(id int64) (*model.School, error) {
	var s model.School
	if err := db.DB.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// SearchSchools returns schools matching the name keyword.
func SearchSchools(keyword string, limit int) ([]model.School, error) {
	var schools []model.School
	err := db.DB.Where("name LIKE ?", "%"+keyword+"%").
		Limit(limit).Find(&schools).Error
	return schools, err
}

// ListSchools returns all schools.
func ListSchools() ([]model.School, error) {
	var schools []model.School
	err := db.DB.Order("id asc").Find(&schools).Error
	return schools, err
}

// UpdateUserInfo 更新用户昵称和头像
func UpdateUserInfo(userID int64, nickname, avatarURL string) error {
	return db.DB.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"nickname":   nickname,
			"avatar_url": avatarURL,
		}).Error
}

