# RFC-0012 对象存储

> 状态：已采纳（2026-10-26 修订：S3 Provider；2026-10-29 修订：范围读取、分片上传与预签名；2026-11-01 修订：S3 绑定类型改为 blob；2026-11-01 修订：命名实例总是登记、缺省实例的三个契约同一存储；2026-11-01 修订：同一存储只约束被要求的契约；2026-11-01 修订：列举的页大小越界返回校验错误；2026-11-01 修订：一致性验证改对 RustFS；2026-11-01 修订：不覆盖写入并发原子、S3 探测条件写入；2026-11-01 修订：条件写入探测改为启动照常、后台与首次使用时重试，键的保留前缀 `.xihan`，云厂商的兼容服务只走标准条件头；2026-11-02 修订：条件写入探测被拒绝、探测桶不存在恢复为启动失败，只有连不上照常启动、后台重试；2026-11-04 修订：基础设施失败的 `reason` 只写异常类型；同日再修订：S3 命名实例的客户端过出站地址策略；2026-11-24 修订：分片上传批量放弃、命名实例批量移除与登记数上限、S3BlobStore 不再有公开构造；同日再修订：一致性向量；同日第三次修订：S3 的会话令牌、请求超时、尝试次数与 HTTP 代理成为设置；同日第四次修订：对象键、查询前缀与保存的文件名的长度按 UTF-8 字节计，内容类型的 `;` 之后必须有参数；2026-11-25 修订：S3 命名实例在设置里配代理要运营方开关，缺省实例只给一项密钥是配置错误，服务端点不合法时报错不带原值；同日再修订：内容类型照 RFC 9110 允许空参数，内容类型原样存取，写不出原值的以 `OBJECTSTORAGE_VALIDATION_007` 失败；同日第三次修订：S3 的服务端点与绑定的主值不能带用户信息；同日第四次修订：S3 发起分片上传的应答没有可用的上传标识时返回 `OBJECTSTORAGE_UNAVAILABLE_001`；2026-11-26 修订：内容类型不能以空格结尾；同日再修订：「S3 Provider」照实测写明签名按合并空白之后的值计算，只差在连续空格的内容类型验签通过而被改写） | 日期：2026-10-23 | 对应 ADR：ADR-0018（S3）、ADR-0049（范围读取、分片上传与预签名）、ADR-0050（命名存储实例）、ADR-0075（登记规则）、ADR-0082（被要求的契约）、ADR-0086（验证用的 S3 兼容服务）、ADR-0089（条件写入探测）、ADR-0090（探测重试、保留前缀）、ADR-0093（被拒绝与缺桶启动失败）、ADR-0097（启动之后被拒绝或缺桶的处理已定）、ADR-0115、ADR-0266、ADR-0277（一致性向量）、ADR-0281（S3 客户端的设置）、ADR-0282（长度的单位与内容类型的参数）、ADR-0294（S3 配置的加固）、ADR-0295（内容类型的空参数与原样存取）、ADR-0302（S3 端点的用户信息、发起分片的应答缺上传标识）、ADR-0308（内容类型不以空格结尾、S3 内容类型的验签实测） | 参考实现：`XiHan.Framework.ObjectStorage.Abstractions`、`XiHan.Framework.ObjectStorage.FileSystem`、`XiHan.Framework.ObjectStorage.S3`、`XiHan.Framework.Hosting.AspNetCore`（宿主代收发的预签名） | 一致性：`XiHan.Framework.ObjectStorage.Conformance`，向量 `conformance/vectors/objectstorage/` | 一致性档：`objectstorage`

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
- 键 `BlobKey`：`/` 分段的路径，按 UTF-8 编码后 1 到 1024 字节（`BlobKey.MaxBytes`；2026-11-24 写明单位，ADR-0282：以前写「1 到 1024 个字符」，没有说按什么计，.NET 按 UTF-16 码元计，而 S3 的键上限是 1024 个 UTF-8 字节，含非 ASCII 字符的长键在文件系统存储上能写、在 S3 上被拒）；每段非空、不是 `.` 或 `..`；不含空白、控制字符与 `\`；区分大小写。
- 查询前缀 `BlobQuery.Prefix`（2026-11-24 写明，ADR-0282；以前正文没有写前缀的文法）：字符约束与字节上限同键（不含空白、控制字符与 `\`，按 UTF-8 编码后不超过 1024 字节），但可以为空（列举全部）、可以以 `/` 结尾，对各段不作要求（`a//b`、`./a` 只是筛选不到东西）；`BlobKey.IsValidPrefix` 判定，不合法在构造 `BlobQuery` 时抛出 `ArgumentException`。
- 保留前缀（2026-11-01 增补，ADR-0090）：首段是 `.xihan` 的键（`.xihan` 本身与 `.xihan/…`，区分大小写，`BlobKey.ReservedSegment`）留给框架自己的对象（如 S3 的条件写入探测键），不在键文法之内：构造时抛出 `ArgumentException`，`BlobKey.IsValid` 为假，业务写不进、读不到、删不掉；各 Provider 列举时跳过存储里已有的这类对象（框架经底层客户端写入的，或从外部放入的），不出现在业务列举里。查询前缀 `BlobQuery.Prefix` 只是筛选，不受此限，以 `.xihan` 开头的前缀列举总是空的。
- 自定义元数据名：`^[a-z0-9][a-z0-9-]{0,63}$`；值不含控制字符（含 CR、LF）；名与值合计不超过 2048 个 UTF-8 字节（`BlobPutOptions.MaxMetadataBytes`，与 S3 用户元数据的上限一致）。2026-11-13 修订，ADR-0214；以前值为任意字符串、不限大小，换行进了 S3 的请求头与文件系统的元数据文件，一个对象的元数据可以无限大。
- 内容类型 `BlobPutOptions.ContentType`（2026-11-13 修订，ADR-0214；以前只要求不空白）：媒体类型 `type/subtype`，两段都是 HTTP 的 token 字符，可跟若干个 `;` 起头的参数，整体 1 到 255 个字符（`MaxContentTypeLength`；内容类型只有 ASCII，字符数即字节数），`BlobPutOptions.IsValidContentType` 判定。参数的写法照 RFC 9110 §5.6.6 的 `parameters = *( OWS ";" OWS [ parameter ] )`（2026-11-25 修订，ADR-0295；2026-11-24 的 ADR-0282 曾要求每个 `;` 之后必须有参数，`text/plain;`、结尾多出的 `;` 与连续的 `;` 不合法，宿主代收的上传回 400；RFC 9110 的参数本来可以为空，野外也有客户端这样发；再往前 `;` 之后可以为空，媒体类型与 `;` 之间的空格正文没有写而 .NET 照收）：
  - 每个 `;` 两侧可以有空格，即 RFC 9110 的 OWS；只取其中的空格，制表符是控制字符（ADR-0214），在 `;` 的哪一侧都不收。
  - 每个 `;` 之后的参数可以为空：`text/plain;`、`text/plain ;`、结尾多出的 `;`（`text/plain; charset=utf-8;`）与连续的 `;`（`text/plain;;charset=utf-8`、`text/plain; ;charset=utf-8`）都合法。
  - 不为空的参数是不含 `;` 的可打印 ASCII 与空格（U+0020–U+007E），以非空格字符开头；参数内部不再细分：不要求 `name=value`，也不解析引号，引号里的 `;` 同样算分隔（`boundary="a;b"`、`boundary=";;"` 都合法）。
  - 媒体类型之后只有空格、没有 `;` 的不合法（`text/plain `）：OWS 只出现在 `;` 两侧。
  - 整串不以空格结尾（2026-11-26 修订，ADR-0308；以前以空格结尾的写法合法，如 `text/plain; charset=utf-8 `，2026-11-25 的 ADR-0295 起还有 `text/plain; `、`text/plain ; ` 与 `text/plain; charset=utf-8; `）。内容类型最终是 HTTP 的字段值——S3 的 `Content-Type` 请求头与响应头、预签名上传与宿主代收发的 `Content-Type`——而 RFC 9110 §5.5 的字段值不含首尾空白，收发的一方遇到首尾的空白要先去掉，所以任何靠 HTTP 头存内容类型的存储都存不住结尾的空格，做不到「契约」的原样存取（RustFS 1.0.6 上实测：经预签名地址原样发出的 `text/plain; ` 读回是 `text/plain;`，`text/plain; charset=UTF-8 ` 读回去掉了结尾的空格）。这一条对所有存储都成立，不只是 S3：文件系统存储自己存得住，但同一个值经宿主代收发、预签名地址或换一个存储都要过 HTTP 头，合法的内容类型应当在哪条路上都是同一个值。`;` 之后的空格因此只在后面还有字符时才合法：`text/plain ;`、`text/plain; charset=utf-8 ;`、`text/plain;  ;` 合法，`text/plain; `、`text/plain;  `、`text/plain ; ; ; `、`text/plain; charset=utf-8 ` 不合法。开头的空白本来就不合法（媒体类型以 token 字符开头）；空格之外的空白（制表符、换行）在哪里都不收。
  - 文法按整串匹配：`^<token>/<token>( *;([\x20-\x7E]*[\x21-\x7E])?)?$`，`<token>` 是一个或多个 token 字符（2026-11-26 修订，ADR-0308；以前是 `^<token>/<token>( *;[\x20-\x7E]*)?$`）。即第一个 `;`（之前可以有空格）之后为空，或是最后一个字符不是空格的一串可打印 ASCII 与空格；它就是以前的集合去掉以空格结尾的，与按上面几条逐段写出的 `^<token>/<token>( *; *([\x21-\x3A\x3C-\x7E][\x20-\x3A\x3C-\x7E]*)?)*$` 再加上「最后一个字符不是空格」是同一个集合（任意一串可打印 ASCII 与空格都能在每个 `;` 处切成可以为空的参数），实现取哪一种都可以。
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
- 内容类型原样存取（2026-11-25 写明，ADR-0295；以前不作规定）：写入（含分片上传的发起）把 `ContentType` 原样交给存储，读取把存储里的值原样放进 `BlobMetadata.ContentType`，Provider 自己不规范化——不去掉空参数，不改 `;` 两侧的空格，不改大小写。把读出的元数据原样写回（复制对象之类）得到同一个内容类型。所用的存储客户端写不出原值的（它会改写这个值），在发出任何请求之前以 `OBJECTSTORAGE_VALIDATION_007` 失败（HTTP 400，参数 `contentType`），不改写之后照写；预签名上传限定的内容类型由客户端自己发送，不经 Provider 的客户端，不受此限。S3 的情况见「S3 Provider」。
- `Overwrite = false` 且已存在 → `OBJECTSTORAGE_CONFLICT_001`，不写入。不覆盖写入必须原子：多个写入方同时以 `Overwrite = false` 写同一个新键，恰好一个成功，其余都是 `OBJECTSTORAGE_CONFLICT_001`，不能先查是否存在再写（并发时后写的会静默覆盖先写的）；做不到的存储不能声称支持不覆盖写入（S3 见下文「S3 的条件写入探测」）。
- 容器按需创建（文件系统）或由 Provider 的基础设施预先存在（S3 桶）；契约不管理容器。
- 基础设施失败（IO、网络）→ `OBJECTSTORAGE_UNAVAILABLE_001`，`reason` 只写异常类型的全名，完整异常进日志（2026-11-04，ADR-0115）：IO 异常的消息常带服务器上的完整路径；配额或单对象大小超限 → `OBJECTSTORAGE_UNAVAILABLE_002`，不写入。
- 批量操作在两项之间检查取消；已完成的项不回滚。
- 写入是原子的：读取方要么看到旧对象，要么看到完整的新对象。

