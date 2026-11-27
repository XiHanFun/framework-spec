# RFC-0013 调度与领导选举

> 状态：已采纳 | 日期：2026-10-23，2026-10-26 修订（ADR-0020：作业存储、动态作业、多触发器、misfire、暂停、历史、集群分发），2026-11-01 修订（ADR-0074：调度器启动之前的手动触发同样不可用、一致性套件由用例推进时钟并主动轮询；ADR-0082：运行查询的分页越界返回校验错误；ADR-0091：应用不在服务状态时不持有领导租约、数据库实现的租约释放不经管道），2026-11-02 修订（ADR-0094：领导选举一致性套件加上应用生命周期用例），2026-11-24 修订（ADR-0277：一致性向量；ADR-0270：调度器与存储的查询方法改名，补上动词 `List` / `Find`；ADR-0286：cron 的分段、取值、步长、年份、`L-n` 与搜索上限逐条写明，段的两侧不再收别的空白，搜索上限按下一次的墙上时间判定），2026-11-25 修订（ADR-0297：cron 的步长不超过这一段的取值上限；计划的文本形式写成文法——各部分之间只认单个空格，间隔、次数、cron 与选项的读法与写法相同、选项有固定次序且不能重复，时刻按 RFC-0003 规则 8 读，时区名是 IANA 名；新增向量 `schedule-texts.json`），2026-11-27 修订（ADR-0321：作业与触发器的说明与数据不含孤立的代理项；ADR-0323：事件带上按 `type` 登记的 `dataschema`；ADR-0335：代码登记的作业不合法即登记或启动失败，不再每个周期重试） | 对应 ADR：ADR-0020、ADR-0074、ADR-0082、ADR-0091、ADR-0094、ADR-0270（查询方法改名）、ADR-0277（一致性向量）、ADR-0286（cron 的细则）、ADR-0297（cron 步长的上限、计划的文本形式）、ADR-0321（定义里的孤立代理项）、ADR-0323（事件的数据模式）、ADR-0335（代码登记的作业不合法即启动失败） | 参考实现：`XiHan.Framework.Scheduling.Abstractions`、`XiHan.Framework.Scheduling.InMemory`、`XiHan.Framework.Scheduling.Database` | 一致性：`XiHan.Framework.Scheduling.Conformance`，向量 `conformance/vectors/scheduling/` | 一致性档：`scheduling`

## 摘要

调度子系统把"按计划运行的工作"做成与请求同构的调用：作业是作用域服务，每次运行在新作用域、新上下文里经完整内核管道执行。引擎（`JobSchedulerEngine`）与协议性的规则（计划推算、misfire、并发、恢复、历史）在抽象包里，Provider 只实现作业存储 `IJobStore` 与领导选举 `ILeaderElection`：内存实现是单进程，数据库实现是多实例集群——每次按计划的触发在整个集群只运行一次，由抢到触发器的节点运行；节点失联由领导者恢复。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `scheduling.jobs@1` | 作业与触发器的登记、持久化、按计划运行、手动触发、暂停与恢复、运行历史，`IJobScheduler` | `Scheduling.InMemory`（进程内存储，单进程轮询） |
| `scheduling.leader@1` | 租约式领导选举，`ILeaderElection` | `Scheduling.InMemory`（进程内租约；多实例换 `Scheduling.Database` / `.Kubernetes`） |

## 作业与触发器

Quartz 的"作业（做什么）"与"触发器（何时做）"分离：一个作业可以有多个触发器，每个触发器各自推算下一次运行。

