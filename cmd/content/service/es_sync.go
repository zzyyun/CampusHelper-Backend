package service

import (
	"context"
	"log"

	content_db "go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/cmd/content/repo"
	"go_projects/praProject1/pkg/es"
	"go_projects/praProject1/pkg/mq"
)

// ESSyncConsumer ES 异步同步消费者。
// 监听 RabbitMQ content.events 队列，将 Post 变更同步到 Elasticsearch。
type ESSyncConsumer struct {
	consumer *mq.Consumer
	esClient *es.Client
}

// NewESSyncConsumer 创建 ES 同步消费者。
// mqAddr 格式: amqp://user:pass@host:port/
func NewESSyncConsumer(mqAddr string, esClient *es.Client) *ESSyncConsumer {
	c := mq.NewConsumer(mqAddr, "content.events")
	consumer := &ESSyncConsumer{
		consumer: c,
		esClient: esClient,
	}
	c.RegisterHandler(mq.EventContentPublished, consumer.handlePublished)
	c.RegisterHandler(mq.EventContentTakenDown, consumer.handleTakenDown)
	return consumer
}

// Start 启动消费者（阻塞调用）。
func (c *ESSyncConsumer) Start(ctx context.Context) error {
	return c.consumer.Start(ctx)
}

// Stop 停止消费者。
func (c *ESSyncConsumer) Stop() {
	c.consumer.Stop()
}

// handlePublished 处理 content.published 事件：索引到 ES。
func (c *ESSyncConsumer) handlePublished(ctx context.Context, event *mq.ContentEvent) error {
	// 从 MySQL 查询完整帖子数据
	post, err := repo.GetByID(event.SchoolID, event.PostID)
	if err != nil {
		log.Printf("[ES-Sync] 查询帖子失败 post=%d: %v", event.PostID, err)
		return err // 重新入队重试
	}

	// 转换为 ES 文档
	doc := &es.PostDocument{
		PostID:       post.ID,
		SchoolID:     post.SchoolID,
		UserID:       post.UserID,
		Type:         post.Type,
		Title:        post.Title,
		Content:      post.Content,
		Status:       int8(post.Status),
		LikesCount:   post.LikesCount,
		CommentCount: post.CommentCount,
		CreatedAt:    post.CreatedAt.Unix(),
		UpdatedAt:    post.UpdatedAt.Unix(),
	}

	if err := c.esClient.IndexPost(ctx, doc); err != nil {
		log.Printf("[ES-Sync] 索引到 ES 失败 post=%d: %v", event.PostID, err)
		return err
	}

	log.Printf("[ES-Sync] 帖子已同步到 ES: post=%d title=%s", event.PostID, post.Title)
	return nil
}

// handleTakenDown 处理 content.taken_down 事件：从 ES 删除文档。
func (c *ESSyncConsumer) handleTakenDown(ctx context.Context, event *mq.ContentEvent) error {
	if err := c.esClient.DeletePost(ctx, event.PostID); err != nil {
		log.Printf("[ES-Sync] 从 ES 删除失败 post=%d: %v", event.PostID, err)
		return err
	}

	log.Printf("[ES-Sync] 帖子已从 ES 删除: post=%d", event.PostID)
	return nil
}

// 确保 content_db 引用可用
var _ = content_db.PostStatusPending
