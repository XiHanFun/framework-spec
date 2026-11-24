package rules

import (
	"regexp"
	"strings"
)

var tenantIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// MaxTenantIDLength 是租户 ID 的最大长度（RFC-0007「租户 ID」）；文法只含 ASCII，字符数就是字节数。
const MaxTenantIDLength = 64

// Tenant 阶段结束调用时的错误码（RFC-0007「解析」）。
const (
	// TenantInvalidID 是解析出的租户 ID 不合文法。
	TenantInvalidID = "MULTITENANCY_VALIDATION_001"
	// TenantMissing 是要求带租户而没有解析到。
	TenantMissing = "MULTITENANCY_VALIDATION_002"
	// TenantClaimsConflict 是已认证主体带着值不同的多个 tenant 声明。
	TenantClaimsConflict = "MULTITENANCY_VALIDATION_004"
	// TenantNotFound 是租户存储里没有这个租户。
	TenantNotFound = "MULTITENANCY_NOTFOUND_001"
	// TenantDisabled 是租户已禁用。
	TenantDisabled = "MULTITENANCY_DENIED_001"
	// TenantStoreUnavailable 是租户存储不可达。
	TenantStoreUnavailable = "MULTITENANCY_UNAVAILABLE_003"
)

// 默认分片解析器读的租户属性（RFC-0007「分片」）。
const (
	// TenantShardConnectionStringProperty 有值即分库，值是分片的连接串。
	TenantShardConnectionStringProperty = "ConnectionString"
	// TenantShardProviderProperty 是提供者名。
	TenantShardProviderProperty = "DatabaseProvider"
	// TenantShardPassThroughPrefix 是透传属性的键前缀。
	TenantShardPassThroughPrefix = "Shard."
)

// IsTenantID 判断租户 ID 是否合文法（RFC-0007「租户 ID」）：小写字母或数字开头，
// 之后是小写字母、数字、.、_、-，最长 64 个字符；含大写的不合文法。
func IsTenantID(id string) bool { return tenantIDPattern.MatchString(id) }

// TenantEntry 是租户存储里的一个租户：DisplayName 为 nil 表示没有显示名。
type TenantEntry struct {
	DisplayName *string
	Enabled     bool
}

// TenantCall 是 Tenant 阶段看到的一次调用与它的选项：Claims 是已认证主体的 tenant 声明的值，
// Baggage 是 Baggage 里的 xihan.tenant，Tenants 是租户存储的内容（StoreUnavailable 为真时读不到）。
type TenantCall struct {
	Authenticated        bool
	Claims               []string
	Baggage              *string
	PlatformCall         bool
	DefaultTenant        *string
	RequireTenant        bool
	ValidateAgainstStore bool
	StoreUnavailable     bool
	Tenants              map[string]TenantEntry
}

// TenantDecision 是 Tenant 阶段的裁定：Error 非空是结束调用的错误码；
// 否则 Tenant 是写进安全上下文的租户 ID（nil 表示无租户继续），DisplayName 是随之带入的显示名。
type TenantDecision struct {
	Tenant      *string
	DisplayName *string
	Error       string
}

// ResolveTenant 按 RFC-0007「解析」裁定一次调用的租户，解析器是缺省的两个、按缺省顺序：
// 已认证主体的 tenant 声明在前，Baggage 的 xihan.tenant 在后。
func ResolveTenant(call TenantCall) TenantDecision {
	// 平台调用有意不带租户：不问解析器、不补默认租户、不要求带租户
	if call.PlatformCall {
		return TenantDecision{}
	}

	var candidate *string
	if call.Authenticated {
		// 多个声明值都相同按一个算；值不同即失败，不取第一个，也不再问后面的解析器
		for i := range call.Claims {
			if candidate == nil {
				candidate = &call.Claims[i]
			} else if *candidate != call.Claims[i] {
				return TenantDecision{Error: TenantClaimsConflict}
			}
		}
	}
	if candidate == nil {
		candidate = call.Baggage
	}
	if candidate == nil {
		candidate = call.DefaultTenant
	}
	if candidate == nil {
		if call.RequireTenant {
			return TenantDecision{Error: TenantMissing}
		}
		return TenantDecision{}
	}

	// 文法先于存储：来自外部、没有校验过的 ID 不交给租户存储
	tenant := *candidate
	if !IsTenantID(tenant) {
		return TenantDecision{Error: TenantInvalidID}
	}
	if !call.ValidateAgainstStore {
		return TenantDecision{Tenant: &tenant}
	}
	// 存储读不到不是没有：故障不当作租户不存在
	if call.StoreUnavailable {
		return TenantDecision{Error: TenantStoreUnavailable}
	}
	entry, found := call.Tenants[tenant]
	if !found {
		return TenantDecision{Error: TenantNotFound}
	}
	if !entry.Enabled {
		return TenantDecision{Error: TenantDisabled}
	}
	return TenantDecision{Tenant: &tenant, DisplayName: entry.DisplayName}
}

// TenantShard 是从租户属性推出的分片：Provider 为空表示没有提供者名，Properties 是透传给持久化 Provider 的属性。
type TenantShard struct {
	ConnectionString string
	Provider         string
	Properties       map[string]string
}

// TenantShardFromProperties 从租户属性推出分片（RFC-0007「分片」的默认解析器）；共库的租户返回假。
func TenantShardFromProperties(properties map[string]string) (TenantShard, bool) {
	connection := properties[TenantShardConnectionStringProperty]
	// 只有空白的连接串也算没有值
	if strings.TrimFunc(connection, IsWhitespace) == "" {
		return TenantShard{}, false
	}
	shard := TenantShard{
		ConnectionString: connection,
		Provider:         strings.ToLower(properties[TenantShardProviderProperty]),
		Properties:       map[string]string{},
	}
	for key, value := range properties {
		// 只去掉开头的一个前缀；键正好是前缀本身时没有名字可透传
		if name, ok := strings.CutPrefix(key, TenantShardPassThroughPrefix); ok && name != "" {
			shard.Properties[name] = value
		}
	}
	return shard, true
}
