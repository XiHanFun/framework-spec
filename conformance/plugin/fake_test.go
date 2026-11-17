package plugin

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/xihanfun/framework-spec/conformance/internal/pluginv1"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

// fakePluginName 是测试把自己的可执行文件复制成的插件入口名：以这个名字、带着会话令牌启动时，TestMain 扮演插件。
const fakePluginName = "xihan-fake-plugin"

// fakeModeFile 是插件目录里选择假插件行为的文件：内容为 legacy 时按以前各 SDK 不合规范的做法应答。
const fakeModeFile = "fake-mode"

// fakeManifest 是假插件的清单。
const fakeManifest = `{"id":"xihan.conformance.fake","version":"1.0.0","primitives":"1","stability":"experimental","kind":"plugin","isolation":"out-of-process","contributes":{"operations":["echoes.create"],"health":["fake"]},"permissions":["capability:kernel.pipeline"],"entry":"` + fakePluginName + `"}`

// fakeOperation 与 fakeTopic 是假插件声明的操作与订阅主题。
const (
	fakeOperation = "echoes.create"
	fakeTopic     = "echo.events"
)

func TestMain(m *testing.M) {
	if strings.HasPrefix(filepath.Base(os.Args[0]), fakePluginName) && os.Getenv("XIHAN_PLUGIN_SESSION") != "" {
		os.Exit(runFakePlugin())
	}
	os.Exit(m.Run())
}

func TestRunnerPassesARulesAbidingPlugin(t *testing.T) {
	results, log := runAgainstFake(t, "")
	if len(results) != len(AllCases) {
		t.Fatalf("ran %d cases, want %d", len(results), len(AllCases))
	}
	for _, result := range results {
		if !result.Passed || result.Skipped {
			t.Errorf("%s: passed=%v skipped=%v %s", result.Case, result.Passed, result.Skipped, result.Message)
		}
	}
	if t.Failed() {
		t.Log(log)
	}
}

func TestRunnerReportsAPluginThatBreaksTheRules(t *testing.T) {
	results, log := runAgainstFake(t, "legacy")
	// legacy 只违反插件一侧的新规则，原有的用例照常通过
	failing := map[string]bool{
		CaseHandshake: true, CaseSession: true, CaseOrder: true, CaseUnavailable: true,
		CaseUnsubscribed: true, CaseStream: true, CaseConfiguration: true,
	}
	for _, result := range results {
		if result.Passed == failing[result.Case] {
			t.Errorf("%s: passed=%v, want %v (%s)", result.Case, result.Passed, !failing[result.Case], result.Message)
		}
		if !result.Passed {
			t.Logf("%s: %s", result.Case, result.Message)
		}
	}
	if t.Failed() {
		t.Log(log)
	}
}

// runAgainstFake 把测试的可执行文件连同清单复制进临时的插件目录，以 mode 选定的行为跑全部用例。
func runAgainstFake(t *testing.T, mode string) ([]Result, string) {
	t.Helper()
	directory := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	name := fakePluginName
	if filepath.Ext(executable) == ".exe" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(directory, name), binary, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ManifestFileName), []byte(fakeManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, fakeModeFile), []byte(mode), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var log syncBuffer
	results, err := Run(ctx, Options{Directory: directory, Timeout: 10 * time.Second, Stderr: &log})
	if err != nil {
		t.Fatal(err)
	}
	return results, log.String()
}

// syncBuffer 是可以被多个协程写入的缓冲区，收集插件的 stderr。
type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