单条便捷方法（`BlobStoreExtensions`）：`Put(container, key, stream | bytes, options)`、`Delete(container, key)`、`Exists`、`ReadAllBytes`，都转调批量方法。

## 默认实现

`ObjectStorage.FileSystem`：根目录（默认 `<AppContext.BaseDirectory>/blobs`）下 `<container>/data/<key>` 存内容、`<container>/meta/<key>.json` 存元数据；写入先落临时文件再原子替换，不覆盖的写入（含分片上传的完成）由文件系统原子判定谁先占住内容路径，目标已存在即 `OBJECTSTORAGE_CONFLICT_001`，多个写入方（含共享目录的多个进程）只有一个成功：Unix 上先以独占创建（`O_CREAT | O_EXCL` 语义）的空文件占位，再以 `rename` 把内容原子替换上去，不能用「先查目标再改名」的不替换移动，.NET 在 Unix 上的 `File.Move(…, overwrite: false)` 就是这样实现的（2026-11-11 修订，ADR-0168）；Windows 上用不替换的改名（`MoveFileEx` 不带替换标志，本身原子），不用占位再替换——替换要求目标此刻没有被打开，并发写入方探测占位文件的那一下就让占到位的一方失败（2026-11-12 修订，ADR-0172）；`ETag` 是内容的 SHA-256；配额 `MaxTotalBytes`（默认 1 GiB）与 `MaxBlobBytes`（默认 256 MiB），启动时扫描目录得到已用量，之后在进程内计量；单进程正确，多进程共享目录时配额只是近似。写入边复制边在进程内预留额度，总配额按已用量加全部预留判定，并发写入合计不超过它；覆盖期间新旧内容同时在盘上，预留按新内容的全长，旧对象的字节到提交时才释放。提交（改名）、删除与记账在同一把锁里完成，按此刻磁盘上旧内容的实际长度计算差值，并发覆盖与删除同一个键时用量与磁盘一致（2026-11-12 修订，ADR-0183；以前只检查不预留，并发写入能超额最多 N×`MaxBlobBytes`，并发覆盖按开始时读到的旧长度扣减会少记用量）。存储只经 `FileSystemBlobStore.CreateAsync` 创建：建目录、扫描已用量并清掉过期的分片上传后才交出，不提供不初始化的公开构造，否则配额从零算起会放过超额写入（2026-11-11 修订，ADR-0163）。

