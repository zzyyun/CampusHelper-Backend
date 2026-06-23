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

	content_pb "go_projects/praProject1/PB/pb/content_pb"
	"go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/cmd/content/repo"
	"go_projects/praProject1/cmd/content/service"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/discovery"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/snowflake"
	"go_projects/praProject1/pkg/tracer"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	// ── Config ──────────────────────────────────────────────────────────────
	config.InitConfig("")

	// ── Tracer ──────────────────────────────────────────────────────────────
	shutdown, err := tracer.Init("content-service")
	if err != nil {
		log.Printf("[WARN] tracer init: %v (continuing without tracing)\n", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// ── MySQL ────────────────────────────────────────────────────────────────
	if err = db.InitMySQL(); err != nil {
		log.Fatalf("mysql init: %v", err)
	}
	// Auto-migrate posts 表
	if err = db.DB.AutoMigrate(&model.Post{}); err != nil {
		log.Fatalf("auto-migrate: %v", err)
	}
	fmt.Println("[content-service] MySQL migrated (posts table)")

	// ── Snowflake ID Generator ──────────────────────────────────────────────
	idGen, err := snowflake.NewSnowflake(1, 1) // datacenterID=1, machineID=1
	if err != nil {
		log.Fatalf("snowflake init: %v", err)
	}
	fmt.Println("[content-service] snowflake ID generator ready")

	// ── etcd ─────────────────────────────────────────────────────────────────
	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	// ── gRPC Server ──────────────────────────────────────────────────────────
	addr := config.Conf.Service["content"].Address
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	// 构造 PostService（实现 ContentServiceServer 接口）
	postRepo := repo.NewPostRepo(db.DB)
	postService := service.NewPostService(postRepo, idGen)

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ConnectionTimeout(10 * time.Second),
	)
	content_pb.RegisterContentServiceServer(grpcServer, postService)

	// 注册到 etcd 服务发现
	serviceName := config.Conf.Service["content"].Name
	regCleanup, err := discovery.Register(context.Background(), serviceName, addr)
	if err != nil {
		log.Fatalf("register to etcd: %v", err)
	}

	fmt.Printf("[content-service] gRPC listening on %s\n", addr)

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Printf("[content-service] serve error: %v", err)
		}
	}()

	<-quit
	fmt.Println("[content-service] shutting down…")
	regCleanup()
	grpcServer.GracefulStop()
}