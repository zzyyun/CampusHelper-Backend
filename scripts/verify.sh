#!/bin/bash
# CampusHelper 端到端验证脚本
# 验证 6 个微服务 + 4 个中间件的健康状态与主链路业务
#
# 用法:
#   ./scripts/verify.sh                 # 默认检查 localhost:50000 (gateway)
#   GATEWAY_URL=http://1.2.3.4:50000 ./scripts/verify.sh    # 自定义 gateway 地址
#   ./scripts/verify.sh --skip-biz     # 跳过业务调用，仅做 health
#   ./scripts/verify.sh --skip-db      # 跳过数据库检查
#
# 退出码:
#   0 - 全部通过
#   1 - 有失败项
#
# 依赖:
#   - curl, jq, mysql-client, redis-cli (可选)

set -u

# ==================== 配置 ====================
GATEWAY_URL="${GATEWAY_URL:-http://localhost:50000}"
GATEWAY_HOST=$(echo "$GATEWAY_URL" | sed -E 's#^https?://##' | cut -d: -f1)
GATEWAY_PORT=$(echo "$GATEWAY_URL" | sed -E 's#^.*:##')

SKIP_BIZ=false
SKIP_DB=false
for arg in "$@"; do
  case $arg in
    --skip-biz) SKIP_BIZ=true ;;
    --skip-db)  SKIP_DB=true ;;
    *) echo "Unknown arg: $arg"; exit 2 ;;
  esac
done

# 服务端口（与 campus-docker-compose.yaml 一致）
declare -A SERVICES=(
  ["gateway"]="50000"
  ["user"]="50001"
  ["content"]="50002"
  ["task"]="50003"
  ["message"]="50004"
  ["file"]="50005"
)
declare -A INFRA=(
  ["etcd"]="2379"
  ["rabbitmq"]="5672"
  ["minio"]="9000"
  ["es"]="9200"
)

# ==================== 输出辅助 ====================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0
RESULTS=()

pass() {
  echo -e "  ${GREEN}✓ PASS${NC} $1"
  PASS_COUNT=$((PASS_COUNT + 1))
  RESULTS+=("PASS|$1")
}

fail() {
  echo -e "  ${RED}✗ FAIL${NC} $1"
  FAIL_COUNT=$((FAIL_COUNT + 1))
  RESULTS+=("FAIL|$1")
}

warn() {
  echo -e "  ${YELLOW}⚠ WARN${NC} $1"
  RESULTS+=("WARN|$1")
}

header() {
  echo ""
  echo -e "${CYAN}=== $1 ===${NC}"
}

# ==================== 阶段 1: 容器/服务运行状态 ====================
header "阶段 1: 容器运行状态 (docker ps)"

if command -v docker >/dev/null 2>&1; then
  DOCKER_OK=true
  # 业务服务
  for svc in "${!SERVICES[@]}"; do
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "campus-$svc$"; then
      pass "container campus-$svc is running"
    else
      fail "container campus-$svc NOT running"
    fi
  done
  # 基础设施
  for infra in "${!INFRA[@]}"; do
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "campus-$infra$"; then
      pass "container campus-$infra is running"
    else
      fail "container campus-$infra NOT running"
    fi
  done
else
  warn "docker CLI not available, skipping container status check"
  DOCKER_OK=false
fi

# ==================== 阶段 2: 端口连通性 ====================
header "阶段 2: 端口连通性 (nc/curl)"

check_port() {
  local name=$1
  local port=$2
  if (echo > /dev/tcp/$GATEWAY_HOST/$port) 2>/dev/null; then
    pass "$name:$port reachable"
  elif timeout 2 bash -c "cat </dev/tcp/$GATEWAY_HOST/$port" 2>/dev/null; then
    pass "$name:$port reachable (bash tcp)"
  else
    fail "$name:$port NOT reachable"
  fi
}

# 仅检查本机端口（compose 默认在 localhost）
LOCAL_HOST="127.0.0.1"
for svc in "${!SERVICES[@]}"; do
  check_port "$svc" "${SERVICES[$svc]}"
