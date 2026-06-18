package discovery

import (
	"context"
	"fmt"
	"log"
	"time"

	"go_projects/praProject1/pkg/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
)

// leaseTTL 是注册租约的有效期（秒）。
// 服务崩溃时，etcd 会在该时长后自动回收 key，从而完成「自动注销」。
const leaseTTL = 15

// keyPrefix 返回某服务在 etcd 中的 endpoint 前缀（不含结尾斜杠）。
// endpoints.Manager 要求所有 key 形如 keyPrefix + "/" + instance，
// 因此 prefix 本身不能带结尾 "/"，否则 key 校验会得到双斜杠。
//
//	"service/user-service"
func keyPrefix(serviceName string) string {
	return "service/" + serviceName
}

// instanceKey 返回单个实例在 etcd 中的完整 key。
//
//	"service/user-service/127.0.0.1:10001"
func instanceKey(serviceName, addr string) string {
	return keyPrefix(serviceName) + "/" + addr
}

// Register 将本实例以 serviceName 注册到 etcd，并通过 lease 续约保持在线。
// 返回的 cleanup 用于优雅关闭时主动注销（撤销 lease 即可让 key 立即失效）。
//
// 用法：
//
//	cleanup, err := discovery.Register(ctx, "user-service", addr)
//	if err != nil { log.Fatalf(...) }
//	defer cleanup()
func Register(ctx context.Context, serviceName, addr string) (func(), error) {
	if etcd.Client == nil {
		return nil, fmt.Errorf("etcd client not initialized")
	}

	// 1. 申请租约
	leaseResp, err := etcd.Client.Grant(ctx, leaseTTL)
	if err != nil {
		return nil, fmt.Errorf("grant lease: %w", err)
	}
	leaseID := leaseResp.ID

	// 2. 注册 endpoint，绑定到上面申请的 lease
	manager, err := endpoints.NewManager(etcd.Client, keyPrefix(serviceName))
	if err != nil {
		return nil, fmt.Errorf("new endpoint manager: %w", err)
	}
	key := instanceKey(serviceName, addr)
	if err = manager.AddEndpoint(ctx, key, endpoints.Endpoint{Addr: addr}, clientv3.WithLease(leaseID)); err != nil {
		return nil, fmt.Errorf("add endpoint %s: %w", key, err)
	}

	// 3. 起独立 goroutine 续约；用独立的、可取消的 context，避免随入参 ctx 退出而中断。
	//    KeepAlive 一旦底层连接异常会关闭返回的 channel，此时记日志即可（key 会在 TTL 内被回收）。
	keepCtx, keepCancel := context.WithCancel(context.Background())
	go func() {
		ch, kaErr := etcd.Client.KeepAlive(keepCtx, leaseID)
		if kaErr != nil {
			log.Printf("[discovery] keepalive for %s start failed: %v", key, kaErr)
			return
		}
		for range ch {
			// 消费续约响应即可，无需处理内容
		}
	}()

	fmt.Printf("[discovery] registered %s @ %s\n", serviceName, addr)

	// 4. cleanup：停止续约 + 撤销 lease（etcd 会立即删除绑定该 lease 的 key）
	cleanup := func() {
		keepCancel() // 停止 KeepAlive goroutine

		deregCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 先主动删除 endpoint，再撤销 lease 兜底
		if err := manager.DeleteEndpoint(deregCtx, key); err != nil {
			log.Printf("[discovery] delete endpoint %s: %v", key, err)
		}
		_, _ = etcd.Client.Revoke(deregCtx, leaseID)
		fmt.Printf("[discovery] deregistered %s @ %s\n", serviceName, addr)
	}
	return cleanup, nil
}
