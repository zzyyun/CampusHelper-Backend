// Package ai_moderation 提供 AI 审核服务的模型加载抽象层。
//
// 架构分层（自上而下）：
//   - Service（service.go）→ gRPC handler
//   - ModelLoader interface（types 子包）→ 加载器抽象
//   - MockLoader（本包 model.go）→ 默认实现，零 cgo 依赖
//   - onnxruntime 子包（独立子包）→ 真实 ONNX 推理，仅 -tags onnx_enabled 时编译
//
// 关键约定：
//   - 默认构建 = mock 模式（无 cgo 依赖，可 CGO_ENABLED=0 静态构建）
//   - 启用 ONNX：go build -tags onnx_enabled ./...
//   - 工厂注入：onnx_bridge.go（带 build tag）在 init() 中设置 onnxFactory
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

// OnnxFactoryFunc 创建真实 ONNX loader 的工厂函数签名。
//
// 注入时机：onnx_bridge.go（//go:build onnx_enabled）的 init()。
// 默认 nil：表示当前构建未启用 onnx_enabled tag，真实模式不可用。
type OnnxFactoryFunc func(cfg ModelConfig) (ModelLoader, error)

// onnxFactory 全局工厂变量（默认 nil，启用 onnx_enabled 时由 onnx_bridge.go 注入）。
var onnxFactory OnnxFactoryFunc

// ── Mock 实现 ────────────────────────────────────────────────────────────────

// MockLoader mock 模式实现：固定返回 PASS，用于骨架阶段与单元测试。
// 真实 ONNX 实现见 internal/ai_moderation/onnxruntime（仅 -tags onnx_enabled 编译）。
type MockLoader struct {
	version string
}

// NewMockLoader 创建 mock loader。
func NewMockLoader(version string) *MockLoader {
	return &MockLoader{version: version}
}

// Infer mock 模式：固定返回 PASS，confidence=1.0。
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

// Version 返回模型版本。
func (m *MockLoader) Version() string { return m.version }

// Close mock 模式无需释放。
func (m *MockLoader) Close() error { return nil }

// ── Factory ──────────────────────────────────────────────────────────────────

// NewModelLoader 根据配置选择 mock 或真实实现。
//
// 真实模式（cfg.Enabled=true）：
//   - 启用 -tags onnx_enabled 时，由 onnx_bridge.go 注入的工厂调用 onnxruntime 子包
//   - 未启用 build tag 时，onnxFactory 为 nil，返回明确错误（避免运行时才发现）
//
// mock 模式（cfg.Enabled=false）：直接返回 MockLoader。
func NewModelLoader(cfg ModelConfig) (ModelLoader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid model config: %w", err)
	}

	if !cfg.Enabled {
		// mock 模式
		return NewMockLoader(cfg.ModelVersion), nil
	}

	// 真实模式：通过工厂回调（仅 onnx_enabled 时可用）
	if onnxFactory == nil {
		return nil, errors.New(
			"real ONNX loader unavailable: build with -tags onnx_enabled " +
				"(see internal/ai_moderation/onnx_bridge.go)")
	}
	return onnxFactory(cfg)
}

// verifyModelHash 校验模型文件 SHA256。
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