读取与覆盖、删除交错（2026-11-18 修订，ADR-0241）：

- 正在读的对象照样能覆盖与删除，读取方读完的是打开时的那一版；被换掉或删掉的旧内容在最后一个读取方关闭之前仍占磁盘，不计入用量。读取方打开内容与元数据都容许删除共享。Windows 上普通的替换改名（`MoveFileEx` 带替换标志）换不掉正被打开的文件，读取方容许删除共享也不行，所以替换以 POSIX 语义改名（`FILE_RENAME_FLAG_POSIX_SEMANTICS`）；卷不支持这种改名时（SMB 共享、FAT 等）退回普通的替换改名并记一次警告，正被读取的对象覆盖时仍以 `OBJECTSTORAGE_UNAVAILABLE_001` 失败。Unix 的 `rename` 本来就与目标是否被打开无关。
- 一次读取（`Get`、`Head`、列举的每一项）拿到的元数据与内容属于同一版：提交与删除在记账的同一把锁里成对变更内容与元数据，读取不进锁、读完核对期间有没有变更，有就在锁里重读。以前先读元数据、再打开内容，两步之间被覆盖时元数据（长度、`ETag`）是上一版、内容是下一版；两步之间被删时报 `OBJECTSTORAGE_UNAVAILABLE_001` 而不是不存在，列举整页失败。
- 这两条只在进程内成立：多个进程共享目录时与配额一样只是近似。

## S3 Provider

`ObjectStorage.S3`：容器即桶，键即对象键，经 AWS SDK 访问 Amazon S3 与兼容服务（MinIO、阿里云 OSS、腾讯云 COS、Cloudflare R2 等），不再为每家云单独做包。

- 云厂商的兼容服务（2026-11-01 增补，ADR-0090）：阿里云 OSS、腾讯云 COS、Cloudflare R2 等同样只用这一个包，不为各家的私有请求头（如 OSS 的 `x-oss-forbid-overwrite`、COS 的 `x-cos-forbid-overwrite`）做适配，不覆盖写入只认标准的 `If-None-Match: *`。这几家是否认它框架没有实测，接入前要实测：缺省的条件写入探测就是实测（下文「S3 的条件写入探测」），不认时启动失败（`OBJECTSTORAGE_UNAVAILABLE_003`）或之后在健康项上报告；也可以按探测的做法用 curl 发两次带 `If-None-Match: *` 的 `PutObject`，第二次必须 412。不认就设 `ConditionalWrites = false`，后果是 `Overwrite = false` 的写入与分片上传的发起、完成一律 `OBJECTSTORAGE_UNAVAILABLE_003`，"只在不存在时写入"不再可用——需要它的业务改用不会冲突的键（如带随机后缀）或改为覆盖写入，框架不退回先查再写。
- `Overwrite = false` 先 `Head`（已存在就不必上传内容）再带 `If-None-Match: *` 写入，由服务端保证原子；已存在 → `OBJECTSTORAGE_CONFLICT_001`。服务是否认条件写入要先探明（见下文「S3 的条件写入探测」）：探明之前不覆盖写入返回不可用，探明不认的服务（如 Garage、Zenko CloudServer）上不覆盖写入一律报错，启动时就探明的启动失败，任何时候都不带着可能静默覆盖的不覆盖写入运行。不可定位的内容流先落临时文件取得长度，不整体装进内存。
- 批量删除：S3 不报告对象是否存在过，先逐键 `Head` 定存在性（同一批里重复的键只有第一次算存在），再把存在的键按每批 1000 个 `DeleteObjects`；删除失败只影响本次负责删除的项。
- 内容类型（2026-11-25 写明，ADR-0295）：.NET 参考实现经 AWS SDK for .NET 写入，SDK 把 `Content-Type` 交给 .NET 的媒体类型解析器（`MediaTypeHeaderValue`）重新写出。它解析不了的（`;` 之后为空，未加引号的参数值里有空格等），SDK 抛出 `FormatException`，而这时请求行与其余请求头已经发出（不带 `Content-Type`）；解析得了而写法不同的，它一律写成 `type/subtype; 参数`（`;` 之前没有空格、之后一个空格），线上发的是改写后的值，而签名（SigV4）按原值去掉首尾空白、把连续的空格合并成一个之后计算，服务按收到的值同样处理后验签（2026-11-26 修订，ADR-0308；以前写的是「签名按原值计算，服务验签对不上」，没有分出只差在连续空格的一类）：
  - 合并空格之后仍与改写后的值不同的（`;` 之后没有空格的 `text/plain;charset=UTF-8`，`;` 之前有空格的 `text/plain ;charset=UTF-8`），验签对不上，请求被拒绝。
  - 只差在连续空格的（`text/plain;  charset=UTF-8`），验签通过，服务存下的是改写后的 `text/plain; charset=UTF-8`，不报错。结尾的空格在签名时同样被去掉，服务存下去掉之后的值；以空格结尾的内容类型已不合文法（见「标识」），到不了 Provider。
  - 以上（含解析不了时 SDK 抛出之后请求照样发出、服务回 403 而没有存下）是对 RustFS 1.0.6 的实测（AWS SDK for .NET 4.0，经 http；被拒绝的回 403 `SignatureDoesNotMatch`）；Amazon S3 没有实测，按 SigV4 的规范请求头（值去掉首尾空白、连续空格合并成一个）推断结果相同，其余兼容服务未知。
  - 两类都写不出原值：写入与分片上传的发起一律返回 `OBJECTSTORAGE_VALIDATION_007`，不发请求（按「契约」的内容类型原样存取）。不拒绝的话，解析不了的抛出异常，前一类请求被拒，后一类悄悄存下别的值。
  - 写得出原值的照常写入（如 `text/plain`、`text/plain; charset=utf-8`、`Text/HTML`、`text/plain; boundary=";;"`）。
  - 读取时 SDK 同样按这个写法交出它解析得了的值：存储里的 `text/plain;charset=utf-8`（例如经预签名地址由浏览器上传的）读出 `text/plain; charset=utf-8`，解析不了的原样交出——读取一侧 .NET 的 S3 Provider 做不到原样，是已知的缺口。
