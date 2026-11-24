package conformance

import "encoding/json"

// 本文件是各子系统档（messaging、multitenancy、objectstorage 等）向量的类型与类型化入口；
// 文件名仍只写在 vectors.go 的向量文件表里。

// TopicDeadLetterCase 是一次死信主题的派生：Suffix 为 nil 表示用缺省后缀。
type TopicDeadLetterCase struct {
	Topic      string  `json:"topic"`
	Suffix     *string `json:"suffix"`
	DeadLetter string  `json:"deadLetter"`
}

// TopicDeadLetterVectors 是死信主题的缺省后缀与派生样例。
type TopicDeadLetterVectors struct {
	DefaultSuffix string                `json:"defaultSuffix"`
	Cases         []TopicDeadLetterCase `json:"cases"`
}

// TopicVectors 对应 messaging/topics.json（RFC-0009「信封」，ADR-0277）：主题文法的合法与不合法样例、长度上限与死信主题。
type TopicVectors struct {
	MaxLength  int                    `json:"maxLength"`
	Valid      []string               `json:"valid"`
	Invalid    []string               `json:"invalid"`
	DeadLetter TopicDeadLetterVectors `json:"deadLetter"`
}

// EnvelopeExtensionNames 是扩展属性名的长度上限与合法、不合法样例。
type EnvelopeExtensionNames struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// EnvelopeBaggageMapping 是一个扩展属性与它对应的 Baggage 键。
type EnvelopeBaggageMapping struct {
	Extension  string `json:"extension"`
	BaggageKey string `json:"baggageKey"`
}

// EnvelopeExtensionVectors 对应 messaging/envelope-extensions.json（RFC-0009「信封」，ADR-0277）：
// 扩展属性名的文法、扩展与 Baggage 键的对应、承载追踪与分区键的扩展，以及不进入消息的 Baggage 键。
type EnvelopeExtensionVectors struct {
	Names        EnvelopeExtensionNames   `json:"names"`
	Baggage      []EnvelopeBaggageMapping `json:"baggage"`
	Trace        string                   `json:"trace"`
	PartitionKey string                   `json:"partitionKey"`
	NotCarried   []string                 `json:"notCarried"`
}

// MessagingInstanceIDVectors 对应 messaging/instance-ids.json（RFC-0009「订阅方式」第 6 条，ADR-0277）。
type MessagingInstanceIDVectors struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// OutboxBackoffDefaults 是发件箱三个选项的缺省值。
type OutboxBackoffDefaults struct {
	RetryDelayMs    int64 `json:"retryDelayMs"`
	MaxRetryDelayMs int64 `json:"maxRetryDelayMs"`
	MaxAttempts     int   `json:"maxAttempts"`
}

// OutboxBackoffCase 是一次失败之后的退避间隔：Attempts 是已经失败的尝试次数，DelayMs 是推后的毫秒数。
type OutboxBackoffCase struct {
	Name            string `json:"name"`
	RetryDelayMs    int64  `json:"retryDelayMs"`
	MaxRetryDelayMs int64  `json:"maxRetryDelayMs"`
	Attempts        int    `json:"attempts"`
	DelayMs         int64  `json:"delayMs"`
}

// OutboxDeadLetterCase 是一次进死信的判定。
type OutboxDeadLetterCase struct {
	MaxAttempts  int  `json:"maxAttempts"`
	Attempts     int  `json:"attempts"`
	DeadLettered bool `json:"deadLettered"`
}

// OutboxBackoffVectors 对应 messaging/outbox-backoff.json（RFC-0009「发件箱」，ADR-0277）。
type OutboxBackoffVectors struct {
	Defaults   OutboxBackoffDefaults  `json:"defaults"`
	Cases      []OutboxBackoffCase    `json:"cases"`
	DeadLetter []OutboxDeadLetterCase `json:"deadLetter"`
}

// CloudEventExpectation 是结构化 JSON 文档读出的信封：Time 是 UTC 时刻（nil 表示没有），
// Data 是内联的 JSON 数据，DataBase64 是 data_base64 里的字节（标准 Base64），两者都没有表示没有数据。
type CloudEventExpectation struct {
	ID              string            `json:"id"`
	Source          string            `json:"source"`
	Type            string            `json:"type"`
	Time            *string           `json:"time"`
	Subject         *string           `json:"subject"`
	DataContentType string            `json:"datacontenttype"`
	Data            json.RawMessage   `json:"data,omitempty"`
	DataBase64      *string           `json:"dataBase64,omitempty"`
	Extensions      map[string]string `json:"extensions"`
}

// CloudEventCase 是一条必须读得出的结构化 JSON 文档与读出的信封。
type CloudEventCase struct {
	Name     string                `json:"name"`
	Text     string                `json:"text"`
	Expected CloudEventExpectation `json:"expected"`
}

// CloudEventRejection 是一条必须拒绝的结构化 JSON 文档。
type CloudEventRejection struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// CloudEventEnvelopeVectors 对应 messaging/cloudevents-envelopes.json（RFC-0009「信封」，protocol/cloudevents.md，ADR-0277）：
// ContentType 是结构化编码的内容类型，Code 是拒绝时的错误码。
type CloudEventEnvelopeVectors struct {
	ContentType string                `json:"contentType"`
	Code        string                `json:"code"`
	Valid       []CloudEventCase      `json:"valid"`
	Invalid     []CloudEventRejection `json:"invalid"`
}

// Topics 读取消息主题文法与死信主题的向量。
func Topics() (TopicVectors, error) {
	return typed[TopicVectors]()
}

// EnvelopeExtensions 读取信封扩展属性的向量。
func EnvelopeExtensions() (EnvelopeExtensionVectors, error) {
	return typed[EnvelopeExtensionVectors]()
}

// MessagingInstanceIDs 读取消息子系统实例标识文法的向量。
func MessagingInstanceIDs() (MessagingInstanceIDVectors, error) {
	return typed[MessagingInstanceIDVectors]()
}

// OutboxBackoff 读取发件箱退避间隔与进死信时机的向量。
func OutboxBackoff() (OutboxBackoffVectors, error) {
	return typed[OutboxBackoffVectors]()
}

// CloudEventEnvelopes 读取信封的 CloudEvents 结构化 JSON 编码向量。
func CloudEventEnvelopes() (CloudEventEnvelopeVectors, error) {
	return typed[CloudEventEnvelopeVectors]()
}

// TenantIDVectors 对应 multitenancy/tenant-ids.json（RFC-0007「租户 ID」，ADR-0277）。
type TenantIDVectors struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// TenantDirectoryEntry 是租户存储里的一个租户：DisplayName 为 nil 表示没有显示名，Enabled 为假即禁用。
type TenantDirectoryEntry struct {
	ID          string  `json:"id"`
	DisplayName *string `json:"displayName"`
	Enabled     bool    `json:"enabled"`
}

// TenantResolutionOptions 是 Tenant 阶段的三个选项：DefaultTenant 为 nil 表示没有默认租户。
type TenantResolutionOptions struct {
	DefaultTenant        *string `json:"defaultTenant"`
	RequireTenant        bool    `json:"requireTenant"`
	ValidateAgainstStore bool    `json:"validateAgainstStore"`
}

