#!/usr/bin/env python3
"""
模型导出脚本：将 Hugging Face toxic-bert 模型导出为 ONNX ��式。

用法：
    pip install transformers torch onnx onnxruntime optimum
    python3 scripts/models/export_to_onnx.py

输出目录（默认 ./models/）：
    - moderation_v1.onnx   # ONNX 模型文件（供 ONNX Runtime 加载）
    - vocab.txt             # BERT tokenizer 词表���30522 个 token）
    - config.json           # 模型配置（num_labels=6 等）
    - tokenizer_config.json # tokenizer 配置

模型信息：
    - 来源：unitary/toxic-bert (Hugging Face)
    - 架构：bert-base-uncased → 6 分类
    - 分类标签：toxic / severe_toxic / obscene / threat / insult / identity_hate
    - 输入：input_ids [batch, seq_len] + attention_mask + token_type_ids
    - 输出：logits [batch, 6]
    - 最大序列长度：512

注意事项：
    - 模型使用动态 axis（batch=1 固定，seq_len 动态），与 onnxruntime_go 兼容
    - 需要 ~500MB 磁盘���间（PyTorch 模型 + ONNX 导出）
"""

import argparse
import os
import sys


def check_dependencies():
    """检查所需 Python 包是否安装���"""
    missing = []
    for pkg in ["transformers", "torch", "onnx"]:
        try:
            __import__(pkg)
        except ImportError:
            missing.append(pkg)
    if missing:
        print(f"错误：缺少所需 Python 包: {', '.join(missing)}")
        print("请运行: pip install transformers torch onnx onnxruntime")
        sys.exit(1)


def export_model(model_name: str, output_dir: str, opset: int = 14):
    """将 Hugging Face 模型导出为 ONNX 格式。"""
    import torch
    from transformers import AutoModelForSequenceClassification, AutoTokenizer

    print(f"[1/4] 加��模型: {model_name} ...")
    model = AutoModelForSequenceClassification.from_pretrained(model_name)
    model.eval()

    print(f"[2/4] 加载 tokenizer ...")
    tokenizer = AutoTokenizer.from_pretrained(model_name)

    # 模型输入结构
    max_seq_len = 512
    batch_size = 1

    # 创建 dummy 输入用于 ONNX ��出
    dummy_text = ["hello world"]  # 短文本作为 dummy input
    dummy_encoded = tokenizer(
        dummy_text,
        max_length=max_seq_len,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )

    input_ids = dummy_encoded["input_ids"]
    attention_mask = dummy_encoded["attention_mask"]
    token_type_ids = dummy_encoded.get("token_type_ids", torch.zeros_like(input_ids))

    # 验证输入 shape
    print(f"    input_ids shape: {list(input_ids.shape)}")
    print(f"    attention_mask shape: {list(attention_mask.shape)}")
    print(f"    token_type_ids shape: {list(token_type_ids.shape)}")

    # 验证模型推理
    print(f"[3/4] 验证 PyTorch 推理 ...")
    with torch.no_grad():
        outputs = model(input_ids, attention_mask=attention_mask, token_type_ids=token_type_ids)
    logits = outputs.logits
    print(f"    logits shape: {list(logits.shape)}")
    print(f"    logits values: {logits[0].tolist()}")

    # 导出 ONNX
    os.makedirs(output_dir, exist_ok=True)
    onnx_path = os.path.join(output_dir, "moderation_v1.onnx")

    print(f"[4/4] 导出 ONNX: {onnx_path} (opset={opset}) ...")

    # 使用动态 axis：batch 固定 1，seq_len 动态
    dynamic_axes = {
        "input_ids": {0: "batch_size", 1: "sequence_length"},
        "attention_mask": {0: "batch_size", 1: "sequence_length"},
        "token_type_ids": {0: "batch_size", 1: "sequence_length"},
        "logits": {0: "batch_size"},
    }

    torch.onnx.export(
        model,
        (input_ids, attention_mask, token_type_ids),
        onnx_path,
        input_names=["input_ids", "attention_mask", "token_type_ids"],
        output_names=["logits"],
        dynamic_axes=dynamic_axes,
        opset_version=opset,
        do_constant_folding=True,
    )

    # 保存 tokenizer 文件
    vocab_path = os.path.join(output_dir, "vocab.txt")
    tokenizer.save_pretrained(output_dir)

    # 计算文件 hash
    import hashlib

    sha256 = hashlib.sha256()
    with open(onnx_path, "rb") as f:
        while chunk := f.read(8192):
            sha256.update(chunk)
    onnx_hash = sha256.hexdigest()

    file_size_mb = os.path.getsize(onnx_path) / (1024 * 1024)

    print()
    print("=" * 60)
    print("ONNX 模型导出成功！")
    print(f"  模型文件: {onnx_path} ({file_size_mb:.1f} MB)")
    print(f"  词表文件: {vocab_path}")
    print(f"  SHA256:   {onnx_hash}")
    print(f"  输出目录: {os.path.abspath(output_dir)}/")
    print()
    print("���署步骤：")
    print(f"  1. 将 {output_dir}/ 目录下的文件放到���务器 /models/ 目录")
    print(f"  2. 在 my_config.yaml 中配置:")
    print(f"     aiModeration:")
    print(f"       enabled: true")
    print(f"       modelPath: /models/moderation_v1.onnx")
    print(f"       modelVersion: v1.0-onnx")
    print(f"       modelHash: '{onnx_hash}'  # 可选：启动时校���完整性")
    print("=" * 60)

    return onnx_path, onnx_hash


def main():
    parser = argparse.ArgumentParser(description="导出 toxic-bert 模型为 ONNX 格式")
    parser.add_argument(
        "--model-name",
        default="unitary/toxic-bert",
        help="Hugging Face 模型名称（默认: unitary/toxic-bert）",
    )
    parser.add_argument(
        "--output-dir",
        default="./models",
        help="输出目录（默认: ./models）",
    )
    parser.add_argument(
        "--opset",
        type=int,
        default=14,
        help="ONNX opset 版本（默认: 14）",
    )
    args = parser.parse_args()

    check_dependencies()
    export_model(args.model_name, args.output_dir, args.opset)


if __name__ == "__main__":
    main()
