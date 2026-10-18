# 从零部署：Linux + EC25 + 手机 PWA

默认安装仅绑定 `127.0.0.1:7580`。先在本地改密码，再配置 HTTPS 和访问保护。以下步骤不会刷固件、格式化 SD 卡、发送短信或开启蜂窝数据。若已有系统或短信服务，先备份并识别设备归属，不要直接覆盖。

## 1. 准备系统与网络

在空白/可重装的树莓派上，通过 [Raspberry Pi Imager](https://www.raspberrypi.com/software/) 安装 Raspberry Pi OS 64-bit Lite。确认 SD 卡目标再写入；此步骤会擦除所选介质。已有系统可直接使用，不要求重刷。

在 Imager 中设置自己的主机名、用户名、SSH 公钥和 Wi-Fi（或接以太网），不要使用项目维护者的账号。插好散热器，用 Pi 5 合适电源启动。SIM 按模组厂商要求在断电状态插入，连接 LTE 天线和 USB 成品。不要同时插到其他服务器。确认主机经以太网/Wi-Fi 上网。

```bash
uname -m
cat /etc/os-release
lsusb
ls -l /dev/cdc-wdm*
lsmod | grep -E 'qmi_wwan|cdc_wdm'
```

`lsusb` 的完整输出可能含序列号；不要原样贴到公开 issue。没有 `cdc-wdm` 时先核对硬件/驱动，不能跳过这一步。

## 2. 设备归属与数据状态

仅选一个实际存在的节点，下面以 `/dev/cdc-wdm0` 为例，**不是保证每台机器都叫这个名字**。

```bash
sudo fuser -v /dev/cdc-wdm0
systemctl is-active ModemManager || true
ip route show default
```

如其他进程占用，先明确归属；不自动杀进程或停止 ModemManager。不要运行任何蜂窝拨号命令，也不要在桌面网络面板里创建移动宽带连接。不要为收短信填写 APN。运行 `ip route` 只能检查路由，不能证明运营商计费为零。

## 3. 构建软件

需要 Git、Go（`go.mod` 所列版本）和 Node.js 24/npm。系统依赖可先安装：

```bash
sudo apt update
sudo apt install -y git curl python3 psmisc ca-certificates make
```

按照 [Go 官方安装](https://go.dev/doc/install) 和 [Node.js 官方下载](https://nodejs.org/en/download)安装对应工具链。不要假定发行版旧版 Go/Node 足够。审查包来源，不要把未经审查的下载脚本以 root 执行。

```bash
git clone https://github.com/tigerbryan/sim-sms-pwa.git
cd sim-sms-pwa
# 可选择并审查已发布的 tag；不要把未知更新直接装进生产。
make build
go test -mod=vendor -buildvcs=false ./...
python3 scripts/check-public-tree.py
```

`make build` 安装锁定的 npm 依赖、测试/构建 Vue 前端，再把前端嵌入 Go 单文件服务。Go 依赖以保留许可的 vendor 快照构建；不需要 Python 包、SIM 流量、AI API 或付费 AI 服务。构建下载消耗主机当前互联网连接。

## 4. 安装到 systemd

先可用模拟模式验证服务：

```bash
sudo ./scripts/install-native.sh --binary ./build/qmi-web --mock
```

接入实际设备时，明确使用硬件模式：

```bash
sudo ./scripts/install-native.sh --binary ./build/qmi-web --device /dev/cdc-wdm0
sudo systemctl status qmi-web --no-pager
curl -fsS http://127.0.0.1:7580/ready
```

同一安装从 mock 切换硬件模式时重复第二条安装命令；配置、数据库和主密钥保留，并先备份。安装器拒绝覆盖不是它管理的已有安装。设备忙时拒绝接管，不自动停止其他服务。

本原生服务为访问 QMI 节点以 root 身份运行，并使用空能力集、关闭设备策略、仅允许一个 QMI 节点、只读系统目录和限定写目录；这不是完整沙箱，服务器仍应是受信任的专用主机。容器部署的非 root/ACL 策略见[高级硬件说明](HARDWARE_MODE.md)，不作为本教程的默认安装路径。

如果改过 HTTP 端口，安装器的回环 7580 就绪检查不会适配；请手动检查实际端口后维护自己的服务配置。USB 热插拔或节点编号变化后，需要重新识别并更新明确的设备选择，不能扩大成全 `/dev` 访问。

## 5. 在本地改密码

在自己的电脑建立 SSH 转发（替换占位符）：

```bash
ssh -L 17580:127.0.0.1:7580 USER@PI_HOST
```

打开 `http://localhost:17580/sms`，首次账户 `admin / admin`，进入设置更换密码。密码只由你在本地网页输入；不要发给智能体或写入命令行/公开配置。

本地回环 HTTP 只用于初次设置/演示。手机上的 PWA/Web Push 需要可信 HTTPS，不要把临时的自签名 IP 地址当成最终入口。

## 6. 远程 HTTPS 与访问保护

详细步骤见 [PWA 与远程访问](PWA.md)。需要自己的域名/HTTPS 和反向代理或隧道；它们不是本仓库预配置的第三方公共短信服务器。始终保持应用登录，再增加独立访问保护。开放公网前确认初始密码已经改掉。

## 7. 真正收件与推送验收

1. 登录设备状态/诊断页，确认设备已识别、SIM 已就绪、已注册网络、数据库正常；不得在报告中公开 SIM 标识。
2. 确认 SMS-only 开启、Data Guard 为 safe、WDS 未连接，主机默认网络仍是以太网/Wi-Fi。如果网络状态不符合预期，暂停远程推送并找出其他联网软件，而不是开启流量“试试”。
3. 由你安排另一个号码发一条无敏感内容的测试短信，记录“发送→服务入库→手机展示”各时间。收不到时分别排查 SIM、运营商、模块、应用；不要把网页可打开当成已收短信。
4. iPhone Safari 打开 HTTPS 地址，添加到主屏幕，从主屏幕启动，点设置里的“启用锁屏通知”，由你亲自点允许。发演示通知，再锁屏测试。
5. Mac/Windows 用支持的浏览器安装 PWA；在安装后的应用里授予通知，检查系统通知开关和勿扰模式。窗口关闭和完全退出浏览器的后台能力可能不同，需实际验收。
6. 验证真实测试短信的锁屏内容、点击返回、未读角标与已读清除；推送成功响应不等于用户设备已经显示。
7. 记录结果为“通过、失败、未测”。只有 mock 或测试通知通过，不能宣称整个项目完成。

后续升级与备份见 [OPERATIONS.md](OPERATIONS.md)。
