#!/bin/bash
# ==============================================================================
# CampusHelper 性能压测脚本 — hey 梯度压测 6 服务核心 API
# 用法：./run-benchmark.sh [选项]
#   --url    Gateway 基础地址（默认 http://localhost:8080）
#   --token  JWT 认证 Token（测试需登录接口时传入）
#   --out    输出报告路径（默认 tests/benchmark/baseline-report.md）
# ==============================================================================
set -euo pipefail

# ---- 参数解析 ----
GATEWAY_BASE_URL="${GATEWAY_BASE_URL:-http://localhost:8080}"
JWT_TOKEN="${JWT_TOKEN:-}"
OUTPUT_FILE="${1:-tests/benchmark/baseline-report.md}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)  GATEWAY_BASE_URL="$2"; shift 2 ;;
    --token) JWT_TOKEN="$2"; shift 2 ;;
    --out)  OUTPUT_FILE="$2"; shift 2 ;;
    *) OUTPUT_FILE="$1"; shift ;;
  esac
done

# ---- 工具检查 ----
HEY_BIN="$(go env GOPATH)/bin/hey"
if [ ! -x "$HEY_BIN" ]; then
  echo "ERROR: hey 未安装，请运行 go install github.com/rakyll/hey@latest"
  exit 1
fi

# ---- 配置 ----
CONCURRENCY_LEVELS=(10 50 100)
DURATION_SECONDS=10
REQUEST_BODY_FILE=$(mktemp /tmp/bench_body.XXXXXX)
trap 'rm -f "$REQUEST_BODY_FILE"' EXIT

# ---- 工具函数 ----
log() { echo "[$(date -u +%H:%M:%S)] $*"; }

# 执行单次压测并提取关键指标
# 参数：$1=url $2=method $3=name $4=concurrency $5=duration $6=post_data(可选)
run_one() {
  local url="$1" method="$2" name="$3" conc="$4" dur="$5" post_data="${6:-}"
  local hey_args=(-n 0 -c "$conc" -z "${dur}s" -t 30)

  if [ "$method" = "POST" ] && [ -n "$post_data" ]; then
    hey_args+=(-m POST -d "$post_data" -T "application/json")
  fi

  # 执行压测，hey 输出写入临时文件
  local tmp_out
  tmp_out=$(mktemp /tmp/hey_out.XXXXXX)
  "$HEY_BIN" "${hey_args[@]}" "$url" > "$tmp_out" 2>&1 || true

  # 提取指标（hey 输出格式固定）
  local qps p50 p90 p99 avg err_count total_count
  qps=$(grep "Requests/sec:" "$tmp_out" | awk '{print $2}')
  p50=$(grep "50% in" "$tmp_out" | awk '{print $3}')
  p90=$(grep "90% in" "$tmp_out" | awk '{print $3}')
  p99=$(grep "99% in" "$tmp_out" | awk '{print $3}')
  avg=$(grep "Average:" "$tmp_out" | head -1 | awk '{print $2}')
  err_count=$(grep -c "^\[" "$tmp_out" 2>/dev/null || echo "0")
  total_count=$(grep "^\[200\]" "$tmp_out" | awk '{sum+=$2} END {print sum+0}')

  rm -f "$tmp_out"

  # 返回 CSV 行
  echo "${name},${method},${conc},${qps:-0},${avg:-0},${p50:-0},${p90:-0},${p99:-0},${err_count},${total_count}"
}

# ---- 定义测试用例 ----
# 格式：METHOD|路径|名称|需要认证|POST数据
declare -a TEST_CASES=(
  "GET|/health|健康检查|0|"
  "GET|/api/v1/content/posts|帖子列表|1|"
  "GET|/api/v1/content/posts/1|帖子详情|1|"
  "POST|/api/v1/content/search|内容搜索|1|{\"keyword\":\"test\",\"page\":1,\"page_size\":10}"
  "GET|/api/v1/tasks|任务列表|1|"
  "GET|/api/v1/tasks/1|任务详情|1|"
  "GET|/api/v1/notifications|通知列表|1|"
  "GET|/api/v1/notifications/unread-count|未读计数|1|"
  "GET|/api/v1/schools|学校列表|1|"
  "GET|/api/v1/files/1|文件元数据|1|"
)