- 名字 `JobName`：`^[a-z0-9][a-z0-9._-]{0,127}$`（与主题文法相同）；触发器名、领导选举的资源名同文法。触发器键 `TriggerKey(Job, Trigger)`。
- 作业定义 `JobDefinition`：`Name`、`JobType`（实现 `IJob` 的类型，必须已登记）、`Triggers`（可空：没有触发器的作业只能手动触发，持久保留）、`Data`（`JsonObject`，运行时随 `JobRun.Data` 传给作业）、`AllowConcurrent`（默认 `false`：整个集群同一作业同一时刻最多一次运行，按计划的触发被阻塞直至上一次结束、手动触发返回 `SCHEDULING_CONFLICT_001`）、`RunOnEveryInstance`（默认 `false`：按计划的触发在集群只运行一次；`true`：每个实例各自按本地时钟运行，不经存储抢占）、`RequestsRecovery`（默认 `false`：运行所在节点失联时由领导者重新入队一次，`JobTrigger.Recovery`）、`Retry`（重试策略，空为不重试，见下）、`Timeout`（单次执行时限，空为不限，见下）、`Description`。便利构造 `JobDefinition(name, schedule, jobType)` 生成一个名为 `default` 的触发器。
- 触发器定义 `JobTriggerDefinition`：`Name`、`Schedule`、`Data`（覆盖作业 `Data` 的同名键）、`Priority`（默认 5，到期的触发器多于可运行名额时大者先）、`MisfirePolicy`、`Description`。
- 作业类型登记：`services.AddJobType<TJob>()`（键 = 类型全名）或 `AddJobType<TJob>(key)`；`services.AddJob<TJob>(definition)` / `AddJob<TJob>(name, schedule)` 同时登记类型与作业（启动时写入存储，来源 `Code`）。同名重复 `AddJob` 是编程错误；运行时 `ScheduleAsync` 同名即替换。代码登记的作业不合法同样是编程错误，不等到写入存储（2026-11-27 修订，ADR-0335；以前照收，进入服务状态后每个周期写入存储都以 `SCHEDULING_VALIDATION_001` 失败、记一条日志、下个周期再试，按名字排在它后面的代码作业也一直写不进去）：作业与触发器的说明与数据有孤立的代理项、触发器名重复，`AddJob` 抛出参数异常（`ArgumentException`）；触发器引用的日历没有登记（日历可以在作业之后登记，登记时还判定不了），调度器随 Provider 特性启动时抛出 `FrameworkException`（`SCHEDULING_VALIDATION_001`），应用启动失败。两处的消息都写明作业名与原因，原因与 `ScheduleAsync` 返回的 `reason` 相同（见下文）。`JobTypeRegistry.Keys` 按序数列出已登记的类型键，管理界面据此列出运行时新建作业可选的类型（ADR-0053）。
- 计划 `JobSchedule`：`Manual`（只手动触发）、`Every(interval, repeatCount?)`（上一次完成后再等一个间隔，固定延迟；`RepeatCount` 为总运行次数，空为无限）、`FromCron(expression, timeZoneId?)`（cron，默认 UTC；时区是 IANA tz 数据库的名字，见「计划的文本形式」第 6 条；2026-11-25 修订，ADR-0297，以前写的是 `TimeZoneInfo` ID，.NET 也收 Windows 的时区 ID）、`Once(at)`（一次）；任何计划可带 `StartAt`（之前不运行）、`EndAt`（之后转为 `Complete`）与 `Calendar`（排除日历名）。`ToString()` / `Parse` 为文本往返，存储以文本持久化，文法见「计划的文本形式」。
- cron `CronExpression`：5 段（`m h dom mon dow`，秒 = 0）或 6 段（`s m h dom mon dow`），可选第 7 段年份；`*`、`?`、列表、范围、步长、月与星期名、`L`、`L-n`、`W`、`LW`、`nL`、`n#k`；星期 0/7 = 周日、1 = 周一（Unix 约定，不是 Quartz 的 1 = 周日）；日与星期同时限定取并集；`Next(after)` 按 UTC，`Next(after, timeZone)` 按该时区墙上时间，夏令时跳过的时刻不运行、重复的时刻只运行第一次；搜索上限 5 年（1830 天）。文法与推算的细则（2026-11-24 写明，ADR-0286；以前只有上面这一句，名字的大小写、分隔符、单个值带步长、年份与 `L-n` 的范围、搜索上限的边界都只在 .NET 的代码里）：
  1. 分段：表达式按空格（U+0020）与制表符（U+0009）分段，连着的几个算一个分隔，两端的不计；段数是 5、6 或 7。别的空白（换行、不换行空格等）不是分隔符，出现在表达式里就落在某一段中，那一段不合法（2026-11-24 修订，ADR-0286；以前 .NET 把各段两侧的任何空白都裁掉，结尾带换行、空格之间夹着不换行空格的表达式照收）。
  2. 取值：秒、分 0–59，时 0–23，日 1–31，月 1–12，星期 0–7，年 1970–2099。数字是 ASCII 的十进制数字，可以有前导 0，不带符号。月可写 `JAN`–`DEC`，星期可写 `SUN`–`SAT`（0–6）；名字与下面的 `L`、`W` 都不区分 ASCII 字母的大小写，不认全称，也不认 ASCII 以外的字母。
  3. 一段是 `*`，或逗号分隔的若干项，项不能为空；日与星期的整段还可以是 `?`，等于 `*`（`?` 不能写在别的段上，也不能写在列表里或带步长）。普通的项：`a`（一个值）、`a-b`（范围，a 不大于 b，不回绕：`FRI-MON`、`NOV-FEB` 不合法）、`*/n`（整段每 n）、`a/n`（从 a 取到这一段的最大值每 n）、`a-b/n`。步长 n 是 1 到这一段取值上限的整数，上限即第 2 条里这一段能写的最大值：秒、分 59，时 23，日 31，月 12，星期 7，年 2099；超过上限的表达式不合法（2026-11-25 修订，ADR-0297；以前是 1 到 2147483647，`*/60` 写在分钟上照收、只取起点）。取起点、起点 + n、起点 + 2n……直到超过终点，n 比跨度大时只有起点（`*/31` 写在日上只有 1 日）。
  4. 星期：7 与 0 都是周日。范围与步长在 0–7 里数，数完把 7 归到 0：`1/2` 是 1、3、5、7，即周一、周三、周五与周日；`5-7` 是周五到周日。步长的上限因此是 7 而不是 6：数的时候这一段的最大值是 7，`*/7` 是 0 与 7，即只有周日；`*/8` 不合法（2026-11-25 写明，ADR-0297）。
  5. 年：7 段写法的第 7 段。写 `*` 也只是 1970 到 2099，2099 年之后没有下一次；5 段与 6 段写法不限年份。步长的上限照第 3 条是 2099，130 到 2099 的步长都只取起点（2026-11-25 写明，ADR-0297）。
  6. 日的特殊项：`L`（当月最后一天）、`L-n`（最后一天往前 n 天，n 是 0 到 30，当月没有那一天就不运行）、`nW`（离 n 日最近的工作日，n 是 1 到 31，不跨月：n 日是周六取前一天，1 日是周六取 3 日；是周日取后一天，月末是周日取前两天；当月没有 n 日不运行）、`LW`（离最后一天最近的工作日，同样不跨月）。星期的特殊项：`wL`（当月最后一个周 w）、`w#k`（当月第 k 个周 w，k 是 1 到 5，没有第 k 个的月份不运行），w 是 0–7 的数字或名字。特殊项可以与普通项混在一个列表里，自己不带范围与步长（`1-5W`、`5L/2`、`L-3/2` 不合法）；星期上单独的 `L` 不合法。
  7. 日与星期：整段正好是 `*` 或 `?` 才算不限定，`*/1`、`*,5` 都算限定。两段都限定时取并集，只限定一段时只看它，都不限定时每天都算。
  8. 推算：下一次是严格晚于 `after` 的第一个匹配时刻，精确到秒。按时区推算时把 `after` 换成该时区的墙上时间、去掉秒以下，从下一秒起找墙上时间匹配的时刻；夏令时开始时被跳过的墙上时间不运行，结束时重复的墙上时间只运行第一次出现的那次。
  9. 搜索上限：下一次的墙上时间与 `after` 的墙上时间（去掉秒以下）相差不超过 1830 天（5 × 366 天），超过即没有下一次（2026-11-24 修订，ADR-0286；以前只写「搜索上限 5 年」，.NET 只在按年、月、日跳之前核对上限，跳到的那一天越过上限也照样返回，实际多收最多一个月）。
