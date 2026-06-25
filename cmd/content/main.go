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
	es_pkg "go_projects/praProject1/pkg/es"
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

	// ── RabbitMQ Publisher（用于审核/点赞等事件发布） ────────────────────────
	mqAddr := fmt.Sprintf("amqp://%s:%s@%s/",
		config.Conf.RabbitMQ.Username, config.Conf.RabbitMQ.Password,
		config.Conf.RabbitMQ.Address)
	content_service.InitMQ(mqAddr)
	defer func() {
		// Publisher 关闭（如需）
	}()

	// ── Elasticsearch + ES Sync Consumer（异步同步消费者） ──────────────────
	esAddrs := config.Conf.Elasticsearch.Addresses
	esIndex := config.Conf.Elasticsearch.Index
	if len(esAddrs) == 0 {
		esAddrs = []string{"http://127.0.0.1:9200"}
	}
	if esIndex == "" {
		esIndex = "campus_posts"
	}
	content_service.InitES(esAddrs)
	esClient, err := es_pkg.NewClient(esAddrs, esIndex)
	if err != nil {
		log.Printf("[content-service] WARN: ES 客户端创建失败，搜索功能降级: %v", err)
	}

	// 启动 ES 同步消费者 goroutine（独立 ctx，优雅停止）
	esConsumerCtx, esConsumerCancel := context.WithCancel(context.Background())
	defer esConsumerCancel()
	var esConsumer *content_service.ESSyncConsumer
	if esClient != nil {
		esConsumer = content_service.NewESSyncConsumer(mqAddr, esClient)
		go func() {
			log.Printf("[content-service] ES Sync Consumer 启动中...")
			if err := esConsumer.Start(esConsumerCtx); err != nil {
				log.Printf("[content-service] ES Sync Consumer 退出: %v", err)
			}
		}()
		defer esConsumer.Stop()
		fmt.Println("[content-service] ES Sync Consumer 已注册（异步启动）")
	}

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
	esConsumerCancel() // 通知 ES 消费者停止
	regCleanup()
	grpcServer.GracefulStop()
}