- 列举经 `ListObjectsV2`（S3 保证键的序数顺序），续页标记是其 continuation token；条目只有键、长度、`ETag` 与修改时间，内容类型为默认值、自定义元数据为空，需要时再 `Head`——列举条目的元数据因此是"可能不完整"的，一致性套件只校验键顺序、长度与 `ETag`。不符合键文法的对象（以 `/` 结尾的"目录"占位）跳过。
- 不存在的键与桶 → `OBJECTSTORAGE_NOTFOUND_001`；桶不存在的列举是空页；其余服务端错误、网络错误与 SDK 超时 → `OBJECTSTORAGE_UNAVAILABLE_001`，`reason` 同样只写异常类型的全名（2026-11-04，ADR-0115）；服务应答成功却缺了必需的内容同样是服务端错误（2026-11-25 写明，ADR-0302；分片上传的发起见下文「范围读取、分片上传与预签名」）；批量删除里单个键的失败只写 S3 的错误码。没有配额（由桶策略管理）。`EnsureBuckets` 开启时写入遇到桶不存在就建桶再重试一次，默认由基础设施预先建桶。
- 配置 `XiHan:ObjectStorage:S3`（`ServiceUrl`、`Region`、`AccessKey`、`SecretKey`、`SessionToken`、`ForcePathStyle`、`EnsureBuckets`、`HealthBucket`、`ConditionalWrites`、`HealthProbeIntervalSeconds`，以及下一条的 `RequestTimeoutSeconds`、`MaxAttempts` 与四个 `Proxy…`），绑定 `objectstorage`（类型 `blob`，与 RFC-0016 的绑定类型词表一致）的主值与属性覆盖之（RFC-0016「资源绑定与配置节」）；两项密钥都没有时走 SDK 默认凭据链；应用自己注册的 `IAmazonS3` 优先，这时下一条的设置对它不起作用。
- `AccessKey` 与 `SecretKey` 必须同时给出或都不给（2026-11-25 修订，ADR-0294；以前缺省实例只给一项时悄悄走默认凭据链，命名实例早已拒绝）：只给一项是配置错误，处置同下文「客户端的设置」里的成组设置。核对在绑定的属性 `accessKey`、`secretKey` 覆盖之后进行，两项可以一项来自绑定、一项来自配置节。
- 服务端点不带用户信息（2026-11-25 写明，ADR-0302；以前正文没有规定端点的写法，.NET 只要求绝对的 http 或 https 地址，带用户信息的照样收下、原样保存，打印选项时明文出现，SDK 也不用这段用户信息）：`ServiceUrl` 与绑定 `objectstorage` 的主值是绝对的 `http` 或 `https` 地址，主机之前不能有用户信息（RFC 3986 §3.2.1 的 `userinfo "@"`）：`user:pass@`、`user@`、`:@` 与空的 `@` 都算，路径、查询与片段里的 `@` 不算。S3 的凭据只走 `AccessKey`、`SecretKey`、`SessionToken` 与绑定的同名属性。缺省实例的配置节、绑定的主值与命名实例的设置都核对；带了用户信息是配置错误，处置同下一条「客户端的设置」里的成组设置，消息只报键名或绑定名、指明凭据该写在哪几个键里，不带原值。
- 客户端的设置（2026-11-24 增补，ADR-0281；以前临时凭据、超时、重试与代理只能靠应用向容器注册自己的 `IAmazonS3`，命名实例无从设置）：配置节与命名实例的设置里都有，公开面不出现 SDK 的类型；都不写时建出的客户端与以前相同。
  - `SessionToken`：临时凭据的会话令牌。必须与 `AccessKey`、`SecretKey` 同时给出，缺任何一项是配置错误（不拿着令牌退回凭据链）；绑定属性 `sessionToken` 覆盖它。
  - `RequestTimeoutSeconds`：单次 HTTP 请求的超时，每次尝试各算各的；正数，不超过 86400（24 小时，`S3ObjectStorageOptions.MaxRequestTimeout`）。不写时不另设时限（SDK 对 S3 的缺省，约 24.8 天）。超时照上面的映射是 `OBJECTSTORAGE_UNAVAILABLE_001`。
  - `MaxAttempts`：一次操作最多发出的请求数，含首次；1 到 10（`S3ObjectStorageOptions.MaxAttemptsLimit`），1 即不重试。SDK 判定连接池里的连接已被对端关闭后重发的请求不计入（那是连接池的补救，不是对端失败后的重试）。不写时取 SDK 的缺省（标准重试模式，共 3 次）。
  - `ProxyHost`、`ProxyPort`、`ProxyUserName`、`ProxyPassword`：经 HTTP 代理访问服务。主机是主机名或 IP 字面量，不带协议、端口、路径与用户信息，带区域标识的 IPv6 地址（`fe80::1%eth0`）也不收；端口 1 到 65535；两项必须同时给出。用户名与口令可选，必须同时给出，且只能与主机、端口一起给。只给了一部分是配置错误。
  - 配置错误的处置同 `HealthBucket`：缺省实例装配失败（`ArgumentException`），命名实例 → `OBJECTSTORAGE_VALIDATION_003`。消息里只写键名与问题，带出的原值只限数值与布尔值；会话令牌、代理主机、用户名与口令的值从不进消息，服务端点（`ServiceUrl` 与绑定的主值）不合法时（含带了用户信息的，ADR-0302）同样只报键名（2026-11-25 修订，ADR-0294；以前端点的原值写进消息，写成带用户信息的地址时口令进了异常与日志）。
  - `SessionToken`、`ProxyUserName`、`ProxyPassword` 是凭据，打印选项时遮蔽（RFC-0014「选项里的凭据」）。

