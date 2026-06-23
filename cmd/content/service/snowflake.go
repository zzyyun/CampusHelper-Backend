package service

import (
	"fmt"

	"go_projects/praProject1/pkg/snowflake"
)

// snowflakeGen content 服务的雪花 ID 生成器
// 数据中心 ID=2，机器 ID=2（与 user 服务不同，避免ID冲突）
var snowflakeGen *snowflake.Snowflake

func init() {
	g, err := snowflake.NewSnowflake(2, 2)
	if err != nil {
		panic(fmt.Sprintf("content-service snowflake init: %v", err))
	}
	snowflakeGen = g
}

// nextPostID 生成下一个帖子 ID（雪花算法）
// 时钟回拨等异常情况下返回 error，调用方必须 fail-fast 拒绝写入（避免 ID=0 脏数据）
func nextPostID() (int64, error) {
	return snowflakeGen.NextID()
}

// nextCommentID 生成下一个评论 ID
func nextCommentID() (int64, error) {
	return snowflakeGen.NextID()
}

// nextLikeID 生成下一个点赞 ID
func nextLikeID() (int64, error) {
	return snowflakeGen.NextID()
}