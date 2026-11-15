package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Result 的值类型（RFC-0001「Result」）：有值、无值、值是 Maybe。
const (
	ResultValue = "value"
	ResultUnit  = "unit"
	ResultMaybe = "maybe"
)

// CanonicalResult 按 RFC-0001 读取一条 Result 的线上编码，返回再写出时的规范形式（JSON 值）；
// 不合规则时返回错误。kind 是 ResultValue、ResultUnit 或 ResultMaybe。
func CanonicalResult(kind string, data []byte) (map[string]any, error) {
	members, err := object(data)
	if err != nil {
		return nil, err
	}
	okText, hasOK := members["ok"]
	if !hasOK {
		return nil, fmt.Errorf("result has no 'ok'")
	}
	ok, err := boolean(okText)
	if err != nil {
		return nil, fmt.Errorf("result 'ok': %w", err)
	}
	valueText, hasValue := members["value"]
	errorText, hasError := members["error"]
	if !ok {
		if hasValue {
			return nil, fmt.Errorf("a failed result must not carry 'value'")
		}
		if !hasError {
			return nil, fmt.Errorf("a failed result needs 'error'")
		}
		failure, err := CanonicalError(errorText)
		if err != nil {
			return nil, fmt.Errorf("result 'error': %w", err)
		}
		return map[string]any{"ok": false, "error": failure}, nil
	}
	if hasError {
		return nil, fmt.Errorf("a successful result must not carry 'error'")
	}
	switch kind {
	case ResultUnit:
		// 无值成功只有 ok，{} 与 null 都不算没有值
		if hasValue {
			return nil, fmt.Errorf("a result without a value must not carry 'value'")
		}
		return map[string]any{"ok": true}, nil
	case ResultValue, ResultMaybe:
		if !hasValue {
			return nil, fmt.Errorf("a successful result needs 'value'")
		}
		// null 只表示 Maybe 无值，其余类型的成功值不能是 null
		if isNull(valueText) && kind == ResultValue {
			return nil, fmt.Errorf("the value of a successful result must not be null")
		}
		var value any
		if err := json.Unmarshal(valueText, &value); err != nil {
			return nil, fmt.Errorf("result 'value': %w", err)
		}
		return map[string]any{"ok": true, "value": value}, nil
	default:
		return nil, fmt.Errorf("unknown result kind '%s'", kind)
	}
}

// CanonicalError 按 RFC-0001 读取一条 Error 的线上编码，返回再写出时的规范形式：
// 只留已知成员，空的 details 不写；不合规则时返回错误。
func CanonicalError(data []byte) (map[string]any, error) {
	members, err := object(data)
	if err != nil {
		return nil, err
	}
	canonical := map[string]any{}
	code, err := requiredString(members, "code")
	if err != nil {
		return nil, err
	}
	if !IsErrorCode(code) {
		return nil, fmt.Errorf("error code '%s' is not valid", code)
	}
	canonical["code"] = code
	message, err := requiredString(members, "message")
	if err != nil {
		return nil, err
	}
	if message == "" {
		return nil, fmt.Errorf("error 'message' must not be empty")
	}
	canonical["message"] = message
	if text, has := members["messageKey"]; has {
		key, err := stringValue(text)
		if err != nil {
			return nil, fmt.Errorf("error 'messageKey': %w", err)
		}
		if !IsMessageKey(key) {
			return nil, fmt.Errorf("message key '%s' is not valid", key)
		}
		canonical["messageKey"] = key
	}
	if text, has := members["details"]; has {
		details, err := object(text)
		if err != nil {
			return nil, fmt.Errorf("error 'details': %w", err)
		}
		if len(details) > 0 {
			var value map[string]any
			if err := json.Unmarshal(text, &value); err != nil {
				return nil, fmt.Errorf("error 'details': %w", err)
			}
			canonical["details"] = value
		}
	}
	if text, has := members["inner"]; has {
		inner, err := CanonicalError(text)
		if err != nil {
			return nil, fmt.Errorf("error 'inner': %w", err)
		}
		canonical["inner"] = inner
	}
	return canonical, nil
}

// object 读取一个 JSON 对象的成员；null 与其他类型都不是对象。
func object(data []byte) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("expected a JSON object")
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &members); err != nil {
		return nil, err
	}
	return members, nil
}

// boolean 读取 JSON 布尔；encoding/json 会把 null 当成 false，这里先排除
func boolean(data json.RawMessage) (bool, error) {
	switch string(bytes.TrimSpace(data)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected a JSON boolean")
	}
}

// stringValue 读取 JSON 字符串；null 不是字符串
func stringValue(data json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return "", fmt.Errorf("expected a JSON string")
	}
	var text string
	if err := json.Unmarshal(trimmed, &text); err != nil {
		return "", err
	}
	return text, nil
}

// requiredString 读取必填的字符串成员。
func requiredString(members map[string]json.RawMessage, name string) (string, error) {
	text, has := members[name]
	if !has {
		return "", fmt.Errorf("error has no '%s'", name)
	}
	value, err := stringValue(text)
	if err != nil {
		return "", fmt.Errorf("error '%s': %w", name, err)
	}
	return value, nil
}

// isNull 判断原始 JSON 是不是 null。
func isNull(data json.RawMessage) bool {
	return string(bytes.TrimSpace(data)) == "null"
}
