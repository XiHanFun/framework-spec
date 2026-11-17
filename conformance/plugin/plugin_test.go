package plugin

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestSameManifestIgnoresDefaultsAndEmptyValues(t *testing.T) {
	file := []byte(`{"id":"xihan.samples.echo","version":"1.0.0","primitives":"1","stability":"experimental","kind":"plugin","isolation":"out-of-process","contributes":{"operations":["echoes.create"],"health":["echo"]},"permissions":["capability:kernel.pipeline"],"entry":"echo"}`)
	handshake := []byte(`{"id":"xihan.samples.echo","version":"1.0.0","primitives":"1","stability":"experimental","kind":"plugin","isolation":"out-of-process","provides":[],"requires":[],"contributes":{"operations":["echoes.create"],"health":["echo"],"pipeline":[],"config":null},"activation":["onStartup"],"permissions":["capability:kernel.pipeline"],"entry":"echo"}`)
	_, fileRaw, err := parseManifest(file)
	if err != nil {
		t.Fatal(err)
	}
	_, handshakeRaw, err := parseManifest(handshake)
	if err != nil {
		t.Fatal(err)
	}
	if field, same := sameManifest(fileRaw, handshakeRaw); !same {
		t.Errorf("manifests must be the same, differ in %q", field)
	}

	changed := []byte(`{"id":"xihan.samples.echo","version":"1.0.1","primitives":"1","stability":"experimental","kind":"plugin","isolation":"out-of-process","contributes":{"operations":["echoes.create"],"health":["echo"]},"permissions":["capability:kernel.pipeline"],"entry":"echo"}`)
	_, changedRaw, err := parseManifest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if field, same := sameManifest(fileRaw, changedRaw); same || field != "version" {
		t.Errorf("manifests must differ in version, got same=%v field=%q", same, field)
	}
}

func TestEnvironmentKeepsOnlyTheWhitelist(t *testing.T) {
	t.Setenv("XIHAN_CONFORMANCE_SECRET", "leak")
	t.Setenv("DOTNET_CLI_TELEMETRY_OPTOUT", "1")
	t.Setenv("GOMAXPROCS", "2")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/secrets/key.json")
	hostAddresses, err := newHostAddresses()
	if err != nil {
		t.Fatal(err)
	}
	env := environment(launch{versions: []uint32{1, 2}, primitives: 1, sessionToken: "token", hostAddresses: hostAddresses, instanceID: "id", locale: "en-US"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "XIHAN_CONFORMANCE_SECRET") {
		t.Error("unlisted variables must not be inherited")
	}
	if !strings.Contains(joined, "DOTNET_CLI_TELEMETRY_OPTOUT=1") {
		t.Error("DOTNET_* variables must be inherited")
	}
	if !strings.Contains(joined, "GOMAXPROCS=2") {
		t.Error("Go runtime variables must be inherited")
	}
	if strings.Contains(joined, "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Error("GOOGLE_* credentials must not be inherited (ADR-0208)")
	}
	for _, expected := range []string{"XIHAN_PLUGIN_PROTOCOL_VERSIONS=1,2", "XIHAN_PLUGIN_PRIMITIVES=1", "XIHAN_PLUGIN_SESSION=token", "XIHAN_PLUGIN_INSTANCE=id", "XIHAN_PLUGIN_LOCALE=en-US", "XIHAN_PLUGIN_HOST_ADDRESS="} {
		if !strings.Contains(joined, expected) {
			t.Errorf("environment must contain %s", expected)
		}
	}
}

func TestHostAddressesAreAListEndingWithLoopbackTCP(t *testing.T) {
	hostAddresses, err := newHostAddresses()
	if err != nil {
		t.Fatal(err)
	}
	env := environment(launch{hostAddresses: hostAddresses})
	var value string
	for _, pair := range env {
		if rest, ok := strings.CutPrefix(pair, "XIHAN_PLUGIN_HOST_ADDRESS="); ok {
			value = rest
		}
	}
	// 宿主给出两项、回环 TCP 在最后（handshake.md §4），整个值按规范的文法读得出
	endpoints, ok := rules.ParseHostAddresses(value)
	if !ok || len(endpoints) != 2 || endpoints[1].Network != "tcp" {
		t.Fatalf("XIHAN_PLUGIN_HOST_ADDRESS=%q must be a list of two endpoints ending with loopback TCP", value)
	}
	want := "unix"
	if runtime.GOOS == "windows" {
		want = "npipe"
	}
	if endpoints[0].Network != want && endpoints[0].Network != "tcp" {
		t.Errorf("the first endpoint is %+v, want %s", endpoints[0], want)
	}
	if selected, ok := rules.SelectHostEndpoint(value, []string{"tcp"}); !ok || selected != endpoints[1] {
		t.Errorf("a plugin that only speaks tcp must select %+v, got %+v", endpoints[1], selected)
	}
}

func TestLineWriterForwardsCompleteLines(t *testing.T) {
	var target strings.Builder
	writer := newLineWriter(&target, "> ")
	_, _ = writer.Write([]byte("first\r\nsec"))
	_, _ = writer.Write([]byte("ond\nthird"))
	if target.String() != "> first\n> second\n" {
		t.Errorf("forwarded %q", target.String())
	}
}

func TestRunRejectsUnknownCasesAndMissingDirectories(t *testing.T) {
	if _, err := Run(context.Background(), Options{Directory: t.TempDir()}); err == nil {
		t.Error("a directory without a manifest must be rejected")
	}
	if _, err := Run(context.Background(), Options{Directory: ".", Cases: []string{"nope"}}); err == nil {
		t.Error("an unknown case must be rejected")
	}
}

// TestAgainstAPluginDirectory 以环境变量 XIHAN_CONFORMANCE_PLUGIN_DIR 指向的插件目录跑全部用例；
// XIHAN_CONFORMANCE_EXCLUDE 是逗号分隔的不调用操作。
func TestAgainstAPluginDirectory(t *testing.T) {
	directory := os.Getenv("XIHAN_CONFORMANCE_PLUGIN_DIR")
	if directory == "" {
		t.Skip("XIHAN_CONFORMANCE_PLUGIN_DIR is not set")
	}
	exclude := map[string]bool{}
	for _, operation := range strings.Split(os.Getenv("XIHAN_CONFORMANCE_EXCLUDE"), ",") {
		if operation != "" {
			exclude[operation] = true
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	results, err := Run(ctx, Options{Directory: directory, Exclude: exclude, Stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if !result.Passed {
			t.Errorf("%s: %s", result.Case, result.Message)
		}
	}
	if _, err := conformance.HandshakeLines(); err != nil {
		t.Fatal(err)
	}
}
