// Package ai_moderation - aliases.go 提供对 types 子包的类型别名。
//
// 通过 type alias 让外部调用方继续使用 ai_moderation.ModelConfig / Result /
// ModelLoader 等简短路径，无需感知底层 types 子包拆分。
//
// 依赖方向：types ← ai_moderation（单向），与 onnxruntime 子包无环。
package ai_moderation

import "go_projects/praProject1/internal/ai_moderation/types"

// Result 别名 → types.Result（AI 审核结果枚举）。
type Result = types.Result

// InferenceResult 别名 → types.InferenceResult（单次推理输出）。
type InferenceResult = types.InferenceResult

// ModelConfig 别名 → types.ModelConfig（模型加载配置）。
type ModelConfig = types.ModelConfig

// ModelLoader 别名 → types.ModelLoader（模型加载器接口）。
type ModelLoader = types.ModelLoader

// 枚举值别名（方便外部 if r == ResultPass 等使用）。
const (
	ResultPass   = types.ResultPass
	ResultReview = types.ResultReview
	ResultBlock  = types.ResultBlock
)