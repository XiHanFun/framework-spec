package rules_test

import (
	"reflect"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestAuditExclusionsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.AuditExclusions()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range vectors.Entries.Valid {
		if !rules.IsAuditExclusion(entry) {
			t.Errorf("%q must be a valid exclusion", entry)
		}
	}
	for _, entry := range vectors.Entries.Invalid {
		if rules.IsAuditExclusion(entry) {
			t.Errorf("%q must be an invalid exclusion", entry)
		}
	}
	for _, c := range vectors.Matches {
		if got := rules.AuditExcludes(c.Exclude, c.Operation); got != c.Excluded {
			t.Errorf("%s: excluded = %v, want %v", c.Name, got, c.Excluded)
		}
		// 用例本身得是合法的配置与合法的操作 ID
		if !rules.IsOperationID(c.Operation) {
			t.Errorf("%s: %q is not an operation ID", c.Name, c.Operation)
		}
		for _, entry := range c.Exclude {
			if !rules.IsAuditExclusion(entry) {
				t.Errorf("%s: %q is not a valid exclusion", c.Name, entry)
			}
		}
	}
}

func TestAuditEventSelectionFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.AuditEventSelection()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.Defaults.Selection != rules.AuditSelectionProtected || !vectors.Defaults.IncludeSuccesses {
		t.Errorf("defaults = %+v, want %s and successes included", vectors.Defaults, rules.AuditSelectionProtected)
	}
	text := func(value *string) string {
		if value == nil {
			return ""
		}
		return *value
	}
	for _, c := range vectors.Selection {
		call := rules.AuditCall{
			Selection:        c.Options.Selection,
			IncludeSuccesses: c.Options.IncludeSuccesses,
			Exclude:          c.Options.Exclude,
			ExplicitAction:   text(c.ExplicitAction),
			Outcome:          c.Outcome,
			// 向量里管道写入的失败都是这个错误码
			FailureCode: "ORDERS_VALIDATION_001",
		}
		if c.Operation != nil {
			call.Operation = &rules.AuditOperation{ID: c.Operation.ID, Permission: text(c.Operation.Permission), Policy: text(c.Operation.Policy)}
		}
		record, recorded := rules.SelectAuditEvent(call)
		if recorded != (c.Expected != nil) {
			t.Errorf("%s: recorded = %v, want %v", c.Name, recorded, c.Expected != nil)
			continue
		}
		if c.Expected != nil && (record.Action != c.Expected.Action || record.Succeeded != c.Expected.Succeeded || record.Code != text(c.Expected.Code)) {
			t.Errorf("%s: event = %+v, want %s (succeeded %v, code %q)", c.Name, record, c.Expected.Action, c.Expected.Succeeded, text(c.Expected.Code))
		}
	}
	for _, c := range vectors.Attribution {
		principal, tenant := rules.AuditAttribution(c.Principal, c.BaggagePrincipal, c.TenantResolved, c.Tenant, c.BaggageTenant)
		if !reflect.DeepEqual(principal, c.Expected.Principal) || !reflect.DeepEqual(tenant, c.Expected.Tenant) {
			t.Errorf("%s: principal %q and tenant %q, want %q and %q", c.Name, text(principal), text(tenant), text(c.Expected.Principal), text(c.Expected.Tenant))
		}
	}
}
