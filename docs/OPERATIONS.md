# 运维、升级、备份和恢复

## 运行状态与边界

`Restart=on-failure` 负责应用异常退出后的重启；启动频率限制避免无限重启。它不能解决断电、USB 供电不足、系统卡死、SD 卡损坏或运营商故障。不要自动重启整台树莓派、重置 USB 或刷固件。

```bash
sudo systemctl status qmi-web --no-pager
curl -fsS http://127.0.0.1:7580/ready
sudo journalctl -u qmi-web --since '10 minutes ago'
```

日志可能包含诊断信息，公开前脱敏；不要上传整个日志或数据库。`/ready` 只代表服务/数据库就绪，不一定意味着模组或 SIM 可用。

## 一致性备份

```bash
sudo python3 ops/sms-backup.py
```

默认将 SQLite 在线一致性快照、配置、`secrets/master.key` 保存到 `/var/backups/qmi-web`，权限为仅管理员可读，保留最近七份。备份含真实短信及解密密钥，**没有额外加密**，应转存到自己的加密备份介质，不要提交到 GitHub。

定时备份为可选项，确认路径后安装：

```bash
sudo install -m 0755 ops/sms-backup.py /usr/local/sbin/sms-backup
sudo install -m 0644 ops/sms-backup.service ops/sms-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now sms-backup.timer
systemctl list-timers sms-backup.timer
```

## 健康检查（可选，先适配代理）

`ops/sms-healthcheck.py` 默认分别检查应用和本地 Apache HTTPS：`http://127.0.0.1:7580/ready`、`https://127.0.0.1/ready`。连续三次失败才尝试重启对应服务，冷却十分钟；不触碰模组。HTTPS 证书校验例外仅用于明确的本机回环地址，不应扩大到公网。

**如果不是 Apache，不要直接启用这份样例**；先把服务名和 URL 改为自己的反向代理，并在隔离环境验证。应用健康失败时不连带重启代理。

```bash
python3 -m unittest discover -s ops -p 'test_*.py'
# 仅在完成适配后：
sudo install -m 0755 ops/sms-healthcheck.py /usr/local/sbin/sms-healthcheck
sudo install -m 0644 ops/sms-healthcheck.service ops/sms-healthcheck.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now sms-healthcheck.timer
```

## 更新服务端

PWA 更新按钮不会更新服务端程序。服务端更新过程：

1. 先备份数据库、主密钥、配置、已有二进制和 systemd 单元到私有位置。
2. 检查发行说明，选择并审查固定 tag/commit，不把未知 main 自动装进生产。
3. 在不覆盖旧程序的目录检出新版本，运行构建与检查。
4. 用当前安装模式（`--mock` 或同一个明确的 `--device`）运行 `scripts/install-native.sh --binary ./build/qmi-web ...`。安装器保留配置和加密状态，但服务有短暂中断；失败时按备份恢复，不要反复重装。
5. 核对 `/version`、`/ready`、设备与 SIM、WDS/默认路由；确认真实短信/推送，再在 PWA 中检查更新。
6. 不清除站点数据、不重新生成 master.key、不注销推送订阅。密钥和订阅变化会影响通知。

## 恢复

先停止服务并保留当前状态，再将经过校验的数据库、对应的 `secrets/master.key` 和配置恢复到原路径，保持私有权限。数据库和 key 必须属于同一份备份。不要只回退程序却忽略可能变化的数据库迁移版本；需要时恢复整份匹配快照，评估快照之后新短信的损失。

管理员密码遗忘时可在主机本地可信终端运行：

```bash
sudo /usr/local/bin/qmi-web reset-admin --data /var/lib/qmi-web
```

它通过不回显的交互读取新密码，并使现有应用登录会话失效；不要把密码放在命令行或聊天中。不要使用恢复默认密码选项后继续暴露公网。

## 卸载但保留数据

对本项目原生安装运行 `sudo ./scripts/uninstall.sh`，只移除对应服务和二进制；配置、短信数据库、主密钥、日志、备份全部保留。没有自动清空数据的 `--purge` 选项。可选健康检查/备份定时器要按自己的部署另行停用，不能误删其他服务。
