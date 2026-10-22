package rules_test

import (
	"slices"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestErrorCodesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.ErrorCodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range vectors.Valid {
		if !rules.IsErrorCode(code) {
			t.Errorf("%q must be a valid error code", code)
		}
	}
	for _, code := range vectors.Invalid {
		if rules.IsErrorCode(code) {
			t.Errorf("%q must be an invalid error code", code)
		}
	}
	if kind := rules.ErrorCodeKind("PERSISTENCE_CONFLICT_001"); kind != "CONFLICT" {
		t.Errorf("kind = %q, want CONFLICT", kind)
	}
}

func TestBaggageKeysFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BaggageKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range vectors.WellFormed {
		if !rules.IsBaggageKey(key) {
			t.Errorf("%q must be well-formed", key)
		}
	}
	for _, key := range vectors.IllFormed {
		if rules.IsBaggageKey(key) {
			t.Errorf("%q must be ill-formed", key)
		}
	}
	if !slices.Equal(vectors.Registered, rules.RegisteredBaggageKeys) {
		t.Errorf("registered keys = %v, want %v", rules.RegisteredBaggageKeys, vectors.Registered)
	}
}

func TestCapabilityRequirementsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.CapabilityRequirements()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Valid {
		parsed, ok := rules.ParseCapabilityRequirement(c.Input)
		if !ok {
			t.Errorf("%q must parse", c.Input)
			continue
		}
		if parsed.ID != c.ID || parsed.Version != c.Version || parsed.Optional != c.Optional {
			t.Errorf("%q parsed as %+v, want id=%s version=%d optional=%v", c.Input, parsed, c.ID, c.Version, c.Optional)
		}
	}
	for _, text := range vectors.Invalid {
		if _, ok := rules.ParseCapabilityRequirement(text); ok {
			t.Errorf("%q must not parse", text)
		}
	}
}

func TestLifecycleTransitionsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.LifecycleTransitions()
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[[2]string]bool{}
	for _, pair := range vectors.Allowed {
		allowed[pair] = true
		if !rules.CanTransition(pair[0], pair[1]) {
			t.Errorf("%s -> %s must be allowed", pair[0], pair[1])
		}
	}
	for _, from := range rules.LifecycleStates {
		for _, to := range rules.LifecycleStates {
			if rules.CanTransition(from, to) && !allowed[[2]string{from, to}] {
				t.Errorf("%s -> %s is allowed by the rules but absent from the vector", from, to)
			}
		}
	}
}

func TestHandshakeLinesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.HandshakeLines()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Valid {
		endpoint, ok := rules.ParseHandshakeLine(c.Line)
		if !ok {
			t.Errorf("%q must parse", c.Line)
			continue
		}
		if endpoint.Network != c.Network || endpoint.Address != c.Address {
			t.Errorf("%q parsed as %+v, want %s %s", c.Line, endpoint, c.Network, c.Address)
		}
	}
	for _, line := range vectors.Invalid {
		if _, ok := rules.ParseHandshakeLine(line); ok {
			t.Errorf("%q must not parse", line)
		}
	}
}

func TestVersionNegotiationFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.ProtocolVersions()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Cases {
		selected, ok := rules.NegotiateVersion(c.Host, c.Plugin)
		if c.Error != "" {
			if ok {
				t.Errorf("host %v plugin %v must fail, got %d", c.Host, c.Plugin, selected)
			}
			continue
		}
		if !ok || selected != c.Selected {
			t.Errorf("host %v plugin %v selected %d (ok=%v), want %d", c.Host, c.Plugin, selected, ok, c.Selected)
		}
	}
}

func TestGrammars(t *testing.T) {
	for _, id := range []string{"orders.create", "order-lines.list", "echoes.relay"} {
		if !rules.IsOperationID(id) {
			t.Errorf("%q must be an operation ID", id)
		}
	}
	for _, id := range []string{"", "Orders.Create", "orders", "orders.", "orders.create.now", "-orders.create"} {
		if rules.IsOperationID(id) {
			t.Errorf("%q must not be an operation ID", id)
		}
	}
	if !rules.IsTopic("echo.events") || rules.IsTopic("") || rules.IsTopic("Echo") {
		t.Error("topic grammar")
	}
	if !rules.IsManifestID("xihan.samples.echo") || rules.IsManifestID("samples.echo") {
		t.Error("manifest ID grammar")
	}
	if len(rules.PipelineStages) != 11 || rules.PipelineStages[0] != "ingress" || rules.PipelineStages[10] != "egress" {
		t.Error("pipeline stages")
	}
}
