# RFC-0023 可观测性：导出与运行信息

> 状态：已采纳（2026-11-02 修订：运行进程的用户名照常打印；2026-11-24 修订：一致性向量；同日再修订：新增「内核的遥测」，操作调用的 Span 与指标带操作 ID；同日第三次修订：平台读取路径逐条写明取法，Linux 的取法收进向量；2026-11-27 修订：服务名不含孤立的代理项；同日写明：XGI 入口写不出响应时 Span 与指标记失败） | 日期：2026-10-30 | 对应 ADR：ADR-0056、ADR-0093（用户名不遮蔽）、ADR-0269（内核的遥测）、ADR-0277（一致性向量）、ADR-0286（平台读取路径的取法）、ADR-0334（服务名不含孤立的代理项）、ADR-0341（写不出响应计入失败） | 参考实现：`XiHan.Framework.Observability.Abstractions`、`XiHan.Framework.Observability.Host`、`XiHan.Framework.Observability.OpenTelemetry`、`XiHan.Framework.Kernel`（`FrameworkDiagnostics`、`ObserveMiddleware`） | 一致性：无行为套件（读的是所在机器，按平台写用例）；向量 `conformance/vectors/observability/host-usage.json` | 一致性档：`observability`

## 摘要

可观测性领域有两件事：导出，把内核与运行时的 Activity、Meter 与日志接到后端（`observability.export@1`）；运行信息，读进程与所在主机的 CPU、内存、磁盘、网卡、GPU、主板与运行时长，供管理界面的服务监控使用（`observability.host@1`）。运行信息只读 BCL、操作系统文件、注册表与系统调用，不启动外部命令。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `observability.export@1` | 导出器的装配 | `Observability.OpenTelemetry`（`role: default`） |
| `observability.host@1` | `IHostMonitor`：进程与主机的运行信息 | `Observability.Host`（`role: default`） |

能力 ID 常量 `ObservabilityCapabilities` 在 `Observability.Abstractions`。

## 导出

内核只依赖 `System.Diagnostics`（`FrameworkDiagnostics.ActivitySource` / `Meter`）与 `Microsoft.Extensions.Logging`，导出由 Provider 装配：

- 追踪、指标、日志三类信号各有开关，默认订阅框架、ASP.NET Core、`System.Net.Http` 与 `System.Runtime` 的源与仪表，配置可追加。
- 配置了端点（或标准环境变量）才启用 OTLP 导出器；没有配置就不导出，不向本机空端口重试。协议只与端点一起配置：`Otlp:Protocol` 写了而 `Otlp:Endpoint` 没写时装配失败（参数异常），不静默忽略；只靠标准环境变量启用时，端点与协议都按 `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_EXPORTER_OTLP_PROTOCOL` 与默认端口决定。
- 资源的服务名 `ServiceName`（`service.name`，缺省是入口程序集名）不能只有空白，也不能有孤立的代理项（RFC-0001「孤立的代理项」），不合时构造选项或从配置读取时抛出参数异常、装配失败（孤立的代理项 2026-11-27 修订，ADR-0334；以前只拒空白：服务名随每个信号编成 UTF-8 发给后端，宽松编码之后只差在孤立代理项上的两个服务在后端是同一个服务）。
- 采样比例只作用于根 Span，有上游 trace 的沿用上游决定。内核的 `xihan.invoke` 同样遵守（RFC-0003 规则 7）：有上游（`traceparent`、宿主当前的 Activity）时沿用它的采样标志，上游「不采样」就不采样；没有上游时它是根 Span，按 `SamplingRatio` 采样，trace-id 与调用上下文的一致。

## 内核的遥测

内核发出下列信号（2026-11-24 写明，ADR-0269；以前正文只在上一节与 RFC-0003 规则 7 提到 `xihan.invoke`，两个指标的名字、各信号的标签与取值没有写），源与仪表的名字都是 `XiHan.Framework`（.NET：`FrameworkDiagnostics` 的 ActivitySource 与 Meter）：

