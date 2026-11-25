package rules

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var (
	containerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
	// blobShortNamePattern 是对象元数据名与命名存储实例名共用的文法。
	blobShortNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	// blobContentTypePattern 的两段是 HTTP 的 token 字符；字符集里有反引号，所以不用原始字符串。参数照 RFC 9110 可以为空，
	// 所以第一个 ;（之前可以有空格）之后是任意的可打印 ASCII 与空格：它在每个 ; 处都能切成可以为空的参数，与逐段写的文法是同一个集合。
	blobContentTypePattern = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+/[!#$%&'*+.^_`|~0-9A-Za-z-]+(?: *;[\\x20-\\x7E]*)?$")
)

// 对象存储的标识与上限（RFC-0012「标识」「范围读取、分片上传与预签名」）。
const (
	// MaxBlobKeyBytes 是对象键与查询前缀按 UTF-8 编码后的最大字节数（ADR-0282；以前叫 MaxBlobKeyLength，没有说单位）。
	MaxBlobKeyBytes = 1024
	// BlobKeySeparator 是对象键的分段符。
	BlobKeySeparator = "/"
	// BlobKeyReservedSegment 是保留前缀：首段是它的键留给框架自己的对象。
	BlobKeyReservedSegment = ".xihan"
	// MaxBlobMetadataBytes 是一个对象的元数据名与值合计的最大 UTF-8 字节数。
	MaxBlobMetadataBytes = 2048
	// MaxBlobContentTypeLength 是内容类型的最大长度；内容类型只有 ASCII，字符数即字节数。
	MaxBlobContentTypeLength = 255
	// MaxBlobDownloadFileNameBytes 是预签名下载地址上保存的文件名按 UTF-8 编码后的最大字节数（ADR-0282）。
	MaxBlobDownloadFileNameBytes = 255
	// DefaultBlobContentType 是写入时不给内容类型的缺省值。
	DefaultBlobContentType = "application/octet-stream"
	// MaxBlobPartNumber 是分片编号的上限，编号从 1 起。
	MaxBlobPartNumber = 10000
	// MinBlobPartBytes 是除最后一片外每片的最小字节数（5 MiB）。
	MinBlobPartBytes = 5 << 20
)

// BlobStoreProviders 是内置的存储 Provider 名（RFC-0012「命名存储实例」），不区分大小写。
var BlobStoreProviders = []string{"filesystem", "s3"}

// IsContainerName 判断容器名是否合文法（RFC-0012「标识」）：3 到 63 个字符的小写字母、数字与连字符，首尾是字母或数字。
func IsContainerName(name string) bool { return containerNamePattern.MatchString(name) }

// IsReservedBlobKey 判断键是否在保留前缀之下（RFC-0012「标识」）：首段正好是 BlobKeyReservedSegment，区分大小写。
func IsReservedBlobKey(key string) bool {
	rest, ok := strings.CutPrefix(key, BlobKeyReservedSegment)
	return ok && (rest == "" || strings.HasPrefix(rest, BlobKeySeparator))
}

// IsBlobKey 判断对象键是否合文法（RFC-0012「标识」）：以 / 分段的路径，按 UTF-8 编码后 1 到 MaxBlobKeyBytes 字节，
// 每段非空、不是 . 或 ..，不含空白、控制字符与反斜线，也不在保留前缀之下。
func IsBlobKey(key string) bool {
	if key == "" || !IsBlobKeyPrefix(key) || IsReservedBlobKey(key) {
		return false
	}
	// 每段非空且不是 . 或 ..：键会映射到路径与 URL，这些段会逃逸或产生歧义
	for segment := range strings.SplitSeq(key, BlobKeySeparator) {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// IsBlobKeyPrefix 判断查询前缀是否合法（RFC-0012「标识」）：字符约束与字节上限同键，但可以为空、可以以 / 结尾，
// 对各段不作要求，也不受保留前缀限制（以保留段开头的前缀列举总是空的）。
func IsBlobKeyPrefix(prefix string) bool {
	// Go 的字符串就是 UTF-8 字节序列，len 给出的正是规范要的字节数；按 UTF-16 码元计（.NET 的 string.Length、
	// Java 与 JavaScript 的 length）会把含非 ASCII 字符的超长键放过去，而 S3 的键上限正是 1024 个 UTF-8 字节
	if len(prefix) > MaxBlobKeyBytes {
		return false
	}
	for _, char := range prefix {
		// 空白按 RFC-0001「空白字符」的 25 个码位，控制字符是 Unicode 的 Cc 类
		if IsWhitespace(char) || unicode.IsControl(char) || char == '\\' {
			return false
		}
	}
	return true
}

// IsBlobDownloadFileName 判断预签名下载地址上保存的文件名是否合法（RFC-0012「范围读取、分片上传与预签名」）：
// 不为空、不全是空白、不含控制字符，按 UTF-8 编码后不超过 MaxBlobDownloadFileNameBytes 字节。
func IsBlobDownloadFileName(name string) bool {
	if len(name) > MaxBlobDownloadFileNameBytes {
		return false
	}
	blank := true
	for _, char := range name {
		// 它会进签名的规范串与 Content-Disposition 头：换行能挪动限定项或拆出别的头
		if unicode.IsControl(char) {
			return false
		}
		blank = blank && IsWhitespace(char)
	}
	// 空串也在这里拒绝：没有一个不是空白的字符
	return !blank
}

// IsBlobMetadataName 判断自定义元数据名是否合文法（RFC-0012「标识」）：1 到 64 个字符的小写字母、数字与连字符，字母或数字开头。
func IsBlobMetadataName(name string) bool { return blobShortNamePattern.MatchString(name) }

// IsBlobMetadata 判断一个对象的整组自定义元数据是否合法（RFC-0012「标识」）：名都合文法，值不含控制字符，
// 名与值合计不超过 MaxBlobMetadataBytes 个 UTF-8 字节。
func IsBlobMetadata(metadata map[string]string) bool {
	total := 0
	for name, value := range metadata {
		if !IsBlobMetadataName(name) || strings.ContainsFunc(value, unicode.IsControl) {
			return false
		}
		// Go 的字符串就是 UTF-8 字节序列，len 给出的正是规范要的字节数
		total += len(name) + len(value)
	}
	return total <= MaxBlobMetadataBytes
}

// IsBlobContentType 判断对象的内容类型是否合文法（RFC-0012「标识」）：媒体类型 type/subtype，两段都是 HTTP 的 token 字符，
// 可跟若干个 ; 起头的参数（RFC 9110 的 *( OWS ";" OWS [ parameter ] )，ADR-0295）：每个 ; 两侧可以有空格（不收制表符），
// 参数可以为空，不为空的是不含 ; 的可打印 ASCII 与空格，不解析引号；整体 1 到 255 个字符。
func IsBlobContentType(contentType string) bool {
	return contentType != "" && len(contentType) <= MaxBlobContentTypeLength && blobContentTypePattern.MatchString(contentType)
}

// BlobRange 是范围读取实际读出的范围：起点、长度与对象的总长。
type BlobRange struct {
	Offset int64
	Length int64
	Total  int64
}

// ResolveBlobRange 把请求的范围落到一个对象上（RFC-0012「范围读取、分片上传与预签名」）：length 为 nil 表示读到末尾，
// 超出末尾即截断；起点不在对象内（含空对象）返回假，读取以 OBJECTSTORAGE_VALIDATION_001 失败。
func ResolveBlobRange(offset int64, length *int64, total int64) (BlobRange, bool) {
	if offset < 0 || offset >= total {
		return BlobRange{}, false
	}
	available := total - offset
	// 与可读的长度取较小者，不做 offset + length：长度很大时相加会溢出
	if length != nil && *length < available {
		available = *length
	}
	return BlobRange{Offset: offset, Length: available, Total: total}, true
}

// IsBlobPartNumber 判断分片编号是否在 1 到 MaxBlobPartNumber 之间。
func IsBlobPartNumber(number int) bool { return number >= 1 && number <= MaxBlobPartNumber }

// IsBlobPartManifest 判断完成清单的编号次序是否合法（RFC-0012「范围读取、分片上传与预签名」）：
// 非空、每个编号都在范围内、严格升序。ETag 与分片大小的核对要有已上传的分片，不在这里。
func IsBlobPartManifest(numbers []int) bool {
	if len(numbers) == 0 {
		return false
	}
	for i, number := range numbers {
		if !IsBlobPartNumber(number) || (i > 0 && number <= numbers[i-1]) {
			return false
		}
	}
	return true
}

// IsBlobStoreName 判断命名存储实例的名字是否合文法（RFC-0012「命名存储实例」）：与元数据名同一条文法。
func IsBlobStoreName(name string) bool { return blobShortNamePattern.MatchString(name) }

// IsBlobStoreProvider 判断是不是内置的存储 Provider 名，不区分大小写。
func IsBlobStoreProvider(provider string) bool {
	return slices.ContainsFunc(BlobStoreProviders, func(known string) bool { return strings.EqualFold(known, provider) })
}

// BlobStoreDefinition 是一个命名存储实例的定义：实例名、Provider 名与设置。
type BlobStoreDefinition struct {
	Name     string
	Provider string
	Settings map[string]string
}

// SameBlobStoreDefinition 判断两个定义是不是同一个定义（RFC-0012「命名存储实例」）：实例名相同（区分大小写）、
// Provider 名相同（不区分大小写）、设置完全相同（键不区分大小写，值区分，与次序无关）。
func SameBlobStoreDefinition(left, right BlobStoreDefinition) bool {
	if left.Name != right.Name || !strings.EqualFold(left.Provider, right.Provider) || len(left.Settings) != len(right.Settings) {
		return false
	}
	// 键只含 ASCII 字母的配置名，折成小写比较；折叠后重名的键说明两边条数对不上
	folded := make(map[string]string, len(right.Settings))
	for key, value := range right.Settings {
		folded[strings.ToLower(key)] = value
	}
	if len(folded) != len(right.Settings) {
		return false
	}
	for key, value := range left.Settings {
		if other, ok := folded[strings.ToLower(key)]; !ok || other != value {
			return false
		}
	}
	return true
}