# ---- 主流程 ----
log "CampusHelper 性能压测开始"
log "目标：${GATEWAY_BASE_URL}"
log "并发梯度：${CONCURRENCY_LEVELS[*]}"
log "每级持续时间：${DURATION_SECONDS}s"
log "测试用例数：${#TEST_CASES[@]}"
echo ""

RESULTS_CSV="/tmp/bench_results_$$.csv"
echo "name,method,concurrency,qps,avg_latency,p50,p90,p99,errors,total" > "$RESULTS_CSV"

pass_count=0
fail_count=0

for tc in "${TEST_CASES[@]}"; do
  IFS='|' read -r method path name needs_auth post_data <<< "$tc"
  url="${GATEWAY_BASE_URL}${path}"

  # 跳过需要认证但未提供 token 的用例
  if [ "$needs_auth" = "1" ] && [ -z "$JWT_TOKEN" ]; then
    log "SKIP ${name}（需要 JWT Token）"
    echo "${name},${method},skipped,0,0,0,0,0,0,0" >> "$RESULTS_CSV"
    continue
  fi

  log "▶ 测试：${method} ${name} (${path})"

  for conc in "${CONCURRENCY_LEVELS[@]}"; do
    log "  并发=${conc} 持续=${DURATION_SECONDS}s"
    result=$(run_one "$url" "$method" "$name" "$conc" "$DURATION_SECONDS" "$post_data")
    echo "$result" >> "$RESULTS_CSV"

    # 解析并打印摘要
    IFS=',' read -r r_name r_method r_conc r_qps r_avg r_p50 r_p90 r_p99 r_err r_total <<< "$result"
    if [ "$r_qps" != "0" ] && [ -n "$r_qps" ]; then
      log "  ✓ QPS=${r_qps} avg=${r_avg}s p50=${r_p50}s p90=${r_p90}s p99=${r_p99}s errors=${r_err}"
      ((pass_count++)) || true
    else
      log "  ✗ 压测失败或服务未响应"
      ((fail_count++)) || true
    fi
  done
  echo ""
done

log "压测完成：成功=${pass_count} 失败=${fail_count}"

# ---- 生成 Markdown 报告 ----
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

cat > "$OUTPUT_FILE" << 'HEADER'
# CampusHelper 性能压测基线报告

> 自动生成于 `tests/benchmark/run-benchmark.sh`
> 工具：hey (github.com/rakyll/hey)

## 1. 测试环境

| 项目 | 值 |
|------|----|
HEADER

cat >> "$OUTPUT_FILE" << EOF
| Gateway 地址 | ${GATEWAY_BASE_URL} |
| 测试时间 | ${NOW} |
| 压测工具 | hey v$(hey -h 2>&1 | head -1 || echo "unknown") |
| 持续时间/轮 | ${DURATION_SECONDS}s |
| 并发梯度 | $(IFS='/'; echo "${CONCURRENCY_LEVELS[*]}") |
EOF

cat >> "$OUTPUT_FILE" << 'SECTION2'

## 2. 测试接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 健康检查 | GET | `/health` | 网关存活探针，无中间件 |
| 帖子列表 | GET | `/api/v1/content/posts` | Content 服务核心读接口 |
| 帖子详情 | GET | `/api/v1/content/posts/:id` | 单条帖子查询 |
| 内容搜索 | POST | `/api/v1/content/search` | ES 全文搜索 |
| 任务列表 | GET | `/api/v1/tasks` | Task 服务核心读接口 |
| 任务详情 | GET | `/api/v1/tasks/:id` | 单条任务查询 |
| 通知列表 | GET | `/api/v1/notifications` | Message 服务通知分页 |
| 未读计数 | GET | `/api/v1/notifications/unread-count` | 红点查询 |
| 学校列表 | GET | `/api/v1/schools` | User 服务字典表查询 |
| 文件元数据 | GET | `/api/v1/files/:id` | File 服务元数据查询 |