| 信号 | 名字 | 内容 |
| --- | --- | --- |
| Span | `xihan.invoke` | Observe 阶段为每次经管道的调用记一条，种类 Internal；父级与采样按 RFC-0003 规则 7 |
| Span | `xihan.plugin.invoke` | 宿主调用插件的操作、向插件投递事件时各记一条，种类 Client；进程外与 Wasm 插件相同（`plugin/invocation.md` §6） |
| 计数器 | `xihan.invocations` | 单位 `{invocation}`，每次经管道的调用记一次 |
| 直方图 | `xihan.invocation.duration` | 单位 `ms`，每次经管道的调用的耗时 |

| 标签 | 取值 | 带在哪里 |
| --- | --- | --- |
| `xihan.outcome` | `success`、`failure`，管道没有写结果时为 `unset` | `xihan.invoke` 与两个指标 |
| `xihan.operation` | 操作 ID | 操作调用的 `xihan.invoke`、两个指标与 `xihan.plugin.invoke` |
| `xihan.plugin.id` | 插件的清单 ID | `xihan.plugin.invoke` |

1. `xihan.operation` 只取登记表里操作描述的 ID，取值因此以操作登记表的容量为界。不是操作调用时（消息处理器、作业、直接交给调用器的处理器、没有匹配到操作端点的请求）不带这个标签，不以占位值顶替；主题、作业名、请求路径不进标签。按 ID 找不到操作的调用不进管道（`KERNEL_NOTFOUND_001`），没有 Span 与指标。
2. 操作 ID 在创建 Span 时给出，采样器可据此采样。它在进入 Observe 阶段时读：处理器在同一个上下文上再发起调用不改外层的维度。
3. 调用的结果是失败时 Span 的状态为 Error，说明是错误码。结果以调用方拿到的为准：XGI 入口在管道之后编码响应（RFC-0011 规则 10「编码」），写不出时调用方拿到 `KERNEL_UNEXPECTED_001`，`xihan.invoke` 的状态同样是 Error、说明 `KERNEL_UNEXPECTED_001`，`xihan.outcome` 与两个指标记 `failure`，与幂等记录写不出响应时（RFC-0006 幂等第 2 条）相同。.NET 在 Observe 阶段的内置中间件里、排在它之后的中间件都结束时编码，再定结果、结束 Span，Span 与耗时都包括编码；排在它之后的中间件（审计、熔断等）看到的仍是编码之前的结果（2026-11-27 写明，ADR-0341；以前 .NET 在 Span 结束之后才编码，写不出的调用记成 `success`、状态 Unset）。
4. 没有监听器时不创建 Span，没有人订阅的仪表不记录。

## 运行信息

### 契约

`IHostMonitor` 的七个读取方法都是异步的、收取消令牌，各返回一份不可变记录；可能阻塞的系统调用（挂起的网络盘、首次的硬件探测）不占调用方的线程，调用方取消即不再等待，已开始的系统调用在后台自行结束（2026-11-11 修订，ADR-0161）：

| 方法 | 记录与内容 |
| --- | --- |
| `ReadRuntimeAsync(ct)` | `RuntimeInfo`：操作系统描述、平台、版本与架构，.NET 描述与 RID，进程架构，机器名、用户名、是否交互，进程 ID、名称、启动时刻与运行时长，系统启动时刻与运行时长，工作集、私有字节、托管堆，线程数、句柄数，三代 GC 次数，进程 CPU 使用率 |
| `ReadProcessorAsync(ct)` | `ProcessorInfo`：型号、厂商、架构、物理核心数、逻辑核心数、标称频率（MHz）、最后一级缓存字节数、系统 CPU 使用率 |
| `ReadMemoryAsync(ct)` | `MemoryInfo`：总量、可用、空闲、缓存（字节），以及由总量与可用算出的已用与使用率 |
| `ReadDisksAsync(ct)` | `DiskInfo` 列表：挂载点或盘符、设备、卷标、文件系统、类型，总量、空闲与当前用户可用（字节），已用与使用率 |
| `ReadNetworkInterfacesAsync(ct)` | `NetworkInterfaceInfo` 列表：ID、名称、描述、类型、状态、速率（bit/s）、物理地址、带前缀长度的 IPv4 / IPv6 地址、网关、DNS，累计收发字节与包数，收发速率（字节/秒） |
| `ReadGpusAsync(ct)` | `GpuInfo` 列表：名称、厂商、PCI 厂商与设备 ID、驱动、驱动版本、显存字节数、温度（℃） |
| `ReadBoardAsync(ct)` | `BoardInfo`：主板厂商、型号、版本与序列号，整机厂商与型号，BIOS 厂商、版本与日期 |