// Write 实现 io.Writer。
func (b *syncBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

// String 返回已写入的全部文本。
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// fakePlugin 是按 lifecycle.md §7 与 invocation.md §1–§4 应答的最小插件；legacy 为真时改用以前各 SDK 的不合规做法。
type fakePlugin struct {
	pluginv1.UnimplementedPluginServer
	legacy   bool
	token    string
	manifest string
	server   *grpc.Server
	mu       sync.Mutex
	state    pluginv1.LifecycleState
}

// runFakePlugin 是假插件进程的入口：核对回调端点列表、监听回环 TCP、写握手行，stdin 关闭时退出。
func runFakePlugin() int {
	mode, _ := os.ReadFile(fakeModeFile)
	manifest, err := os.ReadFile(ManifestFileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: read the manifest:", err)
		return 3
	}
	// 运行器必须给出合法的两项列表，回环 TCP 在最后（handshake.md §4）
	value := os.Getenv("XIHAN_PLUGIN_HOST_ADDRESS")
	endpoints, ok := rules.ParseHostAddresses(value)
	if !ok || len(endpoints) != 2 || endpoints[1].Network != "tcp" {
		fmt.Fprintf(os.Stderr, "error: XIHAN_PLUGIN_HOST_ADDRESS=%q is not a list of two endpoints ending with loopback TCP\n", value)
		return 3
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: listen:", err)
		return 3
	}
	plugin := &fakePlugin{legacy: strings.TrimSpace(string(mode)) == "legacy", token: os.Getenv("XIHAN_PLUGIN_SESSION"), manifest: string(manifest)}
	plugin.server = grpc.NewServer(grpc.UnaryInterceptor(plugin.unaryToken), grpc.StreamInterceptor(plugin.streamToken))
	pluginv1.RegisterPluginServer(plugin.server, plugin)
	go func() {
		// stdin 读到结尾即收尾请求（lifecycle.md §7.7）
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}()
	fmt.Printf("xihan-plugin/1 tcp %s\n", listener.Addr().String())
	if err := plugin.server.Serve(listener); err != nil {
		fmt.Fprintln(os.Stderr, "error: serve:", err)
		return 3
	}
	return 0
}

// authorized 核对元数据里恰好一个会话令牌值且与本次启动的令牌相同（handshake.md §4）；legacy 只看第一个值。
func (p *fakePlugin) authorized(ctx context.Context) error {
	incoming, _ := metadata.FromIncomingContext(ctx)
	values := incoming.Get(sessionMetadataKey)
	if p.legacy && len(values) > 0 && values[0] == p.token {
		return nil
	}
	if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(p.token)) != 1 {
		return status.Error(codes.Unauthenticated, "the session token is missing or invalid")
	}
	return nil
}

// unaryToken 在一元调用之前核对会话令牌。
func (p *fakePlugin) unaryToken(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if err := p.authorized(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, request)
}

// streamToken 在流式调用读第一帧之前核对会话令牌。
func (p *fakePlugin) streamToken(server any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := p.authorized(stream.Context()); err != nil {
		return err
	}
	return handler(server, stream)
}

// transition 在状态是 from 之一时迁到 to；legacy 不核对次序。
func (p *fakePlugin) transition(to pluginv1.LifecycleState, from ...pluginv1.LifecycleState) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.legacy && !containsState(from, p.state) {
		return status.Errorf(codes.FailedPrecondition, "the call is not allowed in state %s", p.state)
	}
	p.state = to
	return nil
}

// current 返回此刻的状态。
func (p *fakePlugin) current() pluginv1.LifecycleState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Handshake 实现 pluginv1.PluginServer。
func (p *fakePlugin) Handshake(_ context.Context, request *pluginv1.HandshakeRequest) (*pluginv1.HandshakeResponse, error) {
	if !containsVersion(request.ProtocolVersions, 1) {
		return nil, status.Error(codes.FailedPrecondition, "no common protocol version")
	}
	if err := p.transition(pluginv1.LifecycleState_LIFECYCLE_STATE_RESOLVED, pluginv1.LifecycleState_LIFECYCLE_STATE_DECLARED); err != nil {
		return nil, err
	}
	mode := "competing"
	if p.legacy {
		mode = ""
	}
	return &pluginv1.HandshakeResponse{
		ProtocolVersion: 1,
		Primitives:      1,
		Manifest:        strings.TrimPrefix(p.manifest, "\uFEFF"),
		Sdk:             &pluginv1.SdkInfo{Name: fakePluginName, Version: "0.0.0", Language: "go"},
		Operations:      []*pluginv1.OperationDeclaration{{Id: fakeOperation, Stability: "experimental"}},
		Subscriptions:   []*pluginv1.Subscription{{Topic: fakeTopic, Mode: mode}},
		Catalogs:        []*pluginv1.MessageCatalog{{Name: "fake", Locales: []*pluginv1.LocaleTemplates{{Locale: "en-US", Templates: map[string]string{"fake.hello": "Hello."}}}}},
	}, nil
}

