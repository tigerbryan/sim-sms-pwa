#!/usr/bin/env bash
set -e

PURGE=0

for arg in "$@"; do
    case $arg in
        --purge)
            PURGE=1
            shift
            ;;
    esac
done

echo "正在停止并禁用服务..."
if systemctl is-active --quiet qmi-web; then
    systemctl stop qmi-web
fi
if systemctl is-enabled --quiet qmi-web 2>/dev/null; then
    systemctl disable qmi-web
fi

if [ -f /etc/systemd/system/qmi-web.service ]; then
    rm -f /etc/systemd/system/qmi-web.service
    systemctl daemon-reload
fi

INSTALL_DIR="/opt/qmi-web"

echo "正在删除二进制文件..."
rm -rf "$INSTALL_DIR/bin"
rm -rf "$INSTALL_DIR/scripts"

if [ $PURGE -eq 1 ]; then
    echo "警告：正在清理所有数据和配置文件！"
    rm -rf "$INSTALL_DIR"
    echo "已完全卸载。"
else
    echo "服务及二进制文件已卸载。配置文件和数据保留在 $INSTALL_DIR/data。"
    echo "若要彻底清除数据，请运行: $0 --purge"
fi
