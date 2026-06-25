package user_database

import (
	"log"
	"os"
	"testing"

	"go_projects/praProject1/cmd/user/model"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
)

// TestMain 初始化配置和数据库连接（环境变量 USER_TEST_DB=1 时启用真实数据库测试）。
func TestMain(m *testing.M) {
	if os.Getenv("USER_TEST_DB") != "1" {
		log.Println("[user-dao-test] 跳过数据库测试（设置 USER_TEST_DB=1 启用）")
		os.Exit(0)
	}

	config.InitConfig("")
	if _, err := db.InitUserDB(); err != nil {
		log.Fatalf("[user-dao-test] 数据库连接失败: %v", err)
	}
	os.Exit(m.Run())
}

func TestGetOrCreateByOpenID_NewUser(t *testing.T) {
	openID := "test_openid_" + t.Name()
	u, isNew, err := GetOrCreateByOpenID(openID, "", "test", "http://avatar")
	if err != nil {
		t.Fatalf("GetOrCreateByOpenID 失败: %v", err)
	}
	if !isNew {
		t.Error("新用户应标记 isNew=true")
	}
	if u.WxOpenID != openID {
		t.Errorf("WxOpenID 应为 %s，实际 %s", openID, u.WxOpenID)
	}
	if u.Role != model.RoleStudent {
		t.Errorf("新用户角色应为 student，实际 %v", u.Role)
	}
	if u.Status != model.StatusNormal {
		t.Errorf("新用户状态应为 normal，实际 %v", u.Status)
	}

	// 清理
	cleanupUser(t, u.ID)
}

func TestGetOrCreateByOpenID_ExistingUser(t *testing.T) {
	openID := "test_existing_" + t.Name()
	u1, _, err := GetOrCreateByOpenID(openID, "", "", "")
	if err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}

	u2, isNew, err := GetOrCreateByOpenID(openID, "", "", "")
	if err != nil {
		t.Fatalf("第二次查询失败: %v", err)
	}
	if isNew {
		t.Error("已存在用户应标记 isNew=false")
	}
	if u1.ID != u2.ID {
		t.Errorf("同一 openid 应返回相同 user_id: %d vs %d", u1.ID, u2.ID)
	}

	cleanupUser(t, u1.ID)
}

func TestSearchSchools_WithKeyword(t *testing.T) {
	schools, err := SearchSchools("大学", 5)
	if err != nil {
		t.Fatalf("SearchSchools 失败: %v", err)
	}
	t.Logf("找到 %d 所大学", len(schools))
	for _, s := range schools {
		t.Logf("  - %s (id=%d)", s.Name, s.ID)
	}
}

func TestListSchools(t *testing.T) {
	schools, err := ListSchools()
	if err != nil {
		t.Fatalf("ListSchools 失败: %v", err)
	}
	if len(schools) == 0 {
		t.Log("学校列表为空（数据库可能未初始化）")
	} else {
		t.Logf("共 %d 所学校", len(schools))
	}
}

func TestGetSchoolByID_NotFound(t *testing.T) {
	_, err := GetSchoolByID(999999)
	if err == nil {
		t.Error("不存在的 school_id 应返回错误")
	}
}

func TestBindSchool(t *testing.T) {
	openID := "test_bind_" + t.Name()
	u, _, err := GetOrCreateByOpenID(openID, "", "", "")
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	schools, err := ListSchools()
	if err != nil || len(schools) == 0 {
		t.Skip("跳过：无学校数据")
	}

	if err := BindSchool(u.ID, schools[0].ID); err != nil {
		t.Fatalf("BindSchool 失败: %v", err)
	}

	updated, err := GetByID(u.ID)
	if err != nil {
		t.Fatalf("GetByID 失败: %v", err)
	}
	if updated.SchoolID != schools[0].ID {
		t.Errorf("SchoolID 应为 %d，实际 %d", schools[0].ID, updated.SchoolID)
	}

	cleanupUser(t, u.ID)
}

func TestUpdateUserInfo(t *testing.T) {
	openID := "test_update_" + t.Name()
	u, _, err := GetOrCreateByOpenID(openID, "", "", "")
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	if err := UpdateUserInfo(u.ID, "新昵称", "http://new-avatar"); err != nil {
		t.Fatalf("UpdateUserInfo 失败: %v", err)
	}

	updated, err := GetByID(u.ID)
	if err != nil {
		t.Fatalf("GetByID 失败: %v", err)
	}
	if updated.Nickname != "新昵称" {
		t.Errorf("昵称应为「新昵称」，实际 %s", updated.Nickname)
	}
	if updated.AvatarURL != "http://new-avatar" {
		t.Errorf("头像应为 http://new-avatar，实际 %s", updated.AvatarURL)
	}

	cleanupUser(t, u.ID)
}

// cleanupUser 删除测试用户。
func cleanupUser(t *testing.T, id int64) {
	t.Helper()
	if err := mustUserDB().Unscoped().Delete(&model.User{}, id).Error; err != nil {
		t.Logf("清理用户 %d 失败: %v", id, err)
	}
}
