# PWA、HTTPS 和远程访问

## 先选择访问方案

应用默认只监听 `127.0.0.1:7580`。以下两种常见方式都需要自己的地址和访问授权：

| 方式 | 适合场景 | 要做的事 |
| --- | --- | --- |
| Cloudflare Tunnel + Access | 希望出门也能打开，家庭没有入站公网端口 | 在自己的 Cloudflare 账户创建 Tunnel，把 HTTPS 主机名转发至本机服务；创建 Access 应用，仅允许自己的身份 |
| VPN + 可信 HTTPS 反向代理 | 已有受管理 VPN/家庭网络 | VPN 解决网络可达性，反向代理解决 HTTPS；仍需应用登录和通知授权 |

不要在仓库填你的真实域名、Tunnel token、Access Cookie 或私钥；不要把应用的初始密码暴露到公网。Cloudflare 账号/域名/服务政策和费用由你自行确认，不要求使用该厂商。

Cloudflare 官方步骤：

1. 按 [创建本地服务 Tunnel](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/get-started/)安装官方 `cloudflared`，在自己的账户创建 tunnel。
2. 服务目标设为 `http://127.0.0.1:7580`；浏览器使用你自己的 HTTPS 主机名，例如 `https://sms.example.com/sms`（本文域名仅为保留的示例）。
3. 按 [保护自托管应用](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/self-hosted-public-app/)创建 Access 策略，仅允许自己的身份。不要误选 Everyone 或绕过策略。
4. 确认无授权访问会先进入身份验证；应用自身也要求登录。Access 会话过期后，点通知可能需要再次登录，这是预期行为。
5. 用户自己在可信终端/控制台完成登录，智能体不得要求把 tunnel token、邮箱验证码或 Cookie 粘到聊天中。

应用需要主机出站访问系统推送服务；不要为推送临时启用 EC25 的蜂窝数据。服务器重启后推送工作线程继续处理持久化队列；网络失效可能延迟，不能承诺实时必达。

## iPhone

需要支持 Home Screen Web Push 的 iOS（iOS 16.4 起；以当前系统支持为准）：

1. Safari 打开自己的 HTTPS 地址并完成登录。
2. 分享 → 添加到主屏幕；从新图标启动。
3. 右上角设置 → 启用锁屏通知 → 用户自己点允许。
4. 点发送测试通知；锁屏后再安排一次测试，确认不是只有前台提醒。
5. 在系统通知设置里检查显示预览、声音、锁屏、专注模式等。

推送通知的内容可包含发送方和短信正文；锁屏上也可能显示验证码。应按自己的需求设置系统预览。系统声音、震动和通知样式由 OS 决定，网页不能复制原生“信息”的所有行为。

官方说明：[WebKit Home Screen Web Push](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)、[Home Screen 角标](https://webkit.org/blog/14112/badging-for-home-screen-web-apps/)。

## 桌面安装

Chrome/Edge 使用浏览器的“安装应用”入口；网页只在浏览器提供安装事件时弹系统安装框，不能强行弹出。Mac Safari 可用“添加到程序坞”；[Apple 使用说明](https://support.apple.com/104996)。

请在**安装后的应用**里允许通知。关闭窗口、退出应用和彻底退出浏览器是不同状态，后台推送是否继续由平台控制，需要逐项测。查看系统通知中心、声音、专注模式和后台运行设置。不要为修通知而清空 Cookie、注销 Service Worker 或删除推送订阅。

## 更新按钮究竟更新什么

“设置 → 应用更新 → 检查更新 → 立即更新”更新管理员已经部署到服务器的网页、Service Worker 和图标资源。它不会拉取 GitHub 代码、升级 Linux、刷 EC25 固件或修改树莓派 EEPROM。

图标文件变化会改变构建发布标识及 icon URL，但**无法保证替换已经安装在系统里的图标**：

- iPhone：安装元数据更新通常需要重新添加；不要仅为图标立即删除现有安装，重建后需重新验证通知授权和订阅。
- Mac Safari：[应用 → 设置 → 通用](https://support.apple.com/guide/safari/ibrwcb937bc5/mac)可由用户选择新的 Dock 图标。
- Chrome 144 起，名称和图标属于身份更新，较大的变化由用户在浏览器菜单的“Review app update”中确认；旧版本有不同限制。见 [Chrome 官方更新机制](https://developer.chrome.com/blog/improvements-to-web-app-updates)。

图案未变化时，版本更新后图标外观不变是正常的。不要修改 Manifest `id`、`scope` 或安装起始地址来伪造一次更新；那可能变成另一份安装。

## 为什么不能自动填验证码

远端 PWA 的短信不是 iOS 原生“信息”或 Apple“邮件”收件箱。Web Push 不会把收到的内容注入系统 OTP 自动填充，跨应用粘贴板写入也受用户手势/系统权限限制。本项目提供手动复制短信，不承诺系统自动填码。