- 日历 `IJobCalendar.IsExcluded(at)`：`services.AddJobCalendar(name, calendar)` 按名登记；内置 `HolidayCalendar`（日期集合）、`WeeklyCalendar`（星期集合）、`DailyCalendar`（每日时段）、`CronCalendar`（匹配 cron 的时刻）、`CompositeCalendar`（任一排除即排除）。推算下一次运行时跳过被排除的时刻（`Every` 的下一次也顺延到首个未排除时刻）。未登记的日历名 → `SCHEDULING_VALIDATION_001`。
- misfire：触发器的到期时刻落后当前时刻超过 `MisfireThreshold`（默认 60 秒；进程停机、暂停期间、节点失联都会造成）时按 `MisfirePolicy`：`RunOnce`（默认，立即运行一次，再从现在推算下一次）、`Skip`（不补，从现在推算下一次）、`RunAll`（按原计划逐次补齐，每个轮询周期补一次直至追平）。
- 重试（ADR-0060）：`JobRetryPolicy`：`MaxRetries`（1–100，失败后最多再运行几次）、`Delay`（第一次重试前的等待，大于零）、`Backoff`（`Fixed` 每次都等 `Delay`；`Exponential` 第 n 次重试等 `Delay × 2^(n-1)`）、`MaxDelay`（指数退避的上限，空为不设）；等待时间与上限都在 30 天以内，指数退避算出的等待超过 30 天按 30 天。
  - 只有失败的运行重试：作业返回失败、抛出异常或超过执行时限。被取消的运行（`CancelRunsAsync`、宿主停止）与失联的运行（`Lost`）不重试。
  - 重试是一次新运行：`Trigger` 为 `Retry`，`Attempt` 加一（首次为 1），`RetryOf` 是首次运行的 ID，`Data` 与失败的那次相同。
  - 集群唯一的作业把重试写成存储里的一次性触发器，到期时刻 = 失败时刻 + 等待时间，任一节点抢占运行，进程重启不丢；不允许并发的作业照常独占（上一次未结束时阻塞到它结束）。`RunOnEveryInstance` 的作业在失败的实例上进程内等待重试，实例停止时未到期的重试丢弃。
  - 手动 `TriggerAsync` 失败也按策略重试，调用方拿到的是第一次的结果。
- 执行时限（ADR-0060）：`Timeout`（大于零、不超过 30 天）到时取消作业拿到的令牌；作业返回之前时限已过，运行记为失败，错误码 `SCHEDULING_TIMEOUT_001`（作业不响应取消时，运行在它返回后才结束）。时限内取消请求或宿主停止仍记为取消。
- 重试策略与执行时限随作业持久化，代码作业指纹包含它们（设置了才计入，没设的作业指纹不变），运行时 `ScheduleAsync` 修改与 `RestoreAsync` 恢复同改计划的语义；`JobStatus.Job` 带出两项。
- 运行 `JobRun`：`Id`、`Name`、`TriggerName`（手动为空）、`ScheduledAt`、`Trigger`（`Schedule` / `Manual` / `Recovery` / `Retry`）、`Attempt`（第几次尝试，从 1 起）、`RetryOf`（重试时为首次运行的 ID）、`Data`（作业 `Data` ← 触发器 `Data` ← 手动触发的 `Data` 逐层覆盖）。`JobDispatcher` 为每次运行开新作用域，构造上下文（关联 ID = 运行 ID，运行信息可由 `JobDispatcher.FindRun(context)` 取得），经完整管道执行 `IJob.RunAsync`；返回失败或抛出异常都算失败（异常翻译为 `SCHEDULING_UNEXPECTED_001`）。不写幂等键。作业拿到的取消令牌在 `CancelRunsAsync` 与宿主停止时取消。
- 作业 `IJob.RunAsync(run, context)`：取消与超时在 `context.CancellationToken` 上，与操作的处理器一样不另传令牌。领导租约 `LeaderLease` 的 `RenewAsync` 与 `ReleaseAsync` 都收令牌、以 `Result` 报告；释放幂等，后端没能删掉租约时返回那个失败，租约到期后自然可被接管（2026-11-11 修订，ADR-0157）。
- 计划 `JobSchedule` 里不适用的字段是 `null`：`Interval` 只对间隔计划有值，`At` 只对一次性计划有值，不以 `TimeSpan.Zero` 或 `default` 表示「没有」（2026-11-11 修订，ADR-0161）。