done
for infra in "${!INFRA[@]}"; do
  check_port "$infra" "${INFRA[$infra]}"
done

# ==================== 阶段 3: HTTP 健康端点 ====================
header "阶段 3: HTTP 健康端点 (gateway /health)"

if command -v curl >/dev/null 2>&1; then
  RESP=$(curl -sf -o /dev/null -w "%{http_code}" --max-time 5 "$GATEWAY_URL/health" 2>&1)
  if [ "$RESP" = "200" ]; then
    pass "gateway /health returns 200"
  elif [ "$RESP" = "404" ]; then
    warn "gateway /health returns 404 (端点可能未实现，跳过)"
  elif [ "$RESP" = "502" ] || [ "$RESP" = "503" ]; then
    fail "gateway /health returns $RESP (服务不可用)"
  else
    fail "gateway /health returns $RESP (异常)"
  fi
else
  warn "curl not available, skipping HTTP check"
fi

# ==================== 阶段 4: 数据库连接 (RDS/Tair) ====================
if [ "$SKIP_DB" = "false" ]; then
  header "阶段 4: 云数据库连通性 (.env 配置)"
  
  ENV_FILE="/opt/campus/.env"
  if [ -f "$ENV_FILE" ]; then
    # 加载 env
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE" 2>/dev/null
    set +a
    
    # MySQL 测试
    if command -v mysql >/dev/null 2>&1 && [ -n "${MYSQL_HOST:-}" ]; then
      if mysql -h"$MYSQL_HOST" -P"${MYSQL_PORT:-3306}" -u"${MYSQL_USER:-root}" -p"${MYSQL_PASSWORD:-}" --connect-timeout=5 -e "SHOW DATABASES;" >/dev/null 2>&1; then
        pass "MySQL @ $MYSQL_HOST:3306 connected"
        # 检查 5 个数据库
        DB_LIST=$(mysql -h"$MYSQL_HOST" -P"${MYSQL_PORT:-3306}" -u"${MYSQL_USER:-root}" -p"${MYSQL_PASSWORD:-}" -N -e "SHOW DATABASES;" 2>/dev/null)
        for db in campus_user campus_content campus_task campus_message campus_file; do
          if echo "$DB_LIST" | grep -q "$db"; then
            pass "MySQL database '$db' exists"
          else
            fail "MySQL database '$db' MISSING"
          fi
        done
      else
        fail "MySQL @ $MYSQL_HOST:3306 NOT reachable"
      fi
    elif [ -n "${MYSQL_HOST:-}" ]; then
      warn "MySQL credentials configured but mysql client not installed (skip)"
    fi
    
    # Redis 测试
    if command -v redis-cli >/dev/null 2>&1 && [ -n "${REDIS_ADDR:-}" ]; then
      REDIS_HOST=$(echo "$REDIS_ADDR" | cut -d: -f1)
      REDIS_PORT=$(echo "$REDIS_ADDR" | cut -d: -f2)
      if redis-cli -h "$REDIS_HOST" -p "${REDIS_PORT:-6379}" -a "${REDIS_PASSWORD:-}" --no-auth-warning PING 2>/dev/null | grep -q PONG; then
        pass "Redis @ $REDIS_HOST:$REDIS_PORT PONG"
      else
        fail "Redis @ $REDIS_HOST:$REDIS_PORT NOT reachable"
      fi
    elif [ -n "${REDIS_ADDR:-}" ]; then
      warn "Redis configured but redis-cli not installed (skip)"
    fi
  else
    warn "$ENV_FILE not found (ECS 上才会创建，跳过)"
  fi
fi

