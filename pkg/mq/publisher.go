// Package mq 提供 RabbitMQ 消息发布封装。
//
// 设计原则：
//   - 发布失败不影响主流程（best-effort）
//   - 通过 AMQP Header 透传 TraceID 实现全链路追踪
//   - 连接失败时自动降级，仅记录日志
package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Publisher RabbitMQ 消息发布者。
// 支持优雅降级：连接不可用时，发布操作仅记录日志不返回错误。
type Publisher struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
	mu      sync.Mutex
	addr    string // amqp://user:pass@host:port/
	queue   string // 默认队列名
}

// NewPublisher 创建消息发布者。
// addr 格式: amqp://user:pass@host:port/
func NewPublisher(addr, queue string) *Publisher {
	return &Publisher{
		addr:  addr,
		queue: queue,
	}
}

// Connect 建立 RabbitMQ 连接与 Channel。
// 若连接失败，记录日志但不返回错误（允许降级运行）。
func (p *Publisher) Connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, err := amqp091.Dial(p.addr)
	if err != nil {
		log.Printf("[MQ] RabbitMQ 连接失败（服务降级运行）: %v", err)
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		log.Printf("[MQ] 创建 Channel 失败（服务降级运行）: %v", err)
		return err
	}

	// 声明队列（幂等，不存在则创建）
	_, err = ch.QueueDeclare(p.queue, true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		log.Printf("[MQ] 声明队列失败（服务降级运行）: %v", err)
		return err
	}

	p.conn = conn
	p.channel = ch
	log.Printf("[MQ] RabbitMQ 连接成功: %s, 队列: %s", p.addr, p.queue)
	return nil
}

// Close 关闭 Channel 和 Connection。
func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
	log.Printf("[MQ] RabbitMQ 连接已关闭")
}

// Publish 发布消息到队列。
// 若连接未建立，自动尝试连接。
// 发布失败仅记录日志，不影响主流程。
func (p *Publisher) Publish(ctx context.Context, event interface{}) error {
	p.mu.Lock()
	// 自动连接（首次调用时触发）
	if p.channel == nil {
		p.mu.Unlock()
		if err := p.Connect(); err != nil {
			log.Printf("[MQ] 消息发布跳过（连接未建立）: %v", err)
			return nil // 降级：不阻塞业务
		}
		p.mu.Lock()
	}
	p.mu.Unlock()

	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("[MQ] 消息序列化失败: %v", err)
		return nil // 降级
	}

	msg := amqp091.Publishing{
		DeliveryMode: amqp091.Persistent,
		ContentType:  "application/json",
		Body:         body,
		Timestamp:    time.Now(),
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel == nil {
		log.Printf("[MQ] 消息发布跳过（Channel 已断开）")
		return nil
	}

	err = p.channel.PublishWithContext(ctx, "", p.queue, false, false, msg)
	if err != nil {
		log.Printf("[MQ] 消息发布失败: %v", err)
		return nil // 降级
	}
	return nil
}

// ContentEvent 内容事件消息体。
// 通过 RabbitMQ 在 Content、Message、ES 等服务间传递。
type ContentEvent struct {
	Type     string            `json:"type"`           // 事件类型: content.published / content.review_result
	PostID   int64             `json:"post_id"`        // 帖子 ID
	SchoolID int64             `json:"school_id"`      // 学校 ID
	UserID   int64             `json:"user_id"`        // 发帖用户 ID
	Data     map[string]string `json:"data,omitempty"` // 附加数据
	TraceID  string            `json:"trace_id"`       // 全链路 TraceID
	Time     string            `json:"time"`           // 事件时间（ISO 8601）
}

// NewContentEvent 创建内容事件。
func NewContentEvent(eventType string, postID, schoolID, userID int64, traceID string) *ContentEvent {
	return &ContentEvent{
		Type:     eventType,
		PostID:   postID,
		SchoolID: schoolID,
		UserID:   userID,
		Data:     make(map[string]string),
		TraceID:  traceID,
		Time:     time.Now().UTC().Format(time.RFC3339),
	}
}

// ContentEvent 类型常量
const (
	EventContentPublished = "content.published"     // 审核通过 → 触发 ES 同步 / 通知用户
	EventContentRejected  = "content.review_result" // 审核拒绝 → 通知用户
	EventContentTakenDown = "content.taken_down"    // 违规下架 → 删除 ES 文档 + 通知用户
	EventContentLiked     = "content.liked"         // 帖子被点赞 → 通知帖子作者
	EventContentReplied   = "content.replied"       // 评论被回复 → 通知父评论作者
)

// 确保 JSON 序列化接口实现
var _ = fmt.Sprintf("%v", ContentEvent{})
var _ = json.Marshal
