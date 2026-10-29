# RFC-0012 对象存储

> 状态：已采纳（2026-10-26 修订：S3 Provider；2026-10-29 修订：范围读取、分片上传与预签名） | 日期：2026-10-23 | 对应 ADR：ADR-0018（S3）、ADR-0049（范围读取、分片上传与预签名） | 参考实现：`XiHan.Framework.ObjectStorage.Abstractions`、`XiHan.Framework.ObjectStorage.FileSystem`、`XiHan.Framework.ObjectStorage.S3`、`XiHan.Framework.Hosting.AspNetCore`（宿主代收发的预签名） | 一致性：`XiHan.Framework.ObjectStorage.Conformance`

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
| `List(container, query)` | 按前缀列举，键的序数顺序，`PageSize`（1–1000，默认 1000）分页，`ContinuationToken` 不透明 |

- 元数据 `BlobMetadata`：键、长度、内容类型（默认 `application/octet-stream`）、`ETag`（内容变化即变化）、最后修改时间、自定义元数据。
- `Overwrite = false` 且已存在 → `OBJECTSTORAGE_CONFLICT_001`，不写入。
- 容器按需创建（文件系统）或由 Provider 的基础设施预先存在（S3 桶）；契约不管理容器。
- 基础设施失败（IO、网络）→ `OBJECTSTORAGE_UNAVAILABLE_001`；配额或单对象大小超限 → `OBJECTSTORAGE_UNAVAILABLE_002`，不写入。
- 批量操作在两项之间检查取消；已完成的项不回滚。
- 写入是原子的：读取方要么看到旧对象，要么看到完整的新对象。

单条便捷方法（`BlobStoreExtensions`）：`Put(container, key, stream | bytes, options)`、`Delete(container, key)`、`Exists`、`ReadAllBytes`，都转调批量方法。

## 默认实现

`ObjectStorage.FileSystem`：根目录（默认 `<AppContext.BaseDirectory>/blobs`）下 `<container>/data/<key>` 存内容、`<container>/meta/<key>.json` 存元数据；写入先落临时文件再原子替换；`ETag` 是内容的 SHA-256；配额 `MaxTotalBytes`（默认 1 GiB）与 `MaxBlobBytes`（默认 256 MiB），启动时扫描目录得到已用量，之后在进程内计量；单进程正确，多进程共享目录时配额只是近似。

## S3 Provider

`ObjectStorage.S3`：容器即桶，键即对象键，经 AWS SDK 访问 Amazon S3 与兼容服务（MinIO、阿里云 OSS、腾讯云 COS、Cloudflare R2 等），不再为每家云单独做包。

- `Overwrite = false` 先 `Head` 再带 `If-None-Match: *` 写入，服务端支持条件写入（Amazon S3、MinIO）时由它保证原子；已存在 → `OBJECTSTORAGE_CONFLICT_001`。不可定位的内容流先落临时文件取得长度，不整体装进内存。
- 批量删除：S3 不报告对象是否存在过，先逐键 `Head` 定存在性（同一批里重复的键只有第一次算存在），再把存在的键按每批 1000 个 `DeleteObjects`；删除失败只影响本次负责删除的项。
- 列举经 `ListObjectsV2`（S3 保证键的序数顺序），续页标记是其 continuation token；条目只有键、长度、`ETag` 与修改时间，内容类型为默认值、自定义元数据为空，需要时再 `Head`——列举条目的元数据因此是"可能不完整"的，一致性套件只校验键顺序、长度与 `ETag`。不符合键文法的对象（以 `/` 结尾的"目录"占位）跳过。
- 不存在的键与桶 → `OBJECTSTORAGE_NOTFOUND_001`；桶不存在的列举是空页；其余服务端错误、网络错误与 SDK 超时 → `OBJECTSTORAGE_UNAVAILABLE_001`。没有配额（由桶策略管理）。`EnsureBuckets` 开启时写入遇到桶不存在就建桶再重试一次，默认由基础设施预先建桶。
- 配置 `XiHan:ObjectStorage:S3`（`ServiceUrl`、`Region`、`AccessKey`、`SecretKey`、`ForcePathStyle`、`EnsureBuckets`），绑定 `objectstorage`（`storage:object`）的主值与属性覆盖之；没有密钥时走 SDK 默认凭据链；应用自己注册的 `IAmazonS3` 优先。

## 范围读取、分片上传与预签名（2026-10-29 增补，ADR-0049）

大文件直传、断点续传、音视频拖动与限时下载链接（4.x 的预签名下载，缺省 30 分钟）。

- 范围读取（`objectstorage.blob@1` 的一部分）：`Get(container, key, range)`，范围是起点与可选长度；长度为空读到末尾，超出末尾截断；返回的内容带实际范围（起点、长度、总长），元数据长度仍是总长。起点不在对象内（含空对象）→ `OBJECTSTORAGE_VALIDATION_001`。
- 分片上传（能力 `objectstorage.multipart@1`，`IBlobMultipartStore`）：`Start(container, key, options)` 得到上传标识（不透明）；`UploadPart(upload, n, stream)` 上传编号 1–10000 的一片，可乱序、可重传（后一次覆盖），返回该片 ETag；`ListParts(upload)` 按编号升序列出已上传的分片（续传据此跳过）；`Complete(upload, parts)` 按编号严格升序、ETag 与已上传的一致的清单拼成对象，除最后一片外每片至少 5 MiB；`Abort(upload)` 丢弃，上传不存在也成功。完成前对象不可见；完成是原子的。
  - 清单不合法（为空、乱序、未上传或 ETag 不符、非末片过小）→ `OBJECTSTORAGE_VALIDATION_002`，上传保留可再完成；上传不存在或已结束 → `OBJECTSTORAGE_NOTFOUND_002`；`Overwrite = false` 时对象已存在，发起与完成都 → `OBJECTSTORAGE_CONFLICT_001`。
