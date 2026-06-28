// Package onnxruntime 提供 onnxruntime-go 真实推理能力（cgo 依赖）。
//
// 本包所有源文件均带 //go:build onnx_enabled 标签：
//   - 默认构建（mock 模式）：本包不参与编译，零 cgo 依赖，可 CGO_ENABLED=0 构建
//   - 启用构建（go build -tags onnx_enabled）：ai_moderation 服务的真实 ONNX 推理路径
//
// 运行时依赖：本包需要 libonnxruntime 动态库（libonnxruntime.so / onnxruntime.dll）。
// 本地开发可下载：https://github.com/microsoft/onnxruntime/releases
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md
//   - 任务 task-046 (#93)

//go:build onnx_enabled
// +build onnx_enabled

package onnxruntime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	ort "github.com/yalue/onnxruntime_go"

	"go_projects/praProject1/internal/ai_moderation/types"
)

// OnnxLoader ONNX Runtime 模型加载器（真实推理）。
type OnnxLoader struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *BertTokenizer
	version   string
	timeoutMs int
}

// NewOnnxLoader 创建 ONNX loader。
//
// 参数：
//   - modelPath: .onnx 模型文件路径
//   - version: 模型版本标识
//   - timeoutMs: 单次推理超时
//
// 返回：
//   - *OnnxLoader: 加载器实例
//   - error: 加载失败时返回
func NewOnnxLoader(modelPath, version string, timeoutMs int) (*OnnxLoader, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model file not accessible: %w", err)
	}

	// 创建 ONNX runtime 环境（需 libonnxruntime 动态库）
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("initialize onnxruntime env: %w", err)
	}

	// 创建 DynamicAdvancedSession
	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask"}, // 输入名（取决于模型导出）
		[]string{"logits"},                     // 输出名
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create onnx session: %w", err)
	}

	// 创建 tokenizer（基于 bert-base-chinese vocab）
	tokenizer, err := NewBertTokenizer()
	if err != nil {
		_ = session.Destroy()
		return nil, fmt.Errorf("create tokenizer: %w", err)
	}

	return &OnnxLoader{
		session:   session,
		tokenizer: tokenizer,
		version:   version,
		timeoutMs: timeoutMs,
	}, nil
}

// Infer 执行推理（调用 ONNX Runtime）。
func (l *OnnxLoader) Infer(ctx context.Context, text string) (*types.InferenceResult, error) {
	start := time.Now()

	// Tokenize → input_ids + attention_mask
	inputIDs, attentionMask, err := l.tokenizer.Encode(text, 512)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	// 构造 ONNX 输入 tensor
	shape := []int64{1, int64(len(inputIDs))}
	inputIDsTensor, err := ort.NewTensor(shape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attentionMaskTensor, err := ort.NewTensor(shape, attentionMask)
	if err != nil {
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	defer attentionMaskTensor.Destroy()

	// 运行模型（输入 inputs, 自动分配 outputs）
	outputs := make([]ort.Value, 1) // 占位，Run 会填充
	if err := l.session.Run([]ort.Value{inputIDsTensor, attentionMaskTensor}, outputs); err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}
	defer outputs[0].Destroy()

	// 解析输出 → softmax → confidence
	logits, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, errors.New("unexpected output tensor type")
	}
	logitsData := logits.GetData()

	confidence, err := softmaxAndMax(logitsData)
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}

	// 决策（依据 PRD § Feature 1 阈值）
	resultEnum := decideResult(confidence)
	categories := []string{} // TODO: 多标签分类模型可输出类别

	return &types.InferenceResult{
		Result:       resultEnum,
		Confidence:   confidence,
		Categories:   categories,
		LatencyMs:    time.Since(start).Milliseconds(),
		ModelVersion: l.version,
		FallbackUsed: false,
	}, nil
}

// Version 返回模型版本。
func (l *OnnxLoader) Version() string { return l.version }

// Close 释放 ONNX session 资源。
func (l *OnnxLoader) Close() error {
	if l.session != nil {
		return l.session.Destroy()
	}
	return nil
}

// ─── 决策辅助 ──────────────────────────────────────────────────────────────

// decideResult 根据 confidence 决策 Result。
//
// 阈值（PRD § Feature 1）：
//   - confidence ≥ 0.9 → BLOCK
//   - 0.5 ≤ confidence < 0.9 → REVIEW
//   - confidence < 0.5 → PASS
//
// 注意：模型输出是二分类 logits，单值经过 softmax 后表示"违规概率"。
// 当前简化决策：违规概率 < 0.5 → PASS；0.5-0.9 → REVIEW；≥ 0.9 → BLOCK。
func decideResult(violationProb float32) types.Result {
	if violationProb >= 0.9 {
		return types.ResultBlock
	}
	if violationProb >= 0.5 {
		return types.ResultReview
	}
	return types.ResultPass
}

// softmaxAndMax 对单元素 logits 应用 softmax 并返回 max 概率。
//
// 注：当前模型输出是 [batch, 2] 形状的 logits（正常/违规）。
// 取 batch=0, class=违规 的 softmax 概率。
func softmaxAndMax(logits []float32) (float32, error) {
	if len(logits) < 2 {
		return 0, fmt.Errorf("expected at least 2 logits, got %d", len(logits))
	}

	// 取第一个样本的两个 logits
	z0 := logits[0]
	z1 := logits[1]

	// softmax with numerical stability (subtract max)
	// 这里 x - max 范围在 (-∞, 0]，exp 范围在 (0, 1]
	maxVal := z0
	if z1 > maxVal {
		maxVal = z1
	}
	d0 := z0 - maxVal // ≤ 0
	d1 := z1 - maxVal // ≤ 0
	exp0 := float32(math.Exp(float64(d0)))
	exp1 := float32(math.Exp(float64(d1)))
	sum := exp0 + exp1
	if sum < 1e-10 {
		return 0, errors.New("softmax sum too small")
	}
	probViolation := exp1 / sum

	return probViolation, nil
}

// expFast 保留作为 fallback（实际使用 math.Exp，标准库精度更高）。
//
// 对于 softmax 输入（≤ 0），可使用泰勒级数展开。
// 当前实现直接使用 math.Exp 以保证精度与稳定性。
func expFast(x float64) float64 {
	return math.Exp(x)
}

// ─── Factory ────────────────────────────────────────────────────────────────

// TryCreateOnnxLoader 尝试创建 ONNX loader，失败时返回 error（不 fallback）。
//
// 调用方（如 ai_moderation 主包）可决定 fallback 策略：
//   - 启动时：失败则退出（避免无声降级）
//   - 运行时：失败则 fallback 到 mock
func TryCreateOnnxLoader(cfg types.ModelConfig) (types.ModelLoader, error) {
	if !cfg.Enabled {
		return nil, errors.New("onnx loader requires enabled=true")
	}
	if cfg.ModelPath == "" {
		return nil, errors.New("model_path required")
	}

	log.Printf("[onnx] Loading model from %s (version=%s)...", cfg.ModelPath, cfg.ModelVersion)
	loader, err := NewOnnxLoader(cfg.ModelPath, cfg.ModelVersion, cfg.TimeoutMs)
	if err != nil {
		return nil, fmt.Errorf("create onnx loader: %w", err)
	}
	log.Printf("[onnx] Model loaded successfully (version=%s, threads=%d)",
		cfg.ModelVersion, cfg.IntraOpNumThreads)
	return loader, nil
}