## 计划的文本形式

计划以一段文本持久化（`Scheduling.Database` 的触发器行；内存存储不持久化），.NET 由 `JobSchedule.ToString()` 写出、`Parse` / `TryParse` 读入（2026-11-25 写明，ADR-0297；以前只有 `every 00:10:00 x5` 这样的几个例子，间隔与时刻的写法、选项的次序与能否重复都只在 .NET 的代码里，而且读法比写法宽：各部分两侧的任何空白都裁掉、连着的空格算一个，间隔照收 .NET `TimeSpan` 的 `c` 格式的各种读法（`00:10` 是十分钟、`5` 是五天），次数可以带前导 0，时刻可以不带偏移（当作 UTC），选项不论次序、可以重复（取最前面的一个），`tz=` 跟在 cron 以外的种类后面时悄悄丢掉）：

1. 分隔：整段由若干部分组成，部分之间正好一个空格（U+0020），开头与结尾没有空格。连着的空格，以及制表符、换行、不换行空格等别的空白，出现在任何位置都不合法（与 cron 的分段同一原则，ADR-0286：只认确定的分隔）。
2. 种类：第一部分是种类，区分大小写，之后是种类自己的部分：
   - `manual`：只手动触发，没有自己的部分。
   - `every <间隔>` 或 `every <间隔> x<次数>`：间隔见第 3 条；次数是总运行次数，`x`（小写）接 1 到 2147483647 的十进制整数，不带前导 0，也不带符号。
   - `cron <表达式>`：表达式照「作业与触发器」cron 的第 1～9 条，但在计划文本里各段之间同样只是一个空格，cron 自己收的制表符与连着的空格在这里不收；写回时照原样，名字保留书写时的大小写。
   - `once <时刻>`：时刻见第 4 条。
3. 间隔：`[d.]hh:mm:ss[.fffffff]`。`d` 是天数，不带前导 0 的正整数，不足一天时连同 `.` 不写；`hh` 是 00–23，`mm` 与 `ss` 是 00–59，都正好两位；`fffffff` 是不足一秒的部分，正好七位（以 100 纳秒为单位），为 0 时连同 `.` 不写。间隔大于 0，以 100 纳秒计不超过 9223372036854775807（`10675199.02:48:05.4775807`）。这是 .NET `TimeSpan` 的 `c` 格式对正值写出的形状，读法与写法相同：`00:10`、`5`、`0:10:00`、`00:10:00.5`、`00:10:00.0000000`、`0.00:10:00`、`01.00:00:00` 都不合法。
4. 时刻：读法是 RFC-0003 规则 8 的读法（截止时间与消息信封 `time` 的同一条文法：`Z` 或 `±hh:mm` 偏移，一到九位小数，比实现的时钟精度更细的小数截断）；写出时一律换算成 UTC、七位小数、`+00:00` 偏移（`2026-10-23T10:30:00.0000000+00:00`）。
5. 选项：依次是 `tz=<时区名>`（只能紧跟在 cron 的表达式后面）、`start=<时刻>`、`end=<时刻>`、`calendar=<日历名>`，都可以省略；出现的按这个次序，每个至多一次。选项名区分大小写，`=` 两侧没有空格，`=` 之后不能为空。不认得的选项、次序不对与重复的都不合法。日历名是作业名的文法（`AddJobCalendar` 只收这样的名字）。`start=` 与 `end=` 的先后不作核对。
6. 时区名：IANA tz 数据库里的名字（`Asia/Shanghai`、`America/Argentina/Buenos_Aires`、`Etc/GMT-8`、`UTC`），文法 `^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*$`；Windows 的时区 ID 等别的名字不收。没有 `tz=` 是 UTC，写了 `tz=UTC` 的照样写回。读入时在实现所用的时区库里按名字查找，查不到（名字不在 IANA 数据库里，或本机的时区库缺这一项）这段文本就不合法：同一段文本在缺这个时区的机器上读不出来，部署要带上完整的 IANA 时区数据（Linux 容器装 `tzdata`）。`UTC` 不依赖时区库，任何实现都认得。.NET 的 `FromCron` 对时区名做同样的核对，不合文法、查不到或不是 IANA 的名字抛出 `ArgumentException`，不写出读不回的文本。
7. 写出：种类与它的部分；cron 有时区时接 ` tz=<时区名>`；再依次接有值的 ` start=`、` end=`、` calendar=`。合法的文本写回时只有时刻的写法可能变（换算成 UTC、补足或截到七位小数），其余各部分照原样，所以框架写出的文本读回之后再写出与原文相同。

