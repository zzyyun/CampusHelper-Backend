package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	user_pb "go_projects/praProject1/PB/pb/user_pb"
	"go_projects/praProject1/cmd/user/model"
	"go_projects/praProject1/cmd/user/service"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/discovery"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/rdb"
	"go_projects/praProject1/pkg/tracer"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	// ── Config ───────────────────────────────────────────────────────────────
	config.InitConfig("")

	// ── Tracer ──────────────────────────────────────────────────────────────
	shutdown, err := tracer.Init("user-service")
	if err != nil {
		log.Printf("[WARN] tracer init: %v (continuing without tracing)\n", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// ── MySQL ────────────────────────────────────────────────────────────────
	userDB, err := db.InitUserDB()
	if err != nil {
		log.Fatalf("mysql init: %v", err)
	}
	// Auto-migrate User Service tables
	if err = userDB.AutoMigrate(&model.User{}, &model.School{}); err != nil {
		log.Fatalf("auto-migrate: %v", err)
	}
	fmt.Println("[user-service] MySQL migrated")

	// ── Redis ────────────────────────────────────────────────────────────────
	if err = rdb.InitRedis(); err != nil {
		log.Fatalf("redis init: %v", err)
	}
	fmt.Println("[user-service] Redis connected")

	// ── etcd ─────────────────────────────────────────────────────────────────
	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	// ── gRPC Server ──────────────────────────────────────────────────────────
	addr := config.Conf.Service["user"].Address
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	user_pb.RegisterUserServiceServer(grpcServer, &service.UserServiceServer{})

	// 注册到 etcd 服务发现（必须在 net.Listen 成功后，确保只通告已绑定的地址）
	serviceName := config.Conf.Service["user"].Name
	regCleanup, err := discovery.Register(context.Background(), serviceName, addr)
	if err != nil {
		log.Fatalf("register to etcd: %v", err)
	}

	fmt.Printf("[user-service] gRPC listening on %s\n", addr)

	// ── Graceful shutdown ────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Printf("[user-service] serve error: %v", err)
		}
	}()

	<-quit
	fmt.Println("[user-service] shutting down…")
	// 先从 etcd 注销，再停止接收新请求；已建立连接继续处理完
	regCleanup()
	grpcServer.GracefulStop()
}
