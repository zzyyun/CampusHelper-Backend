// Package aiclient 提供 Content Service 调用 ai-moderation 服务的客户端封装。
//
// 设计目标：
//   - 统一的 gRPC 客户端接口（Content Service 不感知 ai-moderation 实现细节）
//   - 内置 800ms 超时控制
//   - 内置熔断器（sony/gobreaker）保护 ai-moderation 服务异常时不雪崩
//   - 提供降级 fallback：AI 不可用时返回默认 PASS
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md
//   - 熔断器位置在 Content Service 客户端侧（PRD rev2 修正 #1）
//   - ai-moderation 服务在 cmd/ai-moderation/
package aiclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ─── 公共配置 ────────────────────────────────────────────────────────────────

// Config 客户端配置
type Config struct {
	// Addr ai-moderation gRPC 服务地址（如 "127.0.0.1:50061"）
	Addr string
	// Timeout 单次调用总超时（默认 800ms）
	Timeout time.Duration
	// CircuitConfig 熔断器配置（nil 表示使用默认值）
	CircuitConfig *CircuitConfig
}

// ─── 错误定义 ────────────────────────────────────────────────────────────────

// ErrAIServiceUnavailable AI 服务不可用（连接失败/熔断/超时）
var ErrAIServiceUnavailable = errors.New("ai-moderation service unavailable")

// ─── ModerationClient 抽象接口 ───────────────────────────────────────────────

// ModerationClient AI 审核客户端接口（供 Content Service 依赖注入）
type ModerationClient interface {
	// ModerateText 同步审核文本
	// 参数：
	//   - ctx: 调用方上下文（必须带 trace_id）
	//   - text: 待审核文本
	//   - postID: 帖子 ID（用于审计）
	// 返回：
	//   - *ai_moderation_pb.ModerateTextResponse: AI 决策
	//   - error: 调用失败时返回 ErrAIServiceUnavailable（调用方应 fallback）
	ModerateText(ctx context.Context, text string, postID int64) (*ai_moderation_pb.ModerateTextResponse, error)
	// Close 关闭客户端连接
	Close() error
}

// ─── 客户端实现 ──────────────────────────────────────────────────────────────

// grpcClient 基于 gRPC 的客户端实现（含熔断器）
type grpcClient struct {
	conn        *grpc.ClientConn
	stub        ai_moderation_pb.AIModerationServiceClient
	timeout     time.Duration
	circuit     *CircuitBreaker
	serviceName string
}

// NewClient 创建 AI Moderation 客户端
//
// 参数：
//   - cfg: 客户端配置
//
// 返回：
//   - ModerationClient: 客户端接口
//   - error: 连接失败时返回
func NewClient(cfg Config) (ModerationClient, error) {
	if cfg.Addr == "" {
		return nil, errors.New("addr is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 800 * time.Millisecond
	}

	// 建立 gRPC 连接（生产应使用 TLS，本期内网明文）
	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", cfg.Addr, err)
	}

	// 初始化熔断器
	circuitCfg := DefaultCircuitConfig()
	if cfg.CircuitConfig != nil {
		circuitCfg = *cfg.CircuitConfig
	}
	circuit := NewCircuitBreaker(circuitCfg)

	stub := ai_moderation_pb.NewAIModerationServiceClient(conn)

	return &grpcClient{
		conn:        conn,
		stub:        stub,
		timeout:     timeout,
		circuit:     circuit,
		serviceName: "ai-moderation",
	}, nil
}

// ModerateText 实现 ModerationClient 接口
//
// 流程：
//  1. 通过熔断器调用 Execute
//  2. Execute 内部做：
//     - 熔断状态检查（open → 立即 fallback）
//     - gRPC 调用（带 timeout）
//     - 成功 → 关闭熔断或半开试探
//     - 失败 → 记录失败次数
func (c *grpcClient) ModerateText(ctx context.Context, text string, postID int64) (*ai_moderation_pb.ModerateTextResponse, error) {
	// 构造带 trace_id 的 ctx
	traceID, _ := ctx.Value("trace_id").(string)

	result, err := c.circuit.Execute(func() (interface{}, error) {
		callCtx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()

		resp, callErr := c.stub.ModerateText(callCtx, &ai_moderation_pb.ModerateTextRequest{
			Text:    text,
			TraceId: traceID,
			PostId:  postID,
		})
		if callErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrAIServiceUnavailable, callErr)
		}
		return resp, nil
	})

	if err != nil {
		log.Printf("[aiclient] ModerateText failed (post_id=%d, trace_id=%s): %v",
			postID, traceID, err)
		return nil, err
	}

	resp, ok := result.(*ai_moderation_pb.ModerateTextResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
	return resp, nil
}

// IsCircuitOpen 查询熔断器状态（供 metrics / debug 用）
func (c *grpcClient) IsCircuitOpen() bool {
	return c.circuit.IsOpen()
}

// Close 关闭连接
func (c *grpcClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ─── Fallback Response 辅助 ─────────────────────────────────────────────────

// NewFallbackResponse 构造 fallback 响应（AI 不可用时使用）
//
// 返回：Result=PASS, Status=DEGRADED, FallbackUsed=true
func NewFallbackResponse(modelVersion string) *ai_moderation_pb.ModerateTextResponse {
	return &ai_moderation_pb.ModerateTextResponse{
		Result:       ai_moderation_pb.ModerateTextResponse_PASS,
		Status:       ai_moderation_pb.ModerateTextResponse_DEGRADED,
		Confidence:   0.0,
		ModelVersion: modelVersion,
		FallbackUsed: true,
	}
}