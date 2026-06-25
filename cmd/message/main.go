package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go_projects/praProject1/cmd/message/model"
	message_service "go_projects/praProject1/cmd/message/service"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/discovery"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/mq"
	"go_projects/praProject1/pkg/tracer"

	message_pb "go_projects/praProject1/PB/pb/message_pb"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	fmt.Println("=== Message Service 启动 ===")

	// 1. 初始化配置
	config.InitConfig("")

	// 2. 初始化 Tracer
	shutdown, err := tracer.Init("message-service")
	if err != nil {
		log.Printf("[WARN] tracer init: %v (降级运行)", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// 3. 初始化 MySQL
	messageDB, err := db.InitMessageDB()
	if err != nil {
		log.Fatalf("[message-service] 数据库连接失败: %v", err)
	}
	// 自动迁移
	if err := messageDB.AutoMigrate(&model.Notification{}); err != nil {
		log.Fatalf("[message-service] 自动迁移失败: %v", err)
	}
	fmt.Println("[message-service] MySQL 初始化完成")

	// 4. 初始化 etcd
	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	// 5. gRPC Server
	svcCfg := config.Conf.Service["message"]
	addr := svcCfg.Address
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[message-service] 监听 %s 失败: %v", addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	message_pb.RegisterMessageServiceServer(grpcServer, &message_service.MessageServiceServer{})

	// 注册到 etcd 服务发现
	regCleanup, err := discovery.Register(context.Background(), svcCfg.Name, addr)
	if err != nil {
		log.Printf("[message-service] etcd 注册失败（降级运行）: %v", err)
	}

	fmt.Printf("[message-service] gRPC listening on %s\n", addr)

	// 6. 启动 MQ Consumer（订阅 notification.events）
	mqAddr := fmt.Sprintf("amqp://%s:%s@%s/",
		config.Conf.RabbitMQ.Username, config.Conf.RabbitMQ.Password,
		config.Conf.RabbitMQ.Address)
	consumer := startMQConsumer(mqAddr)

	// 7. 启动 gRPC（goroutine）
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("[message-service] serve error: %v", err)
		}
	}()

	// 8. 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("[message-service] shutting down...")

	// 9. 优雅关闭
	consumer.Stop()
	regCleanup()
	grpcServer.GracefulStop()
	fmt.Println("[message-service] 已关闭")
}

// startMQConsumer 初始化并启动 MQ 消费者。
func startMQConsumer(mqAddr string) *mq.Consumer {
	consumer := mq.NewConsumer(mqAddr, "notification.events")

	// 注册事件处理器
	consumer.RegisterHandler(mq.EventContentLiked, message_service.HandleLiked)
	consumer.RegisterHandler(mq.EventContentPublished, message_service.HandleReviewResult)
	consumer.RegisterHandler(mq.EventContentRejected, message_service.HandleReviewResult)
	consumer.RegisterHandler(mq.EventContentTakenDown, message_service.HandleTakenDown)
	consumer.RegisterHandler(mq.EventContentReplied, message_service.HandleReplied)

	go func() {
		ctx := context.Background()
		if err := consumer.Start(ctx); err != nil {
			log.Printf("[message-service] MQ Consumer 退出: %v", err)
		}
	}()
	log.Println("[message-service] MQ Consumer 已启动（队列: notification.events）")

	return consumer
}