## S3 的条件写入探测（2026-11-01 增补，ADR-0089；同日修订，ADR-0090；2026-11-02 修订，ADR-0093）

不覆盖写入在 S3 上靠条件写入（`If-None-Match: *`）保证原子；不认它的兼容服务把条件头当作没有，先 `Head` 再写的两个写入方都会成功、后写的静默覆盖先写的（ADR-0089 实测：Garage、Zenko CloudServer 上 8 个并发的不覆盖写入全部"成功"）。S3 Provider 因此在不覆盖写入之前探明服务是否认条件写入：

- 设置 `ConditionalWrites`（配置节与命名实例的设置里都有，缺省 `true`）。开启时探测，`HealthBucket` 必须配置（探测写在这个桶里），没配是配置错误：缺省实例装配失败（`ArgumentException`），命名实例 → `OBJECTSTORAGE_VALIDATION_003`。
- 探测：对固定的探测键 `.xihan/probe/conditional-write`（保留前缀 `.xihan` 下，见「标识」，业务列举不会出现；空对象）带 `If-None-Match: *` 写入；成功则再写一次，第二次必须被拒（412），还成功就是不认。提供分片上传时（缺省实例被选为 `objectstorage.multipart`；命名实例总是提供）再以同一个键发起分片上传、上传一片、带 `If-None-Match: *` 完成，必须被拒（412），被拒的上传随即放弃；完成了就是不认（探测键被同样为空的内容覆盖，无害）。单次探测以 `HealthProbeIntervalSeconds`（缺省 15 秒，命名实例取缺省值）为超时，按注入的时钟计时。
- 探测键写入后不删除：任何一次 412 都证明服务认得条件写入，键已存在时第一次写入就被拒，所以多个实例同时启动、反复启动、后台重试都得到同一结论，没有"删掉后别人又写入"的竞态；凭据只需要对这一个键的写入（`s3:PutObject`）与放弃分片上传（`s3:AbortMultipartUpload`）的权限，不需要删除、读取或列举；桶里只多一个空对象，在保留前缀下，业务列举跳过它。`EnsureBuckets` 开启时探测桶不存在就建桶后重试一次（同写入）。
- 一次探测的结论：通过（"已确认"）；探明不认（第二次写入或完成没被拒，或服务回 501，`OBJECTSTORAGE_UNAVAILABLE_003`）；其余都是"尚未确认"（服务认不认条件写入还不知道），按原因分两类——配置错误：凭据无效或无权写探测键、建桶、放弃上传（403，`OBJECTSTORAGE_DENIED_002`，参数 `bucket`、`key`、`operation`、`reason` 为服务端错误码）、探测桶不存在（`OBJECTSTORAGE_NOTFOUND_001`，参数 `bucket`）；连不上：连不上、超时或其余服务端错误（`OBJECTSTORAGE_UNAVAILABLE_001`，`reason` 带出是哪一步）。已确认与不认是终态，服务的能力不会在运行中改变；尚未确认会重试。
- 时机：缺省实例在特性启动时探一次，与健康项的首次探测并行（S3 连不上时两者都要等到超时，并行让启动只等一个超时）；命名实例在建出时探一次（每个实例各探各的，实例指向的服务可能不同）。
- 启动或建实例时的结论（2026-11-02 修订，ADR-0093）：探明不认、被拒绝、探测桶不存在都是配置问题，要早暴露 → 缺省实例以 `KERNEL_HOSTING_002`（内层分别为 `OBJECTSTORAGE_UNAVAILABLE_003`、`OBJECTSTORAGE_DENIED_002`、`OBJECTSTORAGE_NOTFOUND_001`，参数同上）启动失败；命名实例不建、不登记（Provider 抛出携带该码的 `FrameworkException`，注册表原样返回该码，下次取用重新探测）。只有连不上（`OBJECTSTORAGE_UNAVAILABLE_001`）→ 照常启动、照常建出并登记实例，在后台重试：第一次 1 秒后，之后每次间隔加倍，最长 `HealthProbeIntervalSeconds`（它小于 1 秒时每次都取它），计时用注入的时钟；有了终态即停止，应用停止或实例释放时也停止；S3 修好（服务就绪、网络恢复）后自动确认，不必重启。`EnsureBuckets` 开启时缺桶先建（同写入），建桶被拒仍是被拒绝。`S3BlobStore` 没有公开构造，公开面不出现 SDK 的类型（2026-11-24 修订，ADR-0266）：缺省实例由特性建出，命名实例由 Provider 建出，都按本节探测。
- 尚未确认时的不覆盖写入（`Overwrite = false` 的写入、分片上传的发起与完成）：有探测正在进行就等它的结论（受调用方取消约束）；没有而距上一次探测结束已满 1 秒，就地探测一次（同一时刻每个实例只有一个探测，后台重试与写入共用它）；不满 1 秒不再探测。之后已确认就照常写入；不认 → `OBJECTSTORAGE_UNAVAILABLE_003`；仍未确认 → `OBJECTSTORAGE_UNAVAILABLE_004`（HTTP 503，参数 `container`、`key`、`reason` 为最近一次探测失败的错误码），不访问写入目标，不退回先查再写。覆盖写入、读取、删除、列举、预签名不受影响。
- 启动之后（后台重试或写入时的探测）才探明不认：应用不停止（已经在服务），此后不覆盖写入一律 `OBJECTSTORAGE_UNAVAILABLE_003`，记一条 Error 日志；换成认条件写入的服务，或关闭条件写入后重启才恢复。
- 启动之后才遇到被拒绝或探测桶不存在（2026-11-02 增补，ADR-0093；只会发生在启动或建实例时连不上、尚未确认的实例上，例如连上时凭据已被吊销、桶已被删除，或配置本来就错而启动时探不到）：应用不停止，状态仍是尚未确认，后台照常按退避重试，权限改好或桶建好后自动确认、不必重启；不覆盖写入仍是 `OBJECTSTORAGE_UNAVAILABLE_004`（HTTP 503，`reason` 为 `OBJECTSTORAGE_DENIED_002` 或 `OBJECTSTORAGE_NOTFOUND_001`），不访问写入目标——这次写入的调用方没有过错，不以 403 / 404 回给它；缺省实例的健康项 Failed（配置错误要人处理，与连不上的 Degraded 区分开），错误为该码；原因变化时记一条 Error 日志（连不上是 Warning），稳定的失败不随每次重试刷屏。已定（2026-11-02，ADR-0097）：维持继续重试、缺省实例健康项 Failed，不改为停止重试（修好要重启）或只记 Degraded（看不出要人处理）；命名实例的健康项按注册表的规则一律 Degraded（RFC 下文「命名存储实例」），不为此改注册表。已确认之后凭据被吊销或桶被删除不在此列：已确认是终态、不再探测，此后的读写照常访问存储，按「S3 Provider」的错误映射失败（被拒等服务端错误 → `OBJECTSTORAGE_UNAVAILABLE_001`），连通性的 `Head` 失败使健康项 Degraded。
- 健康：缺省实例的健康项 `objectstorage.s3.connection` 合并连通性（对 `HealthBucket` 的 `Head`）与条件写入的状态，数据 `conditionalWrites` 为 `confirmed` / `unconfirmed` / `unsupported` / `disabled`：尚未确认时错误是最近一次探测失败，即便 `Head` 成功——连不上（`OBJECTSTORAGE_UNAVAILABLE_001`）为 Degraded，被拒绝或探测桶不存在（`OBJECTSTORAGE_DENIED_002` / `NOTFOUND_001`，只在启动之后才会出现）为 Failed；探明不认时 Failed，错误为 `OBJECTSTORAGE_UNAVAILABLE_003`；状态随探测结论即时变化，不等下一轮健康探测；Failed 的条目让应用的聚合状态为 Degraded，应用仍就绪。命名实例的健康项 `objectstorage.store.<name>` 在连通之后同样看条件写入：尚未确认或不认时 Degraded（注册表把探测失败一律记为 Degraded），错误同上。
- 关闭（`ConditionalWrites = false`）表示应用明确不需要不覆盖写入（只覆盖写入，或凭据只读，或服务实测不认条件写入）：不探测，`HealthBucket` 可以不配（健康项列桶），`Overwrite = false` 的写入、分片上传的发起与完成一律 → `OBJECTSTORAGE_UNAVAILABLE_003`，不访问存储——不做没有保证的先查再写；覆盖写入、读取、删除、列举、预签名不受影响。

