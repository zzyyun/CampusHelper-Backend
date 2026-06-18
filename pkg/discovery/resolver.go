package discovery

import (
	"fmt"

	"go_projects/praProject1/pkg/etcd"

	etcdnaming "go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	gresolver "google.golang.org/grpc/resolver"
)

// etcdBuilder 是全局 etcd resolver builder，避免每次拨号重复创建。
var etcdBuilder gresolver.Builder

// EtcdTarget 构造 gRPC 拨号所用的 target 字符串，由 etcd resolver scheme 解析。
// resolver 会按此前缀在 etcd 下做范围查询，与 register.go 里的 keyPrefix 一致。
//
//	discovery.EtcdTarget("user-service")
//	=> "etcd:///service/user-service"
func EtcdTarget(serviceName string) string {
	return "etcd:///" + keyPrefix(serviceName)
}

// ResolverOption 返回一个可加入 grpc.NewClient 的 DialOption，
// 用于把 etcd resolver 注册到 gRPC，使 "etcd:///" 前缀的 target 能被识别。
//
// 用法：
//
//	resOpt, err := discovery.ResolverOption()
//	conn, err := grpc.NewClient(discovery.EtcdTarget("user-service"), resOpt, ...)
func ResolverOption() (grpc.DialOption, error) {
	if etcd.Client == nil {
		return nil, fmt.Errorf("etcd client not initialized")
	}
	if etcdBuilder == nil {
		b, err := etcdnaming.NewBuilder(etcd.Client)
		if err != nil {
			return nil, fmt.Errorf("new etcd resolver builder: %w", err)
		}
		etcdBuilder = b
	}
	return grpc.WithResolvers(etcdBuilder), nil
}
