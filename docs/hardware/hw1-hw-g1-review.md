# HW1 设备与 BSP 预检评审

- 日期：2026-11-15
- 状态：公开交付资料已复核，固定 BSP 来源和 Mac 串口准备路径已补录。实际交付规格仍待确认，仓库当前为 `HW-G0`
- 范围：三台 ZERO 3E 2GB 的纯有线台架，供 HW-G1 评审及后续 HW-G3 编制执行包
- 依据：[固定镜像离线验签](hw1-armbian-image-verification.md#离线验签记录2026-11-15) · [镜像静态核对](hw1-armbian-image-verification.md#静态兼容性核对结果2026-11-15) · [HW1-01–08 既有用例](hardware-validation-plan.md#到货后的验收顺序)

## 待确认的交付规格

| 对象 | 评审基准 | 待补证据及处置 |
| --- | --- | --- |
| A / B / C 板卡 | 同型号、同 PCB revision 的 Radxa ZERO 3E 2GB，候选 `RS109-D2E0H1`，带排针、无板载 eMMC、三台各一张系统 microSD。Radxa 公开资料列有 3E V1.2 量产版。[ZERO 3E 资源页](https://docs.radxa.com/zero/zero3/download) | 采购评审前取得与该 SKU 和三台实际交付范围绑定的 PCB 丝印或明确的版号/批次说明，确认三台同版、无替代。公开 V1.2 资料只用于比对。到货后逐台把实物丝印和料号映射到 A/B/C，留脱敏照片。版号不明时等待，型号或 revision 与评审不符时暂停上电并重审 |
| B 的 USB 网卡 | 微雪 `20162` / RTL8153，经数据转接接 B 的 USB Host。镜像的 `r8152` 纸面匹配候选 `0bda:8153`，同 VID/PID 还有 CDC 接口类的 `r8153_ecm` 别名。[网卡官方规格](https://www.waveshare.com/wiki/USB_3.2_Gen1_TO_Gigabit_ETH) · [镜像驱动证据](hw1-armbian-image-verification.md#静态兼容性核对结果2026-11-15) | 先确认交付 SKU、控制器、硬件版号或批次及是否可提供 USB 枚举记录。若供应侧无法给出 VID/PID，由评审者决定是否接受到货后验证及对应退换风险，状态保持待实测。上板记录实际 VID/PID、`bcdDevice`、接口类、USB 端口及驱动。VID/PID、芯片或驱动和已审配置不符时暂停该项，复核别名、固件及恢复路径后再评审 |
| 首启与恢复配件 | 三套独立 5 V 供电、三张系统卡加一张同型号共用备卡、读卡器及一套 CH343 `21442` UART。Mac 逐台采集，无同时采三机假设。[BOM](poc-purchase-list.md#hw1-决策成本复核2026-11-15) · [串口准备](hardware-validation-plan.md#mac-串口的来源与判定) | 核对精确料号、赠送线、供电、固定件和数据转接。绑定 Mac 上处理 1.5 Mbaud 的终端及驱动来源、版本与准备方式。需安装时纳入对应授权。记录四张卡的实际容量和脱敏标识，绑定回读与故障原卡保留路径 |
| 固定 OS / BSP | Armbian 26.8.1 Minimal、Debian 13 arm64、`6.1.115-vendor-rk35xx`，验签及镜像内 DTB、驱动、引导链已核对。[固定输入与证据](hw1-armbian-image-verification.md#固定输入与官方主钥) | 评审者确认固定镜像的隔离台架用途。按既有 HW-G1 范围核对内核、U-Boot、板级包、固件版本的来源、许可、更新和维护归属。镜像、版本或用途变化时重新绑定证据 |

走 HW-G2 时，沿[已有成本复核](poc-purchase-list.md#hw1-决策成本复核2026-11-15)核对差额、税运、交付和退换条件。首轮到手加预留须落在 2000 元以内，整套累计硬上限为 3000 元。

### 公开交付资料复核（2026-11-15）

- Arace 的 2GB 商品数据将 variant `42572748325044` 绑定到 `RS109-D2E0H1`，本次该 variant 显示不可售。京东 2GB 和淘宝公开入口未给出能绑定三台交付的 PCB 版号。Radxa V1.2 硬件资料可用于核对丝印和接口，实际版号仍由交付证据确认。[Arace 2GB 商品数据](https://arace.tech/products/radxa-zero-3e.js) · [京东 2GB 入口](https://i-item.jd.com/10123797922358.html) · [淘宝入口](https://item.taobao.com/item.htm?id=763867803082) · [Radxa 3E V1.2 资料](https://docs.radxa.com/zero/zero3/download)
- 微雪公开资料列 RTL8153，国内商品接口未返回硬件版号或 USB 枚举字段。`0bda:8153` 继续作为固定镜像驱动别名的匹配预期，交付批次与枚举仍待确认。[微雪产品规格](https://www.waveshare.com/wiki/USB_3.2_Gen1_TO-Gigabit_ETH) · [国内商品数据](https://api.waveshare.net/api/v1/goods/1379)

所有者核对交付时可使用下面两条。此次只读公开资料。

1. 这三块能否按 ZERO 3E 2GB 带排针、料号 `RS109-D2E0H1`、相同 PCB revision 交付？请标明版号或提供对应批次的丝印照片，并确认无替代发货。
2. 网卡是否为 `20162`、RTL8153？能否提供该批次的硬件版号和通用 USB 枚举记录，例如 VID/PID、`bcdDevice`？如没有枚举记录，请确认实物规格不符时的退换条件，供评审者决定是否到货复核。

## 固定 BSP 来源复核

从已验签镜像只读取得包状态、changelog、版权文件和 APT 配置。镜像写入的构建短标识映射到公开 Armbian Build 提交 `de5fd0aeb69d1670f8d2894c4f06cca931feba9b`。下列依据用于固定版台架评审，重建完整镜像仍需逐项绑定全部构建输入和补丁。[Armbian 固定构建提交](https://github.com/armbian/build/commit/de5fd0aeb69d1670f8d2894c4f06cca931feba9b)

| 组件 | 本次绑定与来源 | 许可和维护边界 |
| --- | --- | --- |
| 内核、DTB 与板级包 | 采用固定构建提交的 ZERO 3 和 rk35xx vendor 配置。镜像内的内核/DTB 包版本为 `26.8.1`，内核版本为 `6.1.115-vendor-rk35xx`。镜像内核制品标识的 `S5280` 与构建脚本的 Git 前缀字段对应，在构建前的 `rk-6.1-rkr5.1` 历史定位到 `5280f9b4336199c4025c8eed894d2b4e2268dcc6`，其 Makefile 版本为 6.1.115。构建配置还指定 `rk35xx-vendor-6.1` 补丁目录。板级包 `armbian-bsp-cli-radxa-zero3-vendor` 为 26.8.1。[固定构建配置](https://github.com/armbian/build/blob/de5fd0aeb69d1670f8d2894c4f06cca931feba9b/config/sources/families/rk35xx.conf) · [内核源码](https://github.com/armbian/linux-rockchip/tree/5280f9b4336199c4025c8eed894d2b4e2268dcc6) | 内核和 DTB 包内版权说明引用 GPL v2，与上游 [COPYING](https://github.com/armbian/linux-rockchip/blob/5280f9b4336199c4025c8eed894d2b4e2268dcc6/COPYING) 一并留证。Armbian Linux 为已装包维护者。成品补丁应用与板级包完整材料仍按未来实际更新或交付范围复核 |
| U-Boot、DDR 与 BL31 | U-Boot v2025.10 上游提交 `e50b1e8715011def8aff1588081a2649a2c6cd47` 由包内元数据直接标识。RkBin 参考提交 `ce50c0d1ad1d4630f5b595d317427b56fa97b7f6` 的 `rk3566_ddr_1056MHz_v1.21.bin` 全文件匹配打包产物中的 DDR 字节，`rk3568_bl31_v1.44.elf` 六个非空装载段也与打包产物对应数据一致。[U-Boot 固定源码](https://github.com/u-boot/u-boot/tree/e50b1e8715011def8aff1588081a2649a2c6cd47) · [Armbian RkBin 参考提交](https://github.com/armbian/rkbin/tree/ce50c0d1ad1d4630f5b595d317427b56fa97b7f6) | 分别保留上游 [U-Boot 许可说明](https://github.com/u-boot/u-boot/blob/e50b1e8715011def8aff1588081a2649a2c6cd47/Licenses/README) 与 [RkBin 厂商条款](https://github.com/armbian/rkbin/blob/ce50c0d1ad1d4630f5b595d317427b56fa97b7f6/LICENSE.TXT)。RkBin 的结论覆盖已匹配的二进制字节，整套引导制品仍随 Armbian 的已装 U-Boot/板级包评审 |
| `r8152` 所需固件 | 已装 `armbian-firmware` 为 26.8.1。制品字段 `SAd984` 对应 Armbian 固件 Git 前缀，在构建前历史定位到 `d9846710f54da5e4383e2d67311819659ac2cf5c`。镜像中模块声明的七个 `rtl_nic` 文件均与该提交及 linux-firmware 固定引用逐字节一致。[Armbian 固定固件目录](https://github.com/armbian/firmware/tree/d9846710f54da5e4383e2d67311819659ac2cf5c/rtl_nic) | 上游 [WHENCE 与归属](https://kernel.googlesource.com/pub/scm/linux/kernel/git/firmware/linux-firmware.git/+/ccc21e6c929735cd0b55c7c892062dedca2c47d9/WHENCE) 逐个列出七个文件，并指向 [Realtek 许可文本](https://kernel.googlesource.com/pub/scm/linux/kernel/git/firmware/linux-firmware.git/+/ccc21e6c929735cd0b55c7c892062dedca2c47d9/LICENCE.rtlwifi_firmware.txt)。该证据覆盖 RTL815x 固件使用范围 |

镜像内 `armbian.sources` 指向 Armbian trixie 仓库，以镜像内 Armbian keyring 作 `Signed-By`；Debian 基础包配置独立的 Debian trixie、updates、backports 和 security 来源。已读取这些配置作更新来源记录，未执行 APT、当前索引验签或升级。评审固定隔离用途时以已验签版本为基线。后续若引入更新，单列仓库签名链、精确版本差量与启动/网口/恢复回归。[Armbian 包仓库说明](https://docs.armbian.com/status/package-repository/)

## Mac 串口准备路径

主体路线为到货后先核对 macOS 现有驱动，再使用固定的 pySerial 3.5 隔离环境做 UART 回环。Mac 本机为 macOS 27.0.1 arm64、Python 3.14.5，PATH 未见 `picocom` 或 `minicom`，当前解释器无 pySerial，`termios` 无 `B1500000`。已从 PyPI 复核 90585-byte wheel 的实际文件 SHA-256 为 `c4451db6ba391ca6ca299fb3ec7bae67a5c55dde170964c7a14ceefec02f2cf0`，28 个 `serial/` Python 模块在 Python 3.14.5 下均可静态解析，Darwin 路径使用 `IOSSIOSPEED` 请求自定义波特率。导入、驱动绑定和 1.5 Mbaud 实际收发仍待执行。[PyPI 固定 wheel](https://pypi.org/project/pyserial/3.5/#files) · [Darwin 实现](https://github.com/pyserial/pyserial/blob/0e7634747568547b8a7f9fd0c48ed74f16af4b23/serial/serialposix.py)

1. 到货后先核对 CH343 `21442` 适配器的实际 VID/PID、USB 接口模式，以及同一次插拔出现的唯一 `/dev/cu.*` 节点。若现有 macOS 驱动未提供可绑定的串口，按[已有 WCH 驱动评审](hardware-validation-plan.md#mac-串口的来源与判定)核对当前系统支持、签名、公证和卸载方式，再决定是否进入单独的安装授权。
2. 工具安装获准后，把核对过摘要的 wheel 用 `pip --no-index --no-deps` 安装到独立的临时 Python 环境。先记录 Python 与 pySerial 版本并核对终端入口，测试参数固定为 `serial.tools.miniterm`、实际唯一串口、`1500000`、8N1、无软硬件流控。该工具留在宿主配置阶段，来源与许可证见[pySerial 固定源码](https://github.com/pyserial/pyserial/blob/0e7634747568547b8a7f9fd0c48ed74f16af4b23/LICENSE.txt)。
3. 板卡未连接时先验证适配器的 3.3 V TTL 和回环。仅在合成输入完整回显后进入 `HW1-02`。无法设置速率、丢字、乱码、端口对应不清时保留诊断，先处置串口路径。上板首启仍要实际核对 U-Boot、`ttyS2`、`ttyFIQ0` 与无 DHCP 交互。

## 上板验收条件

以下沿用 `HW1-01–05` 的编号。先核对到货标签和供电，再验证引导与首启、拓扑与重枚举、备卡与恢复。每次启动绑定镜像摘要、实际板和卡标签，配置和日志只保留脱敏结果。

| 用例 | 通过条件与留证 | 停止或无效条件 |
| --- | --- | --- |
| `HW1-01` 实物对应 | 三台实物丝印、RAM/SKU、排针、电源和卡与交付记录匹配。B 上明确区分板载 RJ45 与 USB Host 网卡，记录数据转接和物理口映射 | SKU、版号或供电偏离已审证据时暂停上电。若网卡盒标与评审不符，先隔离该配件 |
| `HW1-02` 引导选型 | A/B/C 各留冷启动证据，确认从各自 microSD 启动。U-Boot 的板型识别或 FIT 取证明确选中 3E 项，Linux 阶段确认使用固定内核与 `armbianEnv.txt` 指向的通用 `rk3566-radxa-zero3.dtb`，结合活动 DT 的 `model`、`compatible` 和实际接口交叉核对。实际 FIT 的 3W 默认项与 Linux 通用 DT 是两处不同的选择点 | 发现 3W 回退、加载其他 DT/内核、根分区不是指定卡，或无法确认选型时停下，保留日志。只看到登录提示时选型仍待证。额外引导诊断按 HW-G3 范围评审 |
| `HW1-02` 无 DHCP 串口首启 | 先完成 Mac 与 CH343 回环并确认 `UART2_M0`、3.3 V TTL、1500000 8N1、无流控。逐台从送电开始只收日志，待登录提示后交互。在没有 DHCP、外网、Wi-Fi、HDMI 或 SSH 配置依赖的台架，记录 U-Boot 输出、Linux 命令行和活动 console、`ttyFIQ0` getty 及首次配置完成。各接口静态地址与后续点对点拓扑一致，确认处置默认凭据并使用独立节点配置 | 串口乱码、首登中断、等待 DHCP 无法完成或依赖额外外设时停下。若能登录但内核串口日志缺失，保留启动与 getty 结果，诊断 `console=ttyS2,1500000` 与 `ttyFIQ0` 的差异后再裁决。禁止在受测镜像上临时改动 boot 参数使该轮通过 |
| `HW1-03/04` B 双网口、并行与恢复 | A–B、B–C 为两段无旁路的直连。核对 B 无网桥，IPv4/IPv6 通用转发均关闭。保存两侧口映射、驱动/固件、链路及路由，先逐段确认双向可达，再确认两段并行可用，且 A↔C 双向直连失败。分别只断开一段，覆盖 B USB 网卡拔插及重枚举。受影响链路在评审窗口内回到正确端口，另一段保持可达，恢复后重复 A↔C 负例 | 任一邻接链路不通时，直连负例无效。有旁路、端口串位、驱动或固件错误、另一段受影响或超窗未恢复时停下，保留相邻探针、故障时点和内核日志 |
| `HW1-05` 备卡恢复 | 每台从同一固定完整镜像的干净备卡基线开始。核对目标卡真实容量，首次启动前回读原始镜像长度范围并核对 SHA-256。逐台先留正常关机基线，冷关机后取出并保留原卡，单卡替换。以串口证明从备卡冷启，核对根卡、扩容结果、暖重启及本地重新配置，验证能恢复该节点的静态接口条件。共用备卡换到下一台前须在获准范围内恢复基线并重新回读，原卡与备卡始终有独立标签 | 目标卡、回读、引导介质不符，UUID 同时可见导致根卡歧义，扩容/复启失败或无法重新配置时停止。保留原故障卡，后续写入需重新确认。应用持久化、去重和安全状态恢复按 `HW1-08` 对应前置取证 |

沿用现有 HW1 manifest 冻结样本数、观察超时、并行探针窗口和恢复时限，并记录镜像摘要、脱敏硬件标签、端口与卡映射、预期结果、日志、结束状态和失败处置。三台分别保留启动、串口与备卡证据。试验中的单节点和整台架断电范围仍按 `HW1-05` 单独标明。启动日志、终端记录和照片不得保留凭据或真实通信。

## 评审收口

本轮建议保留现有候选，等待板卡和网卡交付证据。板卡需绑定三台同版的交付范围；网卡若只能在到货后枚举，先由评审者明确可接受的交付和退换条件。固定 BSP 按隔离台架用途评审，Mac 的驱动绑定与回环纳入 HW-G3 前置。

- **继续候选评审**：交付版号和网卡规格有可复核证据，评审者接受镜像与 BSP 剩余风险，明确采购前缺口及上板取证安排。上板验收须具备设备及 HW-G3 执行授权。
- **等待**：卖家版号未落实、网卡交付范围不清、串口准备方式或 BSP 证据仍缺。记录缺项、责任方及重新评审的触发条件。
- **改选或重审**：交付偏离已审配置，关键接口或启动/恢复路径失效，或预算超出已确认边界。改选时保留三节点、B 双独立链路与相同验收目标。

本轮只整理评审材料。当前无采购、写卡或上板结果，HW-G1 等待评审。

### 下一批交付与裁决（2026-11-15）

下一批只评审当前完整三节点候选的一套交付材料。交付物为一份 `继续 / 等待 / 改选或重审` 记录，附证据日期、适用数量或批次、评审人，以及下一门所需的未关闭项。当前判定为 **等待交付证据**。

| 输入与责任 | 接受口径 | 缺失或不符时的处置 |
| --- | --- | --- |
| 三台板卡。所有者提供卖方确认或对应交付批次的材料 | 同一份材料可覆盖三台，须同时绑定 `RS109-D2E0H1`、2GB、带排针、确切 PCB revision、数量 3 和无替代发货。照片须能与所选 SKU 和批次对应 | 仅有通用产品页或单板照片时保留等待。交付范围明确包含混版或替代 SKU 时回到候选评审 |
| B 节点网卡。所有者提供同一交付件的规格和退换条件，台架评审者判断风险 | 先确认 `20162` / RTL8153，并将可取得的版号、批次、VID/PID 和 `bcdDevice` 关联到实际供货。若供应侧只能确认型号与芯片，由评审者记录是否允许到货后枚举、覆盖规格不符的退换条件与费用归属。到货记录仍须绑定 USB 端口、接口类和实际驱动 | 型号、芯片或退换适用范围不明时等待。允许延后枚举的裁决须随 HW-G2 材料交接，到货发现偏离时停止 `HW1-03/04` 并复核替换或退换 |
| 固定 BSP。台架评审者记录接受范围 | 以现有验签和来源材料确认固定 Armbian 镜像仅用于隔离有线台架，确认 `HW1-02/05` 的实际引导选型、暖重启和备卡恢复待验项。Mac 串口实际绑定及回环进入 HW-G3 | 固定镜像用途或剩余风险未获评审时等待。镜像、板型或用途改变时重新绑定证据 |

材料齐备后，台架评审者按以上输入形成 HW-G1 裁决。继续时交给所有者准备 HW-G2，并按现有 BOM 填实际到手额 `H`、预留 `R` 与后续预算，核对 `H + R ≤ 2000`、整套累计 ≤ 3000 元。供应证据仍缺时，记录缺项并在收到对应材料后恢复这一批。采购及到货后的写卡、串口、上电和有线测试分别走 HW-G2/HW-G3。 [费用口径](poc-purchase-list.md#hw1-决策成本复核2026-11-15) · [台架执行门](hardware-validation-plan.md#hw-g3台架执行授权)
