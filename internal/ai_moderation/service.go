// Package ai_moderation 提供 AI 审核服务的 gRPC handler 骨架。
//
// 当前 task-039 阶段：
//   - 定义 Service 结构体与 ModerationServiceServer interface
//   - 提供 Mock 实现（待 task-040 生成 pb 后接入 gRPC 注册）
//
// task-040 (#91) 将生成 PB/pb/ai_moderation.pb.go 与 grpc.pb.go，
// 届时本文件会被改造为实现生成的 AIModerationServiceServer 接口。
package ai_moderation

import (
	"context"
	"time"

	"go_projects/praProject1/cmd/ai-moderation/metrics"
)

// Service AI 审核服务（占位实现，待 task-040 接入）
type Service struct {
	loader ModelLoader
}

// NewService 创建 Service
func NewService(loader ModelLoader) *Service {
	return &Service{loader: loader}
}

// ModerateTextRequest 输入请求（占位类型，待 task-040 替换为 pb 类型）
type ModerateTextRequest struct {
	Text    string
	TraceID string
	PostID  int64
}

// ModerateTextResponse 输出响应（占位类型，待 task-040 替换为 pb 类型）
type ModerateTextResponse struct {
	Result       int32    // Result 枚举值
	Status       int32    // Status 枚举值（0=synced, 1=degraded, 2=async）
	Confidence   float32
	Categories   []string
	LatencyMs    int64
	ModelVersion string
	FallbackUsed bool
}

// ModerateText 同步单条文本审核。
// task-039 阶段：直接调用 loader.Infer，转换枚举值。
// task-040 阶段：方法签名将与生成的 gRPC pb 一致。
func (s *Service) ModerateText(ctx context.Context, req *ModerateTextRequest) (*ModerateTextResponse, error) {
	if req.Text == "" {
		// 视为 PASS（mock 兜底），生产可改为 InvalidArgument
		return &ModerateTextResponse{
			Result:       int32(ResultPass),
			Status:       0, // synced
			Confidence:   1.0,
			ModelVersion: s.loader.Version(),
		}, nil
	}

	// 调用推理（带超时）
	timeoutMs := 800
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < time.Duration(timeoutMs)*time.Millisecond {
			timeoutMs = int(remaining / time.Millisecond)
		}
	}
	infCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	infResult, err := s.loader.Infer(infCtx, req.Text)
	if err != nil {
		// 推理失败 → fallback（mock 模式下不应该触发）
		metrics.CallsTotal.WithLabelValues("fallback").Inc()
		return &ModerateTextResponse{
			Result:       int32(ResultPass), // fallback 默认放行
			Status:       1,                 // degraded
			Confidence:   0.0,
			ModelVersion: s.loader.Version(),
			FallbackUsed: true,
		}, nil
	}

	// 记录 metrics
	metrics.CallsTotal.WithLabelValues(infResult.Result.String()).Inc()
	metrics.LatencySeconds.WithLabelValues("sync").Observe(float64(infResult.LatencyMs) / 1000.0)

	return &ModerateTextResponse{
		Result:       int32(infResult.Result),
		Status:       0, // synced
		Confidence:   infResult.Confidence,
		Categories:   infResult.Categories,
		LatencyMs:    infResult.LatencyMs,
		ModelVersion: infResult.ModelVersion,
		FallbackUsed: infResult.FallbackUsed,
	}, nil
}