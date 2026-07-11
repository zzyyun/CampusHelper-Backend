// Package ai_moderation - verify.go 提供模型文件完整性校验。
//
// 设计：
//   - verifyModelHash 在 onnx_bridge.go init() 中注册，仅 onnx_enabled 编译
//   - 使用 crypto/sha256 校验下载的模型文件 Hash
//   - 与 types 子包解耦，避免循环依赖

//go:build onnx_enabled
// +build onnx_enabled

package ai_moderation

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// verifyModelHash 校验模型文件的 SHA256 哈希值。
//
// ��用时机：onnx_bridge.go init() ���入工厂函数中，创建 ONNX loader 前。
// 若配置提供了 ModelHash，先校验文件完整性再加载，防止运行损坏的模型。
func verifyModelHash(modelPath, expectedHash string) error {
	if expectedHash == "" {
		return nil // 未配置 hash 则跳过校验
	}

	f, err := os.Open(modelPath)
	if err != nil {
		return fmt.Errorf("verify hash: open model file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("verify hash: read model file: %w", err)
	}

	actualHash := fmt.Sprintf("%x", h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("model hash mismatch: expected %s, got %s (model file may be corrupted or tampered)", expectedHash, actualHash)
	}

	return nil
}
