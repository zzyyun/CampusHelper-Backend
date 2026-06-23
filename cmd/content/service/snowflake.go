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
func nextPostID() int64 {
	id, err := snowflakeGen.NextID()
	if err != nil {
		// 时钟回拨等极端情况使用 0 占位，调用方应捕获并重试
		return 0
	}
	return id
}

// nextCommentID 生成下一个评论 ID
func nextCommentID() int64 {
	id, err := snowflakeGen.NextID()
	if err != nil {
		return 0
	}
	return id
}

// nextLikeID 生成下一个点赞 ID
func nextLikeID() int64 {
	id, err := snowflakeGen.NextID()
	if err != nil {
		return 0
	}
	return id
}