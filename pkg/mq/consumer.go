package mq

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// EventHandler 事件处理函数类型。
// 接收反序列化后的 ContentEvent，返回 error 时消息重新入队（Nack）。
type EventHandler func(ctx context.Context, event *ContentEvent) error

// Consumer RabbitMQ 消息消费者。
// 支持优雅关闭与自动重连。
type Consumer struct {
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	mu       sync.Mutex
	addr     string
	queue    string
	handlers map[string]EventHandler // eventType → handler
	stopCh   chan struct{}
}

// NewConsumer 创建消息消费者。
func NewConsumer(addr, queue string) *Consumer {
	return &Consumer{
		addr:     addr,
		queue:    queue,
		handlers: make(map[string]EventHandler),
		stopCh:   make(chan struct{}),
	}
}

// RegisterHandler 注册事件处理函数。
// eventType 与 ContentEvent.Type 匹配（如 "content.published"）。
func (c *Consumer) RegisterHandler(eventType string, handler EventHandler) {
	c.handlers[eventType] = handler
	log.Printf("[MQ-Consumer] 注册事件处理器: %s", eventType)
}

// Connect 建立连接并声明队列。
func (c *Consumer) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := amqp091.Dial(c.addr)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}

	// 声明队列（幂等）
	_, err = ch.QueueDeclare(c.queue, true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	c.conn = conn
	c.channel = ch
	log.Printf("[MQ-Consumer] 连接成功: %s, 队列: %s", c.addr, c.queue)
	return nil
}

// Start 开始消费消息（阻塞调用）。
// 连接失败时自动重试（指数退避，最大 30s）。
// 断连后自动重连并重新订阅。
func (c *Consumer) Start(ctx context.Context) error {
	// 首次连接
	if err := c.connectWithRetry(ctx); err != nil {
		return err
	}

	log.Printf("[MQ-Consumer] 开始消费队列: %s", c.queue)

	for {
		// 每次循环重新获取 consume channel（重连后 channel 会变）
		c.mu.Lock()
		ch := c.channel
		c.mu.Unlock()

		msgs, err := ch.Consume(c.queue, "", false, false, false, false, nil)
		if err != nil {
			log.Printf("[MQ-Consumer] 消费失败: %v，尝试重连...", err)
			if err := c.reconnect(ctx); err != nil {
				return err
			}
			continue
		}

		// 消息处理循环
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					log.Printf("[MQ-Consumer] 消息通道关闭，尝试重连...")
					if err := c.reconnect(ctx); err != nil {
						return err
					}
					break // 跳出内层循环，重新 Consume
				}
				c.handleMessage(msg)

			case <-ctx.Done():
				log.Printf("[MQ-Consumer] 收到 ctx 取消，停止消费")
				return ctx.Err()

			case <-c.stopCh:
				log.Printf("[MQ-Consumer] 收到停止信号")
				return nil
			}
		}
	}
}

// connectWithRetry 带重试的连接。
func (c *Consumer) connectWithRetry(ctx context.Context) error {
	for {
		if err := c.Connect(); err != nil {
			log.Printf("[MQ-Consumer] 连接失败，5s 后重试: %v", err)
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				return ctx.Err()
			case <-c.stopCh:
				return nil
			}
		}
		return nil
	}
}

// handleMessage 处理单条消息。
func (c *Consumer) handleMessage(msg amqp091.Delivery) {
	var event ContentEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("[MQ-Consumer] 消息解析失败，丢弃: %v", err)
		msg.Nack(false, false) // 不可恢复，丢弃
		return
	}

	handler, ok := c.handlers[event.Type]
	if !ok {
		log.Printf("[MQ-Consumer] 未注册的事件类型 %s，确认丢弃", event.Type)
		msg.Ack(false) // 确认已处理（无处理器则丢弃）
		return
	}

	ctx := context.Background()
	if err := handler(ctx, &event); err != nil {
		log.Printf("[MQ-Consumer] 处理事件失败 type=%s post=%d: %v，重新入队", event.Type, event.PostID, err)
		msg.Nack(false, true) // 重新入队
		return
	}

	msg.Ack(false)
}

// reconnect 重连逻辑。
func (c *Consumer) reconnect(ctx context.Context) error {
	c.mu.Lock()
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Unlock()

	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return nil
		case <-time.After(backoff):
		}

		if err := c.Connect(); err != nil {
			log.Printf("[MQ-Consumer] 重连失败: %v", err)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		log.Printf("[MQ-Consumer] 重连成功")
		return nil
	}
}

// Stop 停止消费者。
func (c *Consumer) Stop() {
	close(c.stopCh)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// 确保 json 序列化可用
var _ = json.Unmarshal
