package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_projects/praProject1/cmd/task/model"
	task_service "go_projects/praProject1/cmd/task/service"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/discovery"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/mq"
	"go_projects/praProject1/pkg/tracer"

	task_pb "go_projects/praProject1/PB/pb/task_pb"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	fmt.Println("=== Task Service 启动 ===")

	config.InitConfig("")

	shutdown, err := tracer.Init("task-service")
	if err != nil {
		log.Printf("[WARN] tracer init: %v (降级运行)", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	taskDB, err := db.InitTaskDB()
	if err != nil {
		log.Fatalf("[task-service] 数据库连接失败: %v", err)
	}
	if err := taskDB.AutoMigrate(&model.Task{}); err != nil {
		log.Fatalf("[task-service] 自动迁移失败: %v", err)
	}
	fmt.Println("[task-service] MySQL 初始化完成")

	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	svcCfg := config.Conf.Service["task"]
	addr := svcCfg.Address
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[task-service] 监听 %s 失败: %v", addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	task_pb.RegisterTaskServiceServer(grpcServer, &task_service.TaskServiceServer{})

	regCleanup, err := discovery.Register(context.Background(), svcCfg.Name, addr)
	if err != nil {
		log.Printf("[task-service] etcd 注册失败（降级运行）: %v", err)
	}

	fmt.Printf("[task-service] gRPC listening on %s\n", addr)

	// 启动 MQ Publisher
	mqAddr := fmt.Sprintf("amqp://%s:%s@%s/",
		config.Conf.RabbitMQ.Username, config.Conf.RabbitMQ.Password,
		config.Conf.RabbitMQ.Address)
	mqPublisher := initPublisher(mqAddr)

	// 启动自动过期 goroutine
	go startExpiryLoop()

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("[task-service] serve error: %v", err)
		}
	}()

	// 占位：防止 mqPublisher 被 GC
	_ = mqPublisher

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("[task-service] shutting down...")

	regCleanup()
	grpcServer.GracefulStop()
	fmt.Println("[task-service] 已关闭")
}

func initPublisher(mqAddr string) *mq.Publisher {
	p := mq.NewPublisher(mqAddr, "task.events")
	log.Println("[task-service] MQ Publisher 初始化完成（队列: task.events）")
	return p
}

func startExpiryLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	doExpire()

	for range ticker.C {
		doExpire()
	}
}

func doExpire() {
	// 占位：后续在 task-026 中实现 repo.ExpireOpenTasks() 调用
	log.Println("[task-service] 自动过期检查完成")
}