## 调度器 `IJobScheduler`

全部按批量、逐项结果：

- 批量语义（ADR-0148）：写入与按标识的操作都返回与入参等长、同序的逐项结果；空集合不访问存储；重复项按顺序逐项施加；取消在开始前检查，已取消抛出；开始之后取消，已开始的项照常给出结果，尚未开始的项以 `KERNEL_CANCELLED_001` 结束，方法不抛出、不丢已完成项的结果。
- `ScheduleAsync(definitions[])`：新增或整体替换作业与触发器（来源 `Runtime`；替换代码登记的作业时来源记为 `Overridden`，见下；替换保留暂停状态与同名触发器的上一次运行时刻），立即生效，集群内其他节点在下一个轮询周期看到。作业类型未登记 → `SCHEDULING_NOTFOUND_002`；触发器名重复、日历未登记、计划不合法 → `SCHEDULING_VALIDATION_001`。作业与触发器的说明与数据（数据里逐层的成员名与字符串值）有孤立的代理项（RFC-0001「孤立的代理项」）同样是 `SCHEDULING_VALIDATION_001`（`reason` 为 `the definition has a lone surrogate in a description or the data`；作业名与触发器名的文法只有 ASCII，本来就排除了它），代码登记的作业在 `AddJob` 时就按这一条失败，见上文「作业类型登记」（2026-11-27 修订，ADR-0321，同日再修订，ADR-0335：ADR-0321 时代码登记的作业在写入存储时按这一条；以前照收：代码作业的指纹散列的是这些文本的宽松 UTF-8，数据先经 JSON 写出器写成文本，孤立的代理项都换成 U+FFFD，只差在它上面的两份代码定义指纹相同，运行时的修改在代码改了之后照样保留；持久化的定义读回来也已经换成 U+FFFD）。
- `UnscheduleAsync(names[])`：删除作业与触发器（进行中的运行照常结束，历史保留）；未登记 → `SCHEDULING_NOTFOUND_001`。删除代码登记的作业只到下次启动：启动时按代码重新写入。
- `RestoreAsync(names[])`（ADR-0053）：把运行时改过的代码作业恢复为代码定义（来源回到 `Code`，保留暂停状态与同名触发器的进度）；作业没有代码定义 → `SCHEDULING_NOTFOUND_005`。
- `PauseAsync(names[])` / `ResumeAsync(names[])`：暂停或恢复作业的全部触发器（暂停期间手动触发仍可用）；恢复时按 misfire 策略处理暂停期间错过的运行。`PauseTriggersAsync(keys[])` / `ResumeTriggersAsync(keys[])` 作用于单个触发器；触发器不存在 → `SCHEDULING_NOTFOUND_003`。
- `TriggerAsync(requests[])`（`new JobTriggerRequest(name) { Data = … }`）：在本实例立即运行并等待结果，返回与请求同序的逐项 `Result`；不受暂停与领导限制；未登记 → `SCHEDULING_NOTFOUND_001`，正在运行且不允许并发 → `SCHEDULING_CONFLICT_001`，本实例的调度器尚未启动或已开始停止 → `SCHEDULING_UNAVAILABLE_001`（见下）。
- `EnqueueAsync(requests[])`：插入一个立即到期的一次性触发器（名 `manual-<id>`），由集群中任一节点运行，不等待；返回触发器名。
- `CancelRunsAsync(runIds[])`：请求取消运行——本实例的运行立即取消其令牌，其他节点的运行在下一个轮询周期取消；不在运行中 → `SCHEDULING_NOTFOUND_004`。取消的运行记为 `Canceled`，等待它的手动触发得到 `SCHEDULING_CONFLICT_003`。
- `FindStatusesAsync(names[])`（2026-11-24 改名，ADR-0270，原名 `StatusesAsync`；未登记的为无值，按 RFC-0001「查找的命名」用 `Find`）：按名取 `JobStatus`（作业的只读视图 `JobView`、是否暂停、每个触发器的状态与下一次 / 上一次运行时刻与次数、运行中数量、最近一次完成的运行与结果），与入参等长、同序，未登记的为无值，重复的名字各得一份；单个作业用扩展方法 `FindStatusAsync(name)`（原名 `StatusAsync`）。`ListStatusesAsync(JobStatusQuery)` 按作业名的序数升序分页（`PageSize` 1～1000、缺省 100，不透明的续页令牌，读到令牌为空为止），越界或外来令牌 → `SCHEDULING_VALIDATION_003`。存储读不到时整批失败，不当作未登记（2026-11-11 修订，ADR-0148）。
- `ListRunsAsync(query)`（2026-11-24 改名，ADR-0270，原名 `RunsAsync`）：运行历史（按作业、状态、时间范围、触发器名 `TriggerName`、触发来源集合 `Triggers`、执行节点、有无错误；查询成员与运行记录同名，2026-11-11 修订，ADR-0161；分页，计划时刻降序），返回只读视图 `JobRunView`（失败结果在 `Outcome`），存储记录 `JobRunRecord` 只留给 `IJobStore`（ADR-0148）；`CountRunsAsync(query)` 给出同一组过滤项下的总数（不看分页），管理界面分页用（ADR-0053）。`JobRunQuery` 的分页（`Skip` ≥ 0、`Take` 1～`MaxTake`（1000），默认 0 与 100）不在构造时抛出：越界时 `ListRunsAsync` 返回 `SCHEDULING_VALIDATION_002`（HTTP 400，消息键 `scheduling.run.query_page_invalid`，参数 `skip`、`take`、`max`），不进存储——查询常由请求绑定，越界是调用方可以改正的输入（2026-11-01 修订，ADR-0082）；`CountRunsAsync` 不看分页，不校验。存储收到的查询由调度器校验过。

