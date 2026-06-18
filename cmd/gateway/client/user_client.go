package client

import (
	"context"
	"fmt"

	user_pb "go_projects/praProject1/PB/pb/user_pb"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/discovery"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var UserClient user_pb.UserServiceClient

// 初始化客户端（基于 etcd 服务发现拨号，非静态直连）
func InitUserClient() error {
	serviceName := config.Conf.Service["user"].Name
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
	if config.Conf.Service["user"].LoadBalance {
		opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return fmt.Errorf("dial user-service %s: %w", target, err)
	}
	UserClient = user_pb.NewUserServiceClient(conn)
	return nil
}

// injectTrace adds OTel TraceContext to outgoing gRPC metadata.
func InjectTrace(ctx context.Context) context.Context {
	return ctx // otelgrpc interceptor handles this automatically
}
