// Package ai_moderation 提供 AI 审核服务的 gRPC handler。
//
// task-040 (#91) 阶段：实现 ai_moderation.proto 生成的 AIModerationServiceServer 接口。
//
// 关键路径：
//   - ModerateText：800ms 超时，调用 loader.Infer
//   - HealthCheck：返回模型加载状态 + 版本
//   - 不在 Service 内实现熔断器（熔断器在客户端 pkg/aiclient）
package ai_moderation

import (
	"context"
	"time"

	"go_projects/praProject1/cmd/ai-moderation/metrics"
	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service 实现 ai_moderation_pb.AIModerationServiceServer 接口
type Service struct {
	ai_moderation_pb.UnimplementedAIModerationServiceServer

	loader         ModelLoader
	modelLoadedAt  time.Time
	modelLoadMode  string // "mock" 或 "onnxruntime"
	intraOpThreads int
}

// NewService 创建 Service 实例
func NewService(loader ModelLoader) *Service {
	return &Service{
		loader:         loader,
		modelLoadedAt:  time.Now(),
		modelLoadMode:  "mock", // 当前为 mock，task-046 接入 onnxruntime 后改为 "onnxruntime"
		intraOpThreads: 0,
	}
}

// NewServiceWithMode 创建 Service 实例（指定加载模式）
func NewServiceWithMode(loader ModelLoader, mode string, intraOpThreads int) *Service {
	return &Service{
		loader:         loader,
		modelLoadedAt:  time.Now(),
		modelLoadMode:  mode,
		intraOpThreads: intraOpThreads,
	}
}

// ModerateText 实现 gRPC handler：同步单条文本审核
func (s *Service) ModerateText(ctx context.Context, req *ai_moderation_pb.ModerateTextRequest) (*ai_moderation_pb.ModerateTextResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	// 空文本视为 PASS（mock 兜底，生产可改为 InvalidArgument）
	if req.Text == "" {
		return &ai_moderation_pb.ModerateTextResponse{
			Result:       ai_moderation_pb.ModerateTextResponse_PASS,
			Status:       ai_moderation_pb.ModerateTextResponse_SYNCED,
			Confidence:   1.0,
			ModelVersion: s.loader.Version(),
		}, nil
	}

	// 计算剩余超时（ctx deadline 优先，否则用默认 800ms）
	timeoutMs := 800
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < time.Duration(timeoutMs)*time.Millisecond {
			timeoutMs = int(remaining / time.Millisecond)
		}
		if timeoutMs <= 0 {
			// 已超时
			return s.degradedResponse("context deadline exceeded"), nil
		}
	}

	// 调用推理（带超时）
	infCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	infResult, err := s.loader.Infer(infCtx, req.Text)
	if err != nil {
		// 推理失败 → fallback（mock 模式下不应该触发）
		metrics.CallsTotal.WithLabelValues("fallback").Inc()
		return s.degradedResponse("infer failed: " + err.Error()), nil
	}

	// 记录 metrics
	metrics.CallsTotal.WithLabelValues(infResult.Result.String()).Inc()
	metrics.LatencySeconds.WithLabelValues("sync").Observe(float64(infResult.LatencyMs) / 1000.0)

	return &ai_moderation_pb.ModerateTextResponse{
		Result:       moderateTextResultToPb(infResult.Result),
		Status:       ai_moderation_pb.ModerateTextResponse_SYNCED,
		Confidence:   infResult.Confidence,
		Categories:   infResult.Categories,
		LatencyMs:    infResult.LatencyMs,
		ModelVersion: infResult.ModelVersion,
		FallbackUsed: infResult.FallbackUsed,
	}, nil
}

// HealthCheck 实现 gRPC handler：返回模型加载状态
func (s *Service) HealthCheck(ctx context.Context, _ *empty.Empty) (*ai_moderation_pb.HealthCheckResponse, error) {
	status := ai_moderation_pb.HealthCheckResponse_SERVING
	modelLoaded := s.loader != nil
	if !modelLoaded {
		status = ai_moderation_pb.HealthCheckResponse_NOT_SERVING
	}

	return &ai_moderation_pb.HealthCheckResponse{
		Status:            status,
		ModelLoaded:       modelLoaded,
		ModelVersion:      s.loader.Version(),
		ModelLoadTimestamp: s.modelLoadedAt.Unix(),
		Mode:              s.modelLoadMode,
		IntraOpThreads:    int32(s.intraOpThreads),
	}, nil
}

// degradedResponse 构造降级响应（fallback_used=true, status=DEGRADED）
func (s *Service) degradedResponse(reason string) *ai_moderation_pb.ModerateTextResponse {
	_ = reason // 日志记录在 metrics 中
	return &ai_moderation_pb.ModerateTextResponse{
		Result:       ai_moderation_pb.ModerateTextResponse_PASS, // fallback 默认放行
		Status:       ai_moderation_pb.ModerateTextResponse_DEGRADED,
		Confidence:   0.0,
		ModelVersion: s.loader.Version(),
		FallbackUsed: true,
	}
}

// moderateTextResultToPb 内部 Result → pb.Result 枚举转换
func moderateTextResultToPb(r Result) ai_moderation_pb.ModerateTextResponse_Result {
	switch r {
	case ResultPass:
		return ai_moderation_pb.ModerateTextResponse_PASS
	case ResultReview:
		return ai_moderation_pb.ModerateTextResponse_REVIEW
	case ResultBlock:
		return ai_moderation_pb.ModerateTextResponse_BLOCK
	default:
		// 未知值 → REVIEW（安全兜底，与 PRD 一致）
		return ai_moderation_pb.ModerateTextResponse_REVIEW
	}
}