// TenantResolutionExpectation 是 Tenant 阶段的裁定：Error 非空是结束调用的错误码；
// 否则 Tenant 是写进安全上下文的租户 ID（nil 表示无租户继续），DisplayName 是随之带入的显示名。
type TenantResolutionExpectation struct {
	Tenant      *string `json:"tenant"`
	DisplayName *string `json:"displayName"`
	Error       string  `json:"error"`
}

// TenantResolutionCase 是一次调用与 Tenant 阶段对它的裁定：Claims 是已认证主体的 tenant 声明的值，
// Baggage 是 Baggage 里的 xihan.tenant（nil 表示没有）。
type TenantResolutionCase struct {
	Name             string                      `json:"name"`
	Authenticated    bool                        `json:"authenticated"`
	Claims           []string                    `json:"claims"`
	Baggage          *string                     `json:"baggage"`
	PlatformCall     bool                        `json:"platformCall"`
	Options          TenantResolutionOptions     `json:"options"`
	StoreUnavailable bool                        `json:"storeUnavailable"`
	Expected         TenantResolutionExpectation `json:"expected"`
}

// TenantResolutionVectors 对应 multitenancy/tenant-resolution.json（RFC-0007「解析」，ADR-0277）。
type TenantResolutionVectors struct {
	Tenants []TenantDirectoryEntry `json:"tenants"`
	Cases   []TenantResolutionCase `json:"cases"`
}

// TenantShardExpectation 是从租户属性推出的分片：Provider 为 nil 表示没有提供者名。
type TenantShardExpectation struct {
	ConnectionString string            `json:"connectionString"`
	Provider         *string           `json:"provider"`
	Properties       map[string]string `json:"properties"`
}

// TenantShardCase 是一组租户属性与推出的分片：Shard 为 nil 表示共库。
type TenantShardCase struct {
	Name       string                  `json:"name"`
	Properties map[string]string       `json:"properties"`
	Shard      *TenantShardExpectation `json:"shard"`
}

// TenantShardVectors 对应 multitenancy/tenant-shards.json（RFC-0007「分片」，ADR-0277）。
type TenantShardVectors struct {
	ConnectionStringProperty string            `json:"connectionStringProperty"`
	ProviderProperty         string            `json:"providerProperty"`
	PassThroughPrefix        string            `json:"passThroughPrefix"`
	Cases                    []TenantShardCase `json:"cases"`
}

// TenantIDs 读取租户 ID 文法的向量。
func TenantIDs() (TenantIDVectors, error) {
	return typed[TenantIDVectors]()
}

// TenantResolution 读取 Tenant 阶段裁定的向量。
func TenantResolution() (TenantResolutionVectors, error) {
	return typed[TenantResolutionVectors]()
}

// TenantShards 读取从租户属性推出分片的向量。
func TenantShards() (TenantShardVectors, error) {
	return typed[TenantShardVectors]()
}

