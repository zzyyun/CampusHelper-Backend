package etcd

import (
	"context"
	"fmt"
	"log"
	"time"

	"go_projects/praProject1/config"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Client 是全局 etcd 客户端单例，供 discovery 与未来的分布式锁等复用。
var Client *clientv3.Client

// InitEtcd 初始化全局 etcd 客户端并对集群做一次活性探活。
// 失败策略与 MySQL/Redis 一致：直接 Fatal，因为服务发现属于关键基础设施。
func InitEtcd() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Conf.Etcd.Address,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("etcd client init: %v", err)
	}

	// 探活：随便挑一个 endpoint 做一次 Status，确认集群可达
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := cli.Status(ctx, config.Conf.Etcd.Address[0])
	if err != nil {
		log.Fatalf("etcd status probe failed: %v", err)
	}

	Client = cli
	fmt.Printf("[etcd] connected, cluster version=%s\n", resp.Version)
}

// CloseEtcd 优雅关闭 etcd 客户端，供服务退出时调用。
func CloseEtcd() {
	if Client == nil {
		return
	}
	if err := Client.Close(); err != nil {
		log.Printf("[etcd] close: %v", err)
	}
}
