#!/bin/bash
# 模型文件下载/导出脚本
#
# 用途：下载或导出 AI 审核���务所需的 ONNX ��型文件 + tokenizer 词表。
#
# 支持两种模式：
#   1. 本地 Python 导出（推荐）— 导出任意 Hugging Face 模型为 ONNX
#   2. 从远程 URL 下载预导出模型 — 下载 .tar.gz ��式的模型包
#
# 用法：
#   # 导出英文 toxic-bert（默认 6 分类）
#   bash scripts/models/download_model.sh
#
#   # 导出中文内容审核模型（3 分类：正常/疑似违��/违规）
#   bash scripts/models/download_model.sh --chinese
#
#   # 导出指定 Hugging Face 模型
#   bash scripts/models/download_model.sh --model your-org/your-model
#
#   # 从远程下载
#   MODEL_URL=https://your-oss.com/models.tar.gz bash scripts/models/download_model.sh
#
#   # 打包模型用于上传
#   bash scripts/models/download_model.sh --package
#
#   # 指定输出目录
#   MODEL_DIR=/path/to/models bash scripts/models/download_model.sh
#
# 输出文件：
#   $MODEL_DIR/moderation_v1.onnx   # ONNX 模型
#   $MODEL_DIR/vocab.txt             # BERT tokenizer 词表
#   $MODEL_DIR/config.json           # 模型配置
#   $MODEL_DIR/model_info.txt        # 模型元信息（版本/hash/类别映射）

set -euo pipefail

# ── 默认配置 ──────────────────────────────────────────────────────────────
MODEL_DIR="${MODEL_DIR:-./models}"
MODEL_VERSION="${MODEL_VERSION:-v1.0}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ── 辅助函数 ──────────────────────────────────────────────────────────────

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }
success() { echo "✓ $*"; }
fail() { echo "✗ $*" >&2; exit 1; }

# 检查命令是否存在
require_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "缺少命令: $1，请先安装"
}

# ── 模式 1: 本地 Python 导出 ─────────────────────────────────────────────

local_export() {
    local dest_dir="$1"
    local chinese="$2"
    local model_name="$3"

    require_cmd python3

    local export_script="$SCRIPT_DIR/export_to_onnx.py"
    if [[ ! -f "$export_script" ]]; then
        fail "找不到导出脚本: $export_script"
    fi

    log "本地 Python 导出 ONNX 模型..."
    log "这可能需要几分钟，请耐心等待..."

    local extra_args=()
    if [[ -n "$model_name" ]]; then
        extra_args+=(--model-name "$model_name")
    fi
    if [[ "$chinese" == "true" ]]; then
        extra_args+=(--chinese)
    fi

    cd "$PROJECT_ROOT"
    python3 "$export_script" \
        --output-dir "$dest_dir" \
        "${extra_args[@]}"

    success "本地导出完成"
}

# ── 模式 2: 从远程 URL 下载 ─────────────────────────────────────────────

download_from_url() {
    local url="$1"
    local dest_dir="$2"

    require_cmd curl
    require_cmd tar

    log "从远程 URL 下载模型文件..."
    log "  URL: $url"

    local tmpdir
    tmpdir="$(mktemp -d)"
    # shellcheck disable=SC2064
    trap "rm -rf '$tmpdir'" EXIT

    local archive="$tmpdir/model.tar.gz"

    log "下载中..."
    if ! curl -fsSL --retry 3 --retry-delay 5 --connect-timeout 30 \
         --max-time 600 -o "$archive" "$url"; then
        fail "下载失败: $url"
    fi
    success "下载完成"

    log "解压到: $dest_dir"
    mkdir -p "$dest_dir"
    tar -xzf "$archive" -C "$dest_dir"

    # 验证必要文件
    verify_model_files "$dest_dir"
}

# ── 验证模型文件完整性 ──────────────────────────────────────────────────

verify_model_files() {
    local model_dir="$1"
    local errors=0

    log "验证模���文件..."

    local required=("moderation_v1.onnx" "vocab.txt")
    for f in "${required[@]}"; do
        if [[ -f "$model_dir/$f" ]]; then
            local size
            size=$(stat -c%s "$model_dir/$f" 2>/dev/null || stat -f%z "$model_dir/$f" 2>/dev/null || echo 0)
            success "$f ($(numfmt --to=iec 2>/dev/null || echo "${size}") bytes)"
        else
            fail "缺少文件: $model_dir/$f"
            errors=$((errors + 1))
        fi
    done

    if [[ $errors -gt 0 ]]; then
        fail "模型文件不完整，缺少 $errors 个文件"
    fi
}

# ── 写入模型元信息 ──────────────────────────────────────────────────────

