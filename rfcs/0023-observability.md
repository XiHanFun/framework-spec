# RFC-0023 可观测性：导出与运行信息

> 状态：已采纳（2026-11-02 修订：运行进程的用户名照常打印） | 日期：2026-10-30 | 对应 ADR：ADR-0056、ADR-0093（用户名不遮蔽） | 参考实现：`XiHan.Framework.Observability.Abstractions`、`XiHan.Framework.Observability.Host`、`XiHan.Framework.Observability.OpenTelemetry` | 一致性：无套件（读的是所在机器，按平台写用例）

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
- 采样比例只作用于根 Span，有上游 trace 的沿用上游决定。内核的 `xihan.invoke` 同样遵守（RFC-0003 规则 7）：有上游（`traceparent`、宿主当前的 Activity）时沿用它的采样标志，上游「不采样」就不采样；没有上游时它是根 Span，按 `SamplingRatio` 采样，trace-id 与调用上下文的一致。

## 运行信息

### 契约

`IHostMonitor` 的七个读取方法各返回一份不可变记录：

| 方法 | 记录与内容 |
| --- | --- |
| `ReadRuntime()` | `RuntimeInfo`：操作系统描述、平台、版本与架构，.NET 描述与 RID，进程架构，机器名、用户名、是否交互，进程 ID、名称、启动时刻与运行时长，系统启动时刻与运行时长，工作集、私有字节、托管堆，线程数、句柄数，三代 GC 次数，进程 CPU 使用率 |
| `ReadProcessor()` | `ProcessorInfo`：型号、厂商、架构、物理核心数、逻辑核心数、标称频率（MHz）、最后一级缓存字节数、系统 CPU 使用率 |
| `ReadMemory()` | `MemoryInfo`：总量、可用、空闲、缓存（字节），以及由总量与可用算出的已用与使用率 |
| `ReadDisks()` | `DiskInfo` 列表：挂载点或盘符、设备、卷标、文件系统、类型，总量、空闲与当前用户可用（字节），已用与使用率 |
| `ReadNetworkInterfaces()` | `NetworkInterfaceInfo` 列表：ID、名称、描述、类型、状态、速率（bit/s）、物理地址、带前缀长度的 IPv4 / IPv6 地址、网关、DNS，累计收发字节与包数，收发速率（字节/秒） |
| `ReadGpus()` | `GpuInfo` 列表：名称、厂商、PCI 厂商与设备 ID、驱动、驱动版本、显存字节数、温度（℃） |
| `ReadBoard()` | `BoardInfo`：主板厂商、型号、版本与序列号，整机厂商与型号，BIOS 厂商、版本与日期 |

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