调度器在应用不在服务状态时（进入服务状态之前、开始排空之后）不按计划运行，领导租约也不拿不续，持有就立即释放（见「引擎」第 1 步）；进入服务状态后首个周期把代码登记的作业写入存储，写入失败（存储不可用）记一条日志、下个周期再试；定义本身不合法的在登记或启动时就已失败，不会走到这里（2026-11-27 写明，ADR-0335）：

- `ScheduleAsync`、`RestoreAsync`、`UnscheduleAsync` 与 `TriggerAsync`、暂停 / 恢复一样，先把代码作业写进存储：宿主刚启动、首个轮询周期之前的修改也按下面的规则判定来源（ADR-0060）。
- 每个代码作业记下代码定义的指纹 `CodeFingerprint`（类型键、数据、标志、重试策略与执行时限（设置了才计入）与各触发器的 SHA-256）。
- 存储里的作业来源是 `Overridden`（运行时 `ScheduleAsync` 改过的代码作业），且记下的指纹与当前代码定义相同时，保留运行时的修改。
- 其余情况（新作业、来源是 `Code`、代码定义变了）按代码写入，来源 `Code`；覆盖掉运行时修改时记一条警告。
- 暂停状态一律保留。

停止时等待进行中的运行结束（按宿主停止令牌取消）。本实例的调度器启动之前与开始停止之后，到达的 `TriggerAsync` 都不起新的运行，返回 `SCHEDULING_UNAVAILABLE_001`（HTTP 503，按错误 KIND 映射）：启动之前没有轮询照看运行（取消请求、维护都不进行），停止只等待已经开始的运行，这两段时间里起的运行无人等待，记录会停在运行中直到被判失联。调度器随 Provider 特性启动（宿主进入服务状态之前），之后照常，停止后再次启动亦然（ADR-0074）。`EnqueueAsync` 只写存储、由集群中任一节点运行，不受本实例启动与停止的影响。

## 存储 `IJobStore`

Provider 实现，所有方法返回 `Result`、批量优先、存储不可达 → `SCHEDULING_UNAVAILABLE_003`，容量已满（内存实现的历史）→ `SCHEDULING_UNAVAILABLE_004`；六个读取方法以 `List` 起头（2026-11-24 改名，ADR-0270；原名依次是 `TriggersAsync`、`RunsAsync`、`RunningAsync`、`LastRunsAsync`、`CancelRequestedRunsAsync`、`NodesAsync`，参数、返回与行为不变）：

- `IsShared`：存储是否跨实例共享（抢占即集群唯一）。`false`（内存实现）时不共享的"集群只运行一次"作业只在持有领导租约的实例上按计划运行；`true`（数据库实现）时由抢占保证。
- 作业：`FindJobAsync`、`ListJobsAsync`、`SaveJobsAsync`（新增或替换定义与暂停状态；已有行的 `ActiveRunId` 由存储保留，只有 `TryBeginRunAsync` / `CompleteRunAsync` / `RecoverNodesAsync` 改它）、`DeleteJobsAsync`（级联触发器）。`JobRecord`：定义的持久化形态（`JobTypeName` 为类型键）加 `Paused`、`ActiveRunId`（不允许并发时正在运行的运行 ID）、`Source`（`Code` / `Runtime` / `Overridden`）、`CodeFingerprint`（代码作业最近一次按代码写入时的指纹，运行时作业为空）、`UpdatedAt`。
- 触发器：`ListTriggersAsync(job?)`、`SaveTriggersAsync`、`DeleteTriggersAsync`、`AcquireDueAsync(dueBefore, limit, nodeId, now)`（原子地把 `Waiting` 且到期的非本地触发器改成 `Acquired(nodeId)`，按到期时刻升序、优先级降序取至多 `limit` 条；多个节点同时调用时每条触发器只被一个节点拿到）。`TriggerRecord`：定义加 `State`（`Waiting` / `Paused` / `Acquired` / `Blocked` / `Complete` / `Error`）、`NextFireAt`、`PreviousFireAt`、`AcquiredBy`、`AcquiredAt`、`FireCount`、`Error`、`Local`。
- 运行：`TryBeginRunAsync(run, exclusive)`（写入运行记录；`exclusive` 时原子地占用作业的 `ActiveRunId`，已被占用返回 `false`）、`CompleteRunAsync(run)`（写回结果，释放 `ActiveRunId`，把该作业 `Blocked` 的触发器改回 `Waiting`）、`BlockTriggerAsync(trigger)`（`TryBeginRunAsync` 返回 `false` 之后用：同一个原子操作里确认 `ActiveRunId` 仍指向未结束的运行，是则存为 `Blocked` 返回 `true`，占用已释放则存为 `Waiting` 返回 `false`，都清掉抢占者；与 `CompleteRunAsync` 的释放互斥，阻塞的触发器总能在占用释放时回到 `Waiting`，ADR-0062）、`ListRunsAsync(query)`、`CountRunsAsync(query)`、`ListRunningRunsAsync(job?)`、`ListLastRunsAsync(jobs[])`、`RequestCancelAsync(runIds[])`、`ListCancelRequestedRunsAsync(nodeId)`（列出本节点上被请求取消的运行，本身不取消任何运行）、`PurgeRunsAsync(before)`。
- 节点：`HeartbeatAsync(node)`、`ListNodesAsync()`、`RecoverNodesAsync(nodeIds[], now)`（失联节点 `Acquired` / `Blocked` 的触发器改回 `Waiting`，`Running` 的运行记为 `Lost`，释放其 `ActiveRunId`，删除节点记录，返回标记为 `Lost` 的运行）。

