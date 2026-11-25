package rules

import (
	"math/big"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
)

var (
	workflowElementIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	activityTypePattern      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,63}$`)
)

// 工作流里名字的长度上限（RFC-0021「名字与值」）。
const (
	// MaxWorkflowNameLength 是定义名的最大长度；与消息主题同一条文法。
	MaxWorkflowNameLength = 128
	// MaxWorkflowElementIDLength 是节点 ID 与转移 ID 的最大长度。
	MaxWorkflowElementIDLength = 64
	// MaxActivityTypeLength 是活动类型的最大长度。
	MaxActivityTypeLength = 64
)

// WorkflowKeywordFieldSeparator 是把几个被搜字段拼成一列时用的分隔符 U+001F；折叠时换成空格，关键字里就不会有它。
const WorkflowKeywordFieldSeparator = '\u001f'

// WorkflowEventSource 是工作流事件信封的 source（RFC-0021「实例与执行」的「事件」）。
const WorkflowEventSource = "/workflow"

// WorkflowDefinitionStatuses 是定义的状态，按生命周期的先后升序（RFC-0021「定义」的「列表排序」）。
var WorkflowDefinitionStatuses = []string{"Draft", "Published", "Disabled", "Archived"}

// WorkflowInstanceStatuses 是实例的状态，按生命周期的先后升序（RFC-0021「定义」的「列表排序」）。
var WorkflowInstanceStatuses = []string{"Running", "Suspended", "Completed", "Canceled", "Faulted", "Terminated"}

// WorkflowEventTopics 是工作流发布的事件的主题，也是信封的 type（RFC-0021「实例与执行」的「事件」）。
var WorkflowEventTopics = []string{
	"workflow.instance.started", "workflow.instance.completed", "workflow.instance.faulted", "workflow.instance.canceled",
	"workflow.instance.terminated", "workflow.instance.suspended", "workflow.instance.resumed", "workflow.instance.jumped",
	"workflow.instance.migrated", "workflow.instance.timedout",
	"workflow.node.executing", "workflow.node.executed", "workflow.node.faulted",
	"workflow.task.created", "workflow.task.completed", "workflow.task.transferred", "workflow.task.claimed",
	"workflow.task.released", "workflow.task.returned", "workflow.task.urged", "workflow.task.reminder",
	"workflow.external.created", "workflow.external.completed", "workflow.external.failed",
	"workflow.custom",
}

// IsWorkflowName 判断工作流定义名是否合文法（RFC-0021「名字与值」）：与主题文法相同。
func IsWorkflowName(name string) bool { return topicPattern.MatchString(name) }

// IsWorkflowElementID 判断节点 ID 或转移 ID 是否合文法（RFC-0021「名字与值」）：字母或数字开头，
// 其余是字母、数字、.、_、-，最长 64 个字符。
func IsWorkflowElementID(id string) bool { return workflowElementIDPattern.MatchString(id) }

// IsActivityType 判断活动类型是否合文法（RFC-0021「名字与值」）：字母开头，其余是字母、数字、.、_、-，最长 64 个字符。
func IsActivityType(activityType string) bool { return activityTypePattern.MatchString(activityType) }

// FoldWorkflowKeyword 折叠关键字或被搜的文本（RFC-0021「定义」的「关键字」）：小写，字段分隔符换成空格。
func FoldWorkflowKeyword(text string) string {
	return strings.Map(func(char rune) rune {
		if char == WorkflowKeywordFieldSeparator {
			return ' '
		}
		return unicode.ToLower(char)
	}, text)
}

// NormalizeWorkflowKeyword 把调用方给的关键字变成用来匹配的文本：去掉首尾空白再折叠；
// 没有给（nil）、为空或只有空白返回假，表示不按关键字过滤。
func NormalizeWorkflowKeyword(keyword *string) (string, bool) {
	if keyword == nil {
		return "", false
	}
	trimmed := strings.TrimFunc(*keyword, IsWhitespace)
	if trimmed == "" {
		return "", false
	}
	return FoldWorkflowKeyword(trimmed), true
}

// WorkflowKeywordMatches 判断折叠后的关键字是否匹配：被搜的字段各自折叠之后按序数判断包含，任一字段包含即匹配，
// 不跨字段匹配；nil 是没有值的字段。关键字里的 %、_、[、\ 没有通配的含义。
func WorkflowKeywordMatches(keyword string, fields []*string) bool {
	for _, field := range fields {
		if field != nil && strings.Contains(FoldWorkflowKeyword(*field), keyword) {
			return true
		}
	}
	return false
}

// 表达式的上限（RFC-0021「表达式」的「长度与上限」）；长度都按 UTF-16 码元计。
const (
	// MaxWorkflowExpressionLength 是表达式的最大长度。
	MaxWorkflowExpressionLength = 4000
	// MaxWorkflowExpressionDepth 是表达式的最大嵌套层数：括号、函数的实参表、索引与一元运算符各算一层。
	MaxWorkflowExpressionDepth = 64
	// MaxWorkflowExpressionStringLength 是 + 拼出的字符串与模板渲染出的文本的最大长度。
	MaxWorkflowExpressionStringLength = 4194304
	// MaxWorkflowExpressionEchoLength 是错误里带出的文本的最大长度，超过的截断后接 U+2026。
	MaxWorkflowExpressionEchoLength = 200
	// MaxWorkflowExpressionWork 是一次求值（一次渲染的全部占位合计）的工作量上限，超过即 budget_exceeded（ADR-0296）。
	MaxWorkflowExpressionWork = 16777216
	// WorkflowExpressionStepWork 是一步的工作量：每开始求值一个节点、读到的容器里的每个值各记一次。
	WorkflowExpressionStepWork = 64
)

// WorkflowExpressionErrorCode 是表达式语法不合法或求值失败的错误码（RFC-0021「表达式」的「错误」）。
const WorkflowExpressionErrorCode = "WORKFLOW_VALIDATION_003"

// WorkflowExpressionSyntaxErrors 是解析时就报出的表达式错误的消息键（RFC-0021「表达式」的「错误」）：只校验语法也失败。
var WorkflowExpressionSyntaxErrors = []string{
	"workflow.expression.empty", "workflow.expression.unexpected_end", "workflow.expression.unterminated_string",
	"workflow.expression.unexpected_character", "workflow.expression.token_expected", "workflow.expression.member_expected",
	"workflow.expression.not_number", "workflow.expression.too_long", "workflow.expression.too_deep",
	"workflow.expression.unknown_function", "workflow.expression.argument_count", "workflow.expression.argument_range",
}

// WorkflowExpressionEvaluationErrors 是语法合法、求值（含当条件求值与渲染模板）时才报出的表达式错误的消息键。
var WorkflowExpressionEvaluationErrors = []string{
	"workflow.expression.unknown_variable", "workflow.expression.operands_not_boolean", "workflow.expression.operands_not_numeric",
	"workflow.expression.operands_not_scalar", "workflow.expression.division_by_zero", "workflow.expression.index_of_scalar",
	"workflow.expression.number_out_of_range", "workflow.expression.member_of_non_object", "workflow.expression.index_out_of_range",
	"workflow.expression.not_date", "workflow.expression.string_too_long", "workflow.expression.not_boolean",
	"workflow.expression.unterminated_placeholder", "workflow.expression.budget_exceeded",
}

// WorkflowExpressionFunctions 是表达式的内置函数（RFC-0021「表达式」的「内置函数」）；函数名不区分 ASCII 字母的大小写。
var WorkflowExpressionFunctions = []string{
	"len", "contains", "startsWith", "endsWith", "upper", "lower", "trim", "isNullOrEmpty",
	"abs", "min", "max", "round", "format", "toNumber", "toString", "now", "date",
}

// WorkflowExpressionValueTypes 是表达式结果的类型，也就是 JSON 的六种值。
var WorkflowExpressionValueTypes = []string{"null", "boolean", "number", "string", "array", "object"}

// workflowNumberTextPattern 是数字的规范文本的写法：不用指数，小数部分不带结尾的零，最多 28 位小数。
var workflowNumberTextPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]{0,27}[1-9])?$`)

