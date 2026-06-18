package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/router"
	"go_projects/praProject1/config"
	pkg_etcd "go_projects/praProject1/pkg/etcd"
	"go_projects/praProject1/pkg/tracer"
)

func main() {
	// ── Config ──────────────────────────────────────────────────────────────
	config.InitConfig("")

	// ── Tracer ──────────────────────────────────────────────────────────────
	shutdown, err := tracer.Init("gateway")
	if err != nil {
		log.Printf("[WARN] tracer init: %v (continuing without tracing)\n", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// ── etcd (服务发现依赖) ───────────────────────────────────────────────────
	pkg_etcd.InitEtcd()
	defer pkg_etcd.CloseEtcd()

	// ── gRPC Clients ─────────────────────────────────────────────────────────
	if err = client.InitUserClient(); err != nil {
		log.Fatalf("user client init: %v", err)
	}
	fmt.Println("[gateway] user-service client connected")

	// ── HTTP Server ───────────────────────────────────────────────────────────
	addr := config.Conf.Gateway.Address
	srv := &http.Server{
		Addr:         addr,
		Handler:      router.NewRouter(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("[gateway] HTTP listening on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-quit
	fmt.Println("[gateway] shutting down…")
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
