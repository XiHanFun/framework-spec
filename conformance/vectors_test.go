package conformance_test

import (
	"strings"
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
	addresses, err := conformance.HostAddresses()
	if err != nil || len(addresses.Valid) == 0 || len(addresses.Invalid) == 0 || len(addresses.Selection) == 0 {
		t.Errorf("host addresses: %v", err)
	}
	for _, c := range addresses.Valid {
		if len(c.Entries) == 0 {
			t.Errorf("host address %q must list its entries", c.Value)
		}
	}
	deadlines, err := conformance.Deadlines()
	if err != nil || len(deadlines.Valid) == 0 || len(deadlines.Invalid) == 0 || len(deadlines.Effective) == 0 || len(deadlines.Written) == 0 {
		t.Errorf("deadlines: %v", err)
	}
	messages, err := conformance.PluginMessages()
	if err != nil || len(messages.Messages) == 0 || len(messages.Reasons) == 0 {
		t.Errorf("plugin messages: %v", err)
	}
	callbacks, err := conformance.CallbackStatus()
	if err != nil || callbacks.PluginID == "" || callbacks.Operation == "" || len(callbacks.Cases) == 0 {
		t.Errorf("callback status: %v", err)
	}
	for _, c := range callbacks.Cases {
		if (c.RPC != "invoke" && c.RPC != "publish") || c.Status == "" || c.Code == "" || c.Key == "" || len(c.Details) == 0 {
			t.Errorf("callback case %s %s is incomplete", c.RPC, c.Status)
		}
	}
	reading, err := conformance.ErrorReading()
	if err != nil || reading.PluginID == "" || len(reading.Cases) == 0 || len(reading.StatusWithoutError) == 0 {
		t.Errorf("error reading: %v", err)
	}
	names := map[string]bool{}
	for _, c := range reading.Cases {
		if c.Name == "" || names[c.Name] || c.Expected.Code == "" || len(c.Expected.Details) == 0 {
			t.Errorf("error reading case %q must have a unique name, an expected code and details (null for none)", c.Name)
		}
		names[c.Name] = true
	}
	for _, c := range reading.StatusWithoutError {
		// 宿主读插件给出阶段，插件读宿主给出回调的 RPC（Host.Invoke 另有操作 ID）
		host := c.Reader == "host" && c.Phase != "" && c.RPC == ""
		plugin := c.Reader == "plugin" && c.Phase == "" && (c.RPC == "publish" || (c.RPC == "invoke" && c.Operation != nil && *c.Operation != ""))
		if (!host && !plugin) || c.Expected.Code == "" {
			t.Errorf("status without error case %+v is malformed", c)
		}
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
	if err != nil || len(keys.Registered) == 0 || len(keys.Values.Acceptable) == 0 || len(keys.Values.Unacceptable) == 0 {
		t.Errorf("baggage keys: %v", err)
	}
	wire, err := conformance.WireEncoding()
	if err != nil || len(wire.Results.Valid) == 0 || len(wire.Results.Invalid) == 0 || len(wire.Errors.Valid) == 0 || len(wire.Errors.Invalid) == 0 {
		t.Errorf("wire encoding: %v", err)
	}
	templates, err := conformance.MessageTemplates()
	if err != nil || len(templates.Keys.Valid) == 0 || len(templates.Keys.Invalid) == 0 || len(templates.Format) == 0 || len(templates.Describe) == 0 {
		t.Errorf("message templates: %v", err)
	}
	kernel, err := conformance.KernelMessages()
	if err != nil || len(kernel.Messages) == 0 {
		t.Errorf("kernel messages: %v", err)
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
	if err != nil || caching.Capability != "caching.distributed@1" || len(caching.Operations) != 9 || len(caching.Samples) == 0 || len(caching.InvalidKeys) == 0 || caching.Errors["unavailable"] != "CACHING_UNAVAILABLE_001" {
		t.Errorf("caching distributed: %v", err)
	}
	webauthn, err := conformance.WebAuthn()
	if err != nil || webauthn.RelyingParty.ID == "" || len(webauthn.RelyingParty.Origins) == 0 || len(webauthn.Registration) == 0 || len(webauthn.Authentication) == 0 {
		t.Errorf("webauthn: %v", err)
	}
	for _, sample := range append(append([]conformance.WebAuthnCase{}, webauthn.Registration...), webauthn.Authentication...) {
		if (sample.Error == nil) == (len(sample.Expected) == 0) {
			t.Errorf("webauthn sample %q must have exactly one of expected and error", sample.Name)
		}
		if sample.Error != nil && !strings.HasPrefix(sample.Error.Code, "SECURITY_") {
			t.Errorf("webauthn sample %q has a non-security error code", sample.Name)
		}
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
