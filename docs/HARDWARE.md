# 硬件选型与购买指南

核对日期：2026-10-18。链接用于确认型号和寻找销售渠道，不承诺当前价格、库存、送货国家或卖家的实际配置。没有联盟链接。未完成本项目的端到端验收，不应把“厂家说兼容”当成已经能收短信。

## 一套最小可用组合

| 必需 | 建议配置 | 作用与购买入口 |
| --- | --- | --- |
| 是 | Raspberry Pi 5，2GB 或以上内存 | 本项目来源环境使用 Pi 5；只收短信不需要 AI 加速器。[官方型号及授权经销商](https://www.raspberrypi.com/products/raspberry-pi-5/) |
| 是 | Pi 5 官方 27W USB-C 电源，5.1V/5A 档 | USB 模组有瞬时用电需求，普通手机快充不等于支持 5V/5A。[官方电源](https://www.raspberrypi.com/products/27w-power-supply/) |
| 是 | microSD 32GB 或以上，可靠品牌，或已支持的 SSD 启动方案 | 储存系统和短信。SD 方案足够，无需为了本项目买双 NVMe 或 RAID。[Raspberry Pi 官方存储产品](https://www.raspberrypi.com/products/) |
| 是 | **完整的 Quectel EC25-AUX USB 成品**，含 SIM 卡槽和匹配 LTE 天线 | 本项目来源环境使用此型号系列。确认成品确实包含模组，不是裸板/空转接板。[Quectel 官方系列与规格](https://www.quectel.com/product/lte-ec25-series/) |
| 是 | 有效实体 SIM，运营商允许设备/所在地使用并支持接收 SMS | 联系自己的运营商确认频段、LTE 短信、漫游和设备限制。不要公开 ICCID/IMSI、号码或 PIN |
| 是 | 以太网，或稳定 Wi-Fi + 家庭宽带 | 网页与推送上网走这里；项目不使用 SIM 数据上网 |
| 建议 | Pi 5 适配散热器/通风机箱 | 保证长期运行，供电稳定比把所有风扇强开更重要。[官方 Active Cooler](https://www.raspberrypi.com/products/active-cooler/) |
| 可选 | 有独立供电、供电设计可靠的 USB Hub | 遇到模组掉线或 USB 供电不足再考虑；先排除欠压和劣质线材 |
| 可选 | UPS / 稳定电源和路由器备电 | 应用自恢复不能解决物理断电、SD 卡损坏或运营商故障 |

也可以用已有的 Debian/Ubuntu Linux amd64 小主机，不必重复买树莓派。该替代主机需要自己的 USB 驱动和设备验收；不是所有 NAS 都可插 USB 模组即用。

## EC25 在哪里买、怎么买对

下面是检索时能找到 EC25 选项的商家页面，**不是对这些商家的交付质量或本项目兼容性的保证**：

- [EXVIST EC25 Mini PCIe USB 成品](https://www.exvist.com/products/4g-lte-dongle-ec25-mini-pcie-type-c-sim/4736458000004856033)：查看下拉框是否为 **EC25-AUX**，不要选择 `W/O Module`。有 SIM 卡槽和 USB 接口仍需确认天线和频段。
- [JTOKEN EC25 LCC USB 成品](https://www.jtokenapp.com/products/4g-usb-dongle-w-quectel-ec25-lcc-sim-card-slot-gps)：该页面列出多个 EC25 变体；下单前询问 QMI 接口是否可用，以及是否包含 LTE 天线。
- [DigiKey EC25-AU Mini PCIe 示例](https://www.digikey.com.au/en/products/detail/quectel/EC25AUFA-MINIPCIE/13278210)：这是 **AU 裸 Mini PCIe 模组**，不是本项目实际使用的 AUX USB 成品；还需要合适的 USB 转接板、SIM 卡槽和天线，不作为新手默认套装。
- 淘宝/京东/速卖通等渠道可用关键词：`移远 EC25 AUX USB QMI 带卡槽 天线`。不提供未核实的随机店铺购买链接。

发给卖家的核对清单：

> 请确认交付的是哪个完整型号和固件版本，含模组、SIM 卡槽、USB 连接及 LTE 天线。Linux 是否枚举 `/dev/cdc-wdmX` 并由 `qmi_wwan` 驱动，能否使用 QMI WMS 接收短信？请勿只回复“能上网”或“免驱”。请提供地区频段、LTE 短信与目标运营商适用说明；能否退换不兼容产品？不要发任何真实 SIM 标识或短信。

## 频段、运营商与软件兼容性是三件事

EC25 有 AU、AUX、EU、EUX、A、J 等不同变体。请以 [Quectel 规格](https://www.quectel.com/product/lte-ec25-series/)和对应地区运营商要求选型，不能只看“EC25”三个字。即使频段匹配，也可能受 SIM 套餐、设备认证、漫游或 LTE 短信机制影响。

Linux 侧还必须有 `qmi_wwan` 和 `cdc-wdm`，并由本项目独占选中的 QMI 控制节点。只出现 `ttyUSB`、串口能回 AT，或模块能在别的软件里联网，都不能证明本项目可用。不要为了排查而自动刷固件或改 USB 模式。

本仓库是接收端，不含语音、SIP、双向音频或 AI 通话。Windows 驱动存在也不代表本项目的 Linux QMI 后端能在 Windows 使用。Synology DSM 同样不能仅凭硬件是 USB 就视为可用。

## Pironman 5 MAX：可选，不是短信必需品

[SunFounder Pironman 5 MAX 产品](https://www.sunfounder.com/products/pironman-5-max) · [官方安装文档](https://docs.sunfounder.com/projects/pironman5/en/latest/)

它是 Pi 5 机箱/散热、显示和扩展方案；树莓派主板、电源、SSD 是否包含要逐项看所选套餐。短信项目无需它的双 NVMe、RAID 或 AI 扩展。不要把 MAX、Pro MAX 和普通 5 混为同一安装型号。

如果确实使用该机箱，按官方文档安装**对应型号**控制程序，检查风扇/温度/OLED，而不是把该驱动混进短信应用安装流程。CPU PWM 风扇按温度停转可能正常。不要替用户设置全速风扇、更新 EEPROM 或刷固件。

## 验收后再说“兼容”

保留已脱敏的主机架构、OS、模块完整型号/固件、驱动和测试结果。验收必须包括注册网络、实际收到一条由用户安排的无敏感内容短信，以及安装后锁屏推送测试。用模拟短信跑通网页只能证明界面/推送链路的一部分。
