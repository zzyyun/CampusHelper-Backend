// Package main 启动 ai-moderation 第 8 个微服务。
//
// ai-moderation 服务基于 onnxruntime-go 本地推理实现智能内容审核，
// 作为 Content Service 在 DFA 与人工审核之间的"智能中段"。
// 当前（v3.0 MVP）实现为 mock 模式（固定返回 PASS），待 task-046 接入真实 ONNX 模型。
//
// 端口分配：
//   - :50061  gRPC 服务（ModerateText / HealthCheck，待 task-040 proto）
//   - :9091   Prometheus /metrics 端点
//
// 详细设计见 docs/ai-moderation-content-service-v3.0-prd.md。
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"
	"go_projects/praProject1/cmd/ai-moderation/metrics"
	"go_projects/praProject1/config"
	"go_projects/praProject1/internal/ai_moderation"
	"go_projects/praProject1/pkg/discovery"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/tracer"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	// ── Config ────────────────────────────────────────────────────────────────
	config.InitConfig("")

	// ── Tracer ───────────────────────────────────────────────────────────────
	shutdown, err := tracer.Init("ai-moderation")
	if err != nil {
		log.Printf("[ai-moderation] WARN: tracer init: %v (continuing without tracing)", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// ── etcd 服务发现 ─────────────────────────────────────────────────────────
	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	// ── Prometheus /metrics HTTP 端点 ─────────────────────────────────────────
	metricsPort := viper.GetInt("metrics.aiModeration.port")
	if metricsPort == 0 {
		metricsPort = 9091
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	metricsServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", metricsPort),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("[ai-moderation] /metrics listening on :%d", metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ai-moderation] metrics server error: %v", err)
		}
	}()

	// ── 初始化 AI 模型加载器（mock 或 onnxruntime）────────────────────────────
	modelCfg := ai_moderation.ModelConfig{
		ModelPath:          viper.GetString("aiModeration.modelPath"),
		ModelVersion:       viper.GetString("aiModeration.modelVersion"),
		ModelHash:          viper.GetString("aiModeration.modelHash"),
		Enabled:            viper.GetBool("aiModeration.enabled"),
		IntraOpNumThreads:  viper.GetInt("aiModeration.intraOpNumThreads"),
		EnableCpuMemArena:  viper.GetBool("aiModeration.enableCpuMemArena"),
		TimeoutMs:          viper.GetInt("aiModeration.timeoutMs"),
	}
	loader, err := ai_moderation.NewModelLoader(modelCfg)
	if err != nil {
		log.Fatalf("[ai-moderation] model loader init: %v", err)
	}
	defer func() { _ = loader.Close() }()
	log.Printf("[ai-moderation] model loaded: version=%s, enabled=%v, path=%s",
		modelCfg.ModelVersion, modelCfg.Enabled, modelCfg.ModelPath)

	// ── gRPC 服务 ────────────────────────────────────────────────────────────
	addr := config.Conf.Service["ai-moderation"].Address
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[ai-moderation] listen %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	// 健康检查（gRPC standard）
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("ai-moderation", healthpb.HealthCheckResponse_SERVING)
	if !modelCfg.Enabled {
		// mock 模式仍标记 SERVING（后续 task-046 接入真实模型后会校验文件存在性）
		healthServer.SetServingStatus("ai-moderation", healthpb.HealthCheckResponse_SERVING)
	}

	// 反射（便于 grpcurl 调试）
	reflection.Register(grpcServer)

	// 注册 AI Moderation gRPC 服务
	aiService := ai_moderation.NewServiceWithMode(loader, "mock", 0)
	ai_moderation_pb.RegisterAIModerationServiceServer(grpcServer, aiService)
	log.Printf("[ai-moderation] AI Moderation Service registered (mode=%s, version=%s)",
		"mock", modelCfg.ModelVersion)

	// 注册到 etcd
	serviceName := config.Conf.Service["ai-moderation"].Name
	cleanup, err := registerToEtcd(serviceName, addr)
	if err != nil {
		log.Fatalf("[ai-moderation] etcd register: %v", err)
	}
	defer cleanup()

	// ── 优雅关闭 ─────────────────────────────────────────────────────────────
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[ai-moderation] gRPC listening on %s", addr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("[ai-moderation] grpc serve: %v", err)
		}
	}()

	<-stopCh
	log.Printf("[ai-moderation] shutdown signal received")

	// 标记 health 为 NOT_SERVING（让 LB 摘除）
	healthServer.SetServingStatus("ai-moderation", healthpb.HealthCheckResponse_NOT_SERVING)

	// 给予 5 秒优雅关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		log.Printf("[ai-moderation] gRPC gracefully stopped")
	case <-shutdownCtx.Done():
		log.Printf("[ai-moderation] graceful stop timeout, forcing stop")
		grpcServer.Stop()
	}

	// 关闭 metrics server
	_ = metricsServer.Shutdown(shutdownCtx)

	// 更新启动指标
	metrics.RecordServiceStart()
}

// registerToEtcd 注册服务到 etcd（包装避免循环依赖）
func registerToEtcd(serviceName, addr string) (func(), error) {
	return discovery.Register(context.Background(), serviceName, addr)
}