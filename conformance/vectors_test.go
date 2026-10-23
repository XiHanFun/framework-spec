package conformance_test

import (
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
)

func TestEveryVectorFileLoads(t *testing.T) {
	names, err := conformance.Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no vector files are embedded")
	}
	for _, name := range names {
		if _, err := conformance.Load[map[string]any](name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestTypedLoadersAreNonEmpty(t *testing.T) {
	handshake, err := conformance.HandshakeLines()
	if err != nil || len(handshake.Valid) == 0 || len(handshake.Invalid) == 0 {
		t.Errorf("handshake lines: %v", err)
	}
	versions, err := conformance.ProtocolVersions()
	if err != nil || len(versions.Cases) == 0 || len(versions.Primitives) == 0 {
		t.Errorf("protocol versions: %v", err)
	}
	wasm, err := conformance.WasmExports()
	if err != nil || wasm.ABIVersion != 1 || len(wasm.Exports.Required) == 0 || len(wasm.Imports.Xihan) == 0 {
		t.Errorf("wasm exports: %v", err)
	}
	codes, err := conformance.ErrorCodes()
	if err != nil || len(codes.Valid) == 0 {
		t.Errorf("error codes: %v", err)
	}
	keys, err := conformance.BaggageKeys()
	if err != nil || len(keys.Registered) == 0 {
		t.Errorf("baggage keys: %v", err)
	}
	requirements, err := conformance.CapabilityRequirements()
	if err != nil || len(requirements.Valid) == 0 {
		t.Errorf("capability requirements: %v", err)
	}
	transitions, err := conformance.LifecycleTransitions()
	if err != nil || len(transitions.Allowed) == 0 {
		t.Errorf("lifecycle transitions: %v", err)
	}
	caching, err := conformance.CachingDistributed()
	if err != nil || caching.Capability != "caching.distributed@1" || len(caching.Operations) != 8 || len(caching.Samples) == 0 || len(caching.InvalidKeys) == 0 || caching.Errors["unavailable"] != "CACHING_UNAVAILABLE_001" {
		t.Errorf("caching distributed: %v", err)
	}
	for _, sample := range caching.Samples {
		found := false
		for _, operation := range caching.Operations {
			found = found || operation == sample.Operation
		}
		if !found {
			t.Errorf("sample operation %q is not in the operation set", sample.Operation)
		}
	}
}
