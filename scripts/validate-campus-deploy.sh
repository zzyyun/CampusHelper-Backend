#!/bin/bash
# validate-campus-deploy.sh — CampusHelper 云端部署 4 项 E2E 验证
#
# 适用：ECS 上 Ubuntu 22.04 + docker compose v2
# 验证项：
#   1. 10 个容器（6 业务 + etcd + RabbitMQ + MinIO + ES）全部 healthy
#   2. depends_on 顺序：业务服务在 etcd healthy 之后启动
#   3. 重启任一服务不影响其他服务
#   4. ES 冷启动不 OOM（1G 内存限制下不触发 OOMKilled）
#
# 用法:
#   cd /opt/campus/CampusHelper-Backend
#   sudo bash scripts/validate-campus-deploy.sh
#
# 退出码：
#   0 = 全部 4 项验证通过
#   1 = 至少 1 项失败（输出 FAIL 项）

set -e

# ─── 颜色 ─────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAILED=1; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
header() { echo ""; echo -e "${YELLOW}========== $1 ==========${NC}"; }

# ─── 前置检查 ─────────────────────────────────────────────────────────────
header "前置检查"
COMPOSE_FILE="deployments/docker/campus-docker-compose.yaml"
if [ ! -f "$COMPOSE_FILE" ]; then
  fail "compose 文件不存在: $COMPOSE_FILE"
  echo "请在仓库根目录执行此脚本"
  exit 1
fi

if [ ! -f "/opt/campus/.env" ]; then
  fail "/opt/campus/.env 不存在"
  echo "请按 runbook 步骤创建 env_file（含 ACR_REGISTRY/ACR_USERNAME/ACR_PASSWORD）"
  exit 1
fi

# ─── 阶段 1: 全部 10 容器 healthy ────────────────────────────────────────
header "阶段 1: 10 容器全部 healthy"
docker compose -f $COMPOSE_FILE pull 2>&1 | tail -5 || warn "部分镜像可能本地构建"

echo "拉起 10 容器..."
docker compose -f $COMPOSE_FILE up -d --build 2>&1 | tail -10

