#!/usr/bin/env bash
#
# 同步上游 QuantumNous/new-api 的前端到本仓库 web/default，并重贴 wy 定制。
#
# 用法:
#   ./sync-upstream-frontend.sh            # 完整流程
#   ./sync-upstream-frontend.sh --fetch    # 只抓取上游并显示差异，不改动工作区
#
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO"

UPSTREAM_REMOTE="upstream"
UPSTREAM_BRANCH="main"
PATCH="$REPO/.wy-patches/wy-frontend-custom.patch"
TRASH="/Volumes/D/.newapi-trash/sync-$(date +%Y%m%d-%H%M%S)"
STAMP="$(date +%Y%m%d-%H%M%S)"

log()  { printf '\033[36m[sync]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[warn]\033[0m %s\n' "$*"; }
die()  { printf '\033[31m[fail]\033[0m %s\n' "$*" >&2; exit 1; }

# --- 0. 前置检查 -----------------------------------------------------------
[ -z "$(git status --porcelain --untracked-files=no)" ] \
  || die "工作区有未提交改动，先 commit 或 stash"

git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1 \
  || die "缺少 remote '$UPSTREAM_REMOTE'，执行: git remote add upstream https://github.com/QuantumNous/new-api.git"

# --- 1. 抓取上游 -----------------------------------------------------------
# GitHub 大包传输不稳定，放宽超时并加大缓冲。不要走 127.0.0.1:7890 代理。
log "抓取 $UPSTREAM_REMOTE/$UPSTREAM_BRANCH ..."
git -c http.postBuffer=524288000 \
    -c http.lowSpeedLimit=0 \
    -c http.lowSpeedTime=999999 \
    fetch --depth=30 "$UPSTREAM_REMOTE" "$UPSTREAM_BRANCH"

UP_SHA="$(git rev-parse --short "$UPSTREAM_REMOTE/$UPSTREAM_BRANCH")"
log "上游 HEAD: $UP_SHA"

if [ "${1:-}" = "--fetch" ]; then
  log "上游前端变更文件数: $(git diff --name-only HEAD "$UPSTREAM_REMOTE/$UPSTREAM_BRANCH" -- web | wc -l | tr -d ' ')"
  git diff --stat HEAD "$UPSTREAM_REMOTE/$UPSTREAM_BRANCH" -- web | tail -20
  exit 0
fi

# --- 2. 备份分支 -----------------------------------------------------------
BACKUP="backup/pre-sync-$STAMP"
git branch "$BACKUP"
log "已建备份分支 $BACKUP（出问题可 git reset --hard $BACKUP）"

# --- 3. 导出当前 wy 前端定制为补丁 ------------------------------------------
mkdir -p "$(dirname "$PATCH")"
log "导出当前 web/default 定制到补丁 ..."
git diff "$UPSTREAM_REMOTE/$UPSTREAM_BRANCH":web -- web/default > "$PATCH.new" 2>/dev/null || true

# --- 4. 用上游 web/ 覆盖 web/default ---------------------------------------
# 上游目录是 web/，本仓库保留 default/classic 双前端结构，需做前缀映射。
log "用上游 web/ 替换 web/default ..."
git rm -r --cached web/default -q
git read-tree --prefix=web/default/ "$UPSTREAM_REMOTE/$UPSTREAM_BRANCH:web"
git checkout-index -a -f

# 清理索引中已不存在的旧文件（上游删掉的），移入回收目录而非直接删除
mkdir -p "$TRASH"
while IFS= read -r f; do
  [ -e "$f" ] || continue
  mkdir -p "$TRASH/$(dirname "$f")"
  mv "$f" "$TRASH/$f"
done < <(git ls-files --others --exclude-standard web/default)
log "孤儿文件已移至 $TRASH"

# --- 5. 重贴 wy 定制 -------------------------------------------------------
if [ -s "$PATCH" ]; then
  log "重贴 wy 定制补丁 ..."
  # 二进制块无法用 3way 合并，先剔除
  git apply --3way --exclude='*.ico' --exclude='*.png' "$PATCH" || {
    warn "补丁部分冲突，未合并路径:"
    git diff --name-only --diff-filter=U
    warn "手工解决冲突后继续执行第 6 步"
  }
fi

# 从 wy 基线找回二进制资源与独立定制组件
WY_BASE="${WY_BASE:-fe8fd162}"
for f in \
  web/default/public/cc-switch-icon.ico \
  web/default/public/cherry-studio-icon.png \
  web/default/src/features/home/components/app-support-badge.tsx
do
  git cat-file -e "$WY_BASE:$f" 2>/dev/null && git checkout "$WY_BASE" -- "$f" \
    && log "已恢复 $f"
done

# --- 6. i18n 对齐 ---------------------------------------------------------
# 上游结构为 {"translation": {...}}，键顺序由 en.json（base locale）决定。
# 新键必须先追加到 en.json 末尾，再用上游脚本同步，不要自己排序。
log "对齐 i18n ..."
( cd web/default && bun run i18n:sync ) || warn "i18n:sync 失败，需手工检查 locales"
if [ -d web/default/src/i18n/locales/_reports ]; then
  mkdir -p "$TRASH/i18n-reports"
  mv web/default/src/i18n/locales/_reports "$TRASH/i18n-reports/"
fi

# --- 7. 构建校验 ----------------------------------------------------------
log "构建前端校验 ..."
( cd web/default && bun install --frozen-lockfile && bun run build ) \
  || die "前端构建失败，检查冲突残留"

log "完成。上游 $UP_SHA 已并入 web/default"
log "下一步: git add -A && git commit -m \"chore(web): sync upstream frontend $UP_SHA\""
log "然后重建镜像: docker compose -f docker-compose.v2.yml build && docker compose -f docker-compose.v2.yml up -d"
