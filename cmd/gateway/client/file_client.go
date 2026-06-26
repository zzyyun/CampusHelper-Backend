package client

import (
	"fmt"

	file_pb "go_projects/praProject1/PB/pb/file_pb"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/discovery"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FileClient 全局 File Service 客户端（用于 GetFile / DeleteFile）。
var FileClient file_pb.FileServiceClient

// InitFileClient 基于 etcd resolver 建立到 File Service 的 gRPC 连接。
func InitFileClient() error {
	serviceName := config.Conf.Service["file"].Name
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
	if config.Conf.Service["file"].LoadBalance {
		opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return fmt.Errorf("dial file-service %s: %w", target, err)
	}
	FileClient = file_pb.NewFileServiceClient(conn)
	return nil
}