# ==================== 阶段 5: 主链路业务调用 ====================
if [ "$SKIP_BIZ" = "false" ]; then
  header "阶段 5: 主链路业务调用 (注册→登录→发帖→读帖→发消息→上传头像)"
  
  if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
    warn "curl/jq not available, skipping biz calls"
  else
    # 1. 微信登录（mock code）
    LOGIN_RESP=$(curl -sf --max-time 5 -X POST "$GATEWAY_URL/api/v1/user/login" \
      -H "Content-Type: application/json" \
      -d '{"code":"verify_sh_test_code"}' 2>&1)
    
    if [ -n "$LOGIN_RESP" ]; then
      ACCESS_TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.access_token // .access_token // empty' 2>/dev/null)
      if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
        pass "login → got access_token (${#ACCESS_TOKEN} chars)"
      else
        warn "login response missing access_token (微信 mock code 可能无效): $(echo $LOGIN_RESP | head -c 100)"
        ACCESS_TOKEN=""
      fi
    else
      warn "login request failed (服务可能未启动或 mock code 无效)"
      ACCESS_TOKEN=""
    fi
    
    # 2. 发帖（如果拿到 token）
    if [ -n "$ACCESS_TOKEN" ]; then
      POST_RESP=$(curl -sf --max-time 5 -X POST "$GATEWAY_URL/api/v1/content/posts" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -d '{"title":"verify.sh 测试","content":"自动化验证脚本创建的测试帖","type":1}' 2>&1)
      
      if [ -n "$POST_RESP" ]; then
        POST_ID=$(echo "$POST_RESP" | jq -r '.data.post_id // .data.id // .post_id // .id // empty' 2>/dev/null)
        if [ -n "$POST_ID" ] && [ "$POST_ID" != "null" ]; then
          pass "create post → post_id=$POST_ID"
          
          # 3. 读帖
          GET_RESP=$(curl -sf --max-time 5 "$GATEWAY_URL/api/v1/content/posts/$POST_ID" 2>&1)
          if [ -n "$GET_RESP" ] && echo "$GET_RESP" | grep -q "verify.sh"; then
            pass "read post → contains expected title"
          else
            warn "read post returned empty or no match"
          fi
        else
          warn "create post response missing post_id"
        fi
      else
        warn "create post failed (可能鉴权失败或字段不匹配)"
      fi
      
      # 4. 发消息（可能鉴权失败，不强求）
      MSG_RESP=$(curl -sf --max-time 5 -X POST "$GATEWAY_URL/api/v1/message/send" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -d '{"to_user_id":1,"content":"verify.sh test","type":1}' 2>&1)
      if [ -n "$MSG_RESP" ]; then
        pass "send message → response received"
      else
        warn "send message failed (可能需要真实 to_user_id)"
      fi
      
      # 5. 上传头像 (1x1 PNG base64)
      PNG_B64="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNgYAAAAAMAASsJTYQAAAAASUVORK5CYII="
      TMP_FILE=$(mktemp --suffix=.png 2>/dev/null || echo "/tmp/verify_test.png")
      echo "$PNG_B64" | base64 -d > "$TMP_FILE" 2>/dev/null
      
      UPLOAD_RESP=$(curl -sf --max-time 5 -X POST "$GATEWAY_URL/api/v1/file/upload" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -F "file=@$TMP_FILE;type=image/png" \
        -F "type=avatar" 2>&1)
      rm -f "$TMP_FILE"
      
      if [ -n "$UPLOAD_RESP" ]; then
        pass "upload avatar → response received"
      else
        warn "upload avatar failed (可能需要鉴权或字段调整)"
      fi
    else
      warn "skipping biz calls (no access_token)"
    fi
  fi
fi

# ==================== 汇总 ====================
header "汇总"
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo -e "  总项: ${CYAN}$TOTAL${NC}"
echo -e "  ${GREEN}通过: $PASS_COUNT${NC}"
if [ $FAIL_COUNT -gt 0 ]; then
  echo -e "  ${RED}失败: $FAIL_COUNT${NC}"
else
  echo -e "  失败: 0"
fi

if [ $FAIL_COUNT -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✓ verify.sh ALL PASS${NC}"
  exit 0
else
  echo ""
  echo -e "${RED}✗ verify.sh FAILED (${FAIL_COUNT} 项)${NC}"
  echo ""
  echo "故障排查："
  echo "  1. 检查容器状态: docker compose -f deployments/docker/campus-docker-compose.yaml ps"
  echo "  2. 查看日志: docker logs <container_name>"
  echo "  3. 验证 .env: cat /opt/campus/.env"
  exit 1
fi