- 预签名（能力 `objectstorage.presign@1`，`IBlobUrlSigner`）：签出限时地址，客户端不带凭据直接下载（`GET`，支持 `Range`，可带保存的文件名）、上传整个对象（`PUT`，可限定内容类型）或上传分片上传的某一片（`PUT`，响应 `ETag` 即分片 ETag）。有效期缺省 30 分钟，1 秒到 7 天。签出的地址带客户端必须发送的请求头。签名是本地计算，不检查对象是否存在。
  - 原生支持的存储（S3）由存储签名、客户端直连存储。
  - 不支持的存储（文件系统）由宿主代收发：地址指向宿主的端点，签名是对方法、容器、键、过期时刻与各限定项的 HMAC-SHA256，无状态，同一密钥的多个实例互认；端点验签后经存储收发，签名不符或过期 → `OBJECTSTORAGE_DENIED_001`（HTTP 403），范围不可满足回 416。
- 文件系统：范围读取定位后只交出该段；分片暂存在容器的 `uploads/<上传标识>/`，分片文件名带内容的 SHA-256，完成时拼接成同目录临时文件再原子替换并计入配额；暂存的分片不计入配额，单片与拼接后的对象受单对象上限约束；超过 `MultipartExpiry`（默认 7 天）未完成的上传在启动扫描时清掉。
- S3：范围读取经 `Range` 头；分片直接映射 S3 multipart（不覆盖的完成先 `Head` 再带 `If-None-Match: *`）；预签名经 SDK（签名 V4），兼容服务的端点是 http 时按 http 签。

## 命名存储实例（2026-10-29 增补，ADR-0050）

一个应用同时用多个存储：按租户或按用途各配一个桶、各有自己的端点与密钥（4.x 的存储配置），定义可以写在配置里，也可以在运行时来自应用自己的库。

- 定义 `BlobStoreDefinition`：实例名（`^[a-z0-9][a-z0-9-]{0,63}$`）、Provider 名（`filesystem`、`s3`，不区分大小写）与设置。设置的键同该 Provider 的配置节，不区分大小写；未知的键是错误，拼错的键不会被静默忽略。
- Provider `IBlobStoreProvider`：每个存储包一个，按定义建实例、探测实例。文件系统的命名实例必须给 `RootPath`（不给就会与缺省实例落在同一目录）；S3 的命名实例各建各的客户端，不读连接绑定，探测间隔不在设置里。
- 注册表 `BlobStores`（单例；装配任一个存储包即有）：
  - `Get(name)`：已建的直接返回；配置节 `XiHan:ObjectStorage:Stores:<name>`（`Provider` 加设置）里有定义的首次取用时建出；都没有 → `OBJECTSTORAGE_NOTFOUND_003`。
  - `GetOrAdd(definition)`：同名同定义（Provider 名与设置完全相同）返回同一实例；定义变了建新实例替换并释放旧的，替换时仍在旧实例上的调用可能失败。Provider 未登记或设置不合法 → `OBJECTSTORAGE_VALIDATION_003`，后端此刻建不出来（目录无权限、磁盘错误）→ `OBJECTSTORAGE_UNAVAILABLE_001`；这两种情况已有的同名实例都不受影响。
  - `Remove(name)` 移除并释放；应用停止时释放全部。
  - 缺省的 `IBlobStore` 注入方式不变，命名实例与缺省实例互不相干。
- 命名实例的能力随实例本身：S3 实例同时是 `IBlobMultipartStore` 与 `IBlobUrlSigner`（直连存储）；文件系统实例实现分片，预签名由宿主代收发——宿主签名器的 `ForStore(name)` 签出的地址带 `x-store=<name>`，实例名在签名里（改成别的实例或去掉即签名不符），端点经注册表取实例；签名有效但实例不存在 → `OBJECTSTORAGE_NOTFOUND_003`（HTTP 404）。
- 健康：每个实例一条 `objectstorage.store.<name>`，按 `XiHan:ObjectStorage:StoreHealthProbeIntervalSeconds`（缺省 15 秒）在后台逐个探测，新实例建出后立即探一次；尚未探过为 Started，探测失败为 Degraded（实例仍可取用）。文件系统看总配额是否还有余量，S3 对实例的 `HealthBucket` 做 `Head`（没配时列桶）。条目名随实例而定，存储包的清单不列出它们。

## 一致性

Provider 必须继承 `BlobStoreConformance` 并通过全部用例（S3 Provider 对 MinIO 通过）；实现了 `IBlobMultipartStore` 的 Provider 同样要通过分片用例（不实现时这些用例跳过）。命名实例的 Provider 必须继承 `BlobStoreProviderConformance` 并通过全部用例：按定义建出可读写且探测通过的实例、未知的设置键被拒绝、经注册表同定义复用 / 定义变了替换 / 移除后不存在、定义错误翻译为 `OBJECTSTORAGE_VALIDATION_003` 且不影响已有实例。

## 错误码

见 `conventions/error-codes.md` 中 `OBJECTSTORAGE_*`。
