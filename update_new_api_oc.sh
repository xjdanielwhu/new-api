#!/usr/bin/env bash
#
# new-api 自动更新脚本
# 流程: 导出本地镜像 -> SSH 连接测试 -> 上传镜像 tar -> MD5 校验
#       -> docker load -> 镜像 ID 比对 -> 重建容器 -> 健康检查 -> API 验证
#
# 每次执行都会重新 docker save 导出本地镜像为 new-api-image.tar, 并上传到远程。
#
# 用法:
#   ./update_new_api.sh [导出文件路径, 可选]
#
# 可用环境变量(有默认值, 按需覆盖):
#   LOCAL_IMAGE / EXPORT_PATH / SSH_HOST / SSH_USER / SSH_PASS
#   REMOTE_DIR / COMPOSE_DIR / SERVICE / HEALTH_TIMEOUT
#
set -euo pipefail

# ===== 配置区 =====
LOCAL_IMAGE="${LOCAL_IMAGE:-new-api:local}"           # 要导出的本地镜像
EXPORT_PATH="${EXPORT_PATH:-/Volumes/D/new-api/new-api-image.tar}"  # 导出文件(每次覆盖)
SSH_HOST="${SSH_HOST:-43.162.102.227}"
SSH_USER="${SSH_USER:-root}"
SSH_PASS="${SSH_PASS:-tisson2007!}"   # 建议改为从环境变量注入, 勿明文存放
REMOTE_DIR="${REMOTE_DIR:-/home/new-api}"
COMPOSE_DIR="${COMPOSE_DIR:-/home/new-api}"
SERVICE="${SERVICE:-new-api}"
API_URL="${API_URL:-http://localhost:3000/api/status}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-120}"   # 等待容器 healthy 的最长秒数

# ===== 工具函数 =====
log()  { printf '\033[1;34m[%s]\033[0m %s\n' "$(date '+%H:%M:%S')" "$*"; }
ok()   { printf '\033[1;32m[OK]\033[0m   %s\n' "$*"; }
fail() { printf '\033[1;31m[FAIL]\033[0m %s\n' "$*" >&2; exit 1; }

remote() { sshpass -p "$SSH_PASS" ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new "$SSH_USER@$SSH_HOST" "$@"; }

# ===== 0. 前置检查 =====
command -v sshpass >/dev/null 2>&1 || fail "本机未安装 sshpass (brew install sshpass)"
[[ "${1:-}" != "" ]] && EXPORT_PATH="$1"

log "开始更新 new-api 服务"
log "导出镜像: $LOCAL_IMAGE -> $EXPORT_PATH"
log "目标主机: $SSH_USER@$SSH_HOST"

# ===== 1. 导出本地镜像 =====
log "1/8 导出本地镜像 $LOCAL_IMAGE ..."
docker image inspect "$LOCAL_IMAGE" >/dev/null 2>&1 || fail "本机不存在镜像 $LOCAL_IMAGE"
EXPORT_DIR="$(dirname "$EXPORT_PATH")"
mkdir -p "$EXPORT_DIR"
docker save "$LOCAL_IMAGE" -o "$EXPORT_PATH" || fail "docker save 失败"
ok "导出完成 ($(du -h "$EXPORT_PATH" | cut -f1))"

TAR_NAME="$(basename "$EXPORT_PATH")"

# ===== 2. SSH 连接测试 =====
log "2/8 测试 SSH 连接 ..."
remote 'echo connected' >/dev/null 2>&1 || fail "SSH 连接失败, 请检查主机/用户名/密码"
ok "SSH 连接成功"

# ===== 3. 创建远程目录 =====
log "3/8 准备远程目录 $REMOTE_DIR ..."
remote "mkdir -p '$REMOTE_DIR'" || fail "创建远程目录失败"
ok "远程目录就绪"

# ===== 4. 上传镜像 tar =====
log "4/8 上传 $TAR_NAME ($(du -h "$EXPORT_PATH" | cut -f1)) ..."
sshpass -p "$SSH_PASS" scp -o ConnectTimeout=10 "$EXPORT_PATH" "$SSH_USER@$SSH_HOST:$REMOTE_DIR/" || fail "上传失败"
ok "上传完成"

# ===== 5. MD5 校验 =====
log "5/8 校验文件完整性 ..."
if command -v md5 >/dev/null 2>&1; then
  LOCAL_MD5="$(md5 -q "$EXPORT_PATH")"
else
  LOCAL_MD5="$(md5sum "$EXPORT_PATH" | awk '{print $1}')"
fi
REMOTE_MD5="$(remote "md5sum '$REMOTE_DIR/$TAR_NAME'" | awk '{print $1}')"
[[ "$LOCAL_MD5" == "$REMOTE_MD5" ]] || fail "MD5 不一致 (本地 $LOCAL_MD5 vs 远程 $REMOTE_MD5)"
ok "MD5 一致 ($LOCAL_MD5)"

# ===== 6. docker load =====
log "6/8 远程 docker load ..."
LOAD_OUT="$(remote "docker load -i '$REMOTE_DIR/$TAR_NAME'")"
echo "$LOAD_OUT"
IMAGE_TAG="$(echo "$LOAD_OUT" | grep -o 'Loaded image: .*' | awk '{print $3}' | tail -1)"
[[ -n "$IMAGE_TAG" ]] || fail "未识别到加载的镜像名"
ok "已加载镜像 $IMAGE_TAG"

# ===== 7. 镜像 ID 比对 + 重建 =====
log "7/8 比对运行中容器镜像 ..."
OLD_ID="$(remote "docker inspect --format '{{.Image}}' $SERVICE 2>/dev/null || true")"
NEW_ID="$(remote "docker image inspect --format '{{.Id}}' $IMAGE_TAG 2>/dev/null || true")"
ok "运行中: ${OLD_ID:-无} / 新镜像: ${NEW_ID:-无}"

if [[ -n "$NEW_ID" && "$OLD_ID" == "$NEW_ID" ]]; then
  log "容器已在用最新镜像, 无需重建"
else
  log "镜像有更新, 重建容器 $SERVICE ..."
  if remote "command -v docker compose >/dev/null 2>&1"; then
    remote "cd '$COMPOSE_DIR' && docker compose up -d --force-recreate $SERVICE" || fail "docker compose 重建失败"
  else
    remote "cd '$COMPOSE_DIR' && docker-compose up -d --force-recreate $SERVICE" || fail "docker-compose 重建失败"
  fi
  ok "容器已重建"
fi

# ===== 8. 健康检查 + API 验证 =====
log "8/8 等待容器健康并验证 API ..."
for ((i=0; i<HEALTH_TIMEOUT; i+=5)); do
  STATUS="$(remote "docker inspect --format '{{.State.Health.Status}}' $SERVICE 2>/dev/null || echo not_found")"
  if [[ "$STATUS" == "healthy" ]]; then break; fi
  sleep 5
done
[[ "$STATUS" == "healthy" ]] || fail "容器未在 ${HEALTH_TIMEOUT}s 内变为 healthy (当前: $STATUS)"

API_JSON="$(remote "curl -s -m 10 '$API_URL'")"
echo "$API_JSON" | grep -q '"success":true' || fail "API 健康检查未通过"
VERSION="$(echo "$API_JSON" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)"

printf '\n\033[1;32m===== 更新完成 =====\033[0m\n'
ok "镜像: $LOCAL_IMAGE -> $IMAGE_TAG"
ok "容器: $SERVICE ($STATUS)"
ok "版本: ${VERSION:-未知}"
ok "API:  $API_URL"
