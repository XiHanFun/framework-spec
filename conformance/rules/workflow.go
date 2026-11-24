package rules

import (
	"regexp"
	"strings"
	"unicode"
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
