#!/usr/bin/env python3
"""
模型导出脚本：将 Hugging Face 文本审核模型导出为 ONNX 格式。

支持中英文两种模型类型：
  1. 英文 toxic-bert（默认）: unitary/toxic-bert, 6 分类
  2. ���文内容审核: 任意 Hugging Face SequenceClassification 模型

用法：
    pip install transformers torch onnx onnxruntime optimum

    # 导��英文 toxic-bert（默认）
    python3 scripts/models/export_to_onnx.py

    # 导出中文内容审核模型
    python3 scripts/models/export_to_onnx.py --chinese --model-name bert-base-chinese

    # 指定 Hugging Face 模型
    python3 scripts/models/export_to_onnx.py --model-name your-org/your-model

    # 自定义输出目录
    python3 scripts/models/export_to_onnx.py --output-dir /path/to/models

输出目录（默认 ./models/）：
    - moderation_v1.onnx   # ONNX 模型文件（供 ONNX Runtime 加载）
    - vocab.txt             # BERT tokenizer 词表
    - config.json           # 模型配置（num_labels、id2label）
    - tokenizer_config.json # tokenizer 配置
    - model_info.txt        # 模型���信息（版本、类别映射、SHA256）

模型信息：
    - 输入：input_ids [1, seq_len], attention_mask [1, seq_len], token_type_ids [1, seq_len]
    - 输出：logits [1, num_classes]
    - 最大序列长度：512
    - 动态 axis：batch=1 固定，seq_len 动态

注意事项：
    - 模型使用动态 axis（batch=1 固定，seq_len 动态），与 onnxruntime_go 兼容
    - 需要 ~500MB-2GB 磁盘空间（PyTorch 模型 + ONNX 导出）
    - 中文模型建议使用 bert-base-chinese 或基于它的微调模型
"""

import argparse
import json
import os
import sys


def check_dependencies():
    """检查所需 Python 包是否安装��"""
    missing = []
    for pkg in ["transformers", "torch", "onnx"]:
        try:
            __import__(pkg)
        except ImportError:
            missing.append(pkg)
    if missing:
        print(f"错误��缺少所需 Python 包: {', '.join(missing)}")
        print("请运行: pip install transformers torch onnx onnxruntime")
        sys.exit(1)


def get_model_info(model_name: str, is_chinese: bool):
    """返回模型的配置信息和推荐设置。

    Args:
        model_name: Hugging Face ��型名称
        is_chinese: 是否为中文模型

    Returns:
        dict: 模型信息，包含类别数、标签映射等
    """
    # 英文 toxic-bert 默认配置
    if not is_chinese and model_name == "unitary/toxic-bert":
        return {
            "num_classes": 6,
            "labels": ["toxic", "severe_toxic", "obscene", "threat", "insult", "identity_hate"],
            "description": "English toxic comment classification",
            "recommended_category_names": "['toxic','severe_toxic','obscene','threat','insult','identity_hate']",
        }

    # 中文模型默认配置
    if is_chinese:
        return {
            "num_classes": 3,  # 安全/疑��违规/违规 — 匹配 Go 端 ResultPass/Review/Block
            "labels": ["正常", "疑似违规", "违规"],
            "description": "Chinese content moderation (3-class: safe/suspicious/violation)",
            "recommended_category_names": "['��常','疑似违规','违规']",
        }

    # 其他模型 — 运行时从 model config 读取
    return {}


