// Package types 定义 ai_moderation 服务的公共 API 类型（Result/Config/Loader/Result）。
//
// 这些类型被 ai_moderation 主包与 onnxruntime 子包共同使用，独立成包是为了
// 打破"主包 import onnxruntime 子包 ↔ 子包 import 主包"形成的循环依赖。
//
// 引入新类型时务必：保持零外部依赖（仅 stdlib），便于交叉编译。
package types

import (
	"context"
	"errors"
	"fmt"
)

// Result AI 审核结果枚举（与 PB/ai_moderation.proto 一致）。
type Result int32

const (
	// ResultPass 内容判定为正常。
	ResultPass Result = 0
	// ResultReview AI 不确定，进人工池。
	ResultReview Result = 1
	// ResultBlock 内容违规，拦截。
	ResultBlock Result = 2
)

// String Result 的可读字符串。
func (r Result) String() string {
	switch r {
	case ResultPass:
		return "pass"
	case ResultReview:
		return "review"
	case ResultBlock:
		return "block"
	default:
		return fmt.Sprintf("unknown(%d)", int32(r))
	}
}

// InferenceResult 单次 AI 推理输出。
type InferenceResult struct {
	Result       Result    // pass/review/block
	Confidence   float32   // 0.0 - 1.0
	Categories   []string  // 命中类别（如 ["涉政", "广告引流"]）
	LatencyMs    int64     // 推理耗时
	ModelVersion string    // 模型版本
	FallbackUsed bool      // 是否走降级（mock 模式恒为 true，真实模式下推理异常时为 true）
}

// ModelConfig 模型加载配置。
type ModelConfig struct {
	ModelPath         string // ONNX 模型文件路径（volume 挂载）
	ModelVersion      string // 模型版本标识
	ModelHash         string // 模型文件 SHA256（启动时校验）
	Enabled           bool   // true=加载真实 ONNX 模型, false=mock 模式
	IntraOpNumThreads int    // ONNX 内部线程数
	EnableCpuMemArena bool   // ONNX CPU memory arena
	TimeoutMs         int    // 单次推理超时
}

// Validate 校验配置合法性。
func (c *ModelConfig) Validate() error {
	if c.ModelVersion == "" {
		return errors.New("model_version is required")
	}
	if c.Enabled && c.ModelPath == "" {
		return errors.New("model_path is required when enabled=true")
	}
	if c.TimeoutMs <= 0 {
		c.TimeoutMs = 800 // 默认 800ms
	}
	if c.IntraOpNumThreads <= 0 {
		c.IntraOpNumThreads = 4
	}
	return nil
}

// ModelLoader 模型加载器抽象接口。
type ModelLoader interface {
	// Infer 执行推理，返回结果。
	// mock 模式：固定返回 ResultPass。
	// 真实模式：调用 ONNX runtime。
	Infer(ctx context.Context, text string) (*InferenceResult, error)

	// Version 返回当前模型版本。
	Version() string

	// Close 释放资源（关闭 ONNX session）。
	Close() error
}