// ContainerNameVectors 对应 objectstorage/container-names.json（RFC-0012「标识」，ADR-0277）。
type ContainerNameVectors struct {
	MinLength int      `json:"minLength"`
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// BlobKeyCase 是一条对象键的样例：Key 是原文，UTF8Bytes 是它的 UTF-8 字节数，
// UTF16Units 是 UTF-16 码元数（不参与判定，只标出按码元计会判错的用例）。
type BlobKeyCase struct {
	Name       string `json:"name"`
	Key        string `json:"key"`
	UTF8Bytes  int    `json:"utf8Bytes"`
	UTF16Units int    `json:"utf16Units"`
}

// BlobKeyPrefixCase 是一条查询前缀的样例，两个计数同 BlobKeyCase。
type BlobKeyPrefixCase struct {
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	UTF8Bytes  int    `json:"utf8Bytes"`
	UTF16Units int    `json:"utf16Units"`
}

// BlobKeyPrefixSet 是必须接受与必须拒绝的查询前缀。
type BlobKeyPrefixSet struct {
	Valid   []BlobKeyPrefixCase `json:"valid"`
	Invalid []BlobKeyPrefixCase `json:"invalid"`
}

// BlobKeyVectors 对应 objectstorage/blob-keys.json（RFC-0012「标识」，ADR-0277、ADR-0282）：MaxBytes 是键与查询前缀
// 按 UTF-8 编码后的最大字节数；Valid 必须接受，Invalid 与 Reserved 必须拒绝，Reserved 另外要被认作保留前缀下的键；
// Prefixes 是查询前缀。
type BlobKeyVectors struct {
	MaxBytes        int              `json:"maxBytes"`
	Separator       string           `json:"separator"`
	ReservedSegment string           `json:"reservedSegment"`
	Valid           []BlobKeyCase    `json:"valid"`
	Invalid         []BlobKeyCase    `json:"invalid"`
	Reserved        []BlobKeyCase    `json:"reserved"`
	Prefixes        BlobKeyPrefixSet `json:"prefixes"`
}

// DownloadFileNameCase 是一条保存的文件名的样例：Text 是原文，两个计数同 BlobKeyCase。
type DownloadFileNameCase struct {
	Name       string `json:"name"`
	Text       string `json:"text"`
	UTF8Bytes  int    `json:"utf8Bytes"`
	UTF16Units int    `json:"utf16Units"`
}

// DownloadFileNameVectors 对应 objectstorage/download-file-names.json（RFC-0012「范围读取、分片上传与预签名」，ADR-0282）：
// MaxBytes 是文件名按 UTF-8 编码后的最大字节数，Valid 必须接受，Invalid 必须拒绝。
type DownloadFileNameVectors struct {
	MaxBytes int                    `json:"maxBytes"`
	Valid    []DownloadFileNameCase `json:"valid"`
	Invalid  []DownloadFileNameCase `json:"invalid"`
}

// NameGrammarVectors 是一条名字文法的长度上限与合法、不合法样例。
type NameGrammarVectors struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// BlobMetadataCase 是一个对象的整组自定义元数据：UTF8Bytes 是名与值的 UTF-8 字节数之和。
type BlobMetadataCase struct {
	Name      string            `json:"name"`
	Metadata  map[string]string `json:"metadata"`
	UTF8Bytes int               `json:"utf8Bytes"`
}

// BlobMetadataSets 是必须接受与必须拒绝的整组元数据。
type BlobMetadataSets struct {
	Valid   []BlobMetadataCase `json:"valid"`
	Invalid []BlobMetadataCase `json:"invalid"`
}

// BlobMetadataVectors 对应 objectstorage/blob-metadata.json（RFC-0012「标识」，ADR-0277）：
// Names 是元数据名的文法，MaxBytes 是名与值合计的最大 UTF-8 字节数，Sets 是整组元数据。
type BlobMetadataVectors struct {
	Names    NameGrammarVectors `json:"names"`
	MaxBytes int                `json:"maxBytes"`
	Sets     BlobMetadataSets   `json:"sets"`
}

// BlobContentTypeVectors 对应 objectstorage/blob-content-types.json（RFC-0012「标识」，ADR-0277、ADR-0282）。
type BlobContentTypeVectors struct {
	MaxLength int      `json:"maxLength"`
	Default   string   `json:"default"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// BlobRangeExpectation 是范围读取实际读出的范围。
type BlobRangeExpectation struct {
	Offset int64 `json:"offset"`
	Length int64 `json:"length"`
	Total  int64 `json:"total"`
}

// BlobRangeCase 是一次范围读取：Length 为 nil 表示读到末尾，Range 为 nil 表示起点不在对象内。
type BlobRangeCase struct {
	Name   string                `json:"name"`
	Offset int64                 `json:"offset"`
	Length *int64                `json:"length"`
	Total  int64                 `json:"total"`
	Range  *BlobRangeExpectation `json:"range"`
}

// BlobRangeVectors 对应 objectstorage/blob-ranges.json（RFC-0012「范围读取、分片上传与预签名」，ADR-0277）：Code 是范围不可满足时的错误码。
type BlobRangeVectors struct {
	Code  string          `json:"code"`
	Cases []BlobRangeCase `json:"cases"`
}

// MultipartPartNumbers 是在范围内与不在范围内的分片编号。
type MultipartPartNumbers struct {
	Valid   []int `json:"valid"`
	Invalid []int `json:"invalid"`
}

// MultipartManifestCase 是一份完成清单里各片的编号，以及编号次序是否合法。
type MultipartManifestCase struct {
	Name  string `json:"name"`
	Parts []int  `json:"parts"`
	Valid bool   `json:"valid"`
}

// MultipartPartVectors 对应 objectstorage/multipart-parts.json（RFC-0012「范围读取、分片上传与预签名」，ADR-0277）：
// Code 是清单不合法时的错误码。
type MultipartPartVectors struct {
	MaxPartNumber int                     `json:"maxPartNumber"`
	MinPartBytes  int64                   `json:"minPartBytes"`
	Code          string                  `json:"code"`
	PartNumbers   MultipartPartNumbers    `json:"partNumbers"`
	Manifests     []MultipartManifestCase `json:"manifests"`
}

// BlobStoreDefinitionCase 是一个命名存储实例的定义。
type BlobStoreDefinitionCase struct {
	Name     string            `json:"name"`
	Provider string            `json:"provider"`
	Settings map[string]string `json:"settings"`
}

// BlobStoreSameCase 是两个定义与它们是不是同一个定义。
type BlobStoreSameCase struct {
	Name  string                  `json:"name"`
	Left  BlobStoreDefinitionCase `json:"left"`
	Right BlobStoreDefinitionCase `json:"right"`
	Same  bool                    `json:"same"`
}

// BlobStoreDefinitionVectors 对应 objectstorage/store-definitions.json（RFC-0012「命名存储实例」，ADR-0277）。
type BlobStoreDefinitionVectors struct {
	Names     NameGrammarVectors  `json:"names"`
	Providers []string            `json:"providers"`
	Same      []BlobStoreSameCase `json:"same"`
}

// ContainerNames 读取容器名文法的向量。
func ContainerNames() (ContainerNameVectors, error) {
	return typed[ContainerNameVectors]()
}

// BlobKeys 读取对象键与查询前缀的文法、保留前缀的向量。
func BlobKeys() (BlobKeyVectors, error) {
	return typed[BlobKeyVectors]()
}

// DownloadFileNames 读取预签名下载地址上保存的文件名的向量。
func DownloadFileNames() (DownloadFileNameVectors, error) {
	return typed[DownloadFileNameVectors]()
}

// BlobMetadata 读取对象自定义元数据的向量。
func BlobMetadata() (BlobMetadataVectors, error) {
	return typed[BlobMetadataVectors]()
}

// BlobContentTypes 读取对象内容类型文法的向量。
func BlobContentTypes() (BlobContentTypeVectors, error) {
	return typed[BlobContentTypeVectors]()
}

// BlobRanges 读取范围读取的向量。
func BlobRanges() (BlobRangeVectors, error) {
	return typed[BlobRangeVectors]()
}

// MultipartParts 读取分片编号与完成清单的向量。
func MultipartParts() (MultipartPartVectors, error) {
	return typed[MultipartPartVectors]()
}

// BlobStoreDefinitions 读取命名存储实例定义的向量。
func BlobStoreDefinitions() (BlobStoreDefinitionVectors, error) {
	return typed[BlobStoreDefinitionVectors]()
}

// JobNameVectors 对应 scheduling/job-names.json（RFC-0013「作业与触发器」，ADR-0277）。
type JobNameVectors struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// CronCase 是一条 cron 表达式与从 After 起依次推算的触发时刻（UTC）：TimeZone 为 nil 表示 UTC，
// Exhausted 为真表示 Next 列完之后再没有下一次。
type CronCase struct {
	Name       string   `json:"name"`
	Expression string   `json:"expression"`
	TimeZone   *string  `json:"timeZone"`
	After      string   `json:"after"`
	Next       []string `json:"next"`
	Exhausted  bool     `json:"exhausted"`
}

// CronRejection 是一条必须拒绝的 cron 表达式。
type CronRejection struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

// CronExpressionVectors 对应 scheduling/cron-expressions.json（RFC-0013「作业与触发器」的 cron，ADR-0277）。
type CronExpressionVectors struct {
	Cases   []CronCase      `json:"cases"`
	Invalid []CronRejection `json:"invalid"`
}

// JobRetryWaitCase 是一次重试前的等待：Backoff 是 Fixed 或 Exponential，MaxDelayMs 为 nil 表示不设上限，Retry 从 1 起。
type JobRetryWaitCase struct {
	Name       string `json:"name"`
	Backoff    string `json:"backoff"`
	DelayMs    int64  `json:"delayMs"`
	MaxDelayMs *int64 `json:"maxDelayMs"`
	Retry      int    `json:"retry"`
	WaitMs     int64  `json:"waitMs"`
}

// JobRetryPolicyCase 是一组重试策略的取值与它是否合法。
type JobRetryPolicyCase struct {
	Name       string `json:"name"`
	MaxRetries int    `json:"maxRetries"`
	DelayMs    int64  `json:"delayMs"`
	MaxDelayMs *int64 `json:"maxDelayMs"`
	Valid      bool   `json:"valid"`
}

// JobRetryDelayVectors 对应 scheduling/retry-delays.json（RFC-0013「作业与触发器」的重试，ADR-0277）：
// MaxRetries 是重试次数的上限，LimitMs 是等待与上限的上限（毫秒）。
type JobRetryDelayVectors struct {
	MaxRetries int                  `json:"maxRetries"`
	LimitMs    int64                `json:"limitMs"`
	Waits      []JobRetryWaitCase   `json:"waits"`
	Policies   []JobRetryPolicyCase `json:"policies"`
}

// SchedulingEventTopicVectors 对应 scheduling/event-topics.json（RFC-0013「引擎」的事件，ADR-0277）。
type SchedulingEventTopicVectors struct {
	Source string   `json:"source"`
	Topics []string `json:"topics"`
}

// JobNames 读取调度里名字文法的向量。
func JobNames() (JobNameVectors, error) {
	return typed[JobNameVectors]()
}

// CronExpressions 读取 cron 表达式与下一次触发的向量。
func CronExpressions() (CronExpressionVectors, error) {
	return typed[CronExpressionVectors]()
}

// JobRetryDelays 读取作业重试的等待与重试策略取值范围的向量。
func JobRetryDelays() (JobRetryDelayVectors, error) {
	return typed[JobRetryDelayVectors]()
}

// SchedulingEventTopics 读取调度事件主题的向量。
func SchedulingEventTopics() (SchedulingEventTopicVectors, error) {
	return typed[SchedulingEventTopicVectors]()
}

// LocaleNameCase 是一个合法的区域名、它的规范形式与语言子标签。
type LocaleNameCase struct {
	Text      string `json:"text"`
	Canonical string `json:"canonical"`
	Language  string `json:"language"`
}

// LocaleNameVectors 对应 localization/locale-names.json（RFC-0010「区域」，ADR-0277）：
// BuiltIn 是内置的区域，Default 是缺省的默认区域，Longest 是正好到长度上限的合法区域名。
type LocaleNameVectors struct {
	MaxLength int              `json:"maxLength"`
	Default   string           `json:"default"`
	BuiltIn   []string         `json:"builtIn"`
	Valid     []LocaleNameCase `json:"valid"`
	Longest   string           `json:"longest"`
	Invalid   []string         `json:"invalid"`
}

// LocaleNegotiationCase 是一次区域协商：Candidates 是调用方给出的候选，Locale 是协商出的区域。
type LocaleNegotiationCase struct {
	Name       string   `json:"name"`
	Enabled    []string `json:"enabled"`
	Default    string   `json:"default"`
	Candidates []string `json:"candidates"`
	Locale     string   `json:"locale"`
}

// LocaleFallbackCase 是一次取文案的回退链：Locale 是请求的区域，Chain 是依次查找的区域。
type LocaleFallbackCase struct {
	Name    string   `json:"name"`
	Enabled []string `json:"enabled"`
	Default string   `json:"default"`
	Locale  string   `json:"locale"`
	Chain   []string `json:"chain"`
}

// LocaleNegotiationVectors 对应 localization/locale-negotiation.json（RFC-0010「协商与宿主」「解析」，ADR-0277）。
type LocaleNegotiationVectors struct {
	Negotiation []LocaleNegotiationCase `json:"negotiation"`
	Fallback    []LocaleFallbackCase    `json:"fallback"`
}

// CatalogDefinition 是一份消息目录：Messages 是区域到「键 → 模板」的对象。
type CatalogDefinition struct {
	Name     string                       `json:"name"`
	Messages map[string]map[string]string `json:"messages"`
}

// CatalogLookupExpectation 是查到的文案与实际命中的区域。
type CatalogLookupExpectation struct {
	Text   string `json:"text"`
	Locale string `json:"locale"`
}

// CatalogLookup 是一次查找：Expected 为 nil 表示哪一级都没有这个键。
type CatalogLookup struct {
	Key      string                    `json:"key"`
	Locale   string                    `json:"locale"`
	Expected *CatalogLookupExpectation `json:"expected"`
}

// CatalogResolutionCase 是按次序注册的一组目录：Conflict 为真表示注册必须失败，否则 Requests 是合并之后的查找。
type CatalogResolutionCase struct {
	Name     string              `json:"name"`
	Default  string              `json:"default"`
	Catalogs []CatalogDefinition `json:"catalogs"`
	Conflict bool                `json:"conflict"`
	Requests []CatalogLookup     `json:"requests"`
}

// CatalogResolutionVectors 对应 localization/catalog-resolution.json（RFC-0010「语言文件」「解析」，ADR-0277）：Code 是冲突时的错误码。
type CatalogResolutionVectors struct {
	Code  string                  `json:"code"`
	Cases []CatalogResolutionCase `json:"cases"`
}

// LocaleNames 读取区域名文法与规范形式的向量。
func LocaleNames() (LocaleNameVectors, error) {
	return typed[LocaleNameVectors]()
}

// LocaleNegotiation 读取区域协商与回退链的向量。
func LocaleNegotiation() (LocaleNegotiationVectors, error) {
	return typed[LocaleNegotiationVectors]()
}

// CatalogResolution 读取消息目录的合并与查找的向量。
func CatalogResolution() (CatalogResolutionVectors, error) {
	return typed[CatalogResolutionVectors]()
}

// SecretNameVectors 对应 configuration/secret-names.json（RFC-0014「密钥」，ADR-0277）。
type SecretNameVectors struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// RemoteDocumentCase 是一份远程配置文档：Format 是 Json 或 Properties，Content 是原文，
// Version 是它的版本，Values 是展平后的键到值（nil 是保留下来的空值）。
type RemoteDocumentCase struct {
	Name    string             `json:"name"`
	Format  string             `json:"format"`
	Content string             `json:"content"`
	Version string             `json:"version"`
	Values  map[string]*string `json:"values"`
}

// RemoteDocumentRejection 是一份不合声明格式的文档。
type RemoteDocumentRejection struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

// RemoteDocumentVectors 对应 configuration/remote-documents.json（RFC-0014「远程配置」「Nacos Provider」，ADR-0277、ADR-0286）：Code 是文档不合格式时的错误码。
type RemoteDocumentVectors struct {
	Code    string                    `json:"code"`
	Cases   []RemoteDocumentCase      `json:"cases"`
	Invalid []RemoteDocumentRejection `json:"invalid"`
}

// SecretNames 读取密钥名文法的向量。
func SecretNames() (SecretNameVectors, error) {
	return typed[SecretNameVectors]()
}

// RemoteConfigurationDocuments 读取远程配置文档的展平与版本的向量。
func RemoteConfigurationDocuments() (RemoteDocumentVectors, error) {
	return typed[RemoteDocumentVectors]()
}

// IndexAbbreviationCase 是一个属性名与它在索引名里的缩写。
type IndexAbbreviationCase struct {
	Property     string `json:"property"`
	Abbreviation string `json:"abbreviation"`
}

// IndexNameCase 是一个索引声明与推出的名字：Index 为 nil 表示名字超长，是声明错误。
type IndexNameCase struct {
	Name       string   `json:"name"`
	Table      string   `json:"table"`
	Properties []string `json:"properties"`
	Unique     bool     `json:"unique"`
	Index      *string  `json:"index"`
}

// ExplicitIndexNames 是声明里给出的索引名的合法与不合法样例。
type ExplicitIndexNames struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

// IndexNameVectors 对应 persistence/index-names.json（RFC-0005「索引声明」，ADR-0277）。
type IndexNameVectors struct {
	MaxLength     int                     `json:"maxLength"`
	Prefix        string                  `json:"prefix"`
	UniquePrefix  string                  `json:"uniquePrefix"`
	Abbreviations []IndexAbbreviationCase `json:"abbreviations"`
	Names         []IndexNameCase         `json:"names"`
	ExplicitNames ExplicitIndexNames      `json:"explicitNames"`
}

// SnowflakeIDCase 是一个雪花标识与它的三段：Value 与 Milliseconds 是十进制字符串（值会超过 2^53），Time 是 Epoch 加毫秒数的时刻。
type SnowflakeIDCase struct {
	Name         string `json:"name"`
	Epoch        string `json:"epoch"`
	Value        string `json:"value"`
	Milliseconds string `json:"milliseconds"`
	Worker       int    `json:"worker"`
	Sequence     int    `json:"sequence"`
	Time         string `json:"time"`
}

// SnowflakePartsCase 是合成时必须拒绝的三段。
type SnowflakePartsCase struct {
	Name         string `json:"name"`
	Milliseconds string `json:"milliseconds"`
	Worker       int    `json:"worker"`
	Sequence     int    `json:"sequence"`
}

// SnowflakeIDVectors 对应 persistence/snowflake-ids.json（RFC-0005「雪花标识」，ADR-0277）。
type SnowflakeIDVectors struct {
	DefaultEpoch     string               `json:"defaultEpoch"`
	TimestampBits    int                  `json:"timestampBits"`
	WorkerBits       int                  `json:"workerBits"`
	SequenceBits     int                  `json:"sequenceBits"`
	MaxWorker        int                  `json:"maxWorker"`
	MaxSequence      int                  `json:"maxSequence"`
	WorkerLockPrefix string               `json:"workerLockPrefix"`
	Cases            []SnowflakeIDCase    `json:"cases"`
	OutOfRange       []SnowflakePartsCase `json:"outOfRange"`
}

// IndexNames 读取索引名的向量。
func IndexNames() (IndexNameVectors, error) {
	return typed[IndexNameVectors]()
}

// SnowflakeIDs 读取雪花标识位布局的向量。
func SnowflakeIDs() (SnowflakeIDVectors, error) {
	return typed[SnowflakeIDVectors]()
}

// AuditExclusionEntries 是排除列表里合法与不合法的项。
type AuditExclusionEntries struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

// AuditExclusionMatch 是一份排除列表、一个操作 ID 与它是否被排除。
type AuditExclusionMatch struct {
	Name      string   `json:"name"`
	Exclude   []string `json:"exclude"`
	Operation string   `json:"operation"`
	Excluded  bool     `json:"excluded"`
}

// AuditExclusionVectors 对应 auditing/exclusions.json（RFC-0015「事件的产生」，ADR-0277）。
type AuditExclusionVectors struct {
	Entries AuditExclusionEntries `json:"entries"`
	Matches []AuditExclusionMatch `json:"matches"`
}

// AuditOptionsCase 是审计的三个选项。
type AuditOptionsCase struct {
	Selection        string   `json:"selection"`
	IncludeSuccesses bool     `json:"includeSuccesses"`
	Exclude          []string `json:"exclude"`
}

// AuditOperationCase 是一次调用的当前操作：Permission 与 Policy 为 nil 表示契约没有声明。
type AuditOperationCase struct {
	ID         string  `json:"id"`
	Permission *string `json:"permission"`
	Policy     *string `json:"policy"`
}

// AuditEventExpectation 是记下的事件：动作、是否成功与失败时的错误码。
type AuditEventExpectation struct {
	Action    string  `json:"action"`
	Succeeded bool    `json:"succeeded"`
	Code      *string `json:"code"`
}

// AuditSelectionCase 是一次调用与它记成的事件：Operation 为 nil 表示不是操作调用，Expected 为 nil 表示不记。
type AuditSelectionCase struct {
	Name           string                 `json:"name"`
	Options        AuditOptionsCase       `json:"options"`
	Operation      *AuditOperationCase    `json:"operation"`
	ExplicitAction *string                `json:"explicitAction"`
	Outcome        string                 `json:"outcome"`
	Expected       *AuditEventExpectation `json:"expected"`
}

// AuditAttributionExpectation 是事件上的主体与租户，nil 表示匿名与全局。
type AuditAttributionExpectation struct {
	Principal *string `json:"principal"`
	Tenant    *string `json:"tenant"`
}

// AuditAttributionCase 是一次调用的主体与租户来源：安全上下文里的与 Baggage 里的，以及 Tenant 阶段是否裁定过租户。
type AuditAttributionCase struct {
	Name             string                      `json:"name"`
	Principal        *string                     `json:"principal"`
	BaggagePrincipal *string                     `json:"baggagePrincipal"`
	TenantResolved   bool                        `json:"tenantResolved"`
	Tenant           *string                     `json:"tenant"`
	BaggageTenant    *string                     `json:"baggageTenant"`
	Expected         AuditAttributionExpectation `json:"expected"`
}

// AuditSelectionDefaults 是 Selection 与 IncludeSuccesses 的缺省值。
type AuditSelectionDefaults struct {
	Selection        string `json:"selection"`
	IncludeSuccesses bool   `json:"includeSuccesses"`
}

// AuditEventSelectionVectors 对应 auditing/event-selection.json（RFC-0015「事件的产生」，ADR-0277）。
type AuditEventSelectionVectors struct {
	Defaults    AuditSelectionDefaults `json:"defaults"`
	Selection   []AuditSelectionCase   `json:"selection"`
	Attribution []AuditAttributionCase `json:"attribution"`
}

// AuditExclusions 读取审计按操作排除的向量。
func AuditExclusions() (AuditExclusionVectors, error) {
	return typed[AuditExclusionVectors]()
}

// AuditEventSelection 读取审计事件的选择、结果与归属的向量。
func AuditEventSelection() (AuditEventSelectionVectors, error) {
	return typed[AuditEventSelectionVectors]()
}

// WorkflowNameVectors 对应 workflow/workflow-names.json（RFC-0021「名字与值」，ADR-0277）：定义名、节点与转移的 ID、活动类型三条文法。
type WorkflowNameVectors struct {
	DefinitionNames NameGrammarVectors `json:"definitionNames"`
	ElementIDs      NameGrammarVectors `json:"elementIds"`
	ActivityTypes   NameGrammarVectors `json:"activityTypes"`
}

// WorkflowStatusOrderVectors 对应 workflow/status-order.json（RFC-0021「定义」的「列表排序」，ADR-0277）：两张表都按生命周期的先后升序。
type WorkflowStatusOrderVectors struct {
	Definition []string `json:"definition"`
	Instance   []string `json:"instance"`
}

// WorkflowKeywordNormalization 是一个关键字与折叠之后用来匹配的文本：Keyword 为 nil 表示没有给，Normalized 为 nil 表示不过滤。
type WorkflowKeywordNormalization struct {
	Name       string  `json:"name"`
	Keyword    *string `json:"keyword"`
	Normalized *string `json:"normalized"`
}

// WorkflowKeywordMatch 是一个关键字、被搜的各个字段（nil 是没有值的字段）与是否匹配。
type WorkflowKeywordMatch struct {
	Name    string    `json:"name"`
	Keyword string    `json:"keyword"`
	Fields  []*string `json:"fields"`
	Matches bool      `json:"matches"`
}

// WorkflowKeywordVectors 对应 workflow/keyword-matching.json（RFC-0021「定义」的「关键字」，ADR-0277）。
type WorkflowKeywordVectors struct {
	FieldSeparator string                         `json:"fieldSeparator"`
	Normalize      []WorkflowKeywordNormalization `json:"normalize"`
	Matches        []WorkflowKeywordMatch         `json:"matches"`
}

// WorkflowEventTopicVectors 对应 workflow/workflow-event-topics.json（RFC-0021「实例与执行」的「事件」，ADR-0277）。
type WorkflowEventTopicVectors struct {
	Source string   `json:"source"`
	Topics []string `json:"topics"`
}

// WorkflowExpressionValue 是一条求值成功的表达式用例（RFC-0021「表达式」，ADR-0287）：Type 是结果的类型
// （null、boolean、number、string、array、object 之一），Text 是结果的文本（「值的文本」）。
type WorkflowExpressionValue struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Type       string `json:"type"`
	Text       string `json:"text"`
}

// WorkflowExpressionError 是一条求值失败的表达式用例：Syntax 为真表示解析时就报出，
// Arguments 是错误的参数，不含 expression（它就是 Expression，超过 200 个码元时截断）。
type WorkflowExpressionError struct {
	Name       string         `json:"name"`
	Expression string         `json:"expression"`
	Syntax     bool           `json:"syntax"`
	MessageKey string         `json:"messageKey"`
	Arguments  map[string]any `json:"arguments"`
}

// WorkflowExpressionCases 是表达式向量文件共有的部分：错误码、求值时的实例变量（原样的 JSON 文本，
// 数字的写法不经 float64 走样）、求值成功的用例与求值失败的用例。
type WorkflowExpressionCases struct {
	ErrorCode string                    `json:"errorCode"`
	Variables json.RawMessage           `json:"variables"`
	Values    []WorkflowExpressionValue `json:"values"`
	Errors    []WorkflowExpressionError `json:"errors"`
}

// WorkflowExpressionSyntaxVectors 对应 workflow/expression-syntax.json（RFC-0021「表达式」的「词法」「文法」，ADR-0287）。
type WorkflowExpressionSyntaxVectors struct {
	WorkflowExpressionCases
}

// WorkflowExpressionOperatorVectors 对应 workflow/expression-operators.json（RFC-0021「表达式」的「运算符」「成员与索引」，ADR-0287）。
type WorkflowExpressionOperatorVectors struct {
	WorkflowExpressionCases
}

// WorkflowExpressionFunctionVectors 对应 workflow/expression-functions.json（RFC-0021「表达式」的「内置函数」「时间」，ADR-0287）：
// Now 是求值时的时刻。
type WorkflowExpressionFunctionVectors struct {
	Now string `json:"now"`
	WorkflowExpressionCases
}

// WorkflowTemplateRender 是一条渲染成功的模板用例。
type WorkflowTemplateRender struct {
	Name     string `json:"name"`
	Template string `json:"template"`
	Text     string `json:"text"`
}

// WorkflowTemplateRenderError 是一条渲染失败的模板用例：Expression 是错误的 expression 参数
// （出错的占位里去掉首尾空白的文本；占位没有结束时是整个模板），Arguments 是其余的参数。
type WorkflowTemplateRenderError struct {
	Name       string         `json:"name"`
	Template   string         `json:"template"`
	MessageKey string         `json:"messageKey"`
	Expression string         `json:"expression"`
	Arguments  map[string]any `json:"arguments"`
}

// WorkflowCondition 是一条条件用例：表达式当条件求值的结果。
type WorkflowCondition struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Holds      bool   `json:"holds"`
}

// WorkflowExpressionTemplateVectors 对应 workflow/expression-templates.json（RFC-0021「表达式」的「模板」「条件」，ADR-0287）。
type WorkflowExpressionTemplateVectors struct {
	ErrorCode       string                        `json:"errorCode"`
	Now             string                        `json:"now"`
	Variables       json.RawMessage               `json:"variables"`
	Renders         []WorkflowTemplateRender      `json:"renders"`
	RenderErrors    []WorkflowTemplateRenderError `json:"renderErrors"`
	Conditions      []WorkflowCondition           `json:"conditions"`
	ConditionErrors []WorkflowExpressionError     `json:"conditionErrors"`
}

// WorkflowExpressionBlock 是一个很长的字符串变量：Variable 的值是 Repeat 个 Unit。
type WorkflowExpressionBlock struct {
	Variable string `json:"variable"`
	Unit     string `json:"unit"`
	Repeat   int    `json:"repeat"`
}

// WorkflowExpressionLength 是一条长度用例：表达式是 Head 接 Count 个 Padding 再接 Tail，
// Length 是它的 UTF-16 码元数，Valid 为假表示以 too_long 失败。
type WorkflowExpressionLength struct {
	Name    string `json:"name"`
	Head    string `json:"head"`
	Padding string `json:"padding"`
	Count   int    `json:"count"`
	Tail    string `json:"tail"`
	Length  int    `json:"length"`
	Valid   bool   `json:"valid"`
}

// WorkflowExpressionNesting 是一条嵌套用例：表达式是 Levels 个 Open、Leaf、Levels 个 Close，
// Valid 为假表示以 too_deep 失败。
type WorkflowExpressionNesting struct {
	Name   string `json:"name"`
	Open   string `json:"open"`
	Leaf   string `json:"leaf"`
	Close  string `json:"close"`
	Levels int    `json:"levels"`
	Valid  bool   `json:"valid"`
}

// WorkflowExpressionChain 是一条链的用例：表达式是 Head 接 Count 个 Link，求值成功，Type 与 Text 是结果。
type WorkflowExpressionChain struct {
	Name  string `json:"name"`
	Head  string `json:"head"`
	Link  string `json:"link"`
	Count int    `json:"count"`
	Type  string `json:"type"`
	Text  string `json:"text"`
}

// WorkflowExpressionConcatenation 是一条拼接上限的用例：Valid 为真时结果是长度为 Length 的字符串，
// 为假时以 string_too_long 失败、Length 是错误的 length 参数。
type WorkflowExpressionConcatenation struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Valid      bool   `json:"valid"`
	Length     int    `json:"length"`
}

// WorkflowExpressionRendering 是一条渲染上限的用例：Valid 为真时渲染出长度为 Length 的文本，
// 为假时以 string_too_long 失败、Length 是错误的 length 参数。
type WorkflowExpressionRendering struct {
	Name     string `json:"name"`
	Template string `json:"template"`
	Valid    bool   `json:"valid"`
	Length   int    `json:"length"`
}

// WorkflowExpressionEcho 是一条错误里带出文本的用例：Source 按 Mode（evaluate、condition、render）求值，
// 以 MessageKey 失败，Arguments 是全部参数，含截断的 expression。
type WorkflowExpressionEcho struct {
	Name       string         `json:"name"`
	Mode       string         `json:"mode"`
	Source     string         `json:"source"`
	MessageKey string         `json:"messageKey"`
	Arguments  map[string]any `json:"arguments"`
}

// WorkflowExpressionLimitVectors 对应 workflow/expression-limits.json（RFC-0021「表达式」的「长度与上限」，ADR-0287）。
type WorkflowExpressionLimitVectors struct {
	ErrorCode       string                            `json:"errorCode"`
	MaxLength       int                               `json:"maxLength"`
	MaxDepth        int                               `json:"maxDepth"`
	MaxStringLength int                               `json:"maxStringLength"`
	MaxEchoLength   int                               `json:"maxEchoLength"`
	Variables       json.RawMessage                   `json:"variables"`
	Blocks          []WorkflowExpressionBlock         `json:"blocks"`
	Lengths         []WorkflowExpressionLength        `json:"lengths"`
	Nestings        []WorkflowExpressionNesting       `json:"nestings"`
	Chains          []WorkflowExpressionChain         `json:"chains"`
	Concatenations  []WorkflowExpressionConcatenation `json:"concatenations"`
	Renderings      []WorkflowExpressionRendering     `json:"renderings"`
	Echoes          []WorkflowExpressionEcho          `json:"echoes"`
}

// WorkflowNames 读取工作流名字文法的向量。
func WorkflowNames() (WorkflowNameVectors, error) {
	return typed[WorkflowNameVectors]()
}

// WorkflowStatusOrder 读取工作流状态次序的向量。
func WorkflowStatusOrder() (WorkflowStatusOrderVectors, error) {
	return typed[WorkflowStatusOrderVectors]()
}

// WorkflowKeywords 读取列表查询关键字的折叠与匹配的向量。
func WorkflowKeywords() (WorkflowKeywordVectors, error) {
	return typed[WorkflowKeywordVectors]()
}

// WorkflowEventTopics 读取工作流事件主题的向量。
func WorkflowEventTopics() (WorkflowEventTopicVectors, error) {
	return typed[WorkflowEventTopicVectors]()
}

// WorkflowExpressionSyntax 读取表达式词法与文法的向量。
func WorkflowExpressionSyntax() (WorkflowExpressionSyntaxVectors, error) {
	return typed[WorkflowExpressionSyntaxVectors]()
}

// WorkflowExpressionOperators 读取表达式运算符、成员与索引的向量。
func WorkflowExpressionOperators() (WorkflowExpressionOperatorVectors, error) {
	return typed[WorkflowExpressionOperatorVectors]()
}

// WorkflowExpressionFunctions 读取表达式内置函数的向量。
func WorkflowExpressionFunctions() (WorkflowExpressionFunctionVectors, error) {
	return typed[WorkflowExpressionFunctionVectors]()
}

// WorkflowExpressionTemplates 读取模板与条件的向量。
func WorkflowExpressionTemplates() (WorkflowExpressionTemplateVectors, error) {
	return typed[WorkflowExpressionTemplateVectors]()
}

// WorkflowExpressionLimits 读取表达式上限与错误里带出的文本的向量。
func WorkflowExpressionLimits() (WorkflowExpressionLimitVectors, error) {
	return typed[WorkflowExpressionLimitVectors]()
}

// HostSampleIntervalVectors 是采样间隔的缺省值、上限与范围内外的样例（毫秒）。
type HostSampleIntervalVectors struct {
	DefaultMs int64   `json:"defaultMs"`
	MaxMs     int64   `json:"maxMs"`
	Valid     []int64 `json:"valid"`
	Invalid   []int64 `json:"invalid"`
}

// HostCPUSample 是一次读到的系统 CPU 累计空闲时间与总时间。
type HostCPUSample struct {
	Idle  uint64 `json:"idle"`
	Total uint64 `json:"total"`
}

// HostCPUCase 是先后两次采样与算出的系统 CPU 使用率：采样为 nil 表示没有读到，UsagePercent 为 nil 表示该项为空。
type HostCPUCase struct {
	Name         string         `json:"name"`
	Before       *HostCPUSample `json:"before"`
	After        *HostCPUSample `json:"after"`
	UsagePercent *float64       `json:"usagePercent"`
}

// HostProcessCase 是先后两次读到的进程处理器时间与算出的进程 CPU 使用率。
type HostProcessCase struct {
	Name               string   `json:"name"`
	Cores              int      `json:"cores"`
	ElapsedMs          float64  `json:"elapsedMs"`
	ProcessCPUBeforeMs float64  `json:"processCpuBeforeMs"`
	ProcessCPUAfterMs  float64  `json:"processCpuAfterMs"`
	UsagePercent       *float64 `json:"usagePercent"`
}

// HostNetworkCase 是先后两次读到的累计字节数与算出的速率（字节每秒）：计数为 nil 表示没有读到。
type HostNetworkCase struct {
	Name           string   `json:"name"`
	ElapsedMs      float64  `json:"elapsedMs"`
	Before         *int64   `json:"before"`
	After          *int64   `json:"after"`
	BytesPerSecond *float64 `json:"bytesPerSecond"`
}

// HostMemoryCase 是内存的总量、可用与由它们算出的已用、使用率：nil 表示读不到或算不出。
type HostMemoryCase struct {
	Name           string   `json:"name"`
	TotalBytes     *int64   `json:"totalBytes"`
	AvailableBytes *int64   `json:"availableBytes"`
	UsedBytes      *int64   `json:"usedBytes"`
	UsagePercent   *float64 `json:"usagePercent"`
}

// HostDiskCase 是磁盘的总量、空闲与由它们算出的已用、使用率：总量为零时使用率为 nil。
type HostDiskCase struct {
	Name         string   `json:"name"`
	TotalBytes   int64    `json:"totalBytes"`
	FreeBytes    int64    `json:"freeBytes"`
	UsedBytes    int64    `json:"usedBytes"`
	UsagePercent *float64 `json:"usagePercent"`
}

// HostLinuxCPUTimesCase 是 /proc/stat 的内容与读出的系统 CPU 累计时间：Times 为 nil 表示读不出。
type HostLinuxCPUTimesCase struct {
	Name  string         `json:"name"`
	Stat  string         `json:"stat"`
	Times *HostCPUSample `json:"times"`
}

// HostLinuxMemory 是读出的物理内存（字节），nil 表示读不到。
type HostLinuxMemory struct {
	TotalBytes     *int64 `json:"totalBytes"`
	AvailableBytes *int64 `json:"availableBytes"`
	FreeBytes      *int64 `json:"freeBytes"`
	CachedBytes    *int64 `json:"cachedBytes"`
}

// HostLinuxMemoryCase 是 /proc/meminfo 的内容与读出的物理内存。
type HostLinuxMemoryCase struct {
	Name    string          `json:"name"`
	Meminfo string          `json:"meminfo"`
	Memory  HostLinuxMemory `json:"memory"`
}

// HostLinuxUptimeCase 是 /proc/uptime 的内容与读出的系统运行时长（毫秒）：Milliseconds 为 nil 表示读不出。
type HostLinuxUptimeCase struct {
	Name         string `json:"name"`
	Uptime       string `json:"uptime"`
	Milliseconds *int64 `json:"milliseconds"`
}

// HostLinuxMount 是要读容量的一个挂载点：设备、挂载点与文件系统。
type HostLinuxMount struct {
	Device     string `json:"device"`
	MountPoint string `json:"mountPoint"`
	FileSystem string `json:"fileSystem"`
}

// HostLinuxMountsCase 是 /proc/self/mounts 的内容与要读容量的挂载点。
type HostLinuxMountsCase struct {
	Name   string           `json:"name"`
	Mounts string           `json:"mounts"`
	Disks  []HostLinuxMount `json:"disks"`
}

// HostLinuxProcessor 是读出的处理器静态信息，nil 表示读不到。
type HostLinuxProcessor struct {
	Name          *string  `json:"name"`
	Vendor        *string  `json:"vendor"`
	PhysicalCores *int     `json:"physicalCores"`
	FrequencyMhz  *float64 `json:"frequencyMhz"`
	CacheBytes    *int64   `json:"cacheBytes"`
}

// HostLinuxProcessorCase 是一组文件（路径相对文件系统的根）与读出的处理器静态信息。
type HostLinuxProcessorCase struct {
	Name      string             `json:"name"`
	Files     map[string]string  `json:"files"`
	Processor HostLinuxProcessor `json:"processor"`
}

// HostLinuxGPU 是读出的一块显卡，nil 表示读不到。
type HostLinuxGPU struct {
	Name               *string  `json:"name"`
	Vendor             *string  `json:"vendor"`
	VendorID           *string  `json:"vendorId"`
	DeviceID           *string  `json:"deviceId"`
	Driver             *string  `json:"driver"`
	DriverVersion      *string  `json:"driverVersion"`
	MemoryBytes        *int64   `json:"memoryBytes"`
	TemperatureCelsius *float64 `json:"temperatureCelsius"`
}

// HostLinuxGPUCase 是一组文件（路径相对文件系统的根）与读出的各块显卡。
type HostLinuxGPUCase struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
	GPUs  []HostLinuxGPU    `json:"gpus"`
}

// HostLinuxVectors 是 Linux 上各项由 /proc 与 /sys 里的文本读出的取法（RFC-0023「平台读取路径」的「Linux 的取法」，ADR-0286）。
type HostLinuxVectors struct {
	CPUTimes   []HostLinuxCPUTimesCase  `json:"cpuTimes"`
	Memory     []HostLinuxMemoryCase    `json:"memory"`
	Uptime     []HostLinuxUptimeCase    `json:"uptime"`
	Mounts     []HostLinuxMountsCase    `json:"mounts"`
	Processors []HostLinuxProcessorCase `json:"processors"`
	GPUs       []HostLinuxGPUCase       `json:"gpus"`
}

// HostUsageVectors 对应 observability/host-usage.json（RFC-0023「运行信息」的「契约」「采样」与「平台读取路径」，ADR-0277、ADR-0286）。
type HostUsageVectors struct {
	SampleInterval HostSampleIntervalVectors `json:"sampleInterval"`
	CPU            []HostCPUCase             `json:"cpu"`
	Process        []HostProcessCase         `json:"process"`
	Network        []HostNetworkCase         `json:"network"`
	Memory         []HostMemoryCase          `json:"memory"`
	Disks          []HostDiskCase            `json:"disks"`
	Linux          HostLinuxVectors          `json:"linux"`
}

// HostUsage 读取运行信息里使用率与速率的向量。
func HostUsage() (HostUsageVectors, error) {
	return typed[HostUsageVectors]()
}

// AINameVectors 对应 ai/ai-names.json（RFC-0022「Provider 目录」，ADR-0277）。
type AINameVectors struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// AIToolNameCase 是一个操作 ID、由它推出的工具名与描述的消息键。
type AIToolNameCase struct {
	Name           string `json:"name"`
	Operation      string `json:"operation"`
	Tool           string `json:"tool"`
	DescriptionKey string `json:"descriptionKey"`
}

// AIToolNameVectors 对应 ai/tool-names.json（RFC-0022「工具」，ADR-0277、ADR-0286）：Valid 与 Invalid 是工具名本身文法的样例。
type AIToolNameVectors struct {
	MaxLength int              `json:"maxLength"`
	Valid     []string         `json:"valid"`
	Invalid   []string         `json:"invalid"`
	Cases     []AIToolNameCase `json:"cases"`
}

// KnowledgeDefaults 是知识库选项的缺省值。
type KnowledgeDefaults struct {
	Collection   string `json:"collection"`
	Dimensions   int    `json:"dimensions"`
	ChunkSize    int    `json:"chunkSize"`
	ChunkOverlap int    `json:"chunkOverlap"`
	Top          int    `json:"top"`
}

// KnowledgeChunkCase 是一段文本按固定窗口切出的块。
type KnowledgeChunkCase struct {
	Name         string   `json:"name"`
	Text         string   `json:"text"`
	MaxChunkSize int      `json:"maxChunkSize"`
	Overlap      int      `json:"overlap"`
	Chunks       []string `json:"chunks"`
}

// KnowledgeRecordIDCase 是文档 ID、块序号与记录 ID。
type KnowledgeRecordIDCase struct {
	DocumentID string `json:"documentId"`
	ChunkIndex int    `json:"chunkIndex"`
	ID         string `json:"id"`
}

// KnowledgeCollectionCase 是配置的集合、调用的租户（nil 表示不带租户）与默认集合：DefaultCollection 为 nil 表示拼出的名字不合文法。
type KnowledgeCollectionCase struct {
	Name              string  `json:"name"`
	Collection        string  `json:"collection"`
	Tenant            *string `json:"tenant"`
	DefaultCollection *string `json:"defaultCollection"`
}

// KnowledgeChunkVectors 对应 ai/knowledge-chunks.json（RFC-0022「知识库」，ADR-0277）：Code 是默认集合的名字不合文法时的错误码。
type KnowledgeChunkVectors struct {
	Defaults           KnowledgeDefaults         `json:"defaults"`
	MaxTop             int                       `json:"maxTop"`
	MaxDocumentIDs     int                       `json:"maxDocumentIds"`
	Code               string                    `json:"code"`
	Chunks             []KnowledgeChunkCase      `json:"chunks"`
	RecordIDs          []KnowledgeRecordIDCase   `json:"recordIds"`
	DefaultCollections []KnowledgeCollectionCase `json:"defaultCollections"`
}

// AIProviderDefaultCase 是配置的 Provider 名与两个默认，以及目录给出的默认聊天、默认嵌入 Provider 名（nil 表示没有）。
type AIProviderDefaultCase struct {
	Name                     string   `json:"name"`
	Providers                []string `json:"providers"`
	DefaultProvider          *string  `json:"defaultProvider"`
	DefaultEmbeddingProvider *string  `json:"defaultEmbeddingProvider"`
	Chat                     *string  `json:"chat"`
	Embedding                *string  `json:"embedding"`
}

// AIProviderDefaultVectors 对应 ai/provider-defaults.json（RFC-0022「Provider 目录」的「默认名」，ADR-0277）：
// Code 与两个消息键是没有默认而又不给名字时的错误。
type AIProviderDefaultVectors struct {
	Code                string                  `json:"code"`
	ChatMessageKey      string                  `json:"chatMessageKey"`
	EmbeddingMessageKey string                  `json:"embeddingMessageKey"`
	Cases               []AIProviderDefaultCase `json:"cases"`
}

// AIPromptCase 是一个提示词模板、参数（值为 nil 即 null）与渲染出的文本。
type AIPromptCase struct {
	Name      string             `json:"name"`
	Template  string             `json:"template"`
	Arguments map[string]*string `json:"arguments"`
	Text      string             `json:"text"`
}

// AIPromptMissingCase 是一个缺少参数的提示词模板：Argument 是第一个找不到参数的占位符名。
type AIPromptMissingCase struct {
	Name      string             `json:"name"`
	Template  string             `json:"template"`
	Arguments map[string]*string `json:"arguments"`
	Argument  string             `json:"argument"`
}

// AIPromptTemplateVectors 对应 ai/prompt-templates.json（RFC-0022「聊天」的提示词，ADR-0277、ADR-0286）：
// Code 与 MessageKey 是缺少参数时的错误。
type AIPromptTemplateVectors struct {
	Code       string                `json:"code"`
	MessageKey string                `json:"messageKey"`
	Cases      []AIPromptCase        `json:"cases"`
	Missing    []AIPromptMissingCase `json:"missing"`
}

// AINames 读取 AI 子系统名字文法的向量。
func AINames() (AINameVectors, error) {
	return typed[AINameVectors]()
}

// AIToolNames 读取操作工具名的向量。
func AIToolNames() (AIToolNameVectors, error) {
	return typed[AIToolNameVectors]()
}

// KnowledgeChunks 读取知识库分块、记录 ID 与默认集合的向量。
func KnowledgeChunks() (KnowledgeChunkVectors, error) {
	return typed[KnowledgeChunkVectors]()
}

// AIProviderDefaults 读取默认 Provider 名的向量。
func AIProviderDefaults() (AIProviderDefaultVectors, error) {
	return typed[AIProviderDefaultVectors]()
}

// AIPromptTemplates 读取提示词模板渲染的向量。
func AIPromptTemplates() (AIPromptTemplateVectors, error) {
	return typed[AIPromptTemplateVectors]()
}
