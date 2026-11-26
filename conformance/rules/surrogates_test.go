package rules_test

import (
	"testing"
	"unicode/utf8"

	"github.com/xihanfun/framework-spec/conformance"
)

// surrogateCoverage 记下一组标识样例盖住了孤立代理项的哪几种写法（RFC-0012「标识」，ADR-0316）：
// 单独的高代理项、单独的低代理项、颠倒的一对、末尾的高代理项，以及合法一侧的成对代理项。
type surrogateCoverage struct {
	loneHigh, loneLow, reversed, trailingHigh, pair, pairByUnits bool
}

// note 记下一条样例：不合法的按码元判断写法，合法的看有没有平面以外的字符、是不是以码元写出的。
func (c *surrogateCoverage) note(t *testing.T, name, text string, written conformance.CodeUnits, valid bool) {
	t.Helper()
	units, err := written.Units()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if valid {
		for _, char := range text {
			c.pair = c.pair || char > 0xFFFF
		}
		c.pairByUnits = c.pairByUnits || (written != nil && utf8.ValidString(text))
		return
	}
	for i, unit := range units {
		high, low := unit >= 0xD800 && unit <= 0xDBFF, unit >= 0xDC00 && unit <= 0xDFFF
		nextLow := i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF
		previousHigh := i > 0 && units[i-1] >= 0xD800 && units[i-1] <= 0xDBFF
		c.trailingHigh = c.trailingHigh || (high && i == len(units)-1)
		c.loneHigh = c.loneHigh || (high && i < len(units)-1 && !nextLow)
		c.loneLow = c.loneLow || (low && !previousHigh)
		c.reversed = c.reversed || (low && i+1 < len(units) && units[i+1] >= 0xD800 && units[i+1] <= 0xDBFF)
	}
}

// requireAll 要求四种孤立的代理项与合法的成对代理项都有样例；byUnits 为真时还要有一条以码元写出的合法样例，用来核对组字符串的写法。
func (c *surrogateCoverage) requireAll(t *testing.T, what string, byUnits bool) {
	t.Helper()
	if !c.loneHigh || !c.loneLow || !c.reversed || !c.trailingHigh || !c.pair || (byUnits && !c.pairByUnits) {
		t.Errorf("the %s vector must keep a lone high surrogate (found: %v), a lone low surrogate (%v), a reversed pair (%v), a trailing high surrogate (%v), a valid surrogate pair (%v) and, if required, a valid pair written as code units (%v)",
			what, c.loneHigh, c.loneLow, c.reversed, c.trailingHigh, c.pair, c.pairByUnits)
	}
}

// caseInput 取一条样例的文本，组不出来（码元写错了、文本与码元都写了）即失败。
func caseInput(t *testing.T, name string, input func() (string, error)) string {
	t.Helper()
	text, err := input()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return text
}
