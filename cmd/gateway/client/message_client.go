package client

import (
	"fmt"

	message_pb "go_projects/praProject1/PB/pb/message_pb"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/discovery"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MessageClient 全局 Message Service 客户端。
var MessageClient message_pb.MessageServiceClient

// InitMessageClient 基于 etcd resolver 建立到 Message Service 的 gRPC 连接。
func InitMessageClient() error {
	serviceName := config.Conf.Service["message"].Name
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
	if config.Conf.Service["message"].LoadBalance {
		opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return fmt.Errorf("dial message-service %s: %w", target, err)
	}
	MessageClient = message_pb.NewMessageServiceClient(conn)
	return nil
}