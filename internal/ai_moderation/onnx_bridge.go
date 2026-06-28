// Package ai_moderation - onnx_bridge.go 在 build tag 启用时把 onnxruntime 子包
// 注入到主包的 NewModelLoader 真实模式分支。
//
// 本文件带 //go:build onnx_enabled 标签：
//   - 默认构建（mock 模式）：本文件被排除，零 cgo 依赖
//   - 启用构建：init() 把 onnxruntime.TryCreateOnnxLoader 注入到主包 onnxFactory 变量
//
// 设计动机：避免"主包 import onnxruntime ↔ onnxruntime import 主包"循环依赖。
// 实际依赖链：types ← ai_moderation（主包） ← onnxruntime ← onnx_bridge.go（注册回调）

//go:build onnx_enabled
// +build onnx_enabled

package ai_moderation

import (
	"go_projects/praProject1/internal/ai_moderation/onnxruntime"
)

// init 在编译期（启用 onnx_enabled 时）注入真实 ONNX loader 工厂。
func init() {
	onnxFactory = func(cfg ModelConfig) (ModelLoader, error) {
		// 可选：模型文件 SHA256 校验（仅在配置提供 hash 时执行）
		if cfg.ModelHash != "" {
			if err := verifyModelHash(cfg.ModelPath, cfg.ModelHash); err != nil {
				return nil, err
			}
		}
		return onnxruntime.TryCreateOnnxLoader(cfg)
	}
}