// Package onnxruntime 提供 onnxruntime-go 真实推理能力（cgo 依赖）。
//
// 本包���有源文件均带 //go:build onnx_enabled 标签：
//   - 默认构建（mock 模式）：本包不参与编译，零 cgo 依赖
//   - 启��构建（go build -tags onnx_enabled）：真实 ONNX 推理路径
//
// 支持的模型：
//   - 英文 toxic-bert (6 分类): toxic/severe_toxic/obscene/threat/insult/identity_hate
//   - 中文内容审核模型 (3 分类): 安全/疑似违��/违规
//   - 任意 BERT 架构分类模型（自动适配类别数）

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
	"path/filepath"
	"time"

	ort "github.com/yalue/onnxruntime_go"

	"go_projects/praProject1/internal/ai_moderation/types"
)

// OnnxLoader ONNX Runtime 模型加载器（真实推理）。
type OnnxLoader struct {
	session       *ort.DynamicAdvancedSession
	tokenizer     *BertTokenizer
	version       string
	timeoutMs     int
	numClasses    int     // 模型输出类别数（从 ONNX 输出 shape 或配置读取）
	categoryNames []string // 类别名称
}

// NewOnnxLoader 创建 ONNX loader。
//
// 参数：
//   - modelPath: ONNX 模���文件路径
//   - vocabPath: vocab.txt 路径
//   - version: 模型版本
//   - timeoutMs: 推理超时
//   - numClasses: 类别数（0=自动从 ONNX 推断）
//   - categoryNames: 类别名称列表
func NewOnnxLoader(modelPath, vocabPath, version string, timeoutMs int, numClasses int, categoryNames []string) (*OnnxLoader, error) {
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

	// 自动检测输出类别数：优先用配置值，否则运行一次 dummy 输入获取 shape
	if numClasses <= 0 {
		numClasses, err = detectOutputClasses(session)
		if err != nil {
			_ = session.Destroy()
			_ = tokenizer.Close()
			return nil, fmt.Errorf("detect output classes: %w", err)
		}
		log.Printf("[onnx] Auto-detected %d output classes", numClasses)
	}

	// 类别名称回退：数量不匹配时使���默认名
	if len(categoryNames) != numClasses {
		log.Printf("[onnx] category names count (%d) != output classes (%d), using default names", len(categoryNames), numClasses)
		categoryNames = makeDefaultCategoryNames(numClasses)
	}

	return &OnnxLoader{
		session:       session,
		tokenizer:     tokenizer,
		version:       version,
		timeoutMs:     timeoutMs,
		numClasses:    numClasses,
		categoryNames: categoryNames,
	}, nil
}

// detectOutputClasses 通过一次 dummy 推理检测模型输出类别数。
func detectOutputClasses(session *ort.DynamicAdvancedSession) (int, error) {
	// 用最小输入（序列长度=2）做一次推理
	shape := []int64{1, 2}
	dummy := []int64{101, 102} // [CLS] [SEP]

	inputIDs, err := ort.NewTensor(shape, dummy)
	if err != nil {
		return 0, fmt.Errorf("create dummy input_ids: %w", err)
	}
	defer inputIDs.Destroy()

	attentionMask, err := ort.NewTensor(shape, []int64{1, 1})
	if err != nil {
		return 0, fmt.Errorf("create dummy attention_mask: %w", err)
	}
	defer attentionMask.Destroy()

	tokenTypeIDs, err := ort.NewTensor(shape, []int64{0, 0})
	if err != nil {
		return 0, fmt.Errorf("create dummy token_type_ids: %w", err)
	}
	defer tokenTypeIDs.Destroy()

	outputs := make([]ort.Value, 1)
	if err := session.Run(
		[]ort.Value{inputIDs, attentionMask, tokenTypeIDs},
		outputs,
	); err != nil {
		return 0, fmt.Errorf("dummy run: %w", err)
	}
	defer outputs[0].Destroy()

	logitsTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return 0, errors.New("unexpected output tensor type")
	}

	return len(logitsTensor.GetData()), nil
}

// makeDefaultCategoryNames 生成默认的类别名称��
func makeDefaultCategoryNames(numClasses int) []string {
	names := make([]string, numClasses)
	for i := 0; i < numClasses; i++ {
		names[i] = fmt.Sprintf("class_%d", i)
	}
	return names
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

	// 解析输出 logits
	logitsTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, errors.New("unexpected output tensor type")
	}
	logitsData := logitsTensor.GetData()

	// softmax（自适应类别数）
	probs := softmax(logitsData)

	// 对中文二分类(安全/违规)模型：取"违规"类别的概率作为违规置信度
	// ��英文 multi-label 模型：取所有类别中最大的违规概率
	maxProb, _ := maxWithIndex(probs)

	// 拼接命中的类别���概率 ≥ 0.5 的类别）
	categories := make([]string, 0)
	for i, p := range probs {
		if i < len(l.categoryNames) && p >= 0.5 {
			categories = append(categories, l.categoryNames[i])
		}
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

// Version 返回模��版本。
func (l *OnnxLoader) Version() string { return l.version }

// Close 释放 ONNX session 资源。
func (l *OnnxLoader) Close() error {
	var firstErr error
	if l.session != nil {
		if err := l.session.Destroy(); err != nil {
			firstErr = err
		}
	}
	if l.tokenizer != nil {
		if err := l.tokenizer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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

// softmax 对 logits 应用 softmax，返回各分类概率���
//
// 自适应类别数：根据输入 logits 长度自动计算。
func softmax(logits []float32) []float32 {
	n := len(logits)
	if n == 0 {
		return nil
	}

	// 数��稳定 softmax: subtract max
	maxVal := logits[0]
	for i := 1; i < n; i++ {
		if logits[i] > maxVal {
			maxVal = logits[i]
		}
	}

	expSum := float32(0)
	probs := make([]float32, n)
	for i := 0; i < n; i++ {
		probs[i] = float32(math.Exp(float64(logits[i] - maxVal)))
		expSum += probs[i]
	}
	for i := 0; i < n; i++ {
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

	// vocab.txt 优先使用配置路径，否则与模型文件���目录下查找
	var vocabPath string
	if cfg.VocabPath != "" {
		vocabPath = cfg.VocabPath
	} else {
		modelDir := filepath.Dir(cfg.ModelPath)
		vocabPath = filepath.Join(modelDir, "vocab.txt")
	}
	if _, err := os.Stat(vocabPath); err != nil {
		// 尝试从 /models/ 目录加载（Docker volume 挂载路径）
		vocabPath = "/models/vocab.txt"
		if _, err2 := os.Stat(vocabPath); err2 != nil {
			return nil, fmt.Errorf("vocab.txt not found: %w", err)
		}
	}

	log.Printf("[onnx] Loading model from %s (version=%s)...", cfg.ModelPath, cfg.ModelVersion)
	loader, err := NewOnnxLoader(
		cfg.ModelPath,
		vocabPath,
		cfg.ModelVersion,
		cfg.TimeoutMs,
		cfg.NumClasses,
		cfg.CategoryNames,
	)
	if err != nil {
		return nil, fmt.Errorf("create onnx loader: %w", err)
	}
	log.Printf("[onnx] Model loaded successfully (version=%s, classes=%d)", cfg.ModelVersion, loader.numClasses)
	return loader, nil
}