## 引擎

`JobSchedulerEngine`（抽象包，Provider 装配并启停）每个 `PollInterval`（默认 1 秒）执行一个周期：

1. 应用不在服务状态（启动中、排空中）的周期什么也不做，只释放持有的领导租约：排空要等依赖调度的特性逐个停完，这期间续着租约，在服务的另一个实例就拿不到，失联恢复与历史清理停在原地；释放后它下一个周期接手，不等租约到期（2026-11-01 修订，ADR-0091）。在服务状态时：心跳（`IsShared` 时）；尝试获取或续约领导租约（`LeaseResource`，默认 `scheduling.jobs`）。调度器停止时也释放持有的租约。
2. 领导者每 `HousekeepingInterval`（默认 60 秒）做一次维护：心跳超过 `NodeTimeout`（默认 90 秒）的节点视为失联并恢复，`RequestsRecovery` 的失联运行以 `Recovery` 触发重新入队；删除早于 `HistoryRetention`（默认 7 天，0 = 不删）的历史。
3. 取消：本节点被请求取消的运行取消其令牌。
4. 抢占：本周期的名额 = `min(AcquireBatchSize, MaxConcurrentRuns − 本节点运行中数量)`（默认 16 与 32），不大于 0 时本周期不抢占；`AcquireDueAsync(now, 名额, nodeId)`。对每条：作业暂停或类型未登记 → 触发器改为 `Paused` / `Error`；misfire 按策略；`TryBeginRunAsync` 失败（不允许并发且上一次未结束）→ `BlockTriggerAsync` 把触发器存为 `Blocked`，上一次结束时自动回到 `Waiting` 并按 misfire 策略处理（上一次恰在两步之间结束则直接回到 `Waiting`，下个周期再抢占）；否则推算下一次（cron / 一次性从计划时刻推算并立刻回到 `Waiting`，间隔计划完成后再推算，次数用尽或超过 `EndAt` → `Complete`）并在后台运行。间隔触发器在运行结束时先写下一次时刻（仍由本节点持有），运行标为完成、占用释放之后才回到 `Waiting`：状态里不会出现「已不在运行、也没有下一次」的空档，别的节点也不会在本次运行结束前抢到它、再因占用未释放把它阻塞（ADR-0062）。同一节点上对触发器的读改写（抢占后运行、结束时推算、暂停与恢复、登记与删除）互斥，抢占到的触发器在互斥内按存储里的当前状态重读，暂停等写入不会被随后的写回覆盖。
5. 本地触发器（`RunOnEveryInstance`）由每个实例按自己的时钟推算与运行，不经抢占；运行同样写入历史。

事件：装配了 `messaging.queue` 时经 `IMessagePublisher` 发布 CloudEvents 信封（`Source` = `/scheduling`，`Data` = `SchedulingEventData` JSON，`dataschema` 为应用给这个主题（即 `type`）登记的数据模式、没有登记则不带，RFC-0009「框架事件的数据模式」；2026-11-27 写明，ADR-0323）：`scheduling.job.scheduled|unscheduled|paused|resumed`、`scheduling.run.started|succeeded|failed|canceled|lost`；发布失败只记日志。

## 领导选举

- `ILeaderElection.TryAcquire(resource, ttl)`：空闲（或租约已过期）时授予 `LeaderLease`；被持有返回 `SCHEDULING_CONFLICT_002`。
- `LeaderLease.Renew()` 延长 `ttl`；`Release()` 立即放弃，应用不在服务状态时也必须生效（引擎在排空中的周期与停止时释放，ADR-0091）；过期后他人可获取，原持有者续约失败。不可重入。持有者标识是实例身份（进程内实现每次获取生成新值，Kubernetes 实现是 Pod 名，数据库实现是节点 ID）。
- Provider：`Scheduling.InMemory`（进程内）、`Scheduling.Kubernetes`（`coordination.k8s.io` Lease，直接调用 API Server；ServiceAccount 的 CA 存在时是唯一信任根，证书照常校验主机名、叶证书要有服务端认证用途（2026-11-12 修订，ADR-0201；以前只核对证书链，集群 CA 签发的任何证书都能冒充 API Server）；存活时间按秒向上取整写入，API Server 不可达 → `SCHEDULING_UNAVAILABLE_002`）、`Scheduling.Database`（租约行，并发戳 CAS 接管）。

