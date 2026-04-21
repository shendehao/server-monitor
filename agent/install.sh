#!/bin/bash
# Server Monitor Agent 安装脚本
# 用法: curl -sSL http://监控服务器/install.sh | bash -s -- <SERVER_URL> <AGENT_TOKEN>

set -e

SERVER_URL="$1"
AGENT_TOKEN="$2"

if [ -z "$SERVER_URL" ] || [ -z "$AGENT_TOKEN" ]; then
    echo "用法: $0 <监控服务器地址> <Agent令牌>"
    echo "示例: $0 http://192.168.1.100:8080 abc123def456"
    exit 1
fi

INSTALL_DIR="/opt/server-monitor-agent"
SERVICE_NAME="server-monitor-agent"

echo "==> 安装 Server Monitor Agent"
echo "    服务器: $SERVER_URL"

# 创建安装目录
mkdir -p "$INSTALL_DIR"

# 下载 Agent 二进制（如果存在远程下载地址）
# 如果是手动安装，请将编译好的 agent 二进制放到 $INSTALL_DIR/agent
if [ ! -f "$INSTALL_DIR/agent" ]; then
    echo "==> 请将编译好的 agent 二进制文件放到 $INSTALL_DIR/agent"
    echo "    编译命令: GOOS=linux GOARCH=amd64 go build -o agent ."
fi

chmod +x "$INSTALL_DIR/agent" 2>/dev/null || true

# 创建 systemd 服务
cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=Server Monitor Agent
After=network.target

[Service]
Type=simple
Environment=SERVER_URL=${SERVER_URL}
Environment=AGENT_TOKEN=${AGENT_TOKEN}
Environment=INTERVAL=10
ExecStart=${INSTALL_DIR}/agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 启动服务
systemctl daemon-reload
systemctl enable ${SERVICE_NAME}
systemctl start ${SERVICE_NAME}

echo "==> Agent 安装完成！"
echo "    状态: systemctl status ${SERVICE_NAME}"
echo "    日志: journalctl -u ${SERVICE_NAME} -f"