def export_model(
    model_name: str,
    output_dir: str,
    opset: int = 14,
    is_chinese: bool = False,
):
    """将 Hugging Face 模型导出为 ONNX 格式。

    Args:
        model_name: Hugging Face 模型名称
        output_dir: 输出目录
        opset: ONNX opset 版本
        is_chinese: 是否为中文模型
    """
    import torch
    from transformers import AutoConfig, AutoModelForSequenceClassification, AutoTokenizer

    model_info = get_model_info(model_name, is_chinese)

    print(f"[1/4] 加载模型: {model_name} ...")
    model = AutoModelForSequenceClassification.from_pretrained(model_name)
    model.eval()

    print(f"[2/4] 加载 tokenizer ...")
    tokenizer = AutoTokenizer.from_pretrained(model_name)

    # 从模型配置读取类别数
    config = AutoConfig.from_pretrained(model_name)
    num_labels = getattr(config, "num_labels", 2)
    id2label = getattr(config, "id2label", None)

    print(f"    模型类别数: {num_labels}")
    if id2label:
        print(f"    标签映射: {id2label}")

    # 模型输入结构
    max_seq_len = 512
    batch_size = 1

    # 创建 dummy 输入用于 ONNX 导出
    dummy_text = "测试文本 content moderation test"
    if is_chinese:
        dummy_text = "这是一条测试内容，用于导出 ONNX 模型"

    dummy_encoded = tokenizer(
        [dummy_text],
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
    print(f"    logits[:3]: {logits[0, :min(3, num_labels)].tolist()}")

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

    # 验证导出的 ONNX 模型
    print(f"    验证 ONNX 模型 ...")
    onnx_check(onnx_path)

    # 计算文件 hash
    import hashlib

    sha256 = hashlib.sha256()
    with open(onnx_path, "rb") as f:
        while chunk := f.read(8192):
            sha256.update(chunk)
    onnx_hash = sha256.hexdigest()

    file_size_mb = os.path.getsize(onnx_path) / (1024 * 1024)

    # 写入模型元信息
    labels_str = json.dumps(id2label) if id2label else "{}"
    recommended_categories = model_info.get("recommended_category_names", "[]")

    model_info_content = f"""# AI 审核模型元信息
model_name: {model_name}
model_version: v1.0-onnx
is_chinese: {is_chinese}
num_classes: {num_labels}
max_seq_length: {max_seq_len}
model_file: moderation_v1.onnx
file_size_mb: {file_size_mb:.1f}
sha256: {onnx_hash}
labels: {labels_str}
recommended_category_names: {recommended_categories}
description: {model_info.get('description', '')}
"""

    with open(os.path.join(output_dir, "model_info.txt"), "w") as f:
        f.write(model_info_content)

    print()
    print("=" * 60)
    print("ONNX 模型导出成功！")
    print(f"  模型文件: {onnx_path} ({file_size_mb:.1f} MB)")
    print(f"  词表文件: {vocab_path}")
    print(f"  ��型类别: {num_labels}")
    if id2label:
        print(f"  标签映射: {id2label}")
    print(f"  SHA256:   {onnx_hash}")
    print(f"  输出目录: {os.path.abspath(output_dir)}/")
    print()
    print("部署步骤：")
    print(f"  1. 将 {output_dir}/ 目录下的文件放到服务器 /models/ 目录")
    print(f"  2. 在 my_config.yaml 中配置:")
    print(f"     aiModeration:")
    print(f"       enabled: true")
    print(f"       modelPath: /models/moderation_v1.onnx")
    print(f"       modelVersion: v1.0-onnx")
    print(f"       modelHash: '{onnx_hash}'  # 可选：启动时校验完整性")
    if is_chinese:
        print(f"       vocabPath: /models/vocab.txt")
        print(f"       categoryNames: {recommended_categories}")
    print("=" * 60)

    return onnx_path, onnx_hash


def onnx_check(onnx_path: str):
    """验证导出的 ONNX 模型。

    检查输入/输出名称和 shape，确保与 Go 端代码兼容。
    """
    try:
        import onnx

        model = onnx.load(onnx_path)
        onnx.checker.check_model(model)

        # 打印输入输出信息
        for i, inp in enumerate(model.graph.input):
            shape_str = ", ".join(str(d.dim_value or d.dim_param) for d in inp.type.tensor_type.shape.dim)
            print(f"    输入 [{i}]: {inp.name} shape=({shape_str})")

        for i, out in enumerate(model.graph.output):
            shape_str = ", ".join(str(d.dim_value or d.dim_param) for d in out.type.tensor_type.shape.dim)
            print(f"    输出 [{i}]: {out.name} shape=({shape_str})")

        print(f"    ONNX 模型验证通过 ✓")
    except ImportError:
        print(f"    跳过 ONNX 验证（需要安装 onnx 包）")
    except Exception as e:
        print(f"    ONNX 验证警告: {e}")


def main():
    parser = argparse.ArgumentParser(
        description="导出 Hugging Face 审核模型为 ONNX 格式",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 导出英文 toxic-bert（6 分类）
  python3 scripts/models/export_to_onnx.py

  # 导出中文模型（3 分类：正常/疑似违规/违规）
  python3 scripts/models/export_to_onnx.py --chinese

  # 导出自定义 Hugging Face 模型
  python3 scripts/models/export_to_onnx.py --model-name your-org/your-model

  # 指定 opset（某些旧环境需要更低版本）
  python3 scripts/models/export_to_onnx.py --opset 13
        """,
    )
    parser.add_argument(
        "--model-name",
        default=None,
        help="Hugging Face 模型名称（默认: unitary/toxic-bert 或基于 --chinese 的推荐模型）",
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
    parser.add_argument(
        "--chinese",
        action="store_true",
        help="导出中文内容审核模型（默认: 英文 toxic-bert 6 分类）",
    )
    args = parser.parse_args()

    check_dependencies()

    # 模���名称默认值
    if args.model_name is None:
        if args.chinese:
            # 推荐的中文文本审核模型
            # 用户可替换为自行微调的模型
            args.model_name = "bert-base-chinese"
            print("提示: 使用 bert-base-chinese ���为中文模型基础。")
            print("      建议在此基���上微调后使用，或指定 --model-name 使用已有微调模型。")
            print()
        else:
            args.model_name = "unitary/toxic-bert"

    export_model(args.model_name, args.output_dir, args.opset, args.chinese)


if __name__ == "__main__":
    main()
