#!/bin/zsh
# 配置信息，自行修改这里的密码
SERVER_PWD="tisson2007!"
SERVER_IP="124.220.165.189"
USER="root"
LOCAL_DIR="/Volumes/D"
REMOTE_COMPOSE_DIR="/home/new-api"

# 1. 切换本地目录
cd "${LOCAL_DIR}" || exit 1
echo "===== 开始导出镜像并传输至服务器 ====="

# 2. 管道传输镜像+远程load，自动填密码
docker save new-api:local | sshpass -p "${SERVER_PWD}" ssh "${USER}@${SERVER_IP}" "docker load"

echo "===== 镜像导入完成，重启服务 ====="
# 3. 远程进入compose目录重启服务
sshpass -p "${SERVER_PWD}" ssh "${USER}@${SERVER_IP}" << EOF
cd ${REMOTE_COMPOSE_DIR}
docker compose down
docker compose up -d
EOF

echo "===== new-api 更新重启全部完成 ====="