// maxWorkflowMantissa 是数字去掉小数点之后的整数的上限 2^96 - 1。
var maxWorkflowMantissa = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 96), big.NewInt(1))

// WorkflowExpressionLength 返回文本按表达式的算法的长度（RFC-0021「表达式」的「长度与上限」）：
// UTF-16 码元数，基本多文种平面以外的字符算两个。
func WorkflowExpressionLength(text string) int { return len(utf16.Encode([]rune(text))) }

// EchoWorkflowExpression 返回错误里带出的文本（RFC-0021「表达式」的「长度与上限」）：不超过 200 个码元的原样；
// 超过的取前 200 个码元再接 U+2026，第 200 个码元是代理对的前一半时只取 199 个。
func EchoWorkflowExpression(text string) string {
	units := utf16.Encode([]rune(text))
	if len(units) <= MaxWorkflowExpressionEchoLength {
		return text
	}
	length := MaxWorkflowExpressionEchoLength
	if last := units[length-1]; last >= 0xD800 && last < 0xDC00 {
		length--
	}
	return string(utf16.Decode(units[:length])) + "…"
}

// IsWorkflowNumberText 判断文本是不是数字的规范文本（RFC-0021「表达式」的「数字」）：十进制、不用指数，
// 小数部分不带结尾的零，零写 0、没有负零，去掉小数点之后的整数不超过 2^96 - 1。
func IsWorkflowNumberText(text string) bool {
	if !workflowNumberTextPattern.MatchString(text) || text == "-0" {
		return false
	}
	mantissa, ok := new(big.Int).SetString(strings.ReplaceAll(strings.TrimPrefix(text, "-"), ".", ""), 10)
	return ok && mantissa.Cmp(maxWorkflowMantissa) <= 0
}

// IsWorkflowTimeText 判断文本是不是时间文本的写法（RFC-0021「表达式」的「时间」）：与截止时间的写法相同，
// UTC、七位小数、+00:00，写出来再读回去不变。
func IsWorkflowTimeText(text string) bool {
	instant, ok := ParseDeadline(text)
	return ok && FormatDeadline(instant) == text
}