- 读不到的项为空（`null`），不以 0 或空串顶替；列表读不到为空列表。字符串去掉首尾空白后为空即视为读不到。
- 读取不阻塞：不睡眠、不等待采样，不启动外部进程。
- 读取不抛出：单项失败（权限不足、文件缺失、平台不支持）只让该项为空，不影响其他项。
- CPU 型号与核心、GPU、主板首次读取后缓存；其余每次读取当场取。
- 不暴露命令行参数、环境变量与当前目录。
- `RuntimeInfo.UserName` 是运行进程的系统账户，属于主机诊断数据，不是任何服务的登录凭据：`ToString()` 照常打印它，不随选项打印遮蔽登录用户名（RFC-0014「选项里的凭据」规则 2）而遮蔽（2026-11-02 已定，ADR-0093）。

### 采样

使用率与速率要两次采样求差，由后台采样器负责：

- 间隔 `XiHan:Observability:Host:SampleIntervalSeconds`，默认 5，取值 (0, 3600]。
- 启动时同步采一次，之后按间隔采样；读取返回最近一次算出的值，启动后第一个间隔内为空。
- 系统 CPU 使用率 = 1 − 空闲增量 / 总增量；进程 CPU 使用率 = 进程处理器时间增量 /（经过时间 × 逻辑核心数）；网卡速率 = 累计字节增量 / 经过时间。
- 计数变小（回绕、网卡重建）或总增量为零时该项为空；百分比限制在 0–100，不取整。

### 平台读取路径

| 项 | Linux | Windows | 其他（macOS 等） |
| --- | --- | --- | --- |
| CPU 型号、厂商 | `/proc/cpuinfo` 的 `model name`（没有取 `Hardware`）、`vendor_id` | 注册表 `HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0` 的 `ProcessorNameString`、`VendorIdentifier` | 空 |
| 物理核心 | `/sys/devices/system/cpu/cpu<N>/topology/` 不同的 `physical_package_id` 与 `core_id` 组合数 | `GetLogicalProcessorInformation` 的处理器核心条目数 | 空 |
| 标称频率 | `cpu0/cpufreq/base_frequency`，没有取 `cpuinfo_max_freq`（kHz 换算 MHz） | 注册表 `~MHz` | 空 |
| 最后一级缓存 | `cpu0/cache/index<N>/` 里 `level` 最高的 `size` | `GetLogicalProcessorInformation` 里级别最高的缓存条目 | 空 |
| CPU 使用率 | `/proc/stat` 首行 | `GetSystemTimes` | 空 |
| 内存 | `/proc/meminfo` 的 `MemTotal`、`MemAvailable`、`MemFree`、`Buffers` + `Cached` | `K32GetPerformanceInfo` 的物理总量、可用与系统缓存（空闲为空） | 空 |
| 系统运行时长 | `/proc/uptime` | `Environment.TickCount64` | `Environment.TickCount64` |
| 磁盘 | `/proc/self/mounts` 里以 `/dev/` 开头、不是 `/dev/loop` 的设备，每个设备取第一个挂载点，容量取 `DriveInfo` | `DriveInfo` 里就绪的固定、可移动与网络盘 | 同 Windows |
| 网卡 | `NetworkInterface`，平台不支持的属性为空 | 同左 | 同左 |
| GPU | `/sys/class/drm/card<N>/device`：`uevent` 的驱动与 PCI ID、`mem_info_vram_total`、`hwmon/hwmon<N>/temp1_input`，`/sys/module/<驱动>/version`；名称与厂商名查 `pci.ids`（`/usr/share/hwdata`、`/usr/share/misc`），查不到为空 | 注册表显示适配器类 `{4d36e968-e325-11ce-bfc1-08002be10318}` 下 `MatchingDeviceId` 以 `pci` 开头的子键：`DriverDesc`、`ProviderName`、`DriverVersion`、`HardwareInformation.qwMemorySize`（或 `.MemorySize`） | 空列表 |
| 主板 | `/sys/class/dmi/id/` 的 `board_*`、`sys_vendor`、`product_name`、`bios_*` | 注册表 `HKLM\HARDWARE\DESCRIPTION\System\BIOS` | 空 |
| 运行时与进程 | `RuntimeInformation`、`Environment`、`Process`、`GC` | 同左 | 同左 |

