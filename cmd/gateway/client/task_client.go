package client

import (
	"fmt"

	task_pb "go_projects/praProject1/PB/pb/task_pb"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/discovery"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TaskClient 全局 Task Service 客户端。
var TaskClient task_pb.TaskServiceClient

// InitTaskClient 基于 etcd resolver 建立到 Task Service 的 gRPC 连接。
func InitTaskClient() error {
	serviceName := config.Conf.Service["task"].Name
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
	if config.Conf.Service["task"].LoadBalance {
		opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return fmt.Errorf("dial task-service %s: %w", target, err)
	}
	TaskClient = task_pb.NewTaskServiceClient(conn)
	return nil
}
