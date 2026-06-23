package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	content_db "go_projects/praProject1/cmd/content/model"
	content_service "go_projects/praProject1/cmd/content/service"
	pb "go_projects/praProject1/PB/pb/content_pb"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/discovery"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/tracer"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	// ── Config ───────────────────────────────────────────────────────────────
	config.InitConfig("")

	// ── Tracer ───────────────────────────────────────────────────────────────
	shutdown, err := tracer.Init("content-service")
	if err != nil {
		log.Printf("[WARN] tracer init: %v (continuing without tracing)\n", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// ── MySQL（Content Service 独立连接 campus_content 数据库） ──────────────
	contentDB, err := db.InitContentDB()
	if err != nil {
		log.Fatalf("mysql init: %v", err)
	}
	// Auto-migrate Content Service tables
	if err = contentDB.AutoMigrate(
		&content_db.Post{},
		&content_db.PostLike{},
		&content_db.PostComment{},
	); err != nil {
		log.Fatalf("auto-migrate: %v", err)
	}
	fmt.Println("[content-service] MySQL migrated")

	// ── etcd ─────────────────────────────────────────────────────────────────
	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	// ── gRPC Server ──────────────────────────────────────────────────────────
	svcCfg, ok := config.Conf.Service["content"]
	if !ok {
		log.Fatalf("config: 缺少 service.content 配置")
	}
	addr := svcCfg.Address
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterContentServiceServer(grpcServer, &content_service.ContentServiceServer{})

	// 注册到 etcd 服务发现
	regCleanup, err := discovery.Register(context.Background(), svcCfg.Name, addr)
	if err != nil {
		log.Fatalf("register to etcd: %v", err)
	}

	fmt.Printf("[content-service] gRPC listening on %s\n", addr)

	// ── Graceful shutdown ───────────────────────────────────────────────────
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