各项的取法逐条写明如下（2026-11-24 写明，ADR-0286；以前只有上面这张表，给了读哪个文件的哪一项，`/proc/stat` 的空闲算不算 `iowait`、总计取哪几列这些只在 .NET 的代码里）。向量 `conformance/vectors/observability/host-usage.json` 的 `linux` 一节按「Linux 的取法」给出输入与读出的值。

#### Linux 的取法

共同的：文件按文本读，读不到（不存在、没有权限）时该项为空。「一行」指文件的第一行去掉首尾空白（RFC-0001「空白字符」）与结尾的空字符之后的内容，为空即读不到；「一个整数」指这样的一行是十进制整数（可以带正负号）。`cpu<N>`、`card<N>` 指名字是 `cpu`、`card` 加至少一位十进制数字的子目录。

1. CPU 使用率（`/proc/stat`）：取第一个以 `cpu` 加一个空格起头的行（各核的 `cpu0` 等行不取），按空格分列，取名字之后的前八列——`user`、`nice`、`system`、`idle`、`iowait`、`irq`、`softirq`、`steal`，不足八列有几列算几列，至少四列。空闲 = `idle` + `iowait`（等磁盘的时间里处理器没有在干活；只有四列时就是 `idle`），总计 = 这几列之和；第九、十列的 `guest`、`guest_nice` 已经计入 `user` 与 `nice`，不再加。没有这一行、不足四列或这几列里有不是十进制数字的，这一次读不出。使用率由先后两次读到的空闲与总计按「采样」的公式算。
2. 内存（`/proc/meminfo`）：每行是「名字: 数值 kB」。名字是第一个冒号之前去掉两侧空白的部分，区分大小写，同名的行取第一行；数值是冒号之后（去掉两侧空白）到第一个空格之前的十进制数字，单位是 kB，乘以 1024 得字节数（单位不核对，内核对这几项固定写 kB）；不是数字的那一项为空。总量取 `MemTotal`，可用取 `MemAvailable`，空闲取 `MemFree`，缓存 = `Buffers` + `Cached`（只读到一项时就是那一项，两项都没有为空；`SwapCached`、`SReclaimable` 不计）。
3. 系统运行时长（`/proc/uptime`）：第一列（到第一个空格或换行为止）是秒数，写成十进制小数——只有数字与至多一个小数点、至少一位数字，按十进制读，不经二进制浮点；含挂起的时间。带符号、指数、分组符或空白的写法读不出，超出实现的时长类型能表示的范围的同样读不出（2026-11-24 修订，ADR-0286；以前 .NET 按浮点读、连带符号与指数的写法一并照收，负数读成负的时长）。
4. 磁盘（`/proc/self/mounts`）：每行按单个空格分列，前三列是设备、挂载点、文件系统，不足三列的行跳过；设备与挂载点里反斜杠加三位八进制数字的转义还原成那个码位的字符（`\040` 是空格，`\011` 是制表符，`\134` 是反斜杠），别的反斜杠原样保留。只取设备以 `/dev/` 开头、不以 `/dev/loop` 开头的行，每个设备取第一行，按文件里的次序。容量（总量、空闲、当前用户可用）按挂载点向文件系统查询，文件系统取第三列；未就绪或总量不为正的不列。
5. CPU 型号与厂商（`/proc/cpuinfo`）：每行是「名字: 值」，名字的取法同上一条内存的，值是第一个冒号之后的一行。型号取第一个 `model name`，没有这一行或它的值为空时取第一个 `Hardware`；厂商取第一个 `vendor_id`。
6. 物理核心：`/sys/devices/system/cpu/cpu<N>/topology/` 下 `physical_package_id` 与 `core_id` 各是一个整数；两个都读得出的那些 `cpu<N>` 里，不同的（封装，核心）组合数就是物理核心数，一个都读不出为空。
7. 标称频率：`/sys/devices/system/cpu/cpu0/cpufreq/base_frequency` 是一个整数（kHz），读不出时取同目录的 `cpuinfo_max_freq`；除以 1000 得 MHz，不取整。
8. 最后一级缓存：`/sys/devices/system/cpu/cpu0/cache/` 下名字以 `index` 开头的子目录各有 `level`（一个整数）、`type` 与 `size`（各一行）。取 `level` 最高、`type` 不是 `Instruction`、`size` 读得出的那一项的大小；同一级有几项时取目录名按字节序排在最前的（2026-11-24 修订，ADR-0286；以前 .NET 取目录枚举次序里的第一项，次序随文件系统而变）。`size` 是十进制数字后跟可选的单位 `K`、`M`、`G`（不区分大小写，1024 的一、二、三次方），没有单位就是字节数。
9. GPU：`/sys/class/drm/` 下每个 `card<N>` 目录一块（`card0-DP-1` 这样的连接器目录不算），按目录名的字节序。`card<N>/device/uevent` 每行是「名字=值」：驱动取 `DRIVER`；`PCI_ID` 正好是冒号分开的两段时是厂商 ID 与设备 ID，否则取 `device/vendor` 与 `device/device` 两个文件各一行。ID 去掉开头的 `0x`（不区分大小写）之后必须正好是四位十六进制数字，给出时写成小写，不合的那一个为空。驱动版本取 `/sys/module/<驱动>/version` 的一行。显存取 `device/mem_info_vram_total`（一个整数，字节）。温度取 `device/hwmon/` 下按目录名字节序第一个读得出 `temp1_input`（一个整数，毫摄氏度）的子目录，除以 1000，不取整。
10. GPU 的名称与厂商名查 `pci.ids`：依次看 `/usr/share/hwdata/pci.ids`、`/usr/share/misc/pci.ids`、`/usr/share/pci.ids`，用第一个存在的。空行与 `#` 起头的行跳过；厂商行顶格，设备行缩进一个制表符，缩进两个制表符的子系统行不看，`C` 加空格起头的行是类别段的开始、查到这里为止。一行（设备行去掉那一个制表符之后）以四位 ID 起头（不区分大小写）、第五个字符是空格、整行多于六个字符时，ID 之后去掉首尾空白的部分是名称。先找厂商，再在它名下（到下一个顶格的行之前）找设备；厂商找得到而设备找不到时只有厂商名，没有厂商 ID、没有文件或厂商找不到时两个都为空。
11. 主板（`/sys/class/dmi/id/`）：各取一行——主板厂商 `board_vendor`、型号 `board_name`、版本 `board_version`、序列号 `board_serial`，整机厂商 `sys_vendor`、型号 `product_name`，BIOS 厂商 `bios_vendor`、版本 `bios_version`、日期 `bios_date`。

