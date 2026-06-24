package client

import (
	"fmt"

	content_pb "go_projects/praProject1/PB/pb/content_pb"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/discovery"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ContentClient 全局 Content Service 客户端，供 handler 调用。
// 初始化采用与 UserClient 相同的 etcd 服务发现 + otel 追踪链路。
var ContentClient content_pb.ContentServiceClient

// InitContentClient 基于 etcd resolver 建立到 Content Service 的 gRPC 连接。
//
// 与 InitUserClient 的差异仅在 service 名称与 client 构造，其它链路配置
// （insecure / otelgrpc / 可选 round_robin）保持一致以便行为统一。
func InitContentClient() error {
	serviceName := config.Conf.Service["content"].Name
	target := discovery.EtcdTarget(serviceName)

	resOpt, err := discovery.ResolverOption()
	if err != nil {
		return fmt.Errorf("etcd resolver for %s: %w", serviceName, err)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		resOpt,
	}
	// 负载均衡策略：配置开启时用 round_robin，否则用默认 pick_first
	if config.Conf.Service["content"].LoadBalance {
		opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return fmt.Errorf("dial content-service %s: %w", target, err)
	}
	ContentClient = content_pb.NewContentServiceClient(conn)
	return nil
}