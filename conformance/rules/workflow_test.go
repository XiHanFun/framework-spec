package rules_test

import (
	"slices"
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