## 3. 压测结果
SECTION2

# 表头
echo "" >> "$OUTPUT_FILE"
echo "| 接口 | 并发 | QPS | Avg(ms) | P50(ms) | P90(ms) | P99(ms) | 错误数 |" >> "$OUTPUT_FILE"
echo "|------|------|-----|---------|---------|---------|---------|--------|" >> "$OUTPUT_FILE"

# 逐行填充（跳过标题行和 skipped 行）
tail -n +2 "$RESULTS_CSV" | while IFS=',' read -r name method conc qps avg p50 p90 p99 errors total; do
  if [ "$conc" = "skipped" ]; then
    continue
  fi
  # 将秒转为毫秒（hey 输出为秒）
  avg_ms=$(echo "$avg" | awk '{printf "%.1f", $1*1000}')
  p50_ms=$(echo "$p50" | awk '{printf "%.1f", $1*1000}')
  p90_ms=$(echo "$p90" | awk '{printf "%.1f", $1*1000}')
  p99_ms=$(echo "$p99" | awk '{printf "%.1f", $1*1000}')
  echo "| ${name} | ${conc} | ${qps} | ${avg_ms} | ${p50_ms} | ${p90_ms} | ${p99_ms} | ${errors} |" >> "$OUTPUT_FILE"
done

cat >> "$OUTPUT_FILE" << 'SECTION3'

## 4. 性能基准判定

| 指标 | 达标线 | 优秀线 |
|------|--------|--------|
| 核心读接口 QPS（P0: /health） | ≥ 1000 | ≥ 5000 |
| 核心读接口 QPS（P1: 列表/详情） | ≥ 200 | ≥ 1000 |
| 搜索接口 QPS | ≥ 50 | ≥ 200 |
| P99 延迟（读接口） | ≤ 500ms | ≤ 100ms |
| P99 延迟（搜索接口） | ≤ 1000ms | ≤ 300ms |
| 错误率 | < 1% | 0% |

> **说明**：以上基准基于 4C8G ECS + RDS MySQL + Redis 场景估算，正式标准需结合实际部署环境调整。

## 5. 已知限制

- 当前为**单机单 Gateway** 场景，未包含 gRPC 内部调用链路的独立压测
- 数据库/Redis 连接池参数为默认值，未做调优
- 未注入测试数据（帖子数=0、任务数=0），实际生产环境 QPS 会受索引和数据量影响
- 认证接口（需 JWT）在未传 Token 时被跳过，需本地启动服务后补充测试

## 6. 后续建议

1. 本地或测试环境部署后，使用 `--token` 传入 JWT，补充认证接口的压测数据
2. 重点观察 Content 搜索（ES）和 Notification 列表的 P99 延迟
3. 在 task-140（Redis 缓存）、task-141（限流）、task-143（连接池）完成后重跑，对比优化效果
4. 增加写接口压测（CreatePost、CreateTask）以评估数据库写入性能

## 7. 使用方法

```bash
# 基本用法（无认证接口）
./tests/benchmark/run-benchmark.sh --url http://localhost:8080

# 带 JWT Token（测试全部接口）
./tests/benchmark/run-benchmark.sh --url http://localhost:8080 --token "your-jwt-token"

# 自定义输出路径
./tests/benchmark/run-benchmark.sh --url http://localhost:8080 --out tests/benchmark/custom-report.md
```
SECTION3

log "报告已生成：${OUTPUT_FILE}"
echo ""
log "=== 压测完成 ==="
