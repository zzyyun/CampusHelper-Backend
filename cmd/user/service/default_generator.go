package service

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const (
	// 默认头像URL模板
	defaultAvatarTemplate = "https://api.dicebear.com/7.x/avataaars/svg?seed=%s"
	// 默认昵称前缀
	defaultNicknamePrefix = "校园用户"
)

var (
	adjectives = []string{"快乐", "温馨", "活力", "阳光", "友好", "热情", "善良", "聪明", "勇敢", "创意"}
	nouns      = []string{"小草", "微风", "星光", "晨露", "白云", "花朵", "小鸟", "彩虹", "流星", "森林"}
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateDefaultNickname 生成默认昵称
// 格式：校园用户 + 随机形容词 + 随机名词 + 随机4位数字
// 例如：校园用户快乐微风1234
func GenerateDefaultNickname() string {
	adjective := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	randomNum := rand.Intn(10000)

	return fmt.Sprintf("%s%s%s%04d", defaultNicknamePrefix, adjective, noun, randomNum)
}

// GenerateDefaultAvatarURL 生成默认头像URL
// 使用用户ID作为随机种子，确保同一个用户每次生成相同的头像
func GenerateDefaultAvatarURL(userID int64) string {
	// 使用用户ID作为种子，保证同一用户头像一致
	seed := fmt.Sprintf("user_%d", userID)
	return fmt.Sprintf(defaultAvatarTemplate, seed)
}

// isValidAvatarURL 检查头像URL是否有效
func isValidAvatarURL(avatarURL string) bool {
	if avatarURL == "" {
		return false
	}
	// 检查是否以http或https开头
	if strings.HasPrefix(avatarURL, "http://") || strings.HasPrefix(avatarURL, "https://") {
		return true
	}
	return false
}

// isValidNickname 检查昵称是否有效
func isValidNickname(nickname string) bool {
	if nickname == "" {
		return false
	}
	// 昵称长度限制2-20个字符
	if len(nickname) < 2 || len(nickname) > 20 {
		return false
	}
	return true
}