## 范围读取、分片上传与预签名（2026-10-29 增补，ADR-0049）

分片上传的句柄 `BlobMultipartUpload(container, key, uploadId)` 的覆盖标志 `Overwrite` 是必填的初始化成员：重建句柄（从签名地址、存下的状态）时必须写明，与开始上传时一致；完成时 Provider 可能按句柄上的值决定是否条件写入（2026-11-11 修订，ADR-0163）。

大文件直传、断点续传、音视频拖动与限时下载链接（预签名下载，缺省 30 分钟）。

- 范围读取（`objectstorage.blob@1` 的一部分）：`Get(container, key, range)`，范围是起点与可选长度；长度为空读到末尾，超出末尾截断；返回的内容带实际范围（起点、长度、总长），元数据长度仍是总长。起点不在对象内（含空对象）→ `OBJECTSTORAGE_VALIDATION_001`。
- 分片上传（能力 `objectstorage.multipart@1`，`IBlobMultipartStore`）：`Start(container, key, options)` 得到上传标识（不透明）；`UploadPart(upload, n, stream)` 上传编号 1–10000 的一片，可乱序、可重传（后一次覆盖），返回该片 ETag；`ListParts(upload)` 按编号升序列出已上传的分片（续传据此跳过）；`Complete(upload, parts)` 按编号严格升序、ETag 与已上传的一致的清单拼成对象，除最后一片外每片至少 5 MiB；`Abort(uploads)` 批量丢弃（2026-11-24 修订，ADR-0266）：按入参顺序逐项执行，返回等长、同序的逐项结果；上传不存在也成功，所以同一批里重复的上传每一项都成功；空集合不访问存储、返回空；存储不可达只记在该项上（`OBJECTSTORAGE_UNAVAILABLE_001`），其余项照常；每一项之前检查取消，已放弃的不恢复；集合里有空项是编程错误，在放弃任何上传之前抛出。单条是批量的特例。完成前对象不可见；完成是原子的。
  - 清单不合法（为空、乱序、未上传或 ETag 不符、非末片过小）→ `OBJECTSTORAGE_VALIDATION_002`，上传保留可再完成；上传不存在或已结束 → `OBJECTSTORAGE_NOTFOUND_002`；`Overwrite = false` 时对象已存在，发起与完成都 → `OBJECTSTORAGE_CONFLICT_001`。
- 预签名（能力 `objectstorage.presign@1`，`IBlobUrlSigner`）：签出限时地址，客户端不带凭据直接下载（`GET`，支持 `Range`，可带保存的文件名）、上传整个对象（`PUT`，可限定内容类型）或上传分片上传的某一片（`PUT`，响应 `ETag` 即分片 ETag）。有效期缺省 30 分钟，1 秒到 7 天。签出的地址带客户端必须发送的请求头。签名是本地计算，不检查对象是否存在。限定的内容类型须满足内容类型文法（`BlobPutOptions.IsValidContentType`），保存的文件名不为空、不全是空白（RFC-0001「空白字符」）、不含控制字符、按 UTF-8 编码后不超过 255 字节（`BlobUrlRequest.MaxDownloadFileNameBytes`；2026-11-24 写明单位，ADR-0282：以前写「1 到 255 个字符」，没有说按什么计，.NET 按 UTF-16 码元计；全是空白的文件名 .NET 一向拒绝，正文没有写），不合法在构造 `BlobUrlRequest` 时抛出 `ArgumentException`（2026-11-13 修订，ADR-0219；以前只要求不空白：宿主签名的规范串以换行分隔各限定项，值里带换行就能把一项挪成另一项，S3 的响应头覆盖也会把换行写进头）。
  - 原生支持的存储（S3）由存储签名、客户端直连存储。
  - 不支持的存储（文件系统）由宿主代收发：地址指向宿主的端点，签名是对方法、容器、键、过期时刻与各限定项的 HMAC-SHA256，无状态，同一密钥的多个实例互认；端点验签后经存储收发，签名不符或过期 → `OBJECTSTORAGE_DENIED_001`（HTTP 403），范围不可满足回 416。上传的签名没有限定内容类型时对象的内容类型取请求的 `Content-Type`，不合内容类型文法 → `OBJECTSTORAGE_VALIDATION_006`（HTTP 400），不写入（2026-11-13 修订，ADR-0214）。端点的响应都带 `X-Content-Type-Options: nosniff`；写出内容时带 `Content-Security-Policy: sandbox; default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'`，PDF 除外，应用已经设了这个头时不覆盖（2026-11-04 修订，ADR-0125）。