## Provider

- `Scheduling.InMemory`（`role: default`）：`InMemoryJobStore`（进程内，历史上限 `XiHan:Scheduling:InMemory:MaxRuns` 默认 1000，超出丢弃最旧）、进程内领导选举、装配引擎。单进程；多实例部署请换数据库实现，或保留内存存储并换 `Scheduling.Kubernetes` 的领导选举（此时集群唯一的作业只在领导者上运行）。
- `Scheduling.Database`（`role: provider`，依赖 `persistence.relational@1`）：作业、触发器、运行、节点、租约五张表经仓储读写，每个存储操作在自己的管道调用（Transaction 阶段）里提交，抢占与租约接管依赖并发戳 CAS；唯一的例外是租约的释放：它发生在应用不在服务状态的时候（排空中的周期、调度器停止），管道已拒绝调用，所以不经管道，在根提供者的新作用域里开可写工作单元删行后提交（ADR-0091）；节点 ID `XiHan:Scheduling:NodeId`（默认机器名 + 进程 ID + 随机）。多实例部署下作业在节点间分发：每条到期触发器由先抢到的节点运行，不允许并发的作业全集群串行。

引擎选项（`XiHan:Scheduling`）：`PollIntervalSeconds`（1）、`NodeId`、`MaxConcurrentRuns`（32）、`AcquireBatchSize`（16，一个周期最多抢占的触发器数）、`MisfireThresholdSeconds`（60）、`LeaseResource`（`scheduling.jobs`）、`LeaseTimeToLiveSeconds`（30）、`NodeTimeoutSeconds`（90）、`HousekeepingIntervalSeconds`（60）、`HistoryRetentionDays`（7）。

## 一致性

Provider 必须继承 `JobSchedulerConformance`（触发与状态、间隔与 cron 计划、并发、失败、重试与执行时限、动态登记与删除、暂停与恢复、misfire、多触发器与优先级、历史、取消、本地作业）、`JobStoreConformance`（存储的查询、抢占原子性、独占运行、阻塞与释放、恢复）与 `LeaderElectionConformance` 并通过全部用例。调度器用例由用例驱动时间：套件给宿主登记一个只由用例推进的时钟，引擎的到期判定、领导租约、作业的停留与执行时限都按它计时；Provider 把轮询周期设得很长、后台循环不动作，用例推进时钟后经 `JobSchedulerEngine.PollOnceAsync`（执行一个周期，并等本周期在本节点起的运行连同收尾结束）轮询一次再断言，不按墙钟等待。选举的过期用例同样按套件传入的时钟推进。`LeaderElectionConformance` 另有应用生命周期用例（2026-11-02 修订，ADR-0094）：Provider 实现 `StartNodeAsync(clock, probe)`，起一个调度引擎用被测选举的节点（套件的时钟、很长的轮询周期、装配套件给的探针特性——它依赖 `scheduling.jobs`，在调度之后启动、之前停止），并给出与节点争同一租约、全程在服务的竞争者（数据库实现是同一个库上的另一个节点，进程内实现是节点自己的选举）；用例在探针的启动钩子里（应用启动中）执行一个周期，断言节点不拿租约；进入服务后竞争者先拿到、节点这个周期拿不到，竞争者放手后节点下一个周期拿到、竞争者被拒；在探针的停止钩子里（应用排空中）时钟只走半个租约，节点这个周期释放，竞争者立即拿到——Provider 的 `Release` 在应用不在服务状态时必须真正生效。内存、数据库与 Kubernetes 实现都通过。结果与机器快慢、负载无关（ADR-0074）。

向量（档 `scheduling`，`conformance/vectors/scheduling/`；2026-11-24，ADR-0277，以前这个档没有向量）：

| 文件 | 内容 |
| --- | --- |
| `job-names.json` | 作业名、触发器名与领导选举资源名的文法，含 128 个字符的上限两侧 |
| `cron-expressions.json` | cron 表达式与从起点依次推算的触发时刻：各种写法（含名字的大小写、分隔符、单个值带步长、步长与年份的上限两侧、`L-n`）、时区、夏令时的跳过与重复、搜索上限的两侧；必须拒绝的表达式 |
| `retry-delays.json` | 重试前的等待（固定间隔、指数退避、上限与 30 天）与重试策略的取值范围 |
| `event-topics.json` | 调度发布的 9 个事件主题与信封的 `source` |
| `schedule-texts.json` | 计划的文本形式：各种类与选项的组合、间隔与次数的上限两侧、时刻的各种写法与写回的文本；必须拒绝的分隔、次序、重复与写法（2026-11-25，ADR-0297）。时区只用不依赖时区库的 `UTC` |

cron 的细则已经写明并收进向量（2026-11-24，ADR-0286；ADR-0277 补向量时这些正文没有写，没有收）。计划的文本形式写成文法后收进向量（2026-11-25，ADR-0297；ADR-0277 补向量时正文只给了例子，没有收）。向量不收的：misfire、抢占名额、运行数据的逐层覆盖与日历（要有存储与时钟，由 `JobSchedulerConformance` 等核对）。

## 错误码

见 `conventions/error-codes.md` 中 `SCHEDULING_*`。
