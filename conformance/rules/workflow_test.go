package rules_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestWorkflowNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.WorkflowNames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.DefinitionNames.MaxLength != rules.MaxWorkflowNameLength || vectors.ElementIDs.MaxLength != rules.MaxWorkflowElementIDLength || vectors.ActivityTypes.MaxLength != rules.MaxActivityTypeLength {
		t.Fatalf("limits = %d, %d, %d; want %d, %d, %d", vectors.DefinitionNames.MaxLength, vectors.ElementIDs.MaxLength, vectors.ActivityTypes.MaxLength,
			rules.MaxWorkflowNameLength, rules.MaxWorkflowElementIDLength, rules.MaxActivityTypeLength)
	}
	replayGrammar(t, "workflow name", vectors.DefinitionNames.MaxLength, vectors.DefinitionNames.Valid, vectors.DefinitionNames.Invalid, rules.IsWorkflowName)
	replayGrammar(t, "workflow element ID", vectors.ElementIDs.MaxLength, vectors.ElementIDs.Valid, vectors.ElementIDs.Invalid, rules.IsWorkflowElementID)
	replayGrammar(t, "activity type", vectors.ActivityTypes.MaxLength, vectors.ActivityTypes.Valid, vectors.ActivityTypes.Invalid, rules.IsActivityType)
}

func TestWorkflowStatusOrderFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.WorkflowStatusOrder()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(vectors.Definition, rules.WorkflowDefinitionStatuses) || !slices.Equal(vectors.Instance, rules.WorkflowInstanceStatuses) {
		t.Errorf("status order = %v and %v, want %v and %v", vectors.Definition, vectors.Instance, rules.WorkflowDefinitionStatuses, rules.WorkflowInstanceStatuses)
	}
	// 次序是生命周期的先后，不是字母序：两张表都不能恰好是字母序
	if slices.IsSorted(vectors.Definition) || slices.IsSorted(vectors.Instance) {
		t.Error("the lifecycle order must differ from the alphabetical order, otherwise the vector cannot tell them apart")
	}
}

func TestWorkflowKeywordsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.WorkflowKeywords()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.FieldSeparator != string(rules.WorkflowKeywordFieldSeparator) {
		t.Errorf("field separator = %q, want %q", vectors.FieldSeparator, string(rules.WorkflowKeywordFieldSeparator))
	}
	for _, c := range vectors.Normalize {
		normalized, filters := rules.NormalizeWorkflowKeyword(c.Keyword)
		if filters != (c.Normalized != nil) || (filters && normalized != *c.Normalized) {
			t.Errorf("%s: normalized = %q (filters %v), want %v", c.Name, normalized, filters, c.Normalized)
		}
	}
	for _, c := range vectors.Matches {
		keyword, filters := rules.NormalizeWorkflowKeyword(&c.Keyword)
		if !filters {
			t.Errorf("%s: the keyword %q must filter", c.Name, c.Keyword)
			continue
		}
		if got := rules.WorkflowKeywordMatches(keyword, c.Fields); got != c.Matches {
			t.Errorf("%s: matches = %v, want %v", c.Name, got, c.Matches)
		}
	}
}

func TestWorkflowEventTopicsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.WorkflowEventTopics()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.Source != rules.WorkflowEventSource || !slices.Equal(vectors.Topics, rules.WorkflowEventTopics) {
		t.Errorf("source and topics = %q, %v; want %q, %v", vectors.Source, vectors.Topics, rules.WorkflowEventSource, rules.WorkflowEventTopics)
	}
	for _, topic := range vectors.Topics {
		if !rules.IsTopic(topic) {
			t.Errorf("%q must be a valid topic", topic)
		}
	}
}

// expressionKeys 是表达式错误的全部消息键。
func expressionKeys() []string {
	return append(slices.Clone(rules.WorkflowExpressionSyntaxErrors), rules.WorkflowExpressionEvaluationErrors...)
}

// checkExpressionValue 核对一条求值成功的用例：类型是六种之一，文本合这种类型的写法。
func checkExpressionValue(t *testing.T, name, kind, text string) {
	t.Helper()
	ok := false
	switch kind {
	case "null":
		ok = text == ""
	case "boolean":
		ok = text == "true" || text == "false"
	case "number":
		ok = rules.IsWorkflowNumberText(text)
	case "string":
		ok = true
	case "array":
		ok = strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]")
	case "object":
		ok = strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}")
	}
	if !ok || !slices.Contains(rules.WorkflowExpressionValueTypes, kind) {
		t.Errorf("%s: the text %q does not fit the type %q", name, text, kind)
	}
}

