package rules

import (
	"regexp"
	"strings"
)

var auditExclusionPrefixPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z0-9-]*)?\*$`)

// 审计的选择规则（RFC-0015「事件的产生」）。
const (
	// AuditSelectionProtected 只记需要授权的操作（契约声明了权限码或策略名），是缺省值。
	AuditSelectionProtected = "Protected"
	// AuditSelectionAll 记所有操作。
	AuditSelectionAll = "All"
	// AuditSelectionNone 只记显式声明了动作的调用。
	AuditSelectionNone = "None"
)

// 没有写入结果的调用记成的错误码（RFC-0015「事件的产生」）。
const (
	// AuditUnexpectedCode 是没有写入结果而抛出了异常。
	AuditUnexpectedCode = "KERNEL_UNEXPECTED_001"
	// AuditCancelledCode 是没有写入结果而被取消。
	AuditCancelledCode = "KERNEL_CANCELLED_001"
)

// IsAuditExclusion 判断审计排除列表的一项是否合法（RFC-0015「事件的产生」）：合法的操作 ID，
// 或以 * 结尾、* 之前是非空的操作 ID 前缀（小写字母开头，只含小写字母、数字与 -，至多一个 .）。
func IsAuditExclusion(entry string) bool {
	return IsOperationID(entry) || auditExclusionPrefixPattern.MatchString(entry)
}

// AuditExcludes 判断操作是否命中排除列表里的任一项：不带 * 的项精确匹配，带 * 的项按前缀匹配，都按序数比较。
func AuditExcludes(exclude []string, operationID string) bool {
	for _, entry := range exclude {
		if prefix, isPrefix := strings.CutSuffix(entry, "*"); isPrefix {
			if strings.HasPrefix(operationID, prefix) {
				return true
			}
		} else if entry == operationID {
			return true
		}
	}
	return false
}

// AuditOperation 是一次调用的当前操作：ID 与契约上的权限码、策略名（空表示没有声明）。
type AuditOperation struct {
	ID         string
	Permission string
	Policy     string
}

// AuditCall 是审计看到的一次调用：选项、当前操作（nil 表示不是操作调用）、显式声明的动作（空表示没有）与调用怎样结束。
// Outcome 是 success、failure（管道里写入的结果，失败的错误码在 FailureCode）、none（正常返回却没写结果）、
// throws（没有写结果而抛出了异常）或 canceled（没有写结果而被取消）。
type AuditCall struct {
	Selection        string
	IncludeSuccesses bool
	Exclude          []string
	Operation        *AuditOperation
	ExplicitAction   string
	Outcome          string
	FailureCode      string
}

// AuditRecord 是记下的审计事件：动作、是否成功与失败时的错误码。
type AuditRecord struct {
	Action    string
	Succeeded bool
	Code      string
}

// SelectAuditEvent 判断一次调用是否记成审计事件、记成什么（RFC-0015「事件的产生」）；不记返回假。
func SelectAuditEvent(call AuditCall) (AuditRecord, bool) {
	// 显式声明的动作是代码要求必须留痕的调用，排除只针对按规则挑出的操作，不压掉它
	action := call.ExplicitAction
	if action == "" && call.Operation != nil {
		selected := false
		switch call.Selection {
		case AuditSelectionAll:
			selected = true
		case AuditSelectionProtected:
			// 需要授权：声明了权限码或策略名，只用策略保护的操作同样算
			selected = call.Operation.Permission != "" || call.Operation.Policy != ""
		}
		if selected && !AuditExcludes(call.Exclude, call.Operation.ID) {
			action = call.Operation.ID
		}
	}
	// 没有动作就不记
	if action == "" {
		return AuditRecord{}, false
	}

	// 结果优先取管道里已写入的；没有写入而抛出了异常按异常记失败；正常返回却没写结果视为成功
	record := AuditRecord{Action: action}
	switch call.Outcome {
	case "failure":
		record.Code = call.FailureCode
	case "throws":
		record.Code = AuditUnexpectedCode
	case "canceled":
		record.Code = AuditCancelledCode
	default:
		record.Succeeded = true
	}
	if record.Succeeded && !call.IncludeSuccesses {
		return AuditRecord{}, false
	}
	return record, true
}

// AuditAttribution 给出审计事件的主体与租户（RFC-0015「事件的产生」）：主体先取安全上下文里已认证的主体，
// 匿名时回退到 Baggage；租户先取安全上下文，只在 Tenant 阶段没有裁定过这次调用的租户时回退到 Baggage。nil 表示没有。
func AuditAttribution(principal, baggagePrincipal *string, tenantResolved bool, tenant, baggageTenant *string) (eventPrincipal, eventTenant *string) {
	eventPrincipal = principal
	if eventPrincipal == nil {
		eventPrincipal = baggagePrincipal
	}
	eventTenant = tenant
	// 被 Tenant 阶段裁定为无租户或拒绝的调用没有租户：调用方自报的租户只是请求的内容
	if eventTenant == nil && !tenantResolved {
		eventTenant = baggageTenant
	}
	return eventPrincipal, eventTenant
}
