package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"go_projects/praProject1/cmd/task/repo"
	"go_projects/praProject1/pkg/mq"
)

// mqPublisher 全局消息发布者，由 main.go 初始化。
var mqPublisher *mq.Publisher

// InitMQ 初始化 RabbitMQ 发布者。
func InitMQ(mqAddr string) {
	mqPublisher = mq.NewPublisher(mqAddr, "task.events")
	log.Printf("[task-service] MQ Publisher 已初始化（队列: task.events）")
}

// publishEvent 发布任务事件到 task.events（以及通知类到 notification.events）。
func publishEvent(eventType string, taskID, schoolID, userID int64, data map[string]string) {
	if mqPublisher == nil {
		return
	}
	event := mq.NewContentEvent(eventType, taskID, schoolID, userID, "")
	for k, v := range data {
		event.Data[k] = v
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = mqPublisher.Publish(ctx, event)

	// 通知类事件额外投递 notification.events
	if isTaskNotificationEvent(eventType) {
		publishNotificationEvent(event)
	}
}

// publishNotificationEvent 投递到 notification.events（供 Message Service 消费）。
func publishNotificationEvent(event *mq.ContentEvent) {
	addr := fmt.Sprintf("amqp://%s:%s@%s/",
		"", // 从 config 读取，简化处理
		"",
		"localhost:5672",
	)
	p := mq.NewPublisher(addr, "notification.events")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.Publish(ctx, event)
}

// isTaskNotificationEvent 判断是否为需要通知用户的事件。
func isTaskNotificationEvent(eventType string) bool {
	switch eventType {
	case mq.EventTaskClaimed,
		mq.EventTaskCompleted,
		mq.EventTaskCancelled,
		mq.EventTaskExpired:
		return true
	}
	return false
}

// DoExpire 执行自动过期扫描。
func DoExpire() {
	count, err := repo.ExpireOpenTasks()
	if err != nil {
		log.Printf("[task-service] 自动过期失败: %v", err)
		return
	}
	if count > 0 {
		log.Printf("[task-service] 自动过期 %d 个任务", count)
		// 发布过期事件（简化：不逐个发布）
		publishEvent(mq.EventTaskExpired, 0, 0, 0, map[string]string{
			"count": fmt.Sprintf("%d", count),
		})
	}
}
