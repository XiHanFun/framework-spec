# RFC-0012 对象存储

> 状态：已采纳（2026-10-26 修订：S3 Provider；2026-10-29 修订：范围读取、分片上传与预签名；2026-11-01 修订：S3 绑定类型改为 blob；2026-11-01 修订：命名实例总是登记、缺省实例的三个契约同一存储；2026-11-01 修订：同一存储只约束被要求的契约；2026-11-01 修订：列举的页大小越界返回校验错误；2026-11-01 修订：一致性验证改对 RustFS；2026-11-01 修订：不覆盖写入并发原子、S3 探测条件写入；2026-11-01 修订：条件写入探测改为启动照常、后台与首次使用时重试，键的保留前缀 `.xihan`，云厂商的兼容服务只走标准条件头；2026-11-02 修订：条件写入探测被拒绝、探测桶不存在恢复为启动失败，只有连不上照常启动、后台重试；2026-11-04 修订：基础设施失败的 `reason` 只写异常类型；同日再修订：S3 命名实例的客户端过出站地址策略） | 日期：2026-10-23 | 对应 ADR：ADR-0018（S3）、ADR-0049（范围读取、分片上传与预签名）、ADR-0050（命名存储实例）、ADR-0075（登记规则）、ADR-0082（被要求的契约）、ADR-0086（验证用的 S3 兼容服务）、ADR-0089（条件写入探测）、ADR-0090（探测重试、保留前缀）、ADR-0093（被拒绝与缺桶启动失败）、ADR-0097（启动之后被拒绝或缺桶的处理已定）、ADR-0115 | 参考实现：`XiHan.Framework.ObjectStorage.Abstractions`、`XiHan.Framework.ObjectStorage.FileSystem`、`XiHan.Framework.ObjectStorage.S3`、`XiHan.Framework.Hosting.AspNetCore`（宿主代收发的预签名） | 一致性：`XiHan.Framework.ObjectStorage.Conformance`

## 摘要

对象存储子系统只定义一个字节流级的 blob 契约：容器里按键存放不可变的字节与元数据。写入与删除以批量为主（单条是批量的特例），读取按键（可只读一段），列举按前缀分页。Provider 实现六个操作；分片上传与预签名是另两个可选能力。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `objectstorage.blob@1` | 容器 / 键 / 字节 / 元数据的对象存储，`IBlobStore` | `ObjectStorage.FileSystem`（本地目录，带配额） |
| `objectstorage.multipart@1` | 分片上传，`IBlobMultipartStore` | `ObjectStorage.FileSystem`（暂存目录）、`ObjectStorage.S3`（原生） |
| `objectstorage.presign@1` | 预签名地址，`IBlobUrlSigner` | `ObjectStorage.S3`（原生）；文件系统经 `Hosting.AspNetCore` 的宿主代收发 |

## 标识