- 文件系统：范围读取定位后只交出该段；分片暂存在容器的 `uploads/<上传标识>/`，分片文件名带内容的 SHA-256，完成时拼接成同目录临时文件再原子替换并计入配额；单片与拼接后的对象受单对象上限约束。暂存有界（2026-11-12 修订，ADR-0184；以前暂存的分片不计入配额、上传数不限、过期的上传只在启动时清掉且清掉之前照样能传分片）：暂存的分片边写边预留、计入总配额，完成或放弃后扣除；未完成的上传数不超过 `MaxMultipartUploads`（默认 1000），满了发起即 `OBJECTSTORAGE_UNAVAILABLE_002`（消息键 `objectstorage.multipart.uploads_exhausted`，参数 `limit`）；超过 `MultipartExpiry`（默认 7 天）未完成的上传当作不存在（再传分片、列举与完成都是 `OBJECTSTORAGE_NOTFOUND_002`），碰到即清掉，启动扫描时与运行期间发起新上传时（至多每小时一次）也清掉。
- S3：范围读取经 `Range` 头；分片直接映射 S3 multipart（不覆盖的完成先 `Head` 再带 `If-None-Match: *`，服务认条件完成要先由探测确认：尚未确认时不覆盖的发起与完成 → `OBJECTSTORAGE_UNAVAILABLE_004`，探明不认或关闭条件写入时 → `OBJECTSTORAGE_UNAVAILABLE_003`）；预签名经 SDK（签名 V4），兼容服务的端点是 http 时按 http 签。
  - 发起的应答没有可用的上传标识（2026-11-25 写明，ADR-0302；以前 .NET 把应答里的标识直接交给句柄的构造，没有标识时抛出 `ArgumentNullException`，为空或当不了句柄时抛出 `ArgumentException`）：服务回了成功，应答里却没有上传标识、标识为空，或当不了句柄（.NET 的 `BlobMultipartUpload` 要求 1 到 1024 个字符、不含空白与控制字符），是不合规的应答，按「S3 Provider」的映射同其余服务端错误：返回 `OBJECTSTORAGE_UNAVAILABLE_001`，不抛出；`reason` 写明是发起的应答没有可用的上传标识，不带应答里的值。不重发：服务端可能已经建了这个上传，没有标识就放弃不了，由桶的生命周期规则（未完成的分片上传过期即放弃）清理。

## 命名存储实例（2026-10-29 增补，ADR-0050）

一个应用同时用多个存储：按租户或按用途各配一个桶、各有自己的端点与密钥，定义可以写在配置里，也可以在运行时来自应用自己的库。

- 定义 `BlobStoreDefinition`：实例名（`^[a-z0-9][a-z0-9-]{0,63}$`）、Provider 名（`filesystem`、`s3`，不区分大小写）与设置。设置的键同该 Provider 的配置节，不区分大小写；未知的键是错误，拼错的键不会被静默忽略。
- Provider `IBlobStoreProvider`：每个存储包一个，按定义建实例、探测实例。`CreateAsync` 返回 `Result<IBlobStore>`，失败不抛出（2026-11-11 修订，ADR-0150）：设置不合法 → `OBJECTSTORAGE_VALIDATION_003`，后端此刻建不出来 → `OBJECTSTORAGE_UNAVAILABLE_001`（`BlobStoreFailures` 给出标准错误，原因只写异常类型、完整异常由 Provider 记日志），探测发现不能用 → 探测的错误码；失败时 Provider 自己释放已建的资源。文件系统的命名实例必须给 `RootPath`（不给就会与缺省实例落在同一目录）；S3 的命名实例各建各的客户端，不读连接绑定，探测间隔不在设置里；`ConditionalWrites` 开启（缺省）时建实例时探测条件写入，探明不认、被拒绝或探测桶不存在都不建，只有连不上的照常建出、在后台重试（「S3 的条件写入探测」）。
- 命名实例的端点可能来自应用的库，不可信（2026-11-04 修订，ADR-0117）：S3 命名实例的客户端过出站地址策略（RFC-0008），自建存储的地址要放行；被拒绝与连不上同样处理，建实例时的探测只算尚未确认，读写返回 `OBJECTSTORAGE_UNAVAILABLE_001`。缺省实例的端点是运维配的，不过策略。命名实例配了代理时（2026-11-24 增补，ADR-0281），本进程连的是代理：过策略的是代理的地址，被拒绝与连不上同样处理；端点由代理去解析、去连，本进程的策略管不到，要限制经代理能到的目标就在代理上做。命名实例的设置里配代理（`ProxyHost` 等四项）只在运营方打开 `XiHan:ObjectStorage:S3:NamedInstancesAllowProxy` 时允许（2026-11-25 修订，ADR-0294；以前与缺省实例一样总能配）：开关缺省 `false`，不是布尔值时建 Provider 即失败；没打开而定义配了代理是 `OBJECTSTORAGE_VALIDATION_003`，原因不带代理的任何取值；打开之后照上面的规则过策略。策略放行了一个内网正向代理的部署里，能填实例设置的人就能经它到达策略本来拒绝的地址，所以这一步由运营方决定。缺省实例的代理是运营方自己配的，不受这个开关约束。S3 命名实例必须给出 `AccessKey` 与 `SecretKey`（2026-11-12 修订，ADR-0178）：设置可以由租户填写，没给密钥就会用宿主的 AWS 凭据链（环境变量、配置文件、实例角色），等于把宿主的身份借给填设置的人；只给一项总是 `OBJECTSTORAGE_VALIDATION_003`，两项都不给只在运营方打开 `XiHan:ObjectStorage:S3:NamedInstancesUseAmbientCredentials` 时允许。这两个运营方开关都在配置节 `XiHan:ObjectStorage:S3` 下，不在实例设置的键里，不能经实例设置打开。缺省实例两项都空时走凭据链，只给一项同样是配置错误（「S3 Provider」，ADR-0294）。
- 登记规则（2026-11-01 修订，ADR-0075）：存储包只要装配（`UseFeature`）就登记自己的命名实例 Provider，与它是否被选为缺省实例无关。能力全部被别的存储顶替的存储包实现 `IShadowAwareFeature`（RFC-0002 解析规则 4），被顶替时只登记命名实例 Provider：不读自己的配置节与连接绑定、不建缺省实例、不启动。
- 注册表 `BlobStores`（单例；装配任一个存储包即有）：
  - `Get(name)`：已建的直接返回；配置节 `XiHan:ObjectStorage:Stores:<name>`（`Provider` 加设置）里有定义的首次取用时建出；都没有 → `OBJECTSTORAGE_NOTFOUND_003`。
  - `GetOrAdd(definition)`：同名同定义（Provider 名与设置完全相同）返回同一实例；定义变了建新实例替换并释放旧的，替换时仍在旧实例上的调用可能失败。Provider 未登记或设置不合法 → `OBJECTSTORAGE_VALIDATION_003`，后端此刻建不出来（目录无权限、磁盘错误）→ `OBJECTSTORAGE_UNAVAILABLE_001`，登记已满 → `OBJECTSTORAGE_UNAVAILABLE_005`（见下面「容量」），建实例时的探测发现实例不能用 → Provider 给出的错误码（Provider 抛出携带错误码的 `FrameworkException`，注册表转成同码的失败结果；S3 在探明不认条件写入、探测被拒绝或探测桶不存在时如此，连不上的照常建出，见「S3 的条件写入探测」）；这些情况已有的同名实例都不受影响。
  - `Remove(names)` 批量移除并释放（2026-11-24 修订，ADR-0266）：逐项返回是否存在过，与入参同序，同一批里重复的名字第二次为否，空集合返回空；整批一次加锁移除，某个实例释放失败只记日志，不影响其余实例与返回值。应用停止时释放全部。
  - 容量（2026-11-24 修订，ADR-0266）：已建的实例数上限 `XiHan:ObjectStorage:StoreMaxCount`（缺省 64，1–1024；配置定义与运行时登记的都算）。满了再登记新名字（含首次取用配置定义的实例）→ `OBJECTSTORAGE_UNAVAILABLE_005`，不建实例、不淘汰已有的；替换同名的不占名额，移除腾出名额。
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

