// Package ai_moderation 提供本地 AI 内容审核能力。
//
// 本包当前为骨架实现（mock 模式），待 task-046 (#93) 接入 onnxruntime-go 后切换到真实推理。
//
// 接口分层：
//   - ModelLoader：负责加载模型（mock / onnxruntime 二选一）
//   - Service：gRPC handler（待 task-040 proto 生成后实现）
//
// 设计要点：
//   - 启动时校验模型配置（mock 模式跳过文件校验）
//   - 提供统一的 Infer 接口供 gRPC handler 调用
//   - 不在此处实现熔断器（熔断器在客户端侧 pkg/aiclient）
package ai_moderation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"
)

// Result AI 审核结果枚举（与 PB/ai_moderation.proto 一致）
type Result int32

const (
	// ResultPass 内容判定为正常
	ResultPass Result = 0
	// ResultReview AI 不确定，进人工池
	ResultReview Result = 1
	// ResultBlock 内容违规，拦截
	ResultBlock Result = 2
)

// String Result 的可读字符串
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

// InferenceResult 单次 AI 推理输出
type InferenceResult struct {
	Result        Result    // pass/review/block
	Confidence    float32   // 0.0 - 1.0
	Categories    []string  // 命中类别（如 ["涉政", "广告引流"]）
	LatencyMs     int64     // 推理耗时
	ModelVersion  string    // 模型版本
	FallbackUsed  bool      // 是否走降级（mock 模式恒为 true，真实模式下推理异常时为 true）
}

// ModelConfig 模型加载配置
type ModelConfig struct {
	ModelPath          string // ONNX 模型文件路径（volume 挂载）
	ModelVersion       string // 模型版本标识
	ModelHash          string // 模型文件 SHA256（启动时校验）
	Enabled            bool   // true=加载真实 ONNX 模型, false=mock 模式
	IntraOpNumThreads  int    // ONNX 内部线程数
	EnableCpuMemArena  bool   // ONNX CPU memory arena
	TimeoutMs          int    // 单次推理超时
}

// Validate 校验配置合法性
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

// ModelLoader 模型加载器抽象接口
type ModelLoader interface {
	// Infer 执行推理，返回结果。
	// mock 模式：固定返回 ResultPass。
	// 真实模式：调用 ONNX runtime。
	Infer(ctx context.Context, text string) (*InferenceResult, error)

	// Version 返回当前模型版本
	Version() string

	// Close 释放资源（关闭 ONNX session）
	Close() error
}

// ── Mock 实现 ────────────────────────────────────────────────────────────────

// MockLoader mock 模式实现：固定返回 PASS，用于骨架阶段。
// 真实 ONNX 实现见 task-046 (#93)。
type MockLoader struct {
	version string
}

// NewMockLoader 创建 mock loader
func NewMockLoader(version string) *MockLoader {
	return &MockLoader{version: version}
}

// Infer mock 模式：固定返回 PASS，confidence=1.0
func (m *MockLoader) Infer(ctx context.Context, text string) (*InferenceResult, error) {
	start := time.Now()
	// mock 推理"耗时"约 5ms（模拟真实模型加载后的预热开销）
	select {
	case <-time.After(5 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &InferenceResult{
		Result:       ResultPass,
		Confidence:   1.0,
		Categories:   []string{},
		LatencyMs:    time.Since(start).Milliseconds(),
		ModelVersion: m.version,
		FallbackUsed: true, // mock 模式标记为 fallback，便于审计区分
	}, nil
}

// Version 返回模型版本
func (m *MockLoader) Version() string { return m.version }

// Close mock 模式无需释放
func (m *MockLoader) Close() error { return nil }

// ── Factory ──────────────────────────────────────────────────────────────────

// NewModelLoader 根据配置选择 mock 或真实实现。
// 当前（task-039 阶段）仅实现 mock，真实 ONNX loader 在 task-046 (#93) 完成。
func NewModelLoader(cfg ModelConfig) (ModelLoader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid model config: %w", err)
	}

	if !cfg.Enabled {
		// mock 模式
		return NewMockLoader(cfg.ModelVersion), nil
	}

	// 真实模式：校验模型文件存在 + SHA256
	if cfg.ModelHash != "" {
		if err := verifyModelHash(cfg.ModelPath, cfg.ModelHash); err != nil {
			return nil, fmt.Errorf("model hash verify failed: %w", err)
		}
	}

	// 待 task-046 接入 onnxruntime-go
	// 当前为占位，避免编译报错
	return NewMockLoader(cfg.ModelVersion + "-fallback"),
		fmt.Errorf("real ONNX loader not implemented yet (task-046 pending), fallback to mock")
}

// verifyModelHash 校验模型文件 SHA256
func verifyModelHash(path, expected string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read model file: %w", err)
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if actual != expected {
		return fmt.Errorf("hash mismatch: expected=%s, actual=%s", expected, actual)
	}
	return nil
}