- 容器 `ContainerName`：`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`（与 S3 桶名兼容）。
- 键 `BlobKey`：`/` 分段的路径，1 到 1024 个字符；每段非空、不是 `.` 或 `..`；不含空白、控制字符与 `\`；区分大小写。
- 保留前缀（2026-11-01 增补，ADR-0090）：首段是 `.xihan` 的键（`.xihan` 本身与 `.xihan/…`，区分大小写，`BlobKey.ReservedSegment`）留给框架自己的对象（如 S3 的条件写入探测键），不在键文法之内：构造时抛出 `ArgumentException`，`BlobKey.IsValid` 为假，业务写不进、读不到、删不掉；各 Provider 列举时跳过存储里已有的这类对象（框架经底层客户端写入的，或从外部放入的），不出现在业务列举里。查询前缀 `BlobQuery.Prefix` 只是筛选，不受此限，以 `.xihan` 开头的前缀列举总是空的。
- 自定义元数据名：`^[a-z0-9][a-z0-9-]{0,63}$`；值为任意字符串。
- 标识不合法在构造时抛出 `ArgumentException`（编程错误），不进入 Provider。

## 契约

| 操作 | 语义 |
| --- | --- |
| `Put(container, uploads[])` | 按顺序逐项写入；每项带键、内容流与选项（内容类型、自定义元数据、是否允许覆盖）；返回逐项结果（元数据或错误）。空列表返回空；重复键按顺序各执行一次 |
| `Get(container, key)` | 打开内容流与元数据；不存在为 `OBJECTSTORAGE_NOTFOUND_001` |
| `Get(container, key, range)` | 只读一段字节（见下文「范围读取、分片上传与预签名」） |
| `Head(container, key)` | 只取元数据 |
| `Delete(container, keys[])` | 按顺序逐项删除；返回逐项结果（是否存在，或错误）；重复键第二次报告不存在 |
| `List(container, query)` | 按前缀列举，键的序数顺序，`PageSize`（1–1000，默认 1000）分页，`ContinuationToken` 不透明；`PageSize` 越界不在构造时抛出，返回 `OBJECTSTORAGE_VALIDATION_005`（HTTP 400，参数 `pageSize`、`max`），不访问存储。不符合键文法的对象（保留前缀下的、S3 的目录占位）跳过，所以一页可能少于 `PageSize` 条、甚至为空而仍带续页标记，调用方取到续页标记为空为止 |

- 元数据 `BlobMetadata`：键、长度、内容类型（默认 `application/octet-stream`）、`ETag`（内容变化即变化）、最后修改时间、自定义元数据。
- `Overwrite = false` 且已存在 → `OBJECTSTORAGE_CONFLICT_001`，不写入。不覆盖写入必须原子：多个写入方同时以 `Overwrite = false` 写同一个新键，恰好一个成功，其余都是 `OBJECTSTORAGE_CONFLICT_001`，不能先查是否存在再写（并发时后写的会静默覆盖先写的）；做不到的存储不能声称支持不覆盖写入（S3 见下文「S3 的条件写入探测」）。
- 容器按需创建（文件系统）或由 Provider 的基础设施预先存在（S3 桶）；契约不管理容器。
- 基础设施失败（IO、网络）→ `OBJECTSTORAGE_UNAVAILABLE_001`，`reason` 只写异常类型的全名，完整异常进日志（2026-11-04，ADR-0115）：IO 异常的消息常带服务器上的完整路径；配额或单对象大小超限 → `OBJECTSTORAGE_UNAVAILABLE_002`，不写入。
- 批量操作在两项之间检查取消；已完成的项不回滚。
- 写入是原子的：读取方要么看到旧对象，要么看到完整的新对象。

单条便捷方法（`BlobStoreExtensions`）：`Put(container, key, stream | bytes, options)`、`Delete(container, key)`、`Exists`、`ReadAllBytes`，都转调批量方法。

## 默认实现

`ObjectStorage.FileSystem`：根目录（默认 `<AppContext.BaseDirectory>/blobs`）下 `<container>/data/<key>` 存内容、`<container>/meta/<key>.json` 存元数据；写入先落临时文件再原子替换，不覆盖的写入（含分片上传的完成）以不替换的移动原子占位，目标已存在即 `OBJECTSTORAGE_CONFLICT_001`，多个写入方（含共享目录的多个进程）只有一个成功；`ETag` 是内容的 SHA-256；配额 `MaxTotalBytes`（默认 1 GiB）与 `MaxBlobBytes`（默认 256 MiB），启动时扫描目录得到已用量，之后在进程内计量；单进程正确，多进程共享目录时配额只是近似。

## S3 Provider

`ObjectStorage.S3`：容器即桶，键即对象键，经 AWS SDK 访问 Amazon S3 与兼容服务（MinIO、阿里云 OSS、腾讯云 COS、Cloudflare R2 等），不再为每家云单独做包。

- 云厂商的兼容服务（2026-11-01 增补，ADR-0090）：阿里云 OSS、腾讯云 COS、Cloudflare R2 等同样只用这一个包，不为各家的私有请求头（如 OSS 的 `x-oss-forbid-overwrite`、COS 的 `x-cos-forbid-overwrite`）做适配，不覆盖写入只认标准的 `If-None-Match: *`。这几家是否认它框架没有实测，接入前要实测：缺省的条件写入探测就是实测（下文「S3 的条件写入探测」），不认时启动失败（`OBJECTSTORAGE_UNAVAILABLE_003`）或之后在健康项上报告；也可以按探测的做法用 curl 发两次带 `If-None-Match: *` 的 `PutObject`，第二次必须 412。不认就设 `ConditionalWrites = false`，后果是 `Overwrite = false` 的写入与分片上传的发起、完成一律 `OBJECTSTORAGE_UNAVAILABLE_003`，"只在不存在时写入"不再可用——需要它的业务改用不会冲突的键（如带随机后缀）或改为覆盖写入，框架不退回先查再写。
- `Overwrite = false` 先 `Head`（已存在就不必上传内容）再带 `If-None-Match: *` 写入，由服务端保证原子；已存在 → `OBJECTSTORAGE_CONFLICT_001`。服务是否认条件写入要先探明（见下文「S3 的条件写入探测」）：探明之前不覆盖写入返回不可用，探明不认的服务（如 Garage、Zenko CloudServer）上不覆盖写入一律报错，启动时就探明的启动失败，任何时候都不带着可能静默覆盖的不覆盖写入运行。不可定位的内容流先落临时文件取得长度，不整体装进内存。
- 批量删除：S3 不报告对象是否存在过，先逐键 `Head` 定存在性（同一批里重复的键只有第一次算存在），再把存在的键按每批 1000 个 `DeleteObjects`；删除失败只影响本次负责删除的项。
- 列举经 `ListObjectsV2`（S3 保证键的序数顺序），续页标记是其 continuation token；条目只有键、长度、`ETag` 与修改时间，内容类型为默认值、自定义元数据为空，需要时再 `Head`——列举条目的元数据因此是"可能不完整"的，一致性套件只校验键顺序、长度与 `ETag`。不符合键文法的对象（以 `/` 结尾的"目录"占位）跳过。
- 不存在的键与桶 → `OBJECTSTORAGE_NOTFOUND_001`；桶不存在的列举是空页；其余服务端错误、网络错误与 SDK 超时 → `OBJECTSTORAGE_UNAVAILABLE_001`，`reason` 同样只写异常类型的全名（2026-11-04，ADR-0115）；批量删除里单个键的失败只写 S3 的错误码。没有配额（由桶策略管理）。`EnsureBuckets` 开启时写入遇到桶不存在就建桶再重试一次，默认由基础设施预先建桶。
- 配置 `XiHan:ObjectStorage:S3`（`ServiceUrl`、`Region`、`AccessKey`、`SecretKey`、`ForcePathStyle`、`EnsureBuckets`、`HealthBucket`、`ConditionalWrites`、`HealthProbeIntervalSeconds`），绑定 `objectstorage`（类型 `blob`，与 RFC-0016 的绑定类型词表一致）的主值与属性覆盖之（RFC-0016「资源绑定与配置节」）；没有密钥时走 SDK 默认凭据链；应用自己注册的 `IAmazonS3` 优先。

## S3 的条件写入探测（2026-11-01 增补，ADR-0089；同日修订，ADR-0090；2026-11-02 修订，ADR-0093）

不覆盖写入在 S3 上靠条件写入（`If-None-Match: *`）保证原子；不认它的兼容服务把条件头当作没有，先 `Head` 再写的两个写入方都会成功、后写的静默覆盖先写的（ADR-0089 实测：Garage、Zenko CloudServer 上 8 个并发的不覆盖写入全部"成功"）。S3 Provider 因此在不覆盖写入之前探明服务是否认条件写入：

- 设置 `ConditionalWrites`（配置节与命名实例的设置里都有，缺省 `true`）。开启时探测，`HealthBucket` 必须配置（探测写在这个桶里），没配是配置错误：缺省实例装配失败（`ArgumentException`），命名实例 → `OBJECTSTORAGE_VALIDATION_003`。
- 探测：对固定的探测键 `.xihan/probe/conditional-write`（保留前缀 `.xihan` 下，见「标识」，业务列举不会出现；空对象）带 `If-None-Match: *` 写入；成功则再写一次，第二次必须被拒（412），还成功就是不认。提供分片上传时（缺省实例被选为 `objectstorage.multipart`；命名实例总是提供）再以同一个键发起分片上传、上传一片、带 `If-None-Match: *` 完成，必须被拒（412），被拒的上传随即放弃；完成了就是不认（探测键被同样为空的内容覆盖，无害）。单次探测以 `HealthProbeIntervalSeconds`（缺省 15 秒，命名实例取缺省值）为超时，按注入的时钟计时。
- 探测键写入后不删除：任何一次 412 都证明服务认得条件写入，键已存在时第一次写入就被拒，所以多个实例同时启动、反复启动、后台重试都得到同一结论，没有"删掉后别人又写入"的竞态；凭据只需要对这一个键的写入（`s3:PutObject`）与放弃分片上传（`s3:AbortMultipartUpload`）的权限，不需要删除、读取或列举；桶里只多一个空对象，在保留前缀下，业务列举跳过它。`EnsureBuckets` 开启时探测桶不存在就建桶后重试一次（同写入）。
- 一次探测的结论：通过（"已确认"）；探明不认（第二次写入或完成没被拒，或服务回 501，`OBJECTSTORAGE_UNAVAILABLE_003`）；其余都是"尚未确认"（服务认不认条件写入还不知道），按原因分两类——配置错误：凭据无效或无权写探测键、建桶、放弃上传（403，`OBJECTSTORAGE_DENIED_002`，参数 `bucket`、`key`、`operation`、`reason` 为服务端错误码）、探测桶不存在（`OBJECTSTORAGE_NOTFOUND_001`，参数 `bucket`）；连不上：连不上、超时或其余服务端错误（`OBJECTSTORAGE_UNAVAILABLE_001`，`reason` 带出是哪一步）。已确认与不认是终态，服务的能力不会在运行中改变；尚未确认会重试。
- 时机：缺省实例在特性启动时探一次，与健康项的首次探测并行（S3 连不上时两者都要等到超时，并行让启动只等一个超时）；命名实例在建出时探一次（每个实例各探各的，实例指向的服务可能不同）。
- 启动或建实例时的结论（2026-11-02 修订，ADR-0093）：探明不认、被拒绝、探测桶不存在都是配置问题，要早暴露 → 缺省实例以 `KERNEL_HOSTING_002`（内层分别为 `OBJECTSTORAGE_UNAVAILABLE_003`、`OBJECTSTORAGE_DENIED_002`、`OBJECTSTORAGE_NOTFOUND_001`，参数同上）启动失败；命名实例不建、不登记（Provider 抛出携带该码的 `FrameworkException`，注册表原样返回该码，下次取用重新探测）。只有连不上（`OBJECTSTORAGE_UNAVAILABLE_001`）→ 照常启动、照常建出并登记实例，在后台重试：第一次 1 秒后，之后每次间隔加倍，最长 `HealthProbeIntervalSeconds`（它小于 1 秒时每次都取它），计时用注入的时钟；有了终态即停止，应用停止或实例释放时也停止；S3 修好（服务就绪、网络恢复）后自动确认，不必重启。`EnsureBuckets` 开启时缺桶先建（同写入），建桶被拒仍是被拒绝。经公开构造函数直接建的 `S3BlobStore` 不探测，由调用方负责。
- 尚未确认时的不覆盖写入（`Overwrite = false` 的写入、分片上传的发起与完成）：有探测正在进行就等它的结论（受调用方取消约束）；没有而距上一次探测结束已满 1 秒，就地探测一次（同一时刻每个实例只有一个探测，后台重试与写入共用它）；不满 1 秒不再探测。之后已确认就照常写入；不认 → `OBJECTSTORAGE_UNAVAILABLE_003`；仍未确认 → `OBJECTSTORAGE_UNAVAILABLE_004`（HTTP 503，参数 `container`、`key`、`reason` 为最近一次探测失败的错误码），不访问写入目标，不退回先查再写。覆盖写入、读取、删除、列举、预签名不受影响。
- 启动之后（后台重试或写入时的探测）才探明不认：应用不停止（已经在服务），此后不覆盖写入一律 `OBJECTSTORAGE_UNAVAILABLE_003`，记一条 Error 日志；换成认条件写入的服务，或关闭条件写入后重启才恢复。
- 启动之后才遇到被拒绝或探测桶不存在（2026-11-02 增补，ADR-0093；只会发生在启动或建实例时连不上、尚未确认的实例上，例如连上时凭据已被吊销、桶已被删除，或配置本来就错而启动时探不到）：应用不停止，状态仍是尚未确认，后台照常按退避重试，权限改好或桶建好后自动确认、不必重启；不覆盖写入仍是 `OBJECTSTORAGE_UNAVAILABLE_004`（HTTP 503，`reason` 为 `OBJECTSTORAGE_DENIED_002` 或 `OBJECTSTORAGE_NOTFOUND_001`），不访问写入目标——这次写入的调用方没有过错，不以 403 / 404 回给它；缺省实例的健康项 Failed（配置错误要人处理，与连不上的 Degraded 区分开），错误为该码；原因变化时记一条 Error 日志（连不上是 Warning），稳定的失败不随每次重试刷屏。已定（2026-11-02，ADR-0097）：维持继续重试、缺省实例健康项 Failed，不改为停止重试（修好要重启）或只记 Degraded（看不出要人处理）；命名实例的健康项按注册表的规则一律 Degraded（RFC 下文「命名存储实例」），不为此改注册表。已确认之后凭据被吊销或桶被删除不在此列：已确认是终态、不再探测，此后的读写照常访问存储，按「S3 Provider」的错误映射失败（被拒等服务端错误 → `OBJECTSTORAGE_UNAVAILABLE_001`），连通性的 `Head` 失败使健康项 Degraded。
- 健康：缺省实例的健康项 `objectstorage.s3.connection` 合并连通性（对 `HealthBucket` 的 `Head`）与条件写入的状态，数据 `conditionalWrites` 为 `confirmed` / `unconfirmed` / `unsupported` / `disabled`：尚未确认时错误是最近一次探测失败，即便 `Head` 成功——连不上（`OBJECTSTORAGE_UNAVAILABLE_001`）为 Degraded，被拒绝或探测桶不存在（`OBJECTSTORAGE_DENIED_002` / `NOTFOUND_001`，只在启动之后才会出现）为 Failed；探明不认时 Failed，错误为 `OBJECTSTORAGE_UNAVAILABLE_003`；状态随探测结论即时变化，不等下一轮健康探测；Failed 的条目让应用的聚合状态为 Degraded，应用仍就绪。命名实例的健康项 `objectstorage.store.<name>` 在连通之后同样看条件写入：尚未确认或不认时 Degraded（注册表把探测失败一律记为 Degraded），错误同上。
- 关闭（`ConditionalWrites = false`）表示应用明确不需要不覆盖写入（只覆盖写入，或凭据只读，或服务实测不认条件写入）：不探测，`HealthBucket` 可以不配（健康项列桶），`Overwrite = false` 的写入、分片上传的发起与完成一律 → `OBJECTSTORAGE_UNAVAILABLE_003`，不访问存储——不做没有保证的先查再写；覆盖写入、读取、删除、列举、预签名不受影响。

## 范围读取、分片上传与预签名（2026-10-29 增补，ADR-0049）

大文件直传、断点续传、音视频拖动与限时下载链接（4.x 的预签名下载，缺省 30 分钟）。

- 范围读取（`objectstorage.blob@1` 的一部分）：`Get(container, key, range)`，范围是起点与可选长度；长度为空读到末尾，超出末尾截断；返回的内容带实际范围（起点、长度、总长），元数据长度仍是总长。起点不在对象内（含空对象）→ `OBJECTSTORAGE_VALIDATION_001`。
- 分片上传（能力 `objectstorage.multipart@1`，`IBlobMultipartStore`）：`Start(container, key, options)` 得到上传标识（不透明）；`UploadPart(upload, n, stream)` 上传编号 1–10000 的一片，可乱序、可重传（后一次覆盖），返回该片 ETag；`ListParts(upload)` 按编号升序列出已上传的分片（续传据此跳过）；`Complete(upload, parts)` 按编号严格升序、ETag 与已上传的一致的清单拼成对象，除最后一片外每片至少 5 MiB；`Abort(upload)` 丢弃，上传不存在也成功。完成前对象不可见；完成是原子的。
  - 清单不合法（为空、乱序、未上传或 ETag 不符、非末片过小）→ `OBJECTSTORAGE_VALIDATION_002`，上传保留可再完成；上传不存在或已结束 → `OBJECTSTORAGE_NOTFOUND_002`；`Overwrite = false` 时对象已存在，发起与完成都 → `OBJECTSTORAGE_CONFLICT_001`。
- 预签名（能力 `objectstorage.presign@1`，`IBlobUrlSigner`）：签出限时地址，客户端不带凭据直接下载（`GET`，支持 `Range`，可带保存的文件名）、上传整个对象（`PUT`，可限定内容类型）或上传分片上传的某一片（`PUT`，响应 `ETag` 即分片 ETag）。有效期缺省 30 分钟，1 秒到 7 天。签出的地址带客户端必须发送的请求头。签名是本地计算，不检查对象是否存在。
  - 原生支持的存储（S3）由存储签名、客户端直连存储。
  - 不支持的存储（文件系统）由宿主代收发：地址指向宿主的端点，签名是对方法、容器、键、过期时刻与各限定项的 HMAC-SHA256，无状态，同一密钥的多个实例互认；端点验签后经存储收发，签名不符或过期 → `OBJECTSTORAGE_DENIED_001`（HTTP 403），范围不可满足回 416。
- 文件系统：范围读取定位后只交出该段；分片暂存在容器的 `uploads/<上传标识>/`，分片文件名带内容的 SHA-256，完成时拼接成同目录临时文件再原子替换并计入配额；暂存的分片不计入配额，单片与拼接后的对象受单对象上限约束；超过 `MultipartExpiry`（默认 7 天）未完成的上传在启动扫描时清掉。
- S3：范围读取经 `Range` 头；分片直接映射 S3 multipart（不覆盖的完成先 `Head` 再带 `If-None-Match: *`，服务认条件完成要先由探测确认：尚未确认时不覆盖的发起与完成 → `OBJECTSTORAGE_UNAVAILABLE_004`，探明不认或关闭条件写入时 → `OBJECTSTORAGE_UNAVAILABLE_003`）；预签名经 SDK（签名 V4），兼容服务的端点是 http 时按 http 签。

## 命名存储实例（2026-10-29 增补，ADR-0050）

一个应用同时用多个存储：按租户或按用途各配一个桶、各有自己的端点与密钥（4.x 的存储配置），定义可以写在配置里，也可以在运行时来自应用自己的库。

- 定义 `BlobStoreDefinition`：实例名（`^[a-z0-9][a-z0-9-]{0,63}$`）、Provider 名（`filesystem`、`s3`，不区分大小写）与设置。设置的键同该 Provider 的配置节，不区分大小写；未知的键是错误，拼错的键不会被静默忽略。
- Provider `IBlobStoreProvider`：每个存储包一个，按定义建实例、探测实例。文件系统的命名实例必须给 `RootPath`（不给就会与缺省实例落在同一目录）；S3 的命名实例各建各的客户端，不读连接绑定，探测间隔不在设置里；`ConditionalWrites` 开启（缺省）时建实例时探测条件写入，探明不认、被拒绝或探测桶不存在都不建，只有连不上的照常建出、在后台重试（「S3 的条件写入探测」）。
- 命名实例的端点可能来自应用的库，不可信（2026-11-04 修订，ADR-0117）：S3 命名实例的客户端过出站地址策略（RFC-0008），自建存储的地址要放行；被拒绝与连不上同样处理，建实例时的探测只算尚未确认，读写返回 `OBJECTSTORAGE_UNAVAILABLE_001`。缺省实例的端点是运维配的，不过策略。
- 登记规则（2026-11-01 修订，ADR-0075）：存储包只要装配（`UseFeature`）就登记自己的命名实例 Provider，与它是否被选为缺省实例无关。能力全部被别的存储顶替的存储包实现 `IShadowAwareFeature`（RFC-0002 解析规则 4），被顶替时只登记命名实例 Provider：不读自己的配置节与连接绑定、不建缺省实例、不启动。
- 注册表 `BlobStores`（单例；装配任一个存储包即有）：
  - `Get(name)`：已建的直接返回；配置节 `XiHan:ObjectStorage:Stores:<name>`（`Provider` 加设置）里有定义的首次取用时建出；都没有 → `OBJECTSTORAGE_NOTFOUND_003`。
  - `GetOrAdd(definition)`：同名同定义（Provider 名与设置完全相同）返回同一实例；定义变了建新实例替换并释放旧的，替换时仍在旧实例上的调用可能失败。Provider 未登记或设置不合法 → `OBJECTSTORAGE_VALIDATION_003`，后端此刻建不出来（目录无权限、磁盘错误）→ `OBJECTSTORAGE_UNAVAILABLE_001`，建实例时的探测发现实例不能用 → Provider 给出的错误码（Provider 抛出携带错误码的 `FrameworkException`，注册表转成同码的失败结果；S3 在探明不认条件写入、探测被拒绝或探测桶不存在时如此，连不上的照常建出，见「S3 的条件写入探测」）；这些情况已有的同名实例都不受影响。
  - `Remove(name)` 移除并释放；应用停止时释放全部。
  - 缺省的 `IBlobStore` 注入方式不变，命名实例与缺省实例互不相干。
- 缺省实例的三个契约落在同一存储上（2026-11-01 修订，ADR-0075；同日再修订，ADR-0082：只约束被要求的契约）。存储包在 `Configure` 开头按自己被选中的能力判定（`BlobStoreSelection.ProvidesDefaultInstance`）：
  - 被选为 `objectstorage.blob`：它就是缺省存储，照常为被选中的各项能力注册契约（兜底选出的也注册）。
  - 没被选为 `objectstorage.blob`，却被要求提供 `objectstorage.multipart` 或 `objectstorage.presign`（RFC-0002 解析规则 4：有清单依赖这项能力，或 `XiHan:Providers:<能力ID>` / `UseProvider` 指定了它）：缺省存储是别的存储，它签出的地址、建的分片上传会指向另一个存储里的对象，装配时抛出携带 `OBJECTSTORAGE_VALIDATION_004` 的 `FrameworkException`，应用启动失败，消息列出被要求的能力与要改的 `XiHan:Providers` 键；框架不替应用改选。
  - 没被选为 `objectstorage.blob`，分片上传与预签名也都只是兜底选出（没人依赖、没人指定）：它不提供缺省实例，按被顶替处理（`ConfigureShadowed`：只登记命名实例 Provider，不读配置节与连接绑定、不建缺省实例、不注册这两个契约、不启动）。这时容器里没有缺省的 `IBlobMultipartStore` / `IBlobUrlSigner`，要用就在清单里依赖对应能力或指定 Provider，由上一条核对。
  - 宿主代收发（`xihan.hosting.blob-urls`）经缺省 `IBlobStore` 收发，不是存储包，不受此约束。
  - 混装文件系统与 S3、以文件系统为缺省实例：只指定 `objectstorage.blob` = `xihan.objectstorage.filesystem` 即可，S3 的分片上传与预签名是兜底选出，S3 只登记命名实例，不必装 `HostedBlobUrlFeature`；要缺省的分片上传，再指定 `objectstorage.multipart` = `xihan.objectstorage.filesystem`；要缺省的预签名，装配 `HostedBlobUrlFeature` 并指定 `objectstorage.presign` = `xihan.hosting.blob-urls`（S3 与它都是显式 Provider，不指定本来就是 `KERNEL_CAPABILITY_004`）。有特性依赖 `objectstorage.presign` 或 `objectstorage.multipart`（例如应用的文件模块）而没指定时，这项能力解析到 S3 且是被要求的，仍然 `OBJECTSTORAGE_VALIDATION_004`。都不指定时 S3 是显式 Provider，三项都归 S3，文件系统被顶替。
- 命名实例的能力随实例本身：S3 实例同时是 `IBlobMultipartStore` 与 `IBlobUrlSigner`（直连存储）；文件系统实例实现分片，预签名由宿主代收发——宿主签名器的 `ForStore(name)` 签出的地址带 `x-store=<name>`，实例名在签名里（改成别的实例或去掉即签名不符），端点经注册表取实例；签名有效但实例不存在 → `OBJECTSTORAGE_NOTFOUND_003`（HTTP 404）。
- 健康：每个实例一条 `objectstorage.store.<name>`，按 `XiHan:ObjectStorage:StoreHealthProbeIntervalSeconds`（缺省 15 秒）在后台逐个探测，新实例建出后立即探一次；尚未探过为 Started，探测失败为 Degraded（实例仍可取用）。文件系统看总配额是否还有余量，S3 对实例的 `HealthBucket` 做 `Head`（没配时列桶），连通之后再看条件写入是否已确认（「S3 的条件写入探测」）。条目名随实例而定，存储包的清单不列出它们。

## 一致性

Provider 必须继承 `BlobStoreConformance` 并通过全部用例（S3 Provider 对 RustFS 1.0.4 通过，此前对 MinIO 通过；ADR-0086 记录了其余兼容服务的差异）。用例包括并发的不覆盖写入（8 个写入方同时写同一个新键，恰好一个成功，ADR-0089）：先查再写的实现过不了，不认条件写入的 S3 兼容服务上直接建的 `S3BlobStore` 也过不了（Garage、Zenko CloudServer 上 8 个全部成功），这正是 S3 Provider 探明之前不做不覆盖写入、探明不认就拒绝的原因；以及保留前缀（ADR-0090）：首段是 `.xihan` 的键构造时被拒，套件经 Provider 实现的钩子 `PlantOutsideContractAsync` 绕过契约在存储里放入保留前缀下的对象，列举必须跳过它们；实现了 `IBlobMultipartStore` 的 Provider 同样要通过分片用例（不实现时这些用例跳过）。命名实例的 Provider 必须继承 `BlobStoreProviderConformance` 并通过全部用例（参考实现另有混装用例：两种装配顺序下两个存储包的命名实例 Provider 都登记，缺省的三个契约同一存储，被要求的契约选得不一致时装配失败、兜底选出的不失败而按被顶替处理）：按定义建出可读写且探测通过的实例、未知的设置键被拒绝、经注册表同定义复用 / 定义变了替换 / 移除后不存在、定义错误翻译为 `OBJECTSTORAGE_VALIDATION_003` 且不影响已有实例。

## 错误码

见 `conventions/error-codes.md` 中 `OBJECTSTORAGE_*`。
