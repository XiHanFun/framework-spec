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
}