// Configure 实现 pluginv1.PluginServer：配置节不是 JSON 对象时进入 Failed（lifecycle.md §7.2）。
func (p *fakePlugin) Configure(_ context.Context, request *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	if err := p.transition(pluginv1.LifecycleState_LIFECYCLE_STATE_CONFIGURED, pluginv1.LifecycleState_LIFECYCLE_STATE_RESOLVED); err != nil {
		return nil, err
	}
	configuration := strings.TrimSpace(request.Configuration)
	if configuration == "" {
		configuration = "{}"
	}
	if !p.legacy && (rules.CheckJSONText([]byte(configuration)) != nil || configuration[0] != '{') {
		p.mu.Lock()
		p.state = pluginv1.LifecycleState_LIFECYCLE_STATE_FAILED
		p.mu.Unlock()
		return &pluginv1.ConfigureResponse{Status: p.failure("PLUGINS_UNEXPECTED_001", "plugins.process.failed", map[string]string{"phase": "configure", "reason": "the configuration is not a JSON object"})}, nil
	}
	return &pluginv1.ConfigureResponse{Status: &pluginv1.Status{Ok: true}}, nil
}

// Start 实现 pluginv1.PluginServer：没有健康回调，启动后直接 Ready。
func (p *fakePlugin) Start(context.Context, *pluginv1.StartRequest) (*pluginv1.StartResponse, error) {
	if err := p.transition(pluginv1.LifecycleState_LIFECYCLE_STATE_READY, pluginv1.LifecycleState_LIFECYCLE_STATE_CONFIGURED); err != nil {
		return nil, err
	}
	return &pluginv1.StartResponse{Status: &pluginv1.Status{Ok: true}}, nil
}

// Stop 实现 pluginv1.PluginServer：应答之后优雅停止服务并以退出码 0 退出（lifecycle.md §7.5）。
func (p *fakePlugin) Stop(context.Context, *pluginv1.StopRequest) (*pluginv1.StopResponse, error) {
	accepted := []pluginv1.LifecycleState{
		pluginv1.LifecycleState_LIFECYCLE_STATE_RESOLVED, pluginv1.LifecycleState_LIFECYCLE_STATE_CONFIGURED, pluginv1.LifecycleState_LIFECYCLE_STATE_STARTED,
		pluginv1.LifecycleState_LIFECYCLE_STATE_READY, pluginv1.LifecycleState_LIFECYCLE_STATE_DEGRADED, pluginv1.LifecycleState_LIFECYCLE_STATE_FAILED,
	}
	if err := p.transition(pluginv1.LifecycleState_LIFECYCLE_STATE_DRAINING, accepted...); err != nil {
		return nil, err
	}
	go func() {
		// GracefulStop 等这次 Stop 的应答写出之后才返回
		done := make(chan struct{})
		go func() {
			p.server.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		os.Exit(0)
	}()
	return &pluginv1.StopResponse{Status: &pluginv1.Status{Ok: true}}, nil
}

// Health 实现 pluginv1.PluginServer：Start 之前报本地状态、没有健康项。
func (p *fakePlugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return p.health(), nil
}

// WatchHealth 实现 pluginv1.PluginServer：推一次当前状态，等调用方结束。
func (p *fakePlugin) WatchHealth(_ *pluginv1.HealthRequest, stream grpc.ServerStreamingServer[pluginv1.HealthResponse]) error {
	if err := stream.Send(p.health()); err != nil {
		return err
	}
	<-stream.Context().Done()
	return nil
}

// health 组装此刻的健康应答。
func (p *fakePlugin) health() *pluginv1.HealthResponse {
	state := p.current()
	response := &pluginv1.HealthResponse{State: state}
	if serving(state) {
		response.Items = []*pluginv1.HealthItem{{Name: "fake", Status: pluginv1.HealthStatus_HEALTH_STATUS_HEALTHY}}
	}
	return response
}

// Invoke 实现 pluginv1.PluginServer：先核对操作是否声明，再核对是否可服务（invocation.md §1）；legacy 反过来。
func (p *fakePlugin) Invoke(_ context.Context, request *pluginv1.InvokeRequest) (*pluginv1.InvokeResponse, error) {
	state := p.current()
	if p.legacy && !serving(state) {
		return &pluginv1.InvokeResponse{Status: &pluginv1.Status{Error: &pluginv1.Error{Code: "PLUGINS_UNAVAILABLE_001", MessageKey: "plugins.process.unavailable", Text: "The plugin is not ready.", Details: `{"reason":"not ready"}`}}}, nil
	}
	if request.Operation != fakeOperation {
		return nil, status.Errorf(codes.Unimplemented, "operation '%s' is not declared", request.Operation)
	}
	if !serving(state) {
		return &pluginv1.InvokeResponse{Status: p.unavailable(state)}, nil
	}
	payload := request.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	return &pluginv1.InvokeResponse{Status: &pluginv1.Status{Ok: true}, Payload: payload}, nil
}

// InvokeStream 实现 pluginv1.PluginServer：读第一帧，start 以 Unimplemented 结束，其余以 InvalidArgument 结束（invocation.md §1）。
func (p *fakePlugin) InvokeStream(stream grpc.BidiStreamingServer[pluginv1.StreamFrame, pluginv1.StreamFrame]) error {
	if p.legacy {
		return status.Error(codes.Unimplemented, "streaming is not supported")
	}
	frame, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "the stream ended before its first frame")
	}
	if frame.GetStart() == nil {
		return status.Error(codes.InvalidArgument, "the first frame must be start")
	}
	return status.Error(codes.Unimplemented, "streaming operations are not supported")
}

