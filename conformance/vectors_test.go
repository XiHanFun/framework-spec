package conformance_test

import (
	"reflect"
	"slices"
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

func TestTypedVectorsCoverEveryVectorFile(t *testing.T) {
	names, err := conformance.Names()
	if err != nil {
		t.Fatal(err)
	}
	// 表按 Names 的顺序一个文件一项：多一项、少一项、重复或次序不同都对不上
	var listed []string
	for _, vector := range conformance.TypedVectors() {
		listed = append(listed, vector.Name)
	}
	if !slices.Equal(listed, names) {
		t.Errorf("the vector file table lists %v, but the embedded vector files are %v", listed, names)
	}
}

func TestTypedLoadersAreNonEmpty(t *testing.T) {
	vectors := conformance.TypedVectors()
	for _, vector := range vectors {
		check, ok := nonEmptyChecks[vector.Name]
		if !ok {
			t.Errorf("%s: the vector file has no non-empty check", vector.Name)
			continue
		}
		t.Run(vector.Name, func(t *testing.T) {
			// 核对经类型化入口读取；表里的 Load 必须给出同一个值，否则表与入口指向了不同的文件或类型
			want := check(t)
			got, err := vector.Load()
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("the table entry loads a different value than the typed loader (%T, want %T): %v", got, want, err)
			}
		})
	}
	for name := range nonEmptyChecks {
		if !slices.ContainsFunc(vectors, func(vector conformance.TypedVector) bool { return vector.Name == name }) {
			t.Errorf("%s: the non-empty check has no entry in the vector file table", name)
		}
	}
}

