// Package onnxruntime 提供 onnxruntime-go 真实推理能力（cgo 依赖）。
//
// 本包所有源文件均带 //go:build onnx_enabled 标签：
//   - 默认构建（mock 模式）：本包不参与编译，零 cgo 依赖
//   - 启用构建（go build -tags onnx_enabled）：真实 ONNX 推理路径
//
// 模型信息（unitary/toxic-bert）：
//   - 输入：input_ids [1,512], attention_mask [1,512], token_type_ids [1,512]
//   - 输出：logits [1,6]（toxic/severe_toxic/obscene/threat/insult/identity_hate）
//   - 词表：vocab_size=30522（英文 BERT）

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

// 模型输出类别名称（与 config.json id2label 一致）。
var categoryNames = []string{
	"toxic",        // 0
	"severe_toxic", // 1
	"obscene",      // 2
	"threat",       // 3
	"insult",       // 4
	"identity_hate", // 5
}

// OnnxLoader ONNX Runtime 模型加载器（真实推理）。
type OnnxLoader struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *BertTokenizer
	version   string
	timeoutMs int
}

// NewOnnxLoader 创建 ONNX loader。
func NewOnnxLoader(modelPath, vocabPath, version string, timeoutMs int) (*OnnxLoader, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model file not accessible: %w", err)
	}

	// 创建 ONNX runtime 环境
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("initialize onnxruntime env: %w", err)
	}

	// 创建 DynamicAdvancedSession（3 个输入，1 个输出）
	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"logits"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create onnx session: %w", err)
	}

	// 创建 tokenizer（基于真实 vocab.txt）
	tokenizer, err := NewBertTokenizerFromFile(vocabPath)
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

	// 检查 context 是否已超时
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Tokenize → input_ids + attention_mask + token_type_ids
	inputIDs, attentionMask, tokenTypeIDs, err := l.tokenizer.Encode(text, 512)
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

	tokenTypeIDsTensor, err := ort.NewTensor(shape, tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
	}
	defer tokenTypeIDsTensor.Destroy()

	// 运行模型
	outputs := make([]ort.Value, 1)
	if err := l.session.Run(
		[]ort.Value{inputIDsTensor, attentionMaskTensor, tokenTypeIDsTensor},
		outputs,
	); err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}
	defer outputs[0].Destroy()

	// 解析输出 logits [1, 6]
	logitsTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, errors.New("unexpected output tensor type")
	}
	logitsData := logitsTensor.GetData()

	// softmax → 6 个类别的概率
	probs := softmax6(logitsData)

	// 找到最大违规概率及其类别
	maxProb, maxIdx := maxWithIndex(probs)
	categories := []string{}
	if maxProb >= 0.5 {
		categories = append(categories, categoryNames[maxIdx])
	}

	// 决策
	resultEnum := decideResult(maxProb)

	return &types.InferenceResult{
		Result:       resultEnum,
		Confidence:   maxProb,
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

// decideResult 根据最大违规概率决策 Result。
//
// 阈值（PRD § Feature 1）：
//   - max_prob ≥ 0.9 → BLOCK（违规，拦截）
//   - 0.5 ≤ max_prob < 0.9 → REVIEW（不确定，进人工池）
//   - max_prob < 0.5 → PASS（正常，放行）
func decideResult(maxProb float32) types.Result {
	if maxProb >= 0.9 {
		return types.ResultBlock
	}
	if maxProb >= 0.5 {
		return types.ResultReview
	}
	return types.ResultPass
}

// softmax6 对 6 元素 logits 应用 softmax，返回各分类概率。
func softmax6(logits []float32) []float32 {
	if len(logits) < 6 {
		// 不足 6 个时补零
		padded := make([]float32, 6)
		copy(padded, logits)
		logits = padded
	}

	// 数值稳定 softmax: subtract max
	maxVal := logits[0]
	for i := 1; i < 6; i++ {
		if logits[i] > maxVal {
			maxVal = logits[i]
		}
	}

	expSum := float32(0)
	probs := make([]float32, 6)
	for i := 0; i < 6; i++ {
		probs[i] = float32(math.Exp(float64(logits[i] - maxVal)))
		expSum += probs[i]
	}
	for i := 0; i < 6; i++ {
		probs[i] /= expSum
	}

	return probs
}

// maxWithIndex 返回切片中的最大值及其索引。
func maxWithIndex(vals []float32) (float32, int) {
	if len(vals) == 0 {
		return 0, 0
	}
	maxVal := vals[0]
	maxIdx := 0
	for i := 1; i < len(vals); i++ {
		if vals[i] > maxVal {
			maxVal = vals[i]
			maxIdx = i
		}
	}
	return maxVal, maxIdx
}

// ─── Factory ────────────────────────────────────────────────────────────────

// TryCreateOnnxLoader 尝试创建 ONNX loader。
func TryCreateOnnxLoader(cfg types.ModelConfig) (types.ModelLoader, error) {
	if !cfg.Enabled {
		return nil, errors.New("onnx loader requires enabled=true")
	}
	if cfg.ModelPath == "" {
		return nil, errors.New("model_path required")
	}

	// vocab.txt 与模型文件同目录
	vocabPath := cfg.ModelPath[:len(cfg.ModelPath)-len("moderation_v1.onnx")] + "vocab.txt"
	if _, err := os.Stat(vocabPath); err != nil {
		// 尝试从 models/ 目录加载
		vocabPath = "models/vocab.txt"
		if _, err2 := os.Stat(vocabPath); err2 != nil {
			return nil, fmt.Errorf("vocab.txt not found near model: %w", err)
		}
	}

	log.Printf("[onnx] Loading model from %s (version=%s)...", cfg.ModelPath, cfg.ModelVersion)
	loader, err := NewOnnxLoader(cfg.ModelPath, vocabPath, cfg.ModelVersion, cfg.TimeoutMs)
	if err != nil {
		return nil, fmt.Errorf("create onnx loader: %w", err)
	}
	log.Printf("[onnx] Model loaded successfully (version=%s)", cfg.ModelVersion)
	return loader, nil
}
