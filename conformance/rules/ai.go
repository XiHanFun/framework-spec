package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
)

var (
	aiNamePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	aiToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// AI 子系统里名字与工具名的上限（RFC-0022「Provider 目录」「工具」）。
const (
	// MaxAINameLength 是 Provider 名、提示词名与知识库集合名的最大长度。
	MaxAINameLength = 64
	// MaxAIToolNameLength 是工具名的最大长度。
	MaxAIToolNameLength = 64
	// aiToolNamePrefixLength 是超长的工具名保留的前缀长度：后面再接一个下划线与 8 位十六进制。
	aiToolNamePrefixLength = MaxAIToolNameLength - 9
)

// IsAIName 判断 AI 子系统里的名字是否合文法（RFC-0022「Provider 目录」）：小写字母或数字开头，
// 之后是小写字母、数字、.、_、-，最长 64 个字符。
func IsAIName(name string) bool { return aiNamePattern.MatchString(name) }

// IsAIToolName 判断工具名是否合文法（RFC-0022「工具」）：字母、数字、_ 与 -，1 到 64 个字符。
func IsAIToolName(name string) bool { return aiToolNamePattern.MatchString(name) }

// AIToolName 由操作 ID 推出缺省的工具名（RFC-0022「工具」）：<resource>_<verb>，- 与 . 换成 _；
// 超过 64 个字符时取前 55 个字符、去掉结尾的 _，再接 _ 与全名 SHA-256 的前 8 位小写十六进制。
func AIToolName(operationID string) string {
	name := strings.NewReplacer("-", "_", ".", "_").Replace(operationID)
	if len(name) <= MaxAIToolNameLength {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	// 前缀正好以 _ 结尾时去掉它，不留下两个连着的下划线；操作 ID 的文法保证结尾至多一个
	return strings.TrimRight(name[:aiToolNamePrefixLength], "_") + "_" + hex.EncodeToString(sum[:])[:8]
}

// AIToolDescriptionKey 给出操作工具的描述在消息目录里的键：操作 ID 里的 - 换成 _，再加 .description。
func AIToolDescriptionKey(operationID string) string {
	return strings.ReplaceAll(operationID, "-", "_") + ".description"
}

// ChunkFixedWindow 按固定窗口把文本切成块（RFC-0022「知识库」的内置分块）：换行统一成 LF，字符按码位计，窗口 maxChunkSize 个字符，
// 相邻两个窗口重叠 overlap 个字符（裁到 0 到 maxChunkSize − 1 之间），取到文本结尾的窗口是最后一个；每个窗口切出来之后去掉首尾的空白，
// 去掉之后为空的不成块。maxChunkSize 不小于 1，overlap 不小于 0。
func ChunkFixedWindow(text string, maxChunkSize, overlap int) []string {
	normalized := []rune(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))
	step := maxChunkSize - min(overlap, maxChunkSize-1)
	chunks := []string{}
	for start := 0; start < len(normalized); start += step {
		end := min(start+maxChunkSize, len(normalized))
		// 去空白在切分之后：只裁这一块的两端，后面的窗口仍按原文的位置算
		if piece := strings.TrimFunc(string(normalized[start:end]), IsWhitespace); piece != "" {
			chunks = append(chunks, piece)
		}
		if end == len(normalized) {
			break
		}
	}
	return chunks
}

// KnowledgeRecordID 给出知识库记录的 ID：文档 ID 加 # 加块序号（RFC-0022「知识库」）。
func KnowledgeRecordID(documentID string, chunkIndex int) string {
	return documentID + "#" + strconv.Itoa(chunkIndex)
}

// DefaultKnowledgeCollection 给出请求没有指明集合时用的集合（RFC-0022「知识库」）：不带租户（tenant 为空）用 collection，
// 带租户用 collection 加 . 加租户 ID；拼出的名字不合 AI 名字的文法返回假（AI_VALIDATION_001，要显式指明集合）。
func DefaultKnowledgeCollection(collection, tenant string) (string, bool) {
	if tenant == "" {
		return collection, true
	}
	scoped := collection + "." + tenant
	return scoped, IsAIName(scoped)
}

// DefaultAIProviders 给出框架自带的目录的两个默认名（RFC-0022「Provider 目录」的「默认名」），空表示没有默认：
// 默认聊天 Provider 是配置的 defaultProvider，没写且只配了一个 Provider 时是它；默认嵌入 Provider 是配置的
// defaultEmbeddingProvider，没写时同默认聊天 Provider。
func DefaultAIProviders(providers []string, defaultProvider, defaultEmbeddingProvider string) (chat, embedding string) {
	chat = defaultProvider
	if chat == "" && len(providers) == 1 {
		chat = providers[0]
	}
	embedding = defaultEmbeddingProvider
	if embedding == "" {
		embedding = chat
	}
	return chat, embedding
}

// RenderAIPrompt 渲染提示词模板（RFC-0022「聊天」的提示词）：{{ 与 }} 转义成字面量；单独的 { 到其后第一个 } 之间去掉两侧的空白
// 是占位符的名字，按名字取参数的值（nil 写成空串）；找不到 } 时这个 { 连同其后的文本原样写出。缺少参数返回假，
// missing 是第一个找不到参数的占位符名（AI_VALIDATION_001，消息键 ai.prompt.argument_missing 的参数 argument）。
func RenderAIPrompt(template string, arguments map[string]*string) (text, missing string, ok bool) {
	var builder strings.Builder
	for position := 0; position < len(template); {
		switch {
		case strings.HasPrefix(template[position:], "{{"):
			builder.WriteByte('{')
			position += 2
		case strings.HasPrefix(template[position:], "}}"):
			builder.WriteByte('}')
			position += 2
		case template[position] == '{':
			length := strings.IndexByte(template[position+1:], '}')
			if length < 0 {
				// 没有闭合的 {：连同其后的文本原样写出，其中的 {{ 也不再转义
				builder.WriteString(template[position:])
				return builder.String(), "", true
			}
			name := strings.TrimFunc(template[position+1:position+1+length], IsWhitespace)
			value, ok := arguments[name]
			if !ok {
				return "", name, false
			}
			if value != nil {
				builder.WriteString(*value)
			}
			position += length + 2
		default:
			builder.WriteByte(template[position])
			position++
		}
	}
	return builder.String(), "", true
}