#### Windows 的取法

1. CPU 使用率：`GetSystemTimes` 给出空闲、内核、用户三个累计时间（100 纳秒），内核时间已含空闲时间。空闲 = 空闲时间，总计 = 内核时间 + 用户时间。
2. 内存：`K32GetPerformanceInfo` 的页数乘以页大小——总量取 `PhysicalTotal`，可用取 `PhysicalAvailable`，缓存取 `SystemCache`；空闲为空。
3. 物理核心与最后一级缓存：`GetLogicalProcessorInformation` 返回的条目里，关系是处理器核心（`RelationProcessorCore`）的条目数是物理核心数；关系是缓存（`RelationCache`）的条目里取级别最高、类型不是指令缓存的那一项的大小，同一级取先出现的。
4. 型号、厂商与标称频率：注册表 `HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0` 的 `ProcessorNameString`、`VendorIdentifier` 与 `~MHz`（大于 0 才算）。
5. GPU：显示适配器类下名字是四位数字的子键各一块，按名字的字节序，只取 `MatchingDeviceId` 以 `pci\` 开头（不区分大小写）的。名称取 `DriverDesc`，厂商名取 `ProviderName`，驱动版本取 `DriverVersion`；厂商 ID 与设备 ID 取 `MatchingDeviceId` 里 `ven_` 与 `dev_` 之后的四位十六进制数字，写成小写；显存取 `HardwareInformation.qwMemorySize`，读不出正数时取 `HardwareInformation.MemorySize`（64 位或 32 位的数值，或 8 字节、4 字节的小端二进制）。驱动名与温度为空。
6. 主板：注册表 `HKLM\HARDWARE\DESCRIPTION\System\BIOS` 的 `BaseBoardManufacturer`、`BaseBoardProduct`、`BaseBoardVersion`、`SystemManufacturer`、`SystemProductName`、`BIOSVendor`、`BIOSVersion`、`BIOSReleaseDate`；序列号为空。
7. 系统运行时长：系统启动以来的毫秒数（.NET 的 `Environment.TickCount64`）。

### 取不到的项

外部命令（Windows 的 `powershell Get-CimInstance`，Linux 的 `dmidecode`、`lspci`、`nvidia-smi`、`top`、`df`，macOS 的 `system_profiler`、`sysctl`、`vm_stat`）有挂起与注入面。本 RFC 不启动外部命令，因此取不到这些：

- GPU：
  - 没有 NVIDIA 专有驱动的显存与温度；`amdgpu` 等在 sysfs 公开的驱动仍可读。
  - 没有 GPU 与显存的使用率、显示模式（`VideoModeDescription`）与设备状态（`Status`）。
  - Windows 没有温度；列出的是装过驱动的 PCI 显示适配器，已拔下的设备可能仍在列。
- 主板：Windows 没有序列号；Linux 只有 root 读得到 `board_serial`。
- macOS 等其他平台：只有 BCL 读得到的部分（运行时与进程、逻辑核心与架构、系统运行时长、磁盘、网卡）；CPU 型号与使用率、内存、GPU、主板为空。
- 内存：Windows 没有空闲字节。
- 网卡：没有 DHCP 服务器、是否多播、是否只收，以及丢包与错误包计数。
- 运行时：没有命令行参数、环境变量个数、系统目录与当前目录。
- 容器里：内存与 CPU 使用率是宿主机的数值（来自 `/proc`），不是 cgroup 限额；逻辑核心数是 .NET 看到的可用核心数。

### 装配

`Observability.Host` 的 `HostObservabilityFeature`（`IHostedFeature`）被选中提供 `observability.host` 时注册单例 `IHostMonitor`；启动时同步采第一次样并起采样循环，停止时结束循环。

## 一致性

向量（档 `observability`，2026-11-24，ADR-0277；以前这个档没有向量）：`conformance/vectors/observability/host-usage.json`——「采样」一节的三条公式（系统 CPU 使用率、进程 CPU 使用率、网卡速率）连同计数变小、总增量为零与没有读到时为空、百分比限制在 0–100 且不取整，「契约」一节里内存与磁盘由总量算出的已用与使用率，采样间隔的取值范围，以及「平台读取路径」里 Linux 的取法（`linux` 一节：`/proc/stat`、`/proc/meminfo`、`/proc/uptime`、`/proc/self/mounts` 的文本，处理器与显卡在 `/proc`、`/sys` 与 `pci.ids` 里的一组文件，各自读出的值；2026-11-24，ADR-0286）。

这个档没有行为一致性套件：其余的规范性内容——各平台的读取路径、读不到的项为空、读取不阻塞不抛出、导出器的装配——读的是所在机器与运行中的后端，输入不由用例给出，由各实现按平台写用例核对（参考实现在 `Observability.Host.Tests` 与 `Observability.OpenTelemetry.Tests`）。「内核的遥测」同样没有向量，由各实现用进程内的监听器核对（参考实现在 `Kernel.Tests` 的 `ObserveTelemetryTests`、`ObserveTracingTests` 与插件宿主的 `PluginTracingTests`）。Windows 的取法读的是系统调用与注册表，主板在 Linux 上是逐个文件各取一行，磁盘的容量要向文件系统查询：这几处输入不由用例给出或没有可算的内容，没有向量。