Provider 必须继承 `BlobStoreConformance` 并通过全部用例（S3 Provider 对 RustFS 通过，此前对 MinIO 通过；ADR-0086 记录了其余兼容服务的差异）。用例包括并发的不覆盖写入（8 个写入方同时写同一个新键，恰好一个成功，ADR-0089）：先查再写的实现过不了，不认条件写入的 S3 兼容服务上不经探测建的 `S3BlobStore`（参考实现的测试经内部构造）也过不了（Garage、Zenko CloudServer 上 8 个全部成功），这正是 S3 Provider 探明之前不做不覆盖写入、探明不认就拒绝的原因；以及保留前缀（ADR-0090）：首段是 `.xihan` 的键构造时被拒，套件经 Provider 实现的钩子 `PlantOutsideContractAsync` 绕过契约在存储里放入保留前缀下的对象，列举必须跳过它们；以及内容类型原样存取（2026-11-25 增补，ADR-0295）：空参数、`;` 两侧的空格与引号里的 `;` 写入、读出与把读出的值写回都不变，写不出原值的返回 `OBJECTSTORAGE_VALIDATION_007` 且不写入，分片上传的发起同此；实现了 `IBlobMultipartStore` 的 Provider 同样要通过分片用例，含批量放弃（不实现时这些用例跳过）。命名实例的 Provider 必须继承 `BlobStoreProviderConformance` 并通过全部用例（参考实现另有混装用例：两种装配顺序下两个存储包的命名实例 Provider 都登记，缺省的三个契约同一存储，被要求的契约选得不一致时装配失败、兜底选出的不失败而按被顶替处理）：按定义建出可读写且探测通过的实例、未知的设置键被拒绝、经注册表同定义复用 / 定义变了替换 / 批量移除逐项报告、移除后不存在、定义错误翻译为 `OBJECTSTORAGE_VALIDATION_003` 且不影响已有实例。

向量（档 `objectstorage`，`conformance/vectors/objectstorage/`；2026-11-24，ADR-0277，以前这个档没有向量；同日 ADR-0282 补上长度按字节计、查询前缀、保存的文件名与内容类型的参数；2026-11-25 ADR-0295 把 `;` 之后为空的写法挪到合法一侧；2026-11-26 ADR-0308 把以空格结尾的写法挪到不合法一侧）：

| 文件 | 内容 |
| --- | --- |
| `container-names.json` | 容器名文法的合法与不合法样例，含 3 与 63 个字符的两侧 |
| `blob-keys.json` | 对象键的文法（分段、`.` 与 `..`、空白、控制字符、反斜线）、保留前缀与查询前缀；1024 字节的上限两侧，含码元数不超而字节数超过的，每条标出 UTF-8 字节数与 UTF-16 码元数 |
| `blob-metadata.json` | 自定义元数据名的文法，值里的控制字符，名与值合计 2048 个 UTF-8 字节的上限两侧 |
| `blob-content-types.json` | 内容类型文法的合法与不合法样例：`;` 两侧的空格、`;` 之后为空的各种写法（合法）、以空格结尾的写法与制表符（不合法），含 255 个字符的上限两侧，以及正好 255 个字符、只因以空格结尾而不合法的一条 |
| `blob-ranges.json` | 范围读取的实际范围与不可满足的范围 |
| `download-file-names.json` | 预签名下载地址上保存的文件名：空白、控制字符，255 字节的上限两侧，含码元数不超而字节数超过的 |
| `multipart-parts.json` | 分片编号的范围、分片的最小字节数与完成清单的编号次序 |
| `store-definitions.json` | 命名存储实例名的文法、内置的 Provider 名、两个定义是不是同一个定义 |

向量不收的：不能编码成 UTF-8 的键与文件名（不成对的代理项）算不算合法，正文没有写；空的元数据值；预签名的有效期、列举的每页条数（都只是数值范围）；宿主代签地址的签名（正文没有给出规范串）；完成清单里 ETag 与分片大小的核对、配额与并发（由 `BlobStoreConformance` 与 `BlobMultipartStoreConformance` 核对）；S3 Provider 的配置写法（服务端点、代理主机、成组的设置，2026-11-25 写明，ADR-0302）：只有 .NET 的 S3 Provider 读这些配置，由它自己的测试核对。

## 错误码

见 `conventions/error-codes.md` 中 `OBJECTSTORAGE_*`。