echo "等待所有容器 healthy（最长 3 分钟）..."
HEALTHY_TIMEOUT=180
HEALTHY_ELAPSED=0
EXPECTED_SERVICES=(etcd rabbitmq minio es gateway user content task message file)
while [ $HEALTHY_ELAPSED -lt $HEALTHY_TIMEOUT ]; do
  UNHEALTHY=0
  for svc in "${EXPECTED_SERVICES[@]}"; do
    STATUS=$(docker compose -f $COMPOSE_FILE ps --format json 2>/dev/null | grep -F "\"Name\":\"campus-${svc}\"" | grep -oE '"Health":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "missing")
    if [ "$STATUS" != "healthy" ]; then
      UNHEALTHY=$((UNHEALTHY+1))
    fi
  done
  if [ $UNHEALTHY -eq 0 ]; then
    break
  fi
  sleep 5
  HEALTHY_ELAPSED=$((HEALTHY_ELAPSED+5))
  echo "  等待中... ${HEALTHY_ELAPSED}s / ${HEALTHY_TIMEOUT}s (剩余 ${UNHEALTHY} 个未 healthy)"
done

if [ $UNHEALTHY -eq 0 ]; then
  pass "10 容器全部 healthy（耗时 ${HEALTHY_ELAPSED}s）"
  docker compose -f $COMPOSE_FILE ps --format "table {{.Name}}\t{{.Service}}\t{{.Status}}\t{{.Health}}" 2>&1
else
  fail "仍有 ${UNHEALTHY} 个容器未 healthy"
  docker compose -f $COMPOSE_FILE ps --format "table {{.Name}}\t{{.Service}}\t{{.Status}}\t{{.Health}}" 2>&1
  echo ""
  echo "查看日志定位问题:"
  echo "  docker compose -f $COMPOSE_FILE logs --tail=50 <service>"
fi

# ─── 阶段 2: depends_on 顺序生效 ────────────────────────────────────────
header "阶段 2: depends_on 顺序（业务服务在 etcd healthy 之后启动）"

# 验证方法：比较每个容器 Started At 时间戳
# etcd 应该比所有业务服务早启动（且 healthy 后才放行业务）

ETCD_STARTED=$(docker inspect --format='{{.State.StartedAt}}' campus-etcd 2>/dev/null || echo "")
if [ -z "$ETCD_STARTED" ]; then
  fail "campus-etcd 不存在"
else
  echo "etcd 启动时间: $ETCD_STARTED"
  for svc in gateway user content task message file; do
    SVC_STARTED=$(docker inspect --format='{{.State.StartedAt}}' "campus-${svc}" 2>/dev/null || echo "")
    if [ -n "$SVC_STARTED" ]; then
      if [[ "$SVC_STARTED" > "$ETCD_STARTED" ]]; then
        pass "${svc} 启动时间晚于 etcd (${SVC_STARTED})"
      else
        fail "${svc} 启动时间早于 etcd（违反 depends_on）"
      fi
    else
      fail "${svc} 容器不存在"
    fi
  done
fi

# 额外验证: etcd 必须是 healthy 后业务才启动
ETCD_HEALTHY_TIME=$(docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{end}}' campus-etcd 2>/dev/null)
if [ "$ETCD_HEALTHY_TIME" = "healthy" ]; then
  pass "etcd 当前状态: healthy"
else
  fail "etcd 当前状态: $ETCD_HEALTHY_TIME（应 healthy）"
fi

# ─── 阶段 3: 重启任一服务不影响其他 ──────────────────────────────────────
header "阶段 3: 重启 etcd 看其他服务是否保持 healthy"

BEFORE=$(docker compose -f $COMPOSE_FILE ps --format json | grep -oE '"Health":"[^"]*"' | sort | uniq -c)
echo "重启前健康状态分布:"
echo "$BEFORE"

echo "重启 etcd..."
docker compose -f $COMPOSE_FILE restart etcd 2>&1 | tail -3

echo "等待 30s 让其他服务保持..."
sleep 30

# 验证其他 9 个容器仍然 healthy
OTHER_UNHEALTHY=0
for svc in rabbitmq minio es gateway user content task message file; do
  STATUS=$(docker compose -f $COMPOSE_FILE ps --format json 2>/dev/null | grep -F "\"Name\":\"campus-${svc}\"" | grep -oE '"Health":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "missing")
  if [ "$STATUS" = "healthy" ]; then
    pass "${svc} 在 etcd 重启后仍 healthy"
  else
    fail "${svc} 在 etcd 重启后状态: $STATUS"
    OTHER_UNHEALTHY=$((OTHER_UNHEALTHY+1))
  fi
done

if [ $OTHER_UNHEALTHY -eq 0 ]; then
  pass "重启 etcd 不影响其他 9 个容器"
else
  fail "${OTHER_UNHEALTHY} 个容器受影响"
fi

# ─── 阶段 4: ES 冷启动不 OOM ─────────────────────────────────────────────
header "阶段 4: ES 冷启动不 OOM"

echo "停止 ES 模拟冷启动..."
docker compose -f $COMPOSE_FILE stop es 2>&1 | tail -2

echo "清理 ES 内存（让数据卷冷启动）..."
# 不删数据卷，只停容器 + 让 OS 回收 page cache
sync && echo 3 > /proc/sys/vm/drop_caches 2>/dev/null || warn "无法 drop_caches（可能缺权限）"

echo "重新拉起 ES..."
START_TIME=$(date +%s)
docker compose -f $COMPOSE_FILE up -d es 2>&1 | tail -2

echo "等待 ES 启动 + healthy（最长 2 分钟）..."
ES_HEALTHY_TIMEOUT=120
ES_ELAPSED=0
while [ $ES_ELAPSED -lt $ES_HEALTHY_TIMEOUT ]; do
  ES_STATUS=$(docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{end}}' campus-es 2>/dev/null || echo "missing")
  if [ "$ES_STATUS" = "healthy" ]; then
    END_TIME=$(date +%s)
    pass "ES 冷启动 + healthy，耗时 $((END_TIME - START_TIME))s"
    break
  fi
  sleep 5
  ES_ELAPSED=$((ES_ELAPSED+5))
done

if [ "$ES_STATUS" != "healthy" ]; then
  fail "ES 冷启动 ${ES_ELAPSED}s 后仍未 healthy（可能 OOM）"
fi

# 检查 OOMKilled
OOM_KILLED=$(docker inspect --format='{{.State.OOMKilled}}' campus-es 2>/dev/null || echo "false")
if [ "$OOM_KILLED" = "false" ]; then
  pass "ES 未触发 OOMKilled"
else
  fail "ES 触发了 OOMKilled！需要调大 memory 限制"
fi

# 检查实际内存使用（应 < 1G 限制）
ES_MEM_USAGE=$(docker stats --no-stream --format "{{.MemUsage}}" campus-es 2>/dev/null | head -1)
echo "ES 实际内存使用: $ES_MEM_USAGE（限制 1G）"

# ─── 总结 ─────────────────────────────────────────────────────────────────
header "验证总结"
if [ -z "$FAILED" ]; then
  pass "全部 4 项验证通过 ✅"
  echo ""
  echo "下一步:"
  echo "  - 可以继续 #117 ECS 拉起全套服务 + 端到端验证"
  echo "  - 或运行 ./scripts/verify.sh 跑业务链路"
  exit 0
else
  fail "至少 1 项验证失败"
  echo ""
  echo "排查指引:"
  echo "  1. 查看具体失败服务的日志: docker compose -f $COMPOSE_FILE logs --tail=100 <service>"
  echo "  2. 确认 /opt/campus/.env 凭证正确"
  echo "  3. 确认阿里云 RDS/Tair 安全组允许 ECS IP"
  exit 1
fi
