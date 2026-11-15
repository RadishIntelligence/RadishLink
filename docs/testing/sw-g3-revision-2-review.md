# SW-G3 实现前修订评审包

- 状态：整体修订仍为 Draft。首轮合成矩阵的 variant、观察窗及样本数已有有限裁决，见[所有者裁决](#首轮合成矩阵所有者裁决2026-11-15)。
- 日期：2026-09-26
- 目标读者：消息实现、测试与安全评审者
- 基线：[SW-G3 1.0 与勘误](sw-g3-deterministic-validation-design.md)
- 非目标：修改历史证据、SW-V0 输入、生产协议、密码方案或扩大运行范围

## 推荐决定与版本边界

本包把 2026-09-05 的五项勘误落实为候选规则。原设计和四个 SW-V0 profile 仍为既有证据的解释依据。整体修订和正式场景各依其评审与授权推进。

| 对象 | 建议修订 | 兼容与消费者 |
| --- | --- | --- |
| 已接受 SW-G3 1.0、SW-V0 | 保持原文、schema 1、profile version 1 和历史 checksum | 现有 harness 继续只解释其已接受输入 |
| 后续 SW-V1/V2 的 retry | 明确初次发送、四次定时重试和一次恢复补发 | 采用该规则的既有 profile ID 提升到 version 2；seed 保持原值 |
| 后续 profile schema | 候选主版本 2，显式表达恢复额度、长度类别和子用例 | 旧解析器必须拒绝；不把新增关键语义塞进可忽略字段 |
| 后续 evidence schema | 候选主版本 2，记录语义模式、子用例、发送原因、预算消耗与提交点 | 比较器按 schema/profile ID/version/variant/子用例分组，禁止跨版本归一化为相同结果 |
| 新的边界子用例 | 以父 profile ID/version 加稳定 subcase ID 区分 | 三次重复按同一子用例比较，不以三个不同子用例代替三次重复 |

`tools/t0/internal/harness/profile.go` 仍只接受四个 canonical SW-V0 ID、profile version 1、schema 1，并拒绝未知字段。[I4 接入包](sw-g4-synthetic-i4-plan.md)已获有限接受并完成 schema 2 decoder、validator、场景编排、clock、evidence writer/checker、比较器及离线测试；入口独立分派版本，不放宽 V0 校验。其 offline-i3 variant 只消费 I3 小容量路径的五 profile 七子用例，不接受本包完整矩阵或正式运行；其他 version 2 场景尚未实现。后续 [I5 三进程闭环草案](sw-g4-synthetic-i5-plan.md)单独提出 profile/evidence schema 3 与 TCP variant，以实际跨进程证据覆盖同一有限矩阵；I4/V0 合同保持不变；I5-I 已获有限接受并完成实现与离线验收，I5-R 网络运行未执行。

## 重试与 5 秒断链

### 预算和事件规则

以下是合成消息路径的测试策略，不是生产默认值：

- 数据预算按本地 message key 与覆盖层下一邻接计数；delivery evidence 的预算另按同 key、receipt 类型和返回邻接计数。重启和 duplicate 不重置任一预算或起算点。
- 初次发送为一次，另有四次定时重试，退避依次为 `250/500/1000/2000 ms`。定时点从该队列首次发送事件累计，分别为 `0/250/750/1750/3750 ms`；`max_attempts` 明确包含初次发送，普通上限为 5。
- 断链恢复另有一次额度，总上限为 6；只响应同一邻接已观察到的 down→up 事件，初始 up、重复 up 通知、无待发对象均不触发。反复抖动不补充额度。
- 观察到 down 时的计划尝试也消耗定时额度，记录 `blocked_link_down`，不发送 frame；不得悄悄积累为恢复后的突发重传。
- 恢复补发与定时点相同时合并为一次尝试，同时消费到期的定时槽及恢复额度，禁止两次发包。恢复事件不得重启退避序列。
- 每次尝试须重新检查当前状态、配额、hop 和 deadline；时间达到或超过 deadline 时拒绝发送。验证 delivery 后取消剩余尝试；预算耗尽只表示停止本轮自动发送，不代表送达或允许删除 payload。
- `retry.deadline_ms = 30000` 从 origin 首次创建起算，接收节点按 SW-G1 转换为不晚于原截止点的本地 deadline；不从首次转发或恢复时重新起算。version 2 建议用 `timed_max_attempts = 5`（含初次）、`recovery_max_attempts = 1` 取代含义不清的单个 `max_attempts`；相应定时起点及剩余额度须持久化。
- A 验证 B custody 后暂停该消息的 A→B 自动重试并继续保留 payload；由 B 的 B→C 队列等待 destination evidence。C 对未到期 duplicate 重发已有 evidence。custody 丢失时 A 的重传由 B 幂等处理。
- receipt/控制队列沿用独立有界配额，不获得恢复额度绕过限速；本轮不把多条并发、组合丢包或多次断链混入单故障 profile。

### 送达证据的返回责任

2026-09-26 自审发现：A 观察 custody 后暂停数据重试，若 B→A 的 delivery evidence 丢失，仅靠 C 对 duplicate 重发不足以闭环。推荐如下候选补充，不引入 ACK-of-ACK 协议：

- C 的 T-C 同时保留可重建证据状态和 C→B 待发送责任；B 的 T-D 在释放数据副本前，同时提交终态、tombstone 与 B→A 的 evidence 待发送责任。B 不得为了节省空间只留下无法重建 A 所需证据的摘要。
- 每个 evidence 返回邻接采用初次加四次定时发送、至多一次 down→up 恢复发送，与数据队列独立计数，受原消息的保守 deadline 和控制配额限制。B 收到同等语义 proof 不新建队列、不重置起算点；C 收到 duplicate 只能利用原有额度，不能无限重发。
- evidence 没有下游确认协议，单次传输成功不证明对方已验证/提交，不能据此取消剩余定时发送；因此正常场景也可能收到有限重复 evidence。A/B 对其幂等处理，不产生用户重复消息，不把“零冗余 evidence”设为通过条件。
- 配额预留必须覆盖终态、tombstone 和待发证据，T-C/T-D 任一预留/提交失败都不能先回成功或删除唯一恢复材料。到期后不再新生成或外发证据；发送队列可终结，但去重/历史按各自保留条件处理。
- 该策略仅覆盖单故障合成候选：全部返回尝试丢失时 A 可以到期且不知道 C 已提交，不能伪造成功。真实 receipt/proof 的来源、结构和可持久化能力仍由 SW-G2 证明。

新增 `SW-V1-LOSS-EVIDENCE-BA-001` 候选 profile（version 1，seed `0x524c53574733020e`）：唯一故障为丢弃 B→A 第一个 delivery evidence，断言 B 数据副本清理后仍保有返回责任、A 最终验证送达、C 用户交付一次。原 `LOSS-EVIDENCE` 仍只丢 C→B；不得在同一 run 同时丢两段。

### 调度延迟与同刻事件

重试起点定义为队列首次计划尝试的时刻，即使当时断链也必须固定，不等待第一次真正外发才起算。每次调度接收一个相同逻辑时刻的事件批次：先处理已验证终态/暂停，再检查 deadline，最后处理可达性、准入及到期槽；同刻相互矛盾的 link up/down 输入拒绝，不依赖调用顺序决定结果。

调度器晚到时，一次结算全部已到期定时槽；更早槽记为 `missed_schedule` 并消耗，最多尝试最新一个到期槽。若同批次还有恢复触发，合并为一次尝试并消费恢复额度。不得把多个过期定时器补成突发发送；同刻同内容批次重放只做幂等观察，同刻不同内容拒绝，不能先调度发送后再补入同刻终态。已有终态、暂停或到期时只记录停止原因，不补发。

设计初轮尚无预算持久化；后续 [I3](sw-g4-synthetic-i3-plan.md)已完成监督端仍存活条件下的合成持久预算与进程重开验证，真实机器重启后的时钟重建仍未实现。调度算法只消费调用方已证明的保守时间；不得把旧进程 monotonic 数直接传入新进程后宣称恢复正确。

### `SW-V1-DOWN-BC-001` version 2 事件表

run 观察窗和 origin lifetime 均为 `30000 ms`。harness 在 B 首次接纳 A 数据的批次开始前关闭 B→C 发送 gate，并将 down 事件交给该批。该批须提交 T-B custody 且首次转发槽受阻，才确认屏障命中。下表以该批实际逻辑时刻 `t_d` 为相对零点。屏障须在 origin 后 `1000 ms` 内达成。环境有效、合格输入已入批而实现未按时提交 custody 为 FAIL；已有合格触发而注入设施失效为 INVALID。观察窗仍从 origin 起算。

| 相对 t_d 时间 | 事件 | 期望 |
| --- | --- | --- |
| 0 ms | bc down；B 初次发送槽 | 消耗槽 1，无外发；B 保留已提交责任，C 零交付 |
| 250 / 750 / 1750 / 3750 ms | 四个定时槽 | 总计 5 次尝试，均记录断链阻塞；无 custody/delivery 假成功 |
| 5000 ms | bc 恢复，消费一次恢复额度 | 发送同 key、同 payload、同 deadline 的原邻接副本；hop 不再次扣减 |
| 原观察窗结束前 | C 提交并返回 evidence，A/B 各自验证 | C 用户交付恰好 1 次；A/B 终态与 tombstone 提交后释放传输副本 |

`SW-V1-DOWN-AB-001` 使用 A 入队后、首次 A→B 发送前的相同屏障；断链期间 B 不得返回 custody。两者均只断链，不重启任何节点。

### 必须覆盖的重试边界

每一行是独立子用例；未列出的其他故障关闭。用注入单调时钟与确定事件驱动检查，不通过 sleep 或延长 run 掩盖失败。

| 子用例 | 断言 |
| --- | --- |
| 首次发送成功 | 1 次数据尝试，验证 evidence 后不再重试 |
| 恢复发生于 deadline 前 1 ms | 只在其他准入条件也通过时允许一次尝试；不承诺在剩余 1 ms 内交付 |
| 恢复恰在 deadline / 后 1 ms | 零恢复发送，明确到期，无新 delivery evidence |
| 重复 up / 第二次 down→up | 不增加恢复额度；总尝试不超过 6 |
| 恢复与定时点重合 | 同一副本只发送一次，两个触发原因均入证据 |
| evidence 与发送同一时刻 | 先处理已验证终态，再做发送准入；不得在已送达后发包 |
| 调度从 0 跳到 5000 ms | 结算已过定时槽，最多一次尝试；无补发突发，重复同刻调用不再外发 |
| B→A 首个 evidence 丢失 | B 独立返回队列补发；不得依赖已经暂停的 A 数据重试 |
| 恢复时配额不足或 hop 不允许 | 显式拒绝，该恢复机会不积攒；payload 按原责任和 lifetime 处理 |
| 重启恢复 | 恢复已消费计数、保守 deadline；不能证明剩余寿命则隔离/到期，不续期 |

## 长度与拒绝路径

`SW-V2-LENGTH-001` version 1 的 16385 B 保留为历史设计输入。version 2 明确区分以下两条轨，不用合成载荷推导密码开销：

| 边界 | 合成 SW-V1/V2 候选 | 真实 E2EE 前置 |
| --- | --- | --- |
| 正文 | 发送动作编码后最大 16384 B；独立检查 16384 / 16385 B | 接收端解密后也检查正文上限 |
| 不透明测试载荷 | `synthetic_payload_max_bytes = 16384`；16385 B 拒绝 | `ciphertext_max_bytes` 必须由选定库、suite、epoch/认证开销证明，当前待定 |
| 完整测试 frame | 建议 `frame_max_bytes = 32768`，包含编码后测试信封与 payload，不含外层 transport 的 4-byte 长度前缀；只作合成资源边界 | 正式信封编码与上限另行评审，不继承此数值 |

上述建议数值须在评审后写入新 profile；当前不创建可执行 JSON。对每层分别检查刚好上限、上限加一、负数、长度整数溢出、声明与实际长度不符、截断和未知关键字段。frame 上限必须在完整缓存/解析前约束；内层长度在相应分配、认证或落盘前检查。错误不得产生 custody、推进安全状态或生成 delivery evidence。

版本 2 使用 `payload_kind = synthetic_opaque`、`security_mode = synthetic` 和各自显式长度字段；测试报告不得把这些载荷标成已认证密文。SW-V3 的长度 profile 需重新评审，不允许把 synthetic 改名后沿用。

## 故障、拓扑与覆盖

- `DOWN-BC` version 2 删除隐含的 B 重启断言；`SW-V2-RESTART-B-001` 独立验证提交后的重启恢复。断链加重启属于未来组合 profile。
- A—B—C 且 B 唯一时，只验证排队与原路径恢复，不声明替代路径恢复。D 中继或 A/C 直连必须另设拓扑、自检与授权，不改变 V0 的 A/C 隔离结果。
- `SW-V1-REORDER-001` 的窗口交换需要至少两个不同逻辑消息；version 2 明确 `message_count = 2`、两个不同 key，断言各交付一次。单条消息不能证明相邻对象乱序命中。安全 epoch 的窗口外拒绝另属 SW-G2/SW-V3，不用传输交换证明。
- 固定 seed 三次一致只证明当前 profile 的可重复性；并发、多 seed、组合故障和长期运行仍在后续独立范围。

## 接受条件与下一步

评审者须逐项确认：初次发送和恢复额度；custody 后责任/调度；deadline 同时事件排序；长度字段与数值；新 schema 与 profile 映射；单故障和乱序输入；消费者及兼容拒绝测试。已裁决范围、日期、责任人及剩余阻塞见[所有者裁决](#首轮合成矩阵所有者裁决2026-11-15)。

随后才能根据[文字闭环设计](../architecture/minimal-text-slice.md)编制精确 SW-G4 实施包，固定文件、schema 全字段、错误分类、命令、资源上限、运行次数和清理。单元测试、正式 Docker run 与 SW-V3 分别受其前置约束；本轮不执行任何一项。

## 工程自审记录（2026-09-26）

审阅者为本草案编写 Agent，方法是对照 SW-G1、现有 harness/profile/clock/proxy 与探索性 t0node 的实际职责做静态推演；不是独立复核、密码审计或实验结果。本轮未执行消息场景。

| 发现 | 处置 / 剩余边界 |
| --- | --- |
| custody 后 A 暂停，B→A evidence 丢失无人负责补发 | 上文补充独立返回责任、原子提交要求和单故障 profile 候选；仍待接受及真实 proof 适配 |
| 首次实际发包与首次定时槽可能混用，迟到调度可突发 | 固定计划起点、同刻批处理和 missed_schedule 规则；列入离线算法测试 |
| 32768 B 新上限与 V0 proxy 的 65536 B 上限不同 | 新值只属于后续消息边界；不得修改 V0 常量或声称旧 reader 已在新界限分配前拒绝 |
| 时钟/内存状态可恢复不等于掉电事务成立 | 首个实施单元只验证算法及值对象，持久化格式、时钟重建与掉电证据继续阻塞 |

结论：推荐先实施[离线重试与长度边界单元 I1](sw-g4-synthetic-i1-plan.md)，本轮已固定其可审阅范围；整体 SW-G3 修订和文字闭环仍为 Draft。I1 不需要先启用新 JSON schema，也不关闭正式 V1/V2 的实施/运行门。

## P0 首轮合成场景规则核对（2026-11-15）

评审对象为 SW-G3 正式合成场景的规则和参数。已有证据取自 [I1–I5-I 记录](node-security-integration-acceptance.md#已有软件证据的适用范围)，真实三节点取证按 [HW1/P0 证据对照](node-security-integration-acceptance.md#实体台架需要取得的证据)执行。本节仍为评审输入。

| 评审项 | 建议接受的合成规则 | 当前覆盖与待验 |
| --- | --- | --- |
| 重试和期限 | 以首次计划时间为起点，初次加四个定时重试，槽位为 0、250、750、1750、3750 ms。同一邻接恢复最多补发一次，与同刻定时槽合并。到达原 deadline 后停止发送。 | I1 覆盖预算、迟到和同刻事件，I3 覆盖已消耗预算的离线恢复。正式用例仍需逐次记录槽位、计费和原 deadline。 |
| 两段送达证据 | 分别验证 C→B 与 B→A 首个送达证据丢失。C 保留返回责任，B 在清理载荷时一并持久化 B→A 返回责任。A 已暂停数据重试时仍须经既有返回责任收到送达证据，C 仅交付一次。 | 责任边界已有 [D10](../adr/0005-node-security-integration.md#d10证明不超过原消息截止时间) 和 I3–I5-I 合成证据。正式场景仍需分方向留故障命中及返回证据。 |
| 断链、重启与拓扑 | DOWN-AB 与 DOWN-BC 各只注入一次 5000 ms 单向发送不可用。BC 在 B 首次接纳批次前关发送 gate，以该批 T-B 确已提交且首次转发受阻作为命中证据。节点重启放到独立的 V2 用例。B 是唯一路径，复连只验证原路径恢复。 | I4/I5-I 有该边界的离线覆盖。正式场景待核对故障命中、恢复后同 key 及期限。实体网线断开、USB 重枚举和物理掉电各按 HW1 留证。 |
| 长度 | 合成正文和不透明载荷的上限各为 16384 B，测试 frame body 上限为 32768 B，外层 4 B 前缀单独计入传输。真实密文上限待固定候选给出编码及认证开销。 | I1/I2 覆盖边界、溢出、截断和分配前拒绝。Rust Node 当前的 65536 B 是整个接收 frame 的上限，包含 4 B 前缀。正式用例须绑定具体入口及计量位置。[Node framing](../../node/src/framing.rs) |
| RATE 的单位与 burst | 保持 65536 B/s 候选速率及零初始 token，按编码后完整 frame 计费。采用 I3 编码和逐帧准入时，建议仅在新合成 profile 使用 32772 B 数据 burst，即允许一个最大 frame。原 16384 B burst 连最大载荷的 21848 B base64 数据都容纳不下。 | [I3/I4 小容量模式](sw-g4-synthetic-i3-plan.md#发送速率与有限调度)已使用新 burst。BASE 的三种尺寸只覆盖正常路径；正式 RATE 仍须用 1 B、1 KiB、16 KiB 各验证限速命中、实际编码计费及单次交付。 |
| REORDER 输入 | 用两个不同 key 的消息，记录 B→C 两个对象在相邻窗口内实际互换，并按 key 核对各提交和交付一次。 | I4/I5-I 七子用例中没有该场景，仍需单独绑定输入、故障点和原始证据。 |

评审时逐项记录接受的规则、测试 variant、profile 版本和评审责任。首轮可先确认 BASE、两段丢证及两段断链的规则和观察窗。RATE、REORDER 与 V2 负例保留原义务，分别完成设计和试验。正式运行继续使用各自的精确授权和记录。

上述规则核对在 macOS arm64 用 Go 1.26.3 复跑四个现有离线测试包，均通过。首次运行退出 1，沙盒拒绝读取默认 Go build cache。复验使用仓库内忽略的专用缓存，源码、profile 与测试断言保持原样。以下历史命令在 `tools/t0/` 执行，运行范围限于包内离线测试。

```sh
GOCACHE="$(git rev-parse --show-toplevel)/.cache/sw-g3-review/go-build" \
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off \
  go test -count=1 -timeout=120s ./internal/delivery ./internal/harness ./internal/synthetic ./cmd/sw-v0-harness
```

### 首轮五用例设计评审（2026-11-15）

首轮正式合成矩阵采用现有 I5 `i3-small4-tcp-v1`（`docker-tcp-i3`，`security_mode=synthetic`），profile schema 与 evidence schema 均为 3。I4 `i3-small4-offline-v1`（`offline-i3`）的两项 schema 均为 2，作为同用例的离线对照。下表的 v1/v2/v3 是各 ID 自身的 `profile_version`。评审基线为 `2e2f888`。 [I4 profile](../../tools/t0/profiles/i4) · [I5 profile](../../tools/t0/profiles/i5)

| 用例 | Profile ID | I4 / I5 profile version | 固定 seed | 子用例（payload） |
| --- | --- | --- | --- | --- |
| 正常送达 | `SW-V1-BASE-001` | v2 / v3 | `0x524c535747330201` | `p1`（1 B）、`p1024`（1 KiB）、`p16384`（16 KiB） |
| C→B 首份送达证据丢失 | `SW-V1-LOSS-EVIDENCE-001` | v2 / v3 | `0x524c535747330205` | `p1024` |
| B→A 首份送达证据丢失 | `SW-V1-LOSS-EVIDENCE-BA-001` | v1 / v2 | `0x524c53574733020e` | `p1024` |
| AB 单向断链 | `SW-V1-DOWN-AB-001` | v2 / v3 | `0x524c53574733020c` | `p1024` |
| BC 单向断链 | `SW-V1-DOWN-BC-001` | v2 / v3 | `0x524c53574733020b` | `p1024` |

**共同观察窗。** 每个独立样本在自检通过后于逻辑 `0 ms` 创建单条消息（初始 hop 预算 2）。送达及关联提交须在原 deadline `30000 ms` 前完成，继续观察至 `30000 ms` 的到期结算及三节点快照。两条断链另在 `[t_d, t_d+5000 ms)` 检查指定方向并在右端点恢复，断链、补发或提前送达均不改观察窗。I5 固定每方向 `50 ms` 是逻辑入批延迟，实际 socket 耗时和宿主操作截止分别记录。 [I5 监督时钟与批次屏障](sw-g4-synthetic-i5-plan.md#监督时钟与批次屏障)

**共同判定。** `PASS` 要求自检、计划命中、下列业务断言、证据校验和与清理均通过。以同一 key、核心摘要、保守 deadline 关联 T-A、T-B、T-C 及 A/B 的 T-D，C 只交付一次。A 仅在验证 B 返回的送达证据后提交送达并终止传输责任，B 仅在验证 C 的证据后原子提交终态、tombstone 和 B→A 返回责任并清理传输正文。A/C 历史按各自规则保留。`FAIL` 包括环境有效时实现未产生应有的触发对象、超窗、假送达、重复交付、提前释放、越额、续期、到期后产生新送达或外发、同邻接重发再次扣 hop。`INVALID` 包括自检、时钟、资源前提、注入、证据完整性或清理失效。两类问题同时出现时保留原始实现失败和设施错误。 [SW-G3 判定](sw-g3-deterministic-validation-design.md#pass--fail--invalid) · [I5 证据合同](sw-g4-synthetic-i5-plan.md#evidence-schema-3-与兼容)

#### 正常送达

- **命中与观察。** 三种尺寸独立运行，共同 `0–30000 ms` 窗内全程关闭故障注入，命中数为 0。
- **PASS / FAIL / INVALID。** `PASS` 须证明 B 在 T-B 后方可回 custody 或转发，C 在 T-C 后方可返回证据，两端各自据此提交 T-D。零初始 token 下，首个定时槽可能因额度不足被消费，按已提交的槽位和实际帧计费判定，允许预算内的重复控制帧。环境有效时任一尺寸缺少提交或违反共同断言为 `FAIL`。注入器意外启用、出现非计划命中，或实际源/目标记录无法对账为 `INVALID`。
- **已知与待补。** I3 有 1 KiB、16 KiB 的持久路径，I4 三个尺寸各有三次离线比较，I5-I 有三个尺寸的字节流 fixture，V3 写入、校验和三次比较覆盖 `p1024` fixture。正式证据须对每种尺寸绑定实际 TCP 完整写出、ingress、入批、提交和最终快照，并核对编码后全帧字节计费。 [I3 实施记录](sw-g4-synthetic-i3-plan.md#2026-09-26-实施与离线验证记录) · [I5 fixture](../../tools/t0/cmd/sw-v0-harness/process_scenario_test.go)

#### C→B 首份送达证据丢失

- **命中与观察。** 在共同 `0–30000 ms` 窗内，B 对 C→B 第一份完整、匹配固定消息的 delivery frame 在 TCP ingress 后、入批前丢弃一次，以 transfer ID、方向、类型、key、摘要和首帧序号关联完整写出及 drop。B→A 不注入丢证。
- **PASS / FAIL / INVALID。** `PASS` 须见首帧未入批，B 在获验后续证据前维持 custody 和正文，后续证据来自 C 在 T-C 已提交的原返回队列，预算和起点连续。B 验证后完成 T-D，A 最终验证来自 B 的证据，C 历史始终一条。环境有效时缺首帧、缺后续补发、B 提前释放或 A 未能按窗确认均为 `FAIL`。合格首帧已到注入层却漏丢、错丢、多丢或被放入 B 批次为 `INVALID`。重复 data 可以出现，补发因果按原返回队列核对，无法仅从 duplicate 推定。
- **已知与待补。** I3 验证单侧丢证后的持久闭环，I4 有该方向的三个离线比较样本，I5-I 有该方向字节流 fixture。正式须保存首帧 write→ingress→drop 与后续 write→ingress→B 入批的两条关联链，并证明 B 在两者之间未清正文或推进送达。 [I3 提交合同](sw-g4-synthetic-i3-plan.md#一条消息的提交与副作用) · [I4 实施记录](sw-g4-synthetic-i4-plan.md#2026-09-26-实施与验证记录)

#### B→A 首份送达证据丢失

- **命中与观察。** 在共同 `0–30000 ms` 窗内，A 对 B→A 第一份完整、匹配固定消息的 delivery frame 在 TCP ingress 后、入批前丢弃一次，关联字段与上一用例相同。B→A custody 与 C→B delivery 不计入或施加该故障。
- **PASS / FAIL / INVALID。** `PASS` 须见 A 已验证 custody 并暂停 A→B 数据重试，仍保留传输责任。B 在 T-D 删除正文时一并提交送达终态、tombstone 和 B→A 返回责任，丢帧后从同一返回队列按原额度补发。A 只因后续证据验证成功提交送达，C 交付恰好一次。环境有效时缺首帧、清正文后丢失返回责任、以 A 恢复数据重试才换来成功，或 A 未在窗内提交送达均为 `FAIL`。合格首帧进入注入层但漏丢、错丢、多丢或进入 A 批次为 `INVALID`。
- **已知与待补。** I3 有 B 清正文后保留返回责任的离线证据，I4 有该方向的三个离线比较样本，I5-I 有该方向字节流 fixture。正式须用原始 trace 关联 A 的 custody 暂停、B 清正文及返回责任所在的同一提交、首次丢弃和后续补发，留存到期前快照以证明返回责任。 [I3 提交合同](sw-g4-synthetic-i3-plan.md#一条消息的提交与副作用) · [I5-I 实施记录](sw-g4-synthetic-i5-plan.md#i5-i-实施与离线验证结果2026-09-26)

#### AB 单向断链 5 秒

- **命中与观察。** A 在 `0 ms` 提交 T-A 后、首次 `Step(1)` 的发送决定前关闭 A→B gate，并将 `became_down` 交给该批。以 `t_d=1 ms` 到 `5001 ms` 的逻辑区间作断链检查，恢复批次同步打开 gate 并交付 `became_up`。B→A 保持可用。全程观察至共同的 `30000 ms`。
- **PASS / FAIL / INVALID。** `PASS` 须见 A 持有原消息，五个定时槽均以 `blocked_link_down` 消费额度，断链期间 A→B 无写出、B 无 T-B/custody、A 无 custody 或 delivery 确认、C 零交付。槽位按 A 队列原计划起点计算，初槽在 `1 ms` 结算。恢复后只消费一次恢复额度，以同 key、核心摘要、原期限和 A→B 副本 hop 发送，再完成共同交付链。环境有效时 T-A 缺失、gate 关闭时取得发送机会、额度重置或恢复超窗为 `FAIL`，即使 gate 拦住误发。合格 T-A 已有而 gate 漏关、错向、晚关或恢复时刻偏离约定为 `INVALID`。
- **已知与待补。** I1 有重试槽和恢复额度边界，I3 有 AB 断链持久路径，I4 有三个离线比较样本，I5-I 有单向 gate 字节流 fixture。正式须将 T-A、gate 切换、五个阻塞槽与恢复写出绑定到原始控制响应，并用 TCP/ingress 记录、B/C 快照核对零提前副作用。 [I1 离线规则](sw-g4-synthetic-i1-plan.md) · [I5 故障合同](sw-g4-synthetic-i5-plan.md#五-profile七子用例与故障定义)

#### BC 单向断链 5 秒

- **命中与观察。** B 首次接收合格 A→B data 的批次前关闭 B→C gate，向该批注入 `became_down`。以同一批已提交 T-B 且首次 B→C 转发槽被阻塞确认命中，只有关 gate 记录不足以计命中。`t_d` 取该批逻辑时刻，须在 origin 后 `1000 ms` 内；`t_d+5000 ms` 同批恢复 gate 并交付 `became_up`。C→B 和 B→A 保持可用，全程观察至共同的 `30000 ms`。
- **PASS / FAIL / INVALID。** `PASS` 须见 B 接纳并向 A 返回 custody、保留正文和转发责任，B→C 五个定时槽均以 `blocked_link_down` 消费额度，断链期间无 B→C 写出、C 零交付、A/B 无假送达。恢复后 B 消费唯一恢复额度，按原 key、核心摘要、期限及 B→C 副本 hop 转发，完成共同交付链。环境有效时实现未产生 A→B 输入、合格输入已入批而 T-B 未在 `1000 ms` 内提交、关闭期间取得 B→C 发送机会，或恢复后未按窗交付均为 `FAIL`。合格触发已具备而 gate 的位置、方向、时长或时钟有误为 `INVALID`。
- **已知与待补。** I3 有 BC 断链持久路径，I4 有三个离线比较样本，I5-I 有对应字节流 fixture。正式须关联该批的 A→B 入批、gate、T-B 提交代际及首次阻塞，核对 B→A custody 可达、恢复前 C 零交付、恢复后沿原邻接写出。I5 监督器从 `link_gate` 生成 fault 事件，V3 校验器核对事件来源和故障时长。正式前须落实对该事件与同批 T-B、首槽阻塞的独立核对。 [I5 事实转换](../../tools/t0/cmd/sw-v0-harness/process_scenario.go) · [V3 证据校验](../../tools/t0/internal/harness/evidence_v3.go)

#### 正式运行前置

1. **证据比较。** 同一子用例的三次证据逐份校验后比较归一化事实，绑定 schema、profile ID/version/hash、seed、variant、子用例、revision、二进制及镜像。任一 `FAIL`/`INVALID` 停止后续样本，保留原因和未运行清单。 [I5 矩阵及停止规则](sw-g4-synthetic-i5-plan.md#五-profile七子用例与故障定义)
2. **证据与判定。** 正式前核对六个实际拓扑探针（四方向邻接可达、A/C 双向直连不可达）、三进程及独占 store、监督时钟、write/ingress/drop 与提交的原始来源、版本绑定、校验和及清理。BC 同批命中与关 gate 误发判定的离线证据见[离线验证](#断链证据离线验证2026-11-15)。实际 TCP 与进程回收仍待正式环境取证。
3. **执行前置。** I5-R 仍在 `I5_RESOURCE_ISOLATION_REQUIRED` 停止。待取得合格宿主、完整 768 MiB 整批硬写入边界、受限构建与 daemon/store 隔离的运行接入及实际拒绝证据，再核准干净 revision、合同及资源附录、profile 与实际产物的绑定和版本兼容方案。精确执行包另行确认命令、资源与时限、失败保留和回收，然后取得当次运行授权。 [I5 资源放行条件](sw-g4-synthetic-i5-resource-isolation.md#下一实施包与放行条件) · [宿主恢复条件](sw-g4-synthetic-i5-linux-host-assessment.md#收口与恢复条件2026-11-10)

#### 首轮合成矩阵所有者裁决（2026-11-15）

项目所有者萝卜SAMA 确认沿用 `i3-small4-tcp-v1`（`docker-tcp-i3`）及上表现有五个 I5 profile。从 origin 的逻辑 `0 ms` 观察至 `30000 ms`。BASE 三种尺寸先各做三次独立样本，通过后按表顺序做四个故障用例各三次，共七个子用例、21 个独立样本。

此裁决仅接受首轮 variant、观察窗及样本规模。BC 同批命中核对及关闭 gate 时误发判 `FAIL` 进入离线实施方案评审。代码与 profile 的实施、真实网络、签验及 I5-R 各待后续授权。其余 SW-G3 修订、HW1 交付证据、SW-G2 许可证评审及上游 Issue 按各自状态推进。 [当前状态](../status/current.md)

#### 断链证据离线验证（2026-11-15）

所有者随后授权实施并用 `memoryRunner` 验证两项判定。V3 新产物采用 manifest `observation_version=2`。校验器沿 A→B 已接收的 transfer ID 绑定 B 首次接纳的控制交换，重算同批 gate、T-B 提交代际、custody 快照与首槽阻塞。I5 profile、证据主 schema 3、控制协议和快照 DTO 版本均沿用现有合同。历史 `observation_version=1` 仍按原规则读取，两种 observation version 分开比较。 [V3 证据校验](../../tools/t0/internal/harness/evidence_v3.go) · [同批核对](../../tools/t0/internal/harness/network_gate_assessment.go)

有效输入已进入 BC 预定批次但缺少 T-B，或关 gate 时存在已提交的发送决定，监督器在实际逻辑时刻终止业务。节点先核对整批决定再发布发送句柄。监督器记录控制关闭及回收结果，并按原始 ingress 对账本次放弃的待收帧。证据完整且环境、清理有效时输出可复核的 `FAIL`（退出码 `1`）。清理或控制回收失效时输出 `INVALID`（退出码 `2`）并保留已证实的实现违规。错时长单独判为注入失效，重算摘要后的错批绑定仍拒绝。 [归档入口](../../tools/t0/cmd/sw-v0-harness/process_runtime.go) · [离线用例](../../tools/t0/cmd/sw-v0-harness/process_gate_test.go)

本轮验证使用真实合成 Node 和临时 store，网络和进程接口均由 `memoryRunner` 提供。I5-R 的宿主资源、真实 TCP、实际进程退出与清理仍待其运行门验证。
