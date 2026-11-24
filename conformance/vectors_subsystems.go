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
