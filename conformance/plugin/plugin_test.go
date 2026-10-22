package plugin

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xihanfun/framework-spec/conformance"
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
	env := environment(launch{versions: []uint32{1, 2}, primitives: 1, sessionToken: "token", hostEndpoint: placeholderHostEndpoint(), instanceID: "id", locale: "en-US"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "XIHAN_CONFORMANCE_SECRET") {
		t.Error("unlisted variables must not be inherited")
	}
	if !strings.Contains(joined, "DOTNET_CLI_TELEMETRY_OPTOUT=1") {
		t.Error("DOTNET_* variables must be inherited")
	}
	for _, expected := range []string{"XIHAN_PLUGIN_PROTOCOL_VERSIONS=1,2", "XIHAN_PLUGIN_PRIMITIVES=1", "XIHAN_PLUGIN_SESSION=token", "XIHAN_PLUGIN_INSTANCE=id", "XIHAN_PLUGIN_LOCALE=en-US", "XIHAN_PLUGIN_HOST_ADDRESS="} {
		if !strings.Contains(joined, expected) {
			t.Errorf("environment must contain %s", expected)
		}
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