// HandleEvent 实现 pluginv1.PluginServer：按 invocation.md §4 的顺序核对信封与方式、订阅、是否可服务。
func (p *fakePlugin) HandleEvent(_ context.Context, request *pluginv1.HandleEventRequest) (*pluginv1.HandleEventResponse, error) {
	mode := request.Mode
	if !p.legacy && (request.Envelope == nil || (mode != "" && mode != "competing" && mode != "broadcast")) {
		return nil, status.Error(codes.InvalidArgument, "the delivery has no envelope or an unknown mode")
	}
	if mode == "" {
		mode = "competing"
	}
	envelope := request.GetEnvelope()
	if envelope.GetTopic() != fakeTopic || mode != "competing" {
		if p.legacy {
			return &pluginv1.HandleEventResponse{Status: &pluginv1.Status{Error: &pluginv1.Error{Code: "KERNEL_NOTFOUND_001", MessageKey: "kernel.operation.unknown", Details: `{"topic":"` + envelope.GetTopic() + `"}`}}}, nil
		}
		return &pluginv1.HandleEventResponse{Status: p.failure("KERNEL_NOTFOUND_001", "plugins.event.unsubscribed", map[string]string{"topic": envelope.GetTopic(), "type": envelope.GetType(), "mode": mode})}, nil
	}
	if state := p.current(); !serving(state) {
		return &pluginv1.HandleEventResponse{Status: p.unavailable(state)}, nil
	}
	return &pluginv1.HandleEventResponse{Status: &pluginv1.Status{Ok: true}}, nil
}

// unavailable 是不在可服务状态时的错误（lifecycle.md §7.3）。
func (p *fakePlugin) unavailable(state pluginv1.LifecycleState) *pluginv1.Status {
	return p.failure("PLUGINS_UNAVAILABLE_001", "plugins.process.unavailable", map[string]string{"state": rules.LifecycleStates[state]})
}

// failure 写出带消息键的错误：text 留空，details 是参数（含清单 ID）的 JSON 对象文本（invocation.md §3）。
func (p *fakePlugin) failure(code, key string, arguments map[string]string) *pluginv1.Status {
	arguments["id"] = "xihan.conformance.fake"
	details, _ := json.Marshal(arguments)
	return &pluginv1.Status{Error: &pluginv1.Error{Code: code, MessageKey: key, Details: string(details)}}
}

// serving 判断状态是否可服务。
func serving(state pluginv1.LifecycleState) bool {
	return state == pluginv1.LifecycleState_LIFECYCLE_STATE_READY || state == pluginv1.LifecycleState_LIFECYCLE_STATE_DEGRADED
}

// containsState 判断状态是否在列表里。
func containsState(states []pluginv1.LifecycleState, state pluginv1.LifecycleState) bool {
	for _, candidate := range states {
		if candidate == state {
			return true
		}
	}
	return false
}

// containsVersion 判断协议版本是否在列表里。
func containsVersion(versions []uint32, version uint32) bool {
	for _, candidate := range versions {
		if candidate == version {
			return true
		}
	}
	return false
}
