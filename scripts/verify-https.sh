# CampusHelper-Backend HTTPS 上线验证脚本
# 验证 https://rithupc.cn 部署是否成功
# 关联 issue: #130
#
# 用法：
#   bash scripts/verify-https.sh
# 退出码：
#   0 = 全部通过
#   1 = 至少 1 项失败

set -e

# ─── 颜色 ───────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAILED=1; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
header() { echo ""; echo -e "${YELLOW}========== $1 ==========${NC}"; }

DOMAIN="${DOMAIN:-rithupc.cn}"
ECS_IP="${ECS_IP:-121.41.74.238}"

# ─── 1. DNS 解析 ───────────────────────────────────────────────────────
header "1. DNS 解析 $DOMAIN"
if command -v dig >/dev/null 2>&1; then
  RESOLVED_IP=$(dig +short "$DOMAIN" 2>/dev/null | head -1)
elif command -v nslookup >/dev/null 2>&1; then
  RESOLVED_IP=$(nslookup "$DOMAIN" 2>/dev/null | awk '/^Address: /{print $2; exit}')
elif command -v host >/dev/null 2>&1; then
  RESOLVED_IP=$(host "$DOMAIN" 2>/dev/null | awk '/has address/{print $4; exit}')
else
  warn "无 DNS 查询工具（dig/nslookup/host）"
  RESOLVED_IP="未知"
fi

if [ "$RESOLVED_IP" = "$ECS_IP" ]; then
  pass "DNS: $DOMAIN → $RESOLVED_IP"
elif [ "$RESOLVED_IP" = "未知" ]; then
  warn "无法验证 DNS（缺工具）"
else
  fail "DNS: $DOMAIN → $RESOLVED_IP（期望 $ECS_IP）"
fi

# ─── 2. HTTPS /health 返回 200 ─────────────────────────────────────────
header "2. HTTPS /health"
HTTP_CODE=$(curl -sk -o /dev/null -w "%{http_code}" --max-time 10 "https://$DOMAIN/health" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
  HEALTH_BODY=$(curl -sk --max-time 5 "https://$DOMAIN/health" 2>/dev/null)
  pass "HTTPS /health: 200 ($HEALTH_BODY)"
else
  fail "HTTPS /health: $HTTP_CODE（期望 200）"
fi

# ─── 3. HTTP → HTTPS 301 跳转 ──────────────────────────────────────────
header "3. HTTP 强制跳转"
HTTP_REDIRECT=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "http://$DOMAIN/health" 2>/dev/null || echo "000")
if [ "$HTTP_REDIRECT" = "301" ] || [ "$HTTP_REDIRECT" = "302" ]; then
  pass "HTTP /health → $HTTP_REDIRECT 重定向"
else
  fail "HTTP /health: $HTTP_REDIRECT（期望 301）"
fi

# ─── 4. 证书有效 + 剩余天数 ───────────────────────────────────────────
header "4. SSL 证书有效期"
if command -v openssl >/dev/null 2>&1; then
  CERT_INFO=$(echo | openssl s_client -servername "$DOMAIN" -connect "$DOMAIN:443" 2>/dev/null || true)
  EXPIRY=$(echo "$CERT_INFO" | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2 || echo "")
  if [ -n "$EXPIRY" ]; then
    pass "证书到期: $EXPIRY"
    EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s 2>/dev/null || echo "0")
    NOW_EPOCH=$(date +%s)
    DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))
    if [ "$DAYS_LEFT" -lt 30 ] && [ "$DAYS_LEFT" -gt 0 ]; then
      warn "证书剩余 $DAYS_LEFT 天（< 30 天，建议尽快续期）"
    fi
  else
    fail "无法解析证书到期时间"
  fi
else
  warn "openssl 未安装，跳过证书检查"
fi

# ─── 5. SSL 协议版本（必须 TLS 1.2+）──────────────────────────────────
header "5. TLS 协议版本"
if command -v openssl >/dev/null 2>&1; then
  PROTO=$(echo | openssl s_client -servername "$DOMAIN" -connect "$DOMAIN:443" 2>/dev/null | grep -oE "TLSv1\.[0-3]" | head -1 || echo "")
  if [ "$PROTO" = "TLSv1.3" ] || [ "$PROTO" = "TLSv1.2" ]; then
    pass "协议版本: $PROTO"
  else
    fail "协议版本: $PROTO（期望 TLSv1.2 或 TLSv1.3）"
  fi
else
  warn "openssl 未安装，跳过协议检查"
fi

# ─── 6. 业务接口可达性（不验证返回值，只看 HTTP code）─────────────────
header "6. 业务接口 HTTP 状态"
for path in "/api/v1/content/posts?school_id=1" "/health"; do
  CODE=$(curl -sk -o /dev/null -w "%{http_code}" --max-time 10 "https://$DOMAIN$path" 2>/dev/null || echo "000")
  if [ "$CODE" -ge 200 ] && [ "$CODE" -lt 500 ]; then
    pass "$path: $CODE"
  else
    fail "$path: $CODE（期望 2xx/3xx/4xx，5xx 表示服务端错误）"
  fi
done

# ─── 7. HSTS 头检查 ───────────────────────────────────────────────────
header "7. HSTS 安全头"
HSTS=$(curl -sk -I --max-time 10 "https://$DOMAIN/health" 2>/dev/null | grep -i "strict-transport-security" | tr -d '\r' || echo "")
if [ -n "$HSTS" ]; then
  pass "HSTS 头: $HSTS"
else
  warn "HSTS 头未设置（建议启用以增强安全）"
fi

# ─── 总结 ─────────────────────────────────────────────────────────────
header "总结"
if [ -z "$FAILED" ]; then
  pass "✅ 全部 HTTPS 验证通过"
  echo ""
  echo "下一步："
  echo "  - Phase B (#131): 修改小程序 baseURL → https://rithupc.cn/api/v1"
  echo "  - 微信公众平台添加合法域名"
  echo "  - 真机扫码走通主链路"
  exit 0
else
  fail "至少 1 项验证失败"
  echo ""
  echo "排查指引："
  echo "  1. 阿里云 SSL 控制台 → 确认证书已签发"
  echo "  2. ECS 证书路径: /opt/campus/deployments/nginx/certs/{fullchain.pem,privkey.pem}"
  echo "  3. docker logs campus-nginx 看 Nginx 错误"
  echo "  4. ECS 安全组确认 80/443 端口已放行"
  exit 1
fi
