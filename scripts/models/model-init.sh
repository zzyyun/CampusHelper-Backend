#!/bin/bash
# AI 审核服务模型初始化脚本（容器 entrypoint 包装器）
#
# 用途：在服务启动前检查模型���件是否存在，���存在则尝试自动下载。
#       如果下载失败（如未配置远程 URL），输出���确警告但不阻止服务启动。
#
# 用法：
#   方式1: 直接在容器 ENTRYPOINT 中使用
#     COPY scripts/models/model-init.sh /usr/local/bin/model-init.sh
#     ENTRYPOINT ["model-init.sh", "/usr/local/bin/ai-moderation"]
#
#   方式2: 手动运行
#     MODEL_URL=https://your-oss.com/models.tar.gz bash model-init.sh
#
# 环境变量���
#   MODEL_URL          模型下载 URL（.tar.gz 格式）
#   MODEL_DIR          模型文��目录（默认: /models）
#   MODEL_SKIP_DOWNLOAD 设置为 1 ���过自动下载

set -euo pipefail

MODEL_DIR="${MODEL_DIR:-/models}"
MODEL_URL="${MODEL_URL:-}"
MODEL_SKIP_DOWNLOAD="${MODEL_SKIP_DOWNLOAD:-0}"

log() { echo "[model-init] $(date '+%H:%M:%S') $*"; }

# ── ��查模型文件是否存在 ──────────────────────────────────────────────────

check_model() {
    local onnx_file="$MODEL_DIR/moderation_v1.onnx"
    local vocab_file="$MODEL_DIR/vocab.txt"

    if [[ -f "$onnx_file" ]] && [[ -f "$vocab_file" ]]; then
        local size
        size=$(stat -c%s "$onnx_file" 2>/dev/null || stat -f%z "$onnx_file" 2>/dev/null || echo 0)
        log "模型文件已存���: $onnx_file (${size} bytes)"
        return 0
    fi
    return 1
}

# ── 下载模型 ──────────────────────────────────────────────────────────────

download_model() {
    local url="$1"
    local dest="$2"

    log "开始下载模型文件..."
    log "  URL: $url"
    log "  目标: $dest"

    mkdir -p "$dest"

    local tmpfile
    tmpfile="$(mktemp)"
    # shellcheck disable=SC2064
    trap "rm -f '$tmpfile'" EXIT

    if ! curl -fsSL --retry 3 --retry-delay 5 --connect-timeout 30 \
         --max-time 600 -o "$tmpfile" "$url"; then
        log "错误: 模型下载失败"
        return 1
    fi

    log "解压模型文件..."
    if ! tar -xzf "$tmpfile" -C "$dest"; then
        log "错误: 模型文件解压失败"
        return 1
    fi

    rm -f "$tmpfile"
    log "下载完成"
    return 0
}

# ── 主逻辑 ────────────────────────────────────────────────────────────────

main() {
    log "AI 审核服务模型初始化"

    # 检查模型文件
    if check_model; then
        log "模型文件就绪，跳过下载"
    else
        log "未找到模型文件: $MODEL_DIR/moderation_v1.onnx"

        if [[ "$MODEL_SKIP_DOWNLOAD" == "1" ]]; then
            log "MODEL_SKIP_DOWNLOAD=1，跳过自动下载"
        elif [[ -n "$MODEL_URL" ]]; then
            if download_model "$MODEL_URL" "$MODEL_DIR"; then
                if check_model; then
                    log "模型下载并验证成功"
                else
                    log "警告: 下载完成但模型文��验证失败，服务将以 mock 模式运行"
                fi
            else
                log "警告: 模型下载失败，服务将以 mock 模式运行"
            fi
        else
            log "���设置 MODEL_URL，无法自动下载"
            log ""
            log "获取模型文件的方法："
            log "  1. 本��导出: 运行 scripts/models/download_model.sh --local-export"
            log "  2. 从 OSS 下载: MODEL_URL=<url> bash scripts/models/download_model.sh"
            log "  3. Docker volume 挂载: docker run -v /path/to/models:/models:ro ..."
            log ""
            log "服务将以 mock 模式启动（固定返回 PASS）"
        fi
    fi

    # 启动主服务（如果提供了命令参数）
    if [[ $# -gt 0 ]]; then
        log "启动服务: $*"
        exec "$@"
    fi
}

main "$@"