// checkExpressionCases 核对一个表达式向量文件：错误码、每条用例的写法、消息键与它报出的时机，返回文件里出现的消息键。
func checkExpressionCases(t *testing.T, file string, cases conformance.WorkflowExpressionCases) []string {
	t.Helper()
	if cases.ErrorCode != rules.WorkflowExpressionErrorCode || len(cases.Variables) == 0 || len(cases.Values) == 0 || len(cases.Errors) == 0 {
		t.Errorf("%s: the error code must be %q and the variables, values and errors must not be empty", file, rules.WorkflowExpressionErrorCode)
	}
	names, expressions := map[string]bool{}, map[string]bool{}
	for _, c := range cases.Values {
		if c.Name == "" || names[c.Name] || expressions[c.Expression] {
			t.Errorf("%s: the case %q must have a unique name and expression", file, c.Name)
		}
		names[c.Name], expressions[c.Expression] = true, true
		checkExpressionValue(t, file+": "+c.Name, c.Type, c.Text)
		if rules.WorkflowExpressionLength(c.Expression) > rules.MaxWorkflowExpressionLength {
			t.Errorf("%s: the expression of %q is over the length limit", file, c.Name)
		}
	}
	var keys []string
	for _, c := range cases.Errors {
		if c.Name == "" || names[c.Name] || expressions[c.Expression] {
			t.Errorf("%s: the case %q must have a unique name and expression", file, c.Name)
		}
		names[c.Name], expressions[c.Expression] = true, true
		// 解析时报出的与求值时才报出的各是一组键：syntax 必须与键所在的组一致
		if syntax := slices.Contains(rules.WorkflowExpressionSyntaxErrors, c.MessageKey); syntax != c.Syntax || (!syntax && !slices.Contains(rules.WorkflowExpressionEvaluationErrors, c.MessageKey)) {
			t.Errorf("%s: %q fails with %q, syntax = %v", file, c.Name, c.MessageKey, c.Syntax)
		}
		if !rules.IsMessageKey(c.MessageKey) || c.Arguments == nil {
			t.Errorf("%s: %q must have a message key and arguments", file, c.Name)
		}
		if _, echoed := c.Arguments["expression"]; echoed {
			t.Errorf("%s: the arguments of %q must not repeat the expression", file, c.Name)
		}
		keys = append(keys, c.MessageKey)
	}
	return keys
}

func TestWorkflowExpressionVectorsAreWellFormed(t *testing.T) {
	syntax, err := conformance.WorkflowExpressionSyntax()
	if err != nil {
		t.Fatal(err)
	}
	operators, err := conformance.WorkflowExpressionOperators()
	if err != nil {
		t.Fatal(err)
	}
	functions, err := conformance.WorkflowExpressionFunctions()
	if err != nil {
		t.Fatal(err)
	}
	templates, err := conformance.WorkflowExpressionTemplates()
	if err != nil {
		t.Fatal(err)
	}
	limits, err := conformance.WorkflowExpressionLimits()
	if err != nil {
		t.Fatal(err)
	}

	keys := checkExpressionCases(t, "expression-syntax.json", syntax.WorkflowExpressionCases)
	keys = append(keys, checkExpressionCases(t, "expression-operators.json", operators.WorkflowExpressionCases)...)
	keys = append(keys, checkExpressionCases(t, "expression-functions.json", functions.WorkflowExpressionCases)...)

	// 模板与条件
	if templates.ErrorCode != rules.WorkflowExpressionErrorCode || !rules.IsWorkflowTimeText(templates.Now) || len(templates.Variables) == 0 ||
		len(templates.Renders) == 0 || len(templates.RenderErrors) == 0 || len(templates.Conditions) == 0 || len(templates.ConditionErrors) == 0 {
		t.Error("expression-templates.json: the error code, the time and every list must be present")
	}
	names := map[string]bool{}
	unique := func(name string) {
		if name == "" || names[name] {
			t.Errorf("expression-templates.json: the case %q must have a unique name", name)
		}
		names[name] = true
	}
	for _, c := range templates.Renders {
		unique(c.Name)
	}
	for _, c := range templates.RenderErrors {
		unique(c.Name)
		if !slices.Contains(expressionKeys(), c.MessageKey) || c.Arguments == nil || rules.WorkflowExpressionLength(c.Expression) > rules.MaxWorkflowExpressionEchoLength+1 {
			t.Errorf("expression-templates.json: %q must fail with an expression error and a bounded expression argument", c.Name)
		}
		keys = append(keys, c.MessageKey)
	}
	holds := map[bool]bool{}
	for _, c := range templates.Conditions {
		unique(c.Name)
		holds[c.Holds] = true
	}
	if !holds[true] || !holds[false] {
		t.Error("expression-templates.json: the conditions must keep one that holds and one that does not")
	}
	for _, c := range templates.ConditionErrors {
		unique(c.Name)
		if !slices.Contains(expressionKeys(), c.MessageKey) || c.Syntax != slices.Contains(rules.WorkflowExpressionSyntaxErrors, c.MessageKey) {
			t.Errorf("expression-templates.json: %q fails with %q, syntax = %v", c.Name, c.MessageKey, c.Syntax)
		}
		keys = append(keys, c.MessageKey)
	}

	// 上限文件里的错误：超长、过深、拼接与渲染超过上限，以及带出文本的用例
	for _, c := range limits.Lengths {
		if !c.Valid {
			keys = append(keys, "workflow.expression.too_long")
		}
	}
	for _, c := range limits.Nestings {
		if !c.Valid {
			keys = append(keys, "workflow.expression.too_deep")
		}
	}
	for _, c := range limits.Concatenations {
		if !c.Valid {
			keys = append(keys, "workflow.expression.string_too_long")
		}
	}
	for _, c := range limits.Echoes {
		keys = append(keys, c.MessageKey)
	}

	// 每个消息键至少有一条用例
	for _, key := range expressionKeys() {
		if !slices.Contains(keys, key) {
			t.Errorf("no vector case fails with %q", key)
		}
	}
	if len(expressionKeys()) != 25 {
		t.Errorf("the expression has %d message keys, want 25", len(expressionKeys()))
	}

	// 每个内置函数都被求值成功的用例调用过；now() 的结果就是文件里的时刻
	if !rules.IsWorkflowTimeText(functions.Now) {
		t.Errorf("the time %q of expression-functions.json must be a time text", functions.Now)
	}
	for _, function := range rules.WorkflowExpressionFunctions {
		called := slices.ContainsFunc(functions.Values, func(c conformance.WorkflowExpressionValue) bool {
			return strings.Contains(strings.ToLower(c.Expression), strings.ToLower(function)+"(")
		})
		if !called {
			t.Errorf("no case of expression-functions.json calls %s", function)
		}
	}
	for _, c := range functions.Values {
		if c.Expression == "now()" && (c.Type != "string" || c.Text != functions.Now) {
			t.Errorf("now() = %s %q, want the time of the file %q", c.Type, c.Text, functions.Now)
		}
		if strings.HasPrefix(c.Expression, "date(") && c.Type == "string" && !rules.IsWorkflowTimeText(c.Text) {
			t.Errorf("%s: %q is not a time text", c.Name, c.Text)
		}
	}
}

func TestWorkflowExpressionLimitsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.WorkflowExpressionLimits()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.ErrorCode != rules.WorkflowExpressionErrorCode || vectors.MaxLength != rules.MaxWorkflowExpressionLength || vectors.MaxDepth != rules.MaxWorkflowExpressionDepth ||
		vectors.MaxStringLength != rules.MaxWorkflowExpressionStringLength || vectors.MaxEchoLength != rules.MaxWorkflowExpressionEchoLength {
		t.Errorf("limits = %d, %d, %d, %d; want %d, %d, %d, %d", vectors.MaxLength, vectors.MaxDepth, vectors.MaxStringLength, vectors.MaxEchoLength,
			rules.MaxWorkflowExpressionLength, rules.MaxWorkflowExpressionDepth, rules.MaxWorkflowExpressionStringLength, rules.MaxWorkflowExpressionEchoLength)
	}

	// 长度：按 UTF-16 码元数，正好在上限上的通过、多一个的不通过，还要有码位数不超而码元数超过的
	atLimit, overLimit, overOnlyInUnits := false, false, false
	for _, c := range vectors.Lengths {
		expression := c.Head + strings.Repeat(c.Padding, c.Count) + c.Tail
		length := rules.WorkflowExpressionLength(expression)
		if length != c.Length || c.Valid != (length <= rules.MaxWorkflowExpressionLength) {
			t.Errorf("%s: the expression is %d code units long (valid %v), the vector has %d (valid %v)", c.Name, length, length <= rules.MaxWorkflowExpressionLength, c.Length, c.Valid)
		}
		atLimit = atLimit || length == rules.MaxWorkflowExpressionLength
		overLimit = overLimit || length == rules.MaxWorkflowExpressionLength+1
		overOnlyInUnits = overOnlyInUnits || (!c.Valid && len([]rune(expression)) <= rules.MaxWorkflowExpressionLength)
	}
	if !atLimit || !overLimit || !overOnlyInUnits {
		t.Errorf("the lengths must keep one at the limit (%v), one over it by one (%v) and one over it only in code units (%v)", atLimit, overLimit, overOnlyInUnits)
	}

	// 嵌套与链：只核对两侧都有、拼出来的表达式在长度以内
	deep := map[bool]bool{}
	for _, c := range vectors.Nestings {
		deep[c.Valid] = true
		if c.Levels < 1 || c.Open == "" || c.Leaf == "" || rules.WorkflowExpressionLength(strings.Repeat(c.Open, c.Levels)+c.Leaf+strings.Repeat(c.Close, c.Levels)) > rules.MaxWorkflowExpressionLength {
			t.Errorf("%s: the nesting must fit the length limit", c.Name)
		}
	}
	if !deep[true] || !deep[false] {
		t.Error("the nestings must keep a valid and an invalid case")
	}
	for _, c := range vectors.Chains {
		if c.Count < 1 || rules.WorkflowExpressionLength(c.Head+strings.Repeat(c.Link, c.Count)) > rules.MaxWorkflowExpressionLength {
			t.Errorf("%s: the chain must fit the length limit", c.Name)
		}
		checkExpressionValue(t, c.Name, c.Type, c.Text)
	}

	// 拼接与渲染：成功与否只取决于长度有没有超过上限
	for _, block := range vectors.Blocks {
		if block.Variable == "" || block.Unit == "" || block.Repeat < 1 {
			t.Errorf("the block %q is incomplete", block.Variable)
		}
	}
	bounds := map[string]bool{}
	for _, c := range vectors.Concatenations {
		if c.Valid != (c.Length <= rules.MaxWorkflowExpressionStringLength) {
			t.Errorf("%s: length %d, valid %v", c.Name, c.Length, c.Valid)
		}
		bounds["concatenation at the limit"] = bounds["concatenation at the limit"] || c.Length == rules.MaxWorkflowExpressionStringLength
		bounds["concatenation over the limit"] = bounds["concatenation over the limit"] || c.Length == rules.MaxWorkflowExpressionStringLength+1
	}
	for _, c := range vectors.Renderings {
		if c.Valid != (c.Length <= rules.MaxWorkflowExpressionStringLength) {
			t.Errorf("%s: length %d, valid %v", c.Name, c.Length, c.Valid)
		}
		bounds["rendering at the limit"] = bounds["rendering at the limit"] || c.Length == rules.MaxWorkflowExpressionStringLength
		bounds["rendering over the limit"] = bounds["rendering over the limit"] || c.Length == rules.MaxWorkflowExpressionStringLength+1
	}
	if len(bounds) != 4 || slices.Contains([]bool{bounds["concatenation at the limit"], bounds["concatenation over the limit"], bounds["rendering at the limit"], bounds["rendering over the limit"]}, false) {
		t.Errorf("the concatenations and renderings must keep both sides of the limit: %v", bounds)
	}

	// 带出的文本：整段表达式、条件与没有结束的模板带出的就是截断的原文
	kept, cut, pair := false, false, false
	for _, c := range vectors.Echoes {
		echoed, ok := c.Arguments["expression"].(string)
		if !ok || !slices.Contains([]string{"evaluate", "condition", "render"}, c.Mode) || !slices.Contains(expressionKeys(), c.MessageKey) {
			t.Errorf("%s: the case must have a mode, an expression error and the expression argument", c.Name)
			continue
		}
		if c.Mode != "render" || c.MessageKey == "workflow.expression.unterminated_placeholder" {
			if want := rules.EchoWorkflowExpression(c.Source); echoed != want {
				t.Errorf("%s: echoed %q, want %q", c.Name, echoed, want)
			}
		}
		if units := rules.WorkflowExpressionLength(echoed); units > rules.MaxWorkflowExpressionEchoLength+1 {
			t.Errorf("%s: the echoed text is %d code units long", c.Name, units)
		}
		kept = kept || echoed == c.Source
		cut = cut || strings.HasSuffix(echoed, "…")
		pair = pair || (rules.WorkflowExpressionLength(echoed) == rules.MaxWorkflowExpressionEchoLength && strings.HasSuffix(echoed, "…"))
	}
	if !kept || !cut || !pair {
		t.Errorf("the echoes must keep one that is not cut (%v), one that is cut (%v) and one cut before a surrogate pair (%v)", kept, cut, pair)
	}
}

func TestWorkflowNumberTextsFollowTheGrammar(t *testing.T) {
	for _, text := range []string{"0", "7", "-7", "1.5", "-0.25", "100", "0.0000000000000000000000000001", "79228162514264337593543950335", "7.9228162514264337593543950335"} {
		if !rules.IsWorkflowNumberText(text) {
			t.Errorf("%q must be a number text", text)
		}
	}
	for _, text := range []string{"", "-0", "1.50", "1.", ".5", "+1", "007", "1e3", "0.00000000000000000000000000001", "79228162514264337593543950336", "7.9228162514264337593543950336", " 1", "1 "} {
		if rules.IsWorkflowNumberText(text) {
			t.Errorf("%q must not be a number text", text)
		}
	}
}
