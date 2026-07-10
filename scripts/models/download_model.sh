#!/bin/bash
# 模型文件下载脚本
#
# 用途：下载 AI 审核服务所需的 ONNX 模型文件 + tokenizer 词表。
#
# 两个模式：
#   1. 从远程 URL 下载预导出的 ONNX 模型（推荐，生产环境使用）
#   2. 本地 Python 导出（开发环境，需 transformers/torch/onnx）
#
# 用法：
#   # 从远程下载（默认行为）
#   bash scripts/models/download_model.sh
#
#   # 指定模型下载 URL
#   MODEL_URL=https://your-oss.com/models.tar.gz bash scripts/models/download_model.sh
#
#   # ���地 Python 导出
#   bash scripts/models/download_model.sh --local-export
#
#   # 指定输出目录
#   MODEL_DIR=/path/to/models bash scripts/models/download_model.sh
#
# 输出文件：
#   $MODEL_DIR/moderation_v1.onnx   # ONNX 模型
#   $MODEL_DIR/vocab.txt             # BERT tokenizer 词表
#   $MODEL_DIR/config.json           # 模型配置
#   $MODEL_DIR/model_info.txt        # 模型元信��（版本/hash/下载时间）
#
# 环境变量：
#   MODEL_URL          模型下载 URL（.tar.gz 或 .zip 格式）
#   MODEL_DIR          模型输出目录（默认: ./models）
#   MODEL_VERSION      模型版本号（默认: v1.0）

set -euo pipefail

# ── 默认配置 ──────────────────────────────────────────────────────────────
MODEL_DIR="${MODEL_DIR:-./models}"
MODEL_VERSION="${MODEL_VERSION:-v1.0}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 预导出模型下载 URL（可替换为自有 OSS/CDN 地址）
# 注意：此 URL 需指向包含以下文件的 tar.gz 包：
#   - moderation_v1.onnx
#   - vocab.txt
#   - config.json
DEFAULT_MODEL_URL=""

# ── 辅助函数 ──────────────────────────────────────────────────────────────

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }
success() { echo "✓ $*"; }
fail() { echo "✗ $*" >&2; exit 1; }

# 检查命令是否存在
require_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "缺少命令: $1，请先安装"
}

# ── 模式 1: 从远程 URL 下载 ─────────────────────────────────────────────

download_from_url() {
    local url="$1"
    local dest_dir="$2"

    require_cmd curl
    require_cmd tar

    log "从远程 URL 下载模型文件..."
    log "  URL: $url"

    local tmpdir
    tmpdir="$(mktemp -d)"
    # 清理临时目录
    # shellcheck disable=SC2064
    trap "rm -rf '$tmpdir'" EXIT

    local archive="$tmpdir/model.tar.gz"

    log "下载中..."
    if ! curl -fsSL --retry 3 --retry-delay 5 --connect-timeout 30 \
         --max-time 600 -o "$archive" "$url"; then
        fail "下载失���: $url"
    fi
    success "下载完成"

    log "解压到: $dest_dir"
    mkdir -p "$dest_dir"
    tar -xzf "$archive" -C "$dest_dir"

    # 验证必要文件
    verify_model_files "$dest_dir"
}

# ── 模式 2: 本地 Python 导出 ─────────────────────────────────────────────

local_export() {
    local dest_dir="$1"

    require_cmd python3

    local export_script="$SCRIPT_DIR/export_to_onnx.py"

    if [[ ! -f "$export_script" ]]; then
        fail "找��到导出脚本: $export_script"
    fi

    log "本地 Python 导出 ONNX 模型（需要 transformers/torch/onnx）..."
    log "这可能需要几分钟，请耐心等待..."

    cd "$PROJECT_ROOT"
    python3 "$export_script" --output-dir "$dest_dir"

    success "本地导出完成"
}

# ── 验证模型文件完整性 ──────────────────────────────────────────────────

verify_model_files() {
    local model_dir="$1"
    local errors=0

    log "验证模型文件..."

    # 必须有的文件
    local required=("moderation_v1.onnx" "vocab.txt")
    for f in "${required[@]}"; do
        if [[ -f "$model_dir/$f" ]]; then
            local size
            size=$(stat -c%s "$model_dir/$f" 2>/dev/null || stat -f%z "$model_dir/$f" 2>/dev/null || echo 0)
            success "$f (${size} bytes)"
        else
            fail "缺少文件: $model_dir/$f"
            errors=$((errors + 1))
        fi
    done

    # 可选的模型元信息
    if [[ -f "$model_dir/config.json" ]]; then
        success "config.json (模型配置)"
    fi

    if [[ $errors -gt 0 ]]; then
        fail "模型文件不完整，缺少 $errors 个文件"
    fi
}

