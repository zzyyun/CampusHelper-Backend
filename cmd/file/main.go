package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go_projects/praProject1/cmd/file/model"
	file_service "go_projects/praProject1/cmd/file/service"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/discovery"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/tracer"

	file_pb "go_projects/praProject1/PB/pb/file_pb"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	fmt.Println("=== File Service 启动 ===")

	config.InitConfig("")

	shutdown, err := tracer.Init("file-service")
	if err != nil {
		log.Printf("[WARN] tracer init: %v (降级运行)", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// 初始化 MySQL
	fileDB, err := db.InitFileDB()
	if err != nil {
		log.Fatalf("[file-service] 数据库连接失败: %v", err)
	}
	if err := fileDB.AutoMigrate(&model.File{}); err != nil {
		log.Fatalf("[file-service] 自动迁移失败: %v", err)
	}
	fmt.Println("[file-service] MySQL 初始化完成")

	// 初始化 MinIO
	if err := file_service.InitMinIO(); err != nil {
		log.Printf("[file-service] WARN: MinIO 初始化失败（上传功能降级）: %v", err)
	}

	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	// gRPC Server
	svcCfg := config.Conf.Service["file"]
	addr := svcCfg.Address
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[file-service] 监听 %s 失败: %v", addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	file_pb.RegisterFileServiceServer(grpcServer, &file_service.FileServiceServer{})

	regCleanup, err := discovery.Register(context.Background(), svcCfg.Name, addr)
	if err != nil {
		log.Printf("[file-service] etcd 注册失败（降级运行）: %v", err)
	}

	fmt.Printf("[file-service] gRPC listening on %s\n", addr)

	// 启动后台清理 goroutine
	go file_service.StartCleanupTask()

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("[file-service] serve error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("[file-service] shutting down...")

	regCleanup()
	grpcServer.GracefulStop()
	fmt.Println("[file-service] 已关闭")
}