// nonEmptyChecks 是每个向量文件的非空核对，键是文件名：经类型化入口读取、核对该有的部分都在，返回读到的值。
var nonEmptyChecks = map[string]func(t *testing.T) any{
	"plugin/handshake-lines.json": func(t *testing.T) any {
		handshake, err := conformance.HandshakeLines()
		if err != nil || len(handshake.Valid) == 0 || len(handshake.Invalid) == 0 {
			t.Errorf("handshake lines: %v", err)
		}
		return handshake
	},
	"plugin/host-addresses.json": func(t *testing.T) any {
		addresses, err := conformance.HostAddresses()
		if err != nil || len(addresses.Valid) == 0 || len(addresses.Invalid) == 0 || len(addresses.Selection) == 0 {
			t.Errorf("host addresses: %v", err)
		}
		for _, c := range addresses.Valid {
			if len(c.Entries) == 0 {
				t.Errorf("host address %q must list its entries", c.Value)
			}
		}
		return addresses
	},
	"plugin/deadlines.json": func(t *testing.T) any {
		deadlines, err := conformance.Deadlines()
		if err != nil || len(deadlines.Valid) == 0 || len(deadlines.Invalid) == 0 || len(deadlines.Effective) == 0 || len(deadlines.Written) == 0 {
			t.Errorf("deadlines: %v", err)
		}
		return deadlines
	},
	"plugin/plugin-messages.json": func(t *testing.T) any {
		messages, err := conformance.PluginMessages()
		if err != nil || len(messages.Messages) == 0 || len(messages.Reasons) == 0 {
			t.Errorf("plugin messages: %v", err)
		}
		return messages
	},
	"plugin/callback-status.json": func(t *testing.T) any {
		callbacks, err := conformance.CallbackStatus()
		if err != nil || callbacks.PluginID == "" || callbacks.Operation == "" || len(callbacks.Cases) == 0 {
			t.Errorf("callback status: %v", err)
		}
		for _, c := range callbacks.Cases {
			if (c.RPC != "invoke" && c.RPC != "publish") || c.Status == "" || c.Code == "" || c.Key == "" || len(c.Details) == 0 {
				t.Errorf("callback case %s %s is incomplete", c.RPC, c.Status)
			}
		}
		return callbacks
	},
	"plugin/error-reading.json": func(t *testing.T) any {
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
		return reading
	},
	"plugin/protocol-versions.json": func(t *testing.T) any {
		versions, err := conformance.ProtocolVersions()
		if err != nil || len(versions.Cases) == 0 || len(versions.Primitives) == 0 {
			t.Errorf("protocol versions: %v", err)
		}
		return versions
	},
	"plugin/wasm-exports.json": func(t *testing.T) any {
		wasm, err := conformance.WasmExports()
		if err != nil || wasm.ABIVersion != 1 || len(wasm.Exports.Required) == 0 || len(wasm.Imports.Xihan) == 0 {
			t.Errorf("wasm exports: %v", err)
		}
		return wasm
	},
	"primitives/error-codes.json": func(t *testing.T) any {
		codes, err := conformance.ErrorCodes()
		if err != nil || len(codes.Valid) == 0 {
			t.Errorf("error codes: %v", err)
		}
		return codes
	},
	"primitives/baggage-keys.json": func(t *testing.T) any {
		keys, err := conformance.BaggageKeys()
		if err != nil || len(keys.Registered) == 0 || len(keys.Values.Acceptable) == 0 || len(keys.Values.Unacceptable) == 0 {
			t.Errorf("baggage keys: %v", err)
		}
		return keys
	},
	"primitives/wire-encoding.json": func(t *testing.T) any {
		wire, err := conformance.WireEncoding()
		if err != nil || len(wire.Results.Valid) == 0 || len(wire.Results.Invalid) == 0 || len(wire.Errors.Valid) == 0 || len(wire.Errors.Invalid) == 0 {
			t.Errorf("wire encoding: %v", err)
		}
		return wire
	},
	"primitives/message-templates.json": func(t *testing.T) any {
		templates, err := conformance.MessageTemplates()
		if err != nil || len(templates.Keys.Valid) == 0 || len(templates.Keys.Invalid) == 0 || len(templates.Format) == 0 || len(templates.Describe) == 0 {
			t.Errorf("message templates: %v", err)
		}
		return templates
	},
	"primitives/kernel-messages.json": func(t *testing.T) any {
		kernel, err := conformance.KernelMessages()
		if err != nil || len(kernel.Messages) == 0 {
			t.Errorf("kernel messages: %v", err)
		}
		return kernel
	},
	"primitives/capability-requirements.json": func(t *testing.T) any {
		requirements, err := conformance.CapabilityRequirements()
		if err != nil || len(requirements.Valid) == 0 {
			t.Errorf("capability requirements: %v", err)
		}
		return requirements
	},
	"primitives/lifecycle-transitions.json": func(t *testing.T) any {
		transitions, err := conformance.LifecycleTransitions()
		if err != nil || len(transitions.Allowed) == 0 {
			t.Errorf("lifecycle transitions: %v", err)
		}
		return transitions
	},
	"primitives/trace-context.json": func(t *testing.T) any {
		trace, err := conformance.TraceContext()
		if err != nil || len(trace.Parse.Valid) == 0 || len(trace.Parse.Invalid) == 0 || len(trace.Format) == 0 {
			t.Errorf("trace context: %v", err)
		}
		return trace
	},
	"primitives/json-text.json": func(t *testing.T) any {
		text, err := conformance.JSONText()
		if err != nil || len(text.Cases) == 0 {
			t.Errorf("json text: %v", err)
		}
		return text
	},
	"manifest/manifests.json": func(t *testing.T) any {
		manifests, err := conformance.Manifests()
		if err != nil || len(manifests.Valid) == 0 || len(manifests.Invalid) == 0 {
			t.Errorf("manifests: %v", err)
		}
		return manifests
	},
	"pipeline/ordering.json": func(t *testing.T) any {
		pipeline, err := conformance.PipelineOrdering()
		if err != nil || len(pipeline.Cases) == 0 || len(pipeline.Errors) == 0 {
			t.Errorf("pipeline ordering: %v", err)
		}
		return pipeline
	},
	"operations/payload-reasons.json": func(t *testing.T) any {
		reasons, err := conformance.PayloadReasons()
		if err != nil || len(reasons.Requests) == 0 || len(reasons.Cases) == 0 {
			t.Errorf("payload reasons: %v", err)
		}
		return reasons
	},
	"provider/caching-distributed.json": func(t *testing.T) any {
		caching, err := conformance.CachingDistributed()
		if err != nil || caching.Capability != "caching.distributed@1" || len(caching.Operations) != 9 || len(caching.Samples) == 0 || len(caching.InvalidKeys) == 0 || caching.Errors["unavailable"] != "CACHING_UNAVAILABLE_001" {
			t.Errorf("caching distributed: %v", err)
		}
		for _, sample := range caching.Samples {
			if !slices.Contains(caching.Operations, sample.Operation) {
				t.Errorf("sample operation %q is not in the operation set", sample.Operation)
			}
		}
		return caching
	},
	"caching/keys.json": func(t *testing.T) any {
		keys, err := conformance.CacheKeys()
		if err != nil || keys.MaxBytes != 512 || len(keys.Keys.Valid) == 0 || len(keys.Keys.Invalid) == 0 || len(keys.Prefixes.Valid) == 0 || len(keys.Prefixes.Invalid) == 0 {
			t.Errorf("cache keys: %v", err)
		}
		names := map[string]bool{}
		for group, cases := range map[string][]conformance.CacheKeyCase{
			"keys.valid": keys.Keys.Valid, "keys.invalid": keys.Keys.Invalid,
			"prefixes.valid": keys.Prefixes.Valid, "prefixes.invalid": keys.Prefixes.Invalid,
		} {
			for _, c := range cases {
				if c.Name == "" || names[group+"/"+c.Name] {
					t.Errorf("cache key case %q in %s must have a unique name", c.Name, group)
				}
				names[group+"/"+c.Name] = true
			}
		}
		return keys
	},
	"security/webauthn.json": func(t *testing.T) any {
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
		return webauthn
	},
}
