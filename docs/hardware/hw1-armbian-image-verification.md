# HW1 Armbian 固定镜像验签操作包

- 日期：2026-11-15
- 状态：离线验签及 Mac 只读静态核对已完成。ZERO 3E 的实际引导选择、双网口、串口首启和备卡恢复待上板验证
- 目标：为三台 ZERO 3E 2GB 的无射频台架审查同一份 Armbian 压缩镜像
- 范围：一次主机预检、四次固定输入 HTTPS GET、隔离钥匙环验签和证据留存。验签通过后评审本页静态核对项。采购、安装工具、启动虚拟机、解压、挂载、写卡、启动镜像和实体操作各按相应范围处理

## 主机与工具

| 环境 | 已知事实 | 判定 |
| --- | --- | --- |
| 当前 Mac | macOS 27.0.1、arm64，已定位 curl 8.7.1、shasum 6.02、XZ Utils 5.8.4 和 DTC/fdtget 1.8.1。PATH 及已查安装目录未定位 `gpg`、`gpgv`、`sq` 或 `sqv` | 可用于发布元数据和后续部分静态核对，当前缺 OpenPGP 验签器 |
| 通用 `Debian13-ARM64` | 已在 Debian 13.6 arm64 的普通账户确认 Debian 打包的 `/usr/bin/gpg` 为 GnuPG 2.4.7，`curl` 为 8.14.1，`sha256sum` 为 GNU coreutils 9.7 | 已使用现有工具对 Mac 离线交付的固定输入验签。见[离线验签记录](#离线验签记录2026-11-15) |

选用 GnuPG 的 `gpg` 读取证书及验证签名，同时判定过期、撤销和签名状态。`gpgv` 将钥匙环内的密钥视为可信，且不检查密钥过期、撤销，不能直接替换下列命令。Debian 包索引用于确认来源与版本基线，实际安装状态由主机预检确认。[Debian gpg 包](https://packages.debian.org/trixie/gpg) · [GnuPG gpgv 说明](https://www.gnupg.org/documentation/manuals/gnupg/gpgv.html) · [当前测试机记录](../status/current.md#当前阶段)

## 固定输入与官方主钥

镜像 `Armbian_26.8.1_Radxa-zero3_trixie_vendor_6.1.115_minimal.img.xz` 对应 Debian 13 Minimal (CLI)、vendor 6.1.115。镜像及两个旁文件使用命令中同一个版本化归档地址。[Armbian 板型页](https://armbian.com/boards/radxa-zero3) · [发布归档](https://k-space.ee.armbian.com/dl/radxa-zero3/archive/)

| 对象 | 大小（bytes） | SHA-256 |
| --- | ---: | --- |
| 压缩镜像 `.img.xz` | 394195116 | `e78de9cb4b2d8ff1710c65c509d08faef1ba88c07fc3276b9dd637597e8234e8` |
| 发布者校验文本 `.img.xz.sha` | 129 | `c1b22a4efc90946dbed3039cd4deb1bd6374ddb875ba721368d73955cd92f60e` |
| detached signature `.img.xz.asc` | 833 | `559efd4683d27d533206509d68d7b318ce893551faf38399f047490c9a9137c3` |
| Ubuntu keyserver 返回的旧公钥证书 | 6315 | `f4a2a8cde035572584e5f7457519e9a5cd042c9f8e5ed39d68b1da61591732a1` |

- Armbian 官方验签指南明确列出旧主钥 `DF00FAF1C577104B50BF1D0093D6889F9F0E78D5` 及 Ubuntu keyserver 获取路径。本轮用完整指纹经 HTTPS 读取该服务的公开索引和证书。以官方文档为身份来源，keyserver 用于交付证书。[Armbian 官方验签指南](https://docs.armbian.com/getting-started/choosing-an-image/#download-and-verification) · [Ubuntu keyserver 固定指纹查询](https://keyserver.ubuntu.com/pks/lookup?op=index&options=mr&search=0xDF00FAF1C577104B50BF1D0093D6889F9F0E78D5)
- 准备操作包时对已绑定 SHA-256 的 833-byte `.asc` 做了只读包元数据检查。其 issuer 声明为旧主钥完整指纹，签名时间字段为 `2026-08-12T20:41:08Z`，类型为 OpenPGP v4 binary document，算法字段为 RSA 与 SHA-512。验签时要求实际签名键和主钥均等于固定指纹。
- 11 月 15 日官方 `apt.armbian.com/armbian.key` 的读取经官方入口重定向到镜像源，返回新主钥 `8CFA83D13EB2181EEF5843E41EB30FAF236099FE`。该源用于说明当前 APT 公钥与此历史镜像的区别。后续只使用本表的旧证书，证书材料漂移时停下复核，包括可能影响旧钥状态的更新。[Armbian APT 公钥入口](https://apt.armbian.com/armbian.key)

## 来宾直连方案操作命令

在已运行且已获准的 `Debian13-ARM64` 交互终端，先执行以下只读预检。确认是目标会话、arm64 Debian 13、非 root、主机 UTC 正确，确认 GnuPG 版本与包归属、curl 至少 8.4.0，且 `/tmp` 可用至少 512 MiB。把实际路径、版本、包归属与时间关联到本次证据。该版本要求确保 curl 可按接收量执行上限。[curl 文件上限说明](https://curl.se/docs/manpage.html#--max-filesize)

```bash
cat /etc/os-release
uname -m
id -u
date -u '+%Y-%m-%dT%H:%M:%SZ'
df -Pk /tmp
dpkg-query -S /usr/bin/gpg /usr/bin/curl /usr/bin/sha256sum
dpkg-query -W -f='${Package}\t${Version}\t${Architecture}\n' gpg curl coreutils
/usr/bin/gpg --version
/usr/bin/curl --version
/usr/bin/sha256sum --version
```

主机预检和执行范围确认后，在该会话按整段 Bash 执行一次。只向归档镜像及 Ubuntu keyserver 发出 HTTPS GET，不跟随重定向或自动重试。小文件各限 20 秒，镜像限 600 秒，验签限 90 秒，预计最多约 13 分钟，系统调度时延另计。只写新建的 `/tmp/hw1-armbian-verify.*` 私有目录，约需 376 MiB 镜像空间及少量日志。进入验证的镜像保持压缩。

```bash
/bin/bash <<'HW1_VERIFY'
set -Eeuo pipefail
trap 'rc=$?; printf "STOP line=%s exit=%s command=%s\n" "$LINENO" "$rc" "$BASH_COMMAND" >&2' ERR
export LC_ALL=C
umask 077
[[ $EUID -ne 0 && "$(uname -m)" == aarch64 ]]
grep -qx 'ID=debian' /etc/os-release
grep -qx 'VERSION_ID="13"' /etc/os-release
test "$(df -Pk /tmp | awk 'NR == 2 {print $4}')" -ge 524288
gpg_version=$(/usr/bin/gpg --version)
[[ "${gpg_version%%$'\n'*}" == 'gpg (GnuPG) 2.4.7' ]]
curl_version=$(/usr/bin/curl --version)
awk 'NR == 1 {
  if ($1 != "curl" || $2 !~ /^[0-9]+\.[0-9]+\.[0-9]+$/) exit 1
  split($2, version, ".")
  exit !(version[1] > 8 || (version[1] == 8 && version[2] >= 4))
}' <<< "$curl_version"

run=$(mktemp -d /tmp/hw1-armbian-verify.XXXXXXXX)
printf 'evidence_dir=%s\n' "$run"
cd "$run"
mkdir -m 700 gnupg
{
  date -u '+%Y-%m-%dT%H:%M:%SZ'
  uname -m
  printf '%s\n' "$gpg_version"
  printf '%s\n' "$curl_version"
  dpkg-query -W -f='${Package}\t${Version}\t${Architecture}\n' gpg curl coreutils
} > host-tools.log

image=Armbian_26.8.1_Radxa-zero3_trixie_vendor_6.1.115_minimal.img.xz
base=https://k-space.ee.armbian.com/dl/radxa-zero3/archive
fpr=DF00FAF1C577104B50BF1D0093D6889F9F0E78D5
key_url="https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x$fpr"
image_sha=e78de9cb4b2d8ff1710c65c509d08faef1ba88c07fc3276b9dd637597e8234e8

get_fixed() {
  local name=$1 bytes=$2 deadline=$3 url=$4 http rc
  if http=$(/usr/bin/curl -q --proto '=https' --fail --silent --show-error \
      --connect-timeout 15 --max-time "$deadline" --max-filesize "$bytes" \
      --output "$name" --write-out '%{http_code}' "$url" 2>"$name.curl.log"); then
    rc=0
  else
    rc=$?
  fi
  printf '%s\t%s\t%s\t%s\n' "$name" "$rc" "$http" "$url" >> downloads.tsv
  [[ $rc == 0 && $http == 200 ]]
  test "$(stat -c '%s' -- "$name")" -eq "$bytes"
}

get_fixed "$image.sha" 129 20 "$base/$image.sha"
get_fixed "$image.asc" 833 20 "$base/$image.asc"
get_fixed armbian-legacy.asc 6315 20 "$key_url"
printf '%s  %s\n' \
  c1b22a4efc90946dbed3039cd4deb1bd6374ddb875ba721368d73955cd92f60e "$image.sha" \
  559efd4683d27d533206509d68d7b318ce893551faf38399f047490c9a9137c3 "$image.asc" \
  f4a2a8cde035572584e5f7457519e9a5cd042c9f8e5ed39d68b1da61591732a1 armbian-legacy.asc \
  > sidecars.expected.sha256
/usr/bin/sha256sum --strict --check sidecars.expected.sha256 > sidecars.check.log
printf '%s %s\n' "$image_sha" "$image" | cmp -s - "$image.sha"

gpg_local=(/usr/bin/gpg --no-options --homedir "$run/gnupg" --no-default-keyring
  --batch --no-tty --no-autostart --disable-dirmngr --no-auto-key-retrieve --no-auto-key-import)
timeout 10s "${gpg_local[@]}" --with-colons --with-fingerprint \
  --show-keys armbian-legacy.asc > key.colons 2> key.stderr
awk -F : -v want="$fpr" '
  $1 == "pub" { pubs++; primary = 1; if ($2 ~ /[redi]/) bad = 1; next }
  $1 == "sub" { primary = 0 }
  $1 == "fpr" && primary { fingerprints++; if ($10 != want) bad = 1; primary = 0 }
  END { exit !(pubs == 1 && fingerprints == 1 && !bad) }
' key.colons
timeout 10s "${gpg_local[@]}" --output "$run/armbian-legacy.gpg" \
  --dearmor armbian-legacy.asc 2> dearmor.stderr

get_fixed "$image" 394195116 600 "$base/$image"
printf '%s  %s\n' "$image_sha" "$image" > image.expected.sha256
/usr/bin/sha256sum --strict --check image.expected.sha256 > image.check.log

if timeout 90s "${gpg_local[@]}" --keyring "$run/armbian-legacy.gpg" \
    --status-fd 1 --verify "$image.asc" "$image" > verify.status 2> verify.stderr; then
  printf '0\n' > verify.exit
else
  rc=$?
  printf '%s\n' "$rc" > verify.exit
  exit "$rc"
fi
awk -v want="$fpr" '
  $1 == "[GNUPG:]" && $2 == "GOODSIG" { good++ }
  $1 == "[GNUPG:]" && $2 == "VALIDSIG" {
    valid++
    if ($3 != want || $5 != 1786567268 || $7 != 4 ||
        $9 != 1 || $10 != 10 || $11 != "00" || $12 != want) bad = 1
  }
  $1 == "[GNUPG:]" && $2 ~ /^(BADSIG|ERRSIG|NO_PUBKEY|EXPSIG|EXPKEYSIG|REVKEYSIG|KEYEXPIRED|KEYREVOKED|SIGEXPIRED|NODATA|ERROR|FAILURE)$/ { bad = 1 }
  END { exit !(valid == 1 && good == 1 && !bad) }
' verify.status
printf 'Image signature PASS: %s\n' "$image"
HW1_VERIFY
```

## 验收与收尾

- **通过**：四个固定对象实收长度与 SHA-256 都匹配，`.sha` 指向精确文件名。钥匙环只含官方文档列出的这一个主钥证书，实际验签退出 0，状态有且仅有一条 `GOODSIG` 与一条符合固定签名字段的 `VALIDSIG`，签名键和主钥指纹都匹配。评审时核对 `verify.stderr` 的诊断。隔离钥匙环可能显示 Web of Trust 未认证提示，本包以官方指纹绑定作身份依据。[GnuPG 状态字段](https://raw.githubusercontent.com/gpg/gnupg/master/doc/DETAILS) · [Debian GnuPG 用法](https://manpages.debian.org/trixie/gpg/gpg.1.en.html)
- **停止**：主机或时钟无法确认，工具缺失或版本不符，HTTP 非 200、超时、重定向、文件漂移，或证书及签名状态报告错误、过期或撤销，均停止。保留命令退出码和关键诊断，禁止为通过而关闭检查、切换密钥或改用未批准的镜像。
- **留存与清理**：保留输出的精确运行目录、主机预检、`host-tools.log`、下载记录及诊断、SHA-256 结果、密钥列表、`verify.status`、`verify.stderr` 和 `verify.exit`。命令结束后无长期任务。复核后按实际创建的目录路径移交日志，确认已留存所需证据再清理本次目录与文件。镜像本体、公钥及原始日志不进入 Git。若后续换环境检查兼容性，再核对同一压缩镜像 SHA-256。

## 单次执行记录（2026-11-15）

- 所有者授权启动通用 `Debian13-ARM64`，工具预检通过后执行一次本包。以 UTM 注册名称和唯一标识核对目标，启动前为 `stopped`。首次普通账户选择因发现多个本地账户而停止，随后通过来宾会话和普通账户 UID 1000 取得完整只读预检。来宾代理的交互输出回传不完整，使用来宾临时文件取回的正文和完成标记复核结果。
- 11:22 CST 完成预检。来宾为 Debian 13.6、aarch64、非 root，来宾 UTC 与宿主同一时间窗口，`/tmp` 为 tmpfs，预检可用 2003736 KiB。已装包为 `gpg 2.4.7-21+deb13u1+b4`、`curl 8.14.1-2+deb13u4`、`coreutils 9.7-3`，均为 arm64，三个二进制的归属与包记录一致。
- **STOP / 来宾出站网络**。操作包执行一次，退出码 1。第一项 GET 为固定镜像的 `.sha`，curl 退出码 6，HTTP 状态字段为 `000`，诊断为 `Could not resolve host: k-space.ee.armbian.com`。后续只读采样未发现 IPv4/IPv6 全局地址或默认路由，`resolv.conf` 有两项 nameserver。故障发生在取件阶段，尚未取得来宾本次的任何固定输入，也没有镜像验签结果。
- 宿主已保存完整预检、执行退出码、失败下载记录和诊断。11:24 CST 通过 `utmctl stop --request` 正常关机并确认 `stopped`。本次没有安装工具、调整网络、采购或刷写。该单次执行已结束，下一次执行前须明确固定输入如何抵达来宾，例如另行评审由 Mac 下载并将固定输入离线交付到来宾。验签仍在 Debian 中按上述长度、摘要和指纹条件判定。

## 离线验签记录（2026-11-15）

- 所有者授权使用 Mac 下载四个固定输入，经 UTM 来宾代理交付给通用 `Debian13-ARM64`，按相同标准验签一次后正常关机。Mac 从固定归档和 Ubuntu keyserver 获取输入。压缩镜像通过同归档 12 个 HTTP byte range 下载并按顺序拼接，每段的 HTTP 206、Content-Range 与长度均匹配。校验聚合镜像通过后停止较慢的并行整文件下载。Mac 对四个完整输入均通过固定长度与 SHA-256 检查。
- 虚拟机启动前为 `stopped`。12:26 CST 来宾预检通过，Debian 13.6 aarch64，验签用普通账户 UID 1000。来宾 `/tmp` 为 tmpfs，预检可用 2003740 KiB，IPv4/IPv6 默认路由计数均为 0。三个小文件通过 `utmctl file push` 交付，压缩镜像经 `utmctl exec --input` 写入 94 个临时块。来宾复算首块摘要，全部按实收长度复核后按序重组。
- **PASS**。12:28 CST 使用 Debian 包 `gpg 2.4.7-21+deb13u1+b4` 执行一次离线验签。来宾独立复核四项长度及 SHA-256，确认发布者校验文本指向固定文件名，密钥列表仅含预期主钥证书。验证命令和整次操作均退出 0，GPG 分别给出一条 `GOODSIG` 和 `VALIDSIG`，实际签名键及主钥均为 `DF00FAF1C577104B50BF1D0093D6889F9F0E78D5`。签名时刻为 `2026-08-12T20:41:08Z`，状态含预期的 `TRUST_UNDEFINED`，无拒绝状态。发布者身份依据为官方公布的完整指纹。
- 宿主保留固定压缩镜像、三项旁文件和预检、传输、验签证据，来宾临时文件位于 tmpfs。日志取回后用 `utmctl stop --request` 正常关机，12:28:49 CST 确认 `stopped`。本次未改变虚拟机网络、安装工具、解压、挂载、采购或刷写。后续仍须完成镜像内 BSP 与兼容性静态核对。

## 验签通过后的兼容性核对

静态核对先用 `xz --robot --list -- "$image"` 记录展开长度。仅将内容解到临时目录的普通文件，采用只读文件系统检查工具记录分区表、根分区、实际 OS 与镜像标识。

| 核对项 | 静态验收依据 | 仍须在到货后验证 |
| --- | --- | --- |
| ZERO 3E 与 DTB | 从 `/boot/armbianEnv.txt`、引导脚本及镜像实际 DTB 交叉核对板型。记录实际 DTB 名称、`model`、`compatible`、板载 GMAC/PHY、USB Host、microSD 和 UART 节点。按卖家交付 PCB revision 判断适用性，标明所依据的板型和引导选择机制 | 三台实际活动 DTB、冷启动及板载网口功能 |
| B 的两个独立有线接口 | 核对内核配置、`r8152` 模块或内建支持、对应模块别名、实际 USB VID/PID 的覆盖及适用固件。核对板载 GMAC/PHY 与 USB Host 所需驱动 | 两链路并行、A/C 直连负例、B 网卡拔插及重枚举 |
| 首启、串口与备卡 | 核对引导控制台、内核 console、串口登录服务和首次配置流程。用 Radxa 的 UART2_M0/3.3 V TTL/1500000n8 规格核对镜像路径，确认无 DHCP 时仍有配置途径。记录 microSD 启动与同版备卡的救援路径、Mac CH343 串口工具和驱动前置 | 串口冷启动与交互、暖重启、独立断电及备卡恢复 |
| BSP 来源及维护 | 记录镜像中的内核、DTB、U-Boot、固件和软件包版本，核对来源、许可、更新渠道与固定基线的维护责任 | 实际版本、启动与恢复证据，后续更新另行评审 |

静态依据与交付规格汇入 HW-G1。到货后的执行条件与证据见[HW1 台架验收表](hardware-validation-plan.md#hw1-台架验收表2026-11-08)，签名通过只确认固定镜像的完整性和发布者签名。

### 静态兼容性核对结果（2026-11-15）

- Mac 重算已验签压缩件的 SHA-256，结果为 `e78de9cb4b2d8ff1710c65c509d08faef1ba88c07fc3276b9dd637597e8234e8`。XZ 列表与实际解压均为 2332033024 bytes，解压件 SHA-256 为 `a78367cc5d099872cfc0664c0e18215ce4538931205a35b4cfef960220bb0dc8`。用 7-Zip 26.01 从普通文件读取 GPT/ext4，以 DTC/fdtget 1.8.1 检查 DTB 与 U-Boot FIT。未挂载镜像、未启动镜像或虚拟机，未执行镜像内脚本。
- 镜像内标识为 Armbian 26.8.1 stable、Debian 13 trixie、arm64，实际内核为 `6.1.115-vendor-rk35xx`。`linux-image-vendor-rk35xx`、`linux-dtb-vendor-rk35xx`、`linux-u-boot-radxa-zero3-vendor`、板级 BSP 和 `armbian-firmware` 的已装包版本均为 `26.8.1`。U-Boot 制品标识为 `2025.10-Se50b-P0ed0-Hfcd7-V3cde-B5da4-R448a`，包元数据指向上游 v2025.10 的 `e50b1e8715011def8aff1588081a2649a2c6cd47`。打包补丁与固件的完整来源和维护归属仍待 BSP 评审。

| 核对项 | 实际静态证据与判断 | 上板验证 |
| --- | --- | --- |
| ZERO 3E 引导与 DTB | 原始镜像的 U-Boot FIT 有 3W 和 3E 两项，默认项为 3W。包元数据所指的 U-Boot 上游源码有按 ADC 硬件 ID 选择 3E 的逻辑。Linux 阶段另由 `armbianEnv.txt` 固定加载 `rockchip/rk3566-radxa-zero3.dtb`，经 `boot/dtb` 链接指向 `6.1.115-vendor-rk35xx`。实际 Linux DTB 的 `model` 为 `Radxa ZERO 3`，`compatible` 含 `radxa,zero3-aic8800ds2`。已核对 `boot.scr` 的头部和负载 CRC，负载与 `boot.cmd` 一致。两阶段均有可核对的配置，交付版号是否正确选择仍待验证。[U-Boot 固定上游源码](https://github.com/u-boot/u-boot/blob/e50b1e8715011def8aff1588081a2649a2c6cd47/board/radxa/zero3-rk3566/zero3-rk3566.c) | 记录三台 PCB revision、上电时 U-Boot 选中的 FIT 配置和 Linux 活动 DTB。首启后确认板型、启动介质、板载接口映射及纯有线条件 |
| B 双网口 | 实际 Linux DTB 启用 `ethernet@fe010000` 的 RGMII GMAC、MDIO PHY 和 `/usbhost/usb@fd000000` 的 DWC3 Host。内核内建 stmmac、dwmac-rockchip、Realtek PHY、DWC3 和 xHCI；`r8152.ko` 为 v1.12.13，vermagic 对应实际内核，覆盖 `0bda:8153`，模块声明的七个 `rtl_nic` 固件文件均在镜像中。同 VID/PID 还有针对 CDC 接口类的 `r8153_ecm` 别名。DTC 报告的同址 UIO 节点为 `disabled`。[微雪 RTL8153 候选规格](https://www.waveshare.com/product/usb-3.2-gen1-to-gigabit-eth.htm) | 以实际交付网卡核对 VID/PID、接口类、驱动和固件加载。分别验证板载口与 USB 口独立链路、同时通信、A/C 无直连及 USB 拔插后重枚举和恢复 |
| 串口首启 | U-Boot 配置为 1500000 baud，Linux DTB 禁用常规 UART2 节点并启用 FIQ debugger 的 serial-id 2、1500000 baud。镜像启用了 `serial-getty@ttyFIQ0.service`，首登标记和本地 `armbian-firstlogin` 路径齐全。实际 `boot.scr` 却传入 `console=ttyS2,1500000`，与登录服务的 `ttyFIQ0` 名称有差异。默认 Netplan 为 DHCP，`networkd-wait-online` 配有 `--any --timeout=20`。串口登录有静态依据，内核日志连续性与无 DHCP 首启仍是验收重点。[Radxa UART 说明](https://docs.radxa.com/en/zero/zero3/low-level-dev/serial) · [Armbian 首登说明](https://docs.armbian.com/getting-started/first-boot-and-login/) | 先验证 Mac/CH343 的 3.3 V TTL、1500000 8N1 回环。逐台保留 U-Boot、内核命令行/控制台和 getty 证据。在无 DHCP 的纯有线环境验证首登交互完成、默认凭据处置及两口静态地址配置；若内核日志缺失，先确认实际控制台节点 |
| microSD 与备卡 | 512-byte sector 的主、备 GPT 头及表 CRC 均通过且分区表一致，唯一 ext4 根分区从 LBA 32768 开始。LBA 64 起的 9631232-byte 引导段与镜像内 `u-boot-rockchip.bin` 逐字节匹配，FIT 位于 LBA `0x4000`。microSD 节点 `mmc@fe2b0000` 可用，根分区 UUID 与 `armbianEnv.txt`、`fstab` 一致，镜像包含首启根分区扩容服务。备卡应使用完整固定镜像，冷关机后单卡替换，避免相同根 UUID 的两份介质同时可见。[Radxa microSD 启动说明](https://docs.radxa.com/en/zero/zero3/faq#q3-what-storage-devices-does-radxa-zero-3-support-for-boot) | 在 HW1-02/05 下按授权制备与回读验证。逐台从正常卡及备卡冷启动，确认实际根分区、扩容、暖重启、单节点断电和串口重新配置。发生故障时隔离并保留原卡。应用身份、持久化和转发恢复各自取证 |

**结论**：保留该固定镜像作为 HW1 候选。零件交付版号和 USB 网卡 VID/PID 尚待绑定，实际启动与 UART 首启待逐台验证。上述证据可纳入 HW-G1，当前仍处于 HW-G0。实体操作按 HW-G3 另行授权。
