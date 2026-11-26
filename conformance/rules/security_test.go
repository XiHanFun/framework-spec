package rules_test

import (
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestResourceNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.ResourceNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors.Valid) == 0 || len(vectors.Invalid) == 0 {
		t.Fatal("resource name vectors are incomplete")
	}
	var coverage surrogateCoverage
	for _, c := range vectors.Valid {
		text := caseInput(t, c.Name, c.Input)
		coverage.note(t, c.Name, text, c.UTF16, true)
		if !rules.IsResourceName(text) {
			t.Errorf("%s: %q must be a resource name", c.Name, text)
		}
	}
	for _, c := range vectors.Invalid {
		text := caseInput(t, c.Name, c.Input)
		coverage.note(t, c.Name, text, c.UTF16, false)
		if rules.IsResourceName(text) {
			t.Errorf("%s: %q must not be a resource name", c.Name, text)
		}
	}
	coverage.requireAll(t, "resource name", true)
}
