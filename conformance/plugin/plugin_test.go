package plugin

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
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

func TestCaseTableCoversEveryCase(t *testing.T) {
	// 用例名（Case* 常量）与实现（runner 的 case* 方法）从源码里数出来，与用例表一一对上：加了常量或方法却没登记，这里就失败
	constants := map[string]string{}
	methods := map[string]bool{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			switch d := declaration.(type) {
			case *ast.GenDecl:
				if d.Tok != token.CONST {
					continue
				}
				for _, spec := range d.Specs {
					value := spec.(*ast.ValueSpec)
					for index, name := range value.Names {
						if !strings.HasPrefix(name.Name, "Case") || index >= len(value.Values) {
							continue
						}
						literal, ok := value.Values[index].(*ast.BasicLit)
						if !ok || literal.Kind != token.STRING {
							t.Fatalf("%s must be a string literal", name.Name)
						}
						text, err := strconv.Unquote(literal.Value)
						if err != nil {
							t.Fatal(err)
						}
						constants[name.Name] = text
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil || len(d.Recv.List) != 1 || !strings.HasPrefix(d.Name.Name, "case") {
					continue
				}
				if star, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
					if receiver, ok := star.X.(*ast.Ident); ok && receiver.Name == "runner" {
						methods[d.Name.Name] = true
					}
				}
			}
		}
	}
	if len(constants) == 0 || len(methods) == 0 {
		t.Fatalf("found %d case names and %d case methods in the sources", len(constants), len(methods))
	}

	names := map[string]bool{}
	implementations := map[string]bool{}
	for _, c := range caseTable {
		if names[c.name] {
			t.Errorf("case %q is in the table twice", c.name)
		}
		names[c.name] = true
		// 方法表达式的函数名形如 …/plugin.(*runner).caseHandshake
		function := runtime.FuncForPC(reflect.ValueOf(c.run).Pointer()).Name()
		method := function[strings.LastIndex(function, ".")+1:]
		if !methods[method] {
			t.Errorf("case %q runs %s, which is not a case method of the runner", c.name, function)
		}
		if implementations[method] {
			t.Errorf("case %q runs %s, which another case already runs", c.name, method)
		}
		implementations[method] = true
	}
	for constant, name := range constants {
		if !names[name] {
			t.Errorf("%s (%q) is not in the case table", constant, name)
		}
	}
	if len(names) != len(constants) {
		t.Errorf("the case table has %d cases, but %d case names are declared", len(names), len(constants))
	}
	for method := range methods {
		if !implementations[method] {
			t.Errorf("the runner method %s is not in the case table", method)
		}
	}
	if !slices.Equal(AllCases, caseNames()) {
		t.Errorf("AllCases = %v, want the case table's order %v", AllCases, caseNames())
	}
}

func TestVersionComesFromTheBuildInformation(t *testing.T) {
	module := func(path, version string) debug.Module { return debug.Module{Path: path, Version: version} }
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"no build information", nil, false, develVersion},
		{"installed at a tag", &debug.BuildInfo{Main: module(modulePath, "v1.2.3")}, true, "1.2.3"},
		{"installed at a prerelease tag", &debug.BuildInfo{Main: module(modulePath, "v1.0.0-rc.3")}, true, "1.0.0-rc.3"},
		{"built in the repository", &debug.BuildInfo{Main: module(modulePath, "(devel)")}, true, develVersion},
		{"main module without a version", &debug.BuildInfo{Main: module(modulePath, "")}, true, develVersion},
		{"a dependency of another module", &debug.BuildInfo{Main: module("example.com/sdk", "(devel)"), Deps: []*debug.Module{
			{Path: "google.golang.org/grpc", Version: "v1.0.0"},
			{Path: modulePath, Version: "v0.0.0-20261120000000-0123456789ab"},
		}}, true, "0.0.0-20261120000000-0123456789ab"},
		{"replaced by another version", &debug.BuildInfo{Main: module("example.com/sdk", "(devel)"), Deps: []*debug.Module{
			{Path: modulePath, Version: "v1.0.0", Replace: &debug.Module{Path: modulePath, Version: "v1.1.0"}},
		}}, true, "1.1.0"},
		{"replaced by a local directory", &debug.BuildInfo{Main: module("example.com/sdk", "(devel)"), Deps: []*debug.Module{
			{Path: modulePath, Version: "v1.0.0", Replace: &debug.Module{Path: "../spec/conformance"}},
		}}, true, develVersion},
		{"not in the build", &debug.BuildInfo{Main: module("example.com/sdk", "v1.0.0")}, true, develVersion},
	}
	for _, c := range cases {
		if got := moduleVersion(c.info, c.ok); got != c.want {
			t.Errorf("%s: version = %q, want %q", c.name, got, c.want)
		}
	}
	// 握手上报与 version 子命令都用 Version：不能是空的，也不带前缀 v
	if Version == "" || strings.HasPrefix(Version, "v") {
		t.Errorf("Version = %q, want a version without the leading v or %q", Version, develVersion)
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