write_model_info() {
    local model_dir="$1"
    local onnx_path="$model_dir/moderation_v1.onnx"

    if [[ ! -f "$onnx_path" ]]; then
        return
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        local hash
        hash=$(sha256sum "$onnx_path" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        local hash
        hash=$(shasum -a 256 "$onnx_path" | awk '{print $1}')
    else
        local hash="unknown"
    fi

    local file_size
    file_size=$(stat -c%s "$onnx_path" 2>/dev/null || stat -f%z "$onnx_path" 2>/dev/null || echo 0)

    cat > "$model_dir/model_info.txt" <<EOF
# AI 审核模型元信息
model_version: $MODEL_VERSION
model_file: moderation_v1.onnx
file_size_bytes: $file_size
sha256: $hash
download_time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

    log "模��元信息已写��: $model_dir/model_info.txt"
    echo ""
    cat "$model_dir/model_info.txt" 2>/dev/null || true
}

# ── 打包���型（用于上传到 OSS）───────────────────────────────────────────

package_model() {
    local model_dir="$1"

    require_cmd tar

    local package="models-${MODEL_VERSION}.tar.gz"
    log "打包模���文件: $package"

    cd "$model_dir"
    local files=()
    for f in moderation_v1.onnx vocab.txt config.json model_info.txt; do
        if [[ -f "$f" ]]; then
            files+=("$f")
        fi
    done
    tar -czf "$PROJECT_ROOT/$package" "${files[@]}"

    cd "$PROJECT_ROOT"
    local pkg_size
    pkg_size=$(stat -c%s "$package" 2>/dev/null || stat -f%z "$package" 2>/dev/null || echo 0)
    success "模型包已创建: $package ($(numfmt --to=iec 2>/dev/null || echo "${pkg_size}") bytes)"
    echo ""
    echo "上传到 OSS 后可配置："
    echo "  MODEL_URL=https://your-oss.com/$package"
}

# ── 主逻辑 ──────────────────────────────────────────────────────────────

main() {
    local mode="export"
    local chinese=false
    local model_name=""

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --chinese)
                chinese=true
                shift
                ;;
            --model)
                model_name="$2"
                shift 2
                ;;
            --package)
                mode="package"
                shift
                ;;
            --url)
                mode="download"
                MODEL_URL="$2"
                shift 2
                ;;
            *)
                echo "未知参数: $1"
                echo "用法: $0 [--chinese] [--model <name>] [--package] [--url <URL>]"
                echo ""
                echo "示例:"
                echo "  $0                    # 导出英文 toxic-bert"
                echo "  $0 --chinese           # 导出中文内容审核模型"
                echo "  $0 --model your/model  # 导出自定义���型"
                echo "  $0 --package           # 打包已有模型"
                exit 1
                ;;
        esac
    done

    echo "╔══════════════════════════════════════════════════════════╗"
    echo "║     AI 审核模型文件准备工具                               ║"
    if [[ -n "$model_name" ]]; then
        echo "║     模型: $model_name"
    elif [[ "$chinese" == "true" ]]; then
        echo "║     模型: 中文内容审核 (安全/疑似违规/违规)              ║"
    else
        echo "║     模型: unitary/toxic-bert (英文 6 分类)             ║"
    fi
    echo "╚══════════════════════════════════════════════════════════╝"
    echo ""

    case "$mode" in
        export)
            if [[ -n "$MODEL_URL" ]] && [[ -z "$model_name" ]]; then
                # 有 URL 时走下载模式
                download_from_url "$MODEL_URL" "$MODEL_DIR"
                verify_model_files "$MODEL_DIR"
                write_model_info "$MODEL_DIR"

                echo ""
                success "模型文件准备完成！输出目录: $(cd "$MODEL_DIR" && pwd)"
                echo ""
                echo "下一步——在 my_config.yaml 中配置："
                echo "  aiModeration:"
                echo "    enabled: true"
                echo "    modelPath: /models/moderation_v1.onnx"
                echo "    modelVersion: $MODEL_VERSION"
                return 0
            fi

            local_export "$MODEL_DIR" "$chinese" "$model_name"
            verify_model_files "$MODEL_DIR"
            write_model_info "$MODEL_DIR"

            echo ""
            success "模型文件准备完成！输出目录: $(cd "$MODEL_DIR" && pwd)"
            echo ""
            echo "下一步——在 my_config.yaml 中配置："

            if [[ "$chinese" == "true" ]] || [[ -n "$model_name" ]]; then
                echo "  aiModeration:"
                echo "    enabled: true"
                echo "    modelPath: /models/moderation_v1.onnx"
                echo "    modelVersion: $MODEL_VERSION"
                echo "    modelHash: '$(grep sha256 "$MODEL_DIR/model_info.txt" 2>/dev/null | awk '{print $2}')'"
                echo "    numClasses: $(grep "num_classes" "$MODEL_DIR/model_info.txt" 2>/dev/null | awk '{print $2}')"
                echo ""
                echo "也可通过 Docker volume 挂载:"
                echo "  docker run -v \$(pwd)/models:/models:ro ..."
            else
                echo "  aiModeration:"
                echo "    enabled: true"
                echo "    modelPath: /models/moderation_v1.onnx"
                echo "    modelVersion: $MODEL_VERSION"
            fi
            ;;

        package)
            package_model "$MODEL_DIR"
            ;;
    esac
}

main "$@"