# ── 写入模型元信息 ──────────────────────────────────────────────────────

write_model_info() {
    local model_dir="$1"
    local onnx_path="$model_dir/moderation_v1.onnx"

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
model_source: unitary/toxic-bert (Hugging Face)
num_labels: 6
max_seq_length: 512
EOF

    log "模型元信息已写��: $model_dir/model_info.txt"
    echo ""
    cat "$model_dir/model_info.txt"
}

# ── 打包模型（用于上传到 OSS）───────────────────────────────────────────

package_model() {
    local model_dir="$1"

    require_cmd tar

    local package="models-${MODEL_VERSION}.tar.gz"
    log "打包模型文件: $package"

    cd "$model_dir"
    tar -czf "$PROJECT_ROOT/$package" \
        moderation_v1.onnx \
        vocab.txt \
        config.json \
        model_info.txt 2>/dev/null || true

    cd "$PROJECT_ROOT"
    success "模型包已创建: $package ($(stat -c%s "$package" 2>/dev/null || echo 0) bytes)"
    echo ""
    echo "上传到 OSS 后，可通过以下方式下载："
    echo "  MODEL_URL=https://your-oss.com/$package bash scripts/models/download_model.sh"
}

# ── 主逻辑 ──────────────────────────────────────────────────────────────

main() {
    local mode="download"
    local url="$DEFAULT_MODEL_URL"

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --local-export)
                mode="export"
                shift
                ;;
            --package)
                mode="package"
                shift
                ;;
            --url)
                url="$2"
                shift 2
                ;;
            *)
                echo "未知参数: $1"
                echo "用法: $0 [--local-export | --package] [--url <URL>]"
                exit 1
                ;;
        esac
    done

    echo "╔══════════════════════════════════════════════════════════╗"
    echo "║     AI 审核模型文件下载 / 导出工具                       ║"
    echo "║     模型: unitary/toxic-bert (ONNX)                      ║"
    echo "╚══════════════════════════════════════════════════════════╝"
    echo ""

    case "$mode" in
        download)
            if [[ -z "$url" ]]; then
                echo "未设置模型下载 URL。"
                echo ""
                echo "请选择以下方式之一获取模型："
                echo ""
                echo "方式 1（推荐）: 本地 Python 导出"
                echo "  pip install transformers torch onnx onnxruntime"
                echo "  bash scripts/models/download_model.sh --local-export"
                echo ""
                echo "方式 2: 指定远程 URL"
                echo "  MODEL_URL=https://your-oss.com/models.tar.gz bash scripts/models/download_model.sh"
                echo ""
                echo "方式 3: 从 Hugging Face 手动下载"
                echo "  1. 安装依赖: pip install transformers torch onnx onnxruntime optimum"
                echo "  2. 运行导出: python3 scripts/models/export_to_onnx.py"
                echo "  3. 复制文件: cp models/* /path/to/deploy/models/"
                echo ""
                exit 1
            fi
            download_from_url "$url" "$MODEL_DIR"
            ;;
        export)
            local_export "$MODEL_DIR"
            ;;
        package)
            package_model "$MODEL_DIR"
            return 0
            ;;
    esac

    # 验证 + 写入元信息
    verify_model_files "$MODEL_DIR"
    write_model_info "$MODEL_DIR"

    echo ""
    success "模型文��准备完成！输出目录: $(cd "$MODEL_DIR" && pwd)"
    echo ""
    echo "下一步——在 my_config.yaml 中配置:"
    echo "  aiModeration:"
    echo "    enabled: true"
    echo "    modelPath: /models/moderation_v1.onnx"
    echo "    modelVersion: $MODEL_VERSION"
    echo "    modelHash: '$(grep sha256 "$MODEL_DIR/model_info.txt" 2>/dev/null | awk '{print $2}' || echo "")'"
    echo ""
    echo "或���过 Docker volume 挂载:"
    echo "  docker run -v \$(pwd)/models:/models:ro ..."
}

main "$@"
