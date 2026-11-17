package plugin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/xihanfun/framework-spec/conformance/internal/pluginv1"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

// supportedVersions 是运行器支持的协议版本。
var supportedVersions = []uint32{1}

// primitivesVersion 是运行器的原语层大版本。
const primitivesVersion uint32 = 1

// stabilities 是操作声明允许的稳定性。
var stabilities = []string{"stable", "preview", "experimental", "internal", "deprecated"}

// session 是一个握手成功的插件进程。
type session struct {
	process   *process
	handshake *pluginv1.HandshakeResponse
}

// close 结束会话。
func (s *session) close() { s.process.close() }

// client 是插件服务的客户端。
func (s *session) client() pluginv1.PluginClient { return s.process.client }

// newSessionToken 生成 32 字节的随机会话令牌（base64url）。
func newSessionToken() string {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(token)
}

// newHostAddresses 组装运行器交给插件的回调端点列表（handshake.md §4）：与 .NET 宿主一样两项、回环 TCP 在最后，
// Windows 上是命名管道加回环 TCP，其他系统是 Unix 域套接字加回环 TCP。运行器不提供 Host 服务，两项都没人监听，
// 只用来让每个用例都考到插件解析列表、回调端点不可用时照常服务。
func newHostAddresses() ([]rules.Endpoint, error) {
	ports, err := unusedPorts(2)
	if err != nil {
		return nil, err
	}
	suffix := randomID()[:12]
	loopback := rules.Endpoint{Network: "tcp", Address: "127.0.0.1:" + strconv.Itoa(ports[0])}
	if runtime.GOOS == "windows" {
		return []rules.Endpoint{{Network: "npipe", Address: `\\.\pipe\xihan-conformance-` + suffix}, loopback}, nil
	}
	socket := filepath.Join(os.TempDir(), "xihan-conformance-"+suffix, "host.sock")
	// 临时目录的路径含空白或 ; 时宿主不提供 Unix 域套接字（handshake.md §4）；这里改用第二个回环 TCP 端点，列表仍是两项
	if strings.ContainsFunc(socket, rules.IsWhitespace) || strings.Contains(socket, ";") {
		return []rules.Endpoint{{Network: "tcp", Address: "127.0.0.1:" + strconv.Itoa(ports[1])}, loopback}, nil
	}
	return []rules.Endpoint{{Network: "unix", Address: socket}, loopback}, nil
}

// unusedPorts 取 count 个此刻没人监听的回环端口：同时监听端口 0 让系统分配互不相同的端口，再全部关掉。
func unusedPorts(count int) ([]int, error) {
	ports := make([]int, 0, count)
	listeners := make([]net.Listener, 0, count)
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	for range count {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("pick an unused loopback port: %w", err)
		}
		listeners = append(listeners, listener)
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}

// launchProcess 以宿主身份启动插件进程并读握手行。
func (r *runner) launchProcess(ctx context.Context) (*process, error) {
	l := launch{
		versions:      supportedVersions,
		primitives:    primitivesVersion,
		sessionToken:  r.token,
		hostAddresses: r.hostAddresses,
		instanceID:    r.manifest.ID + "#conformance",
		locale:        "en-US",
	}
	return start(ctx, r.opts.Directory, r.manifest, l, r.opts.Timeout, r.opts.Stderr)
}

// handshakeRequest 组装握手请求；host_endpoint 是端点列表的第一项，只供诊断，插件选回调端点以环境变量为准（handshake.md §3）。
func (r *runner) handshakeRequest(versions []uint32) *pluginv1.HandshakeRequest {
	return &pluginv1.HandshakeRequest{
		ProtocolVersions: versions,
		Primitives:       primitivesVersion,
		InstanceId:       "conformance",
		Host:             &pluginv1.HostInfo{FrameworkVersion: "spec", Sdk: "xihan-conformance", SdkVersion: Version, Application: "conformance"},
		HostEndpoint:     &pluginv1.Endpoint{Network: r.hostAddresses[0].Network, Address: r.hostAddresses[0].Address},
		DefaultLocale:    "en-US",
	}
}

// open 启动、握手并按 handshake.md §3 校验。
func (r *runner) open(ctx context.Context) (*session, error) {
	p, err := r.launchProcess(ctx)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := r.deadline(ctx)
	defer cancel()
	response, err := p.client.Handshake(callCtx, r.handshakeRequest(supportedVersions))
	if err != nil {
		p.close()
		return nil, fmt.Errorf("handshake: %w", err)
	}
	if err := r.validateHandshake(response); err != nil {
		p.close()
		return nil, fmt.Errorf("handshake: %w", err)
	}
	return &session{process: p, handshake: response}, nil
}

// validateHandshake 核对版本协商、原语层版本、清单一致性、操作、订阅与目录（handshake.md §3）。
func (r *runner) validateHandshake(response *pluginv1.HandshakeResponse) error {
	if !slices.Contains(supportedVersions, response.ProtocolVersion) {
		return fmt.Errorf("protocol version %d is not supported by the host", response.ProtocolVersion)
	}
	if response.Primitives != primitivesVersion {
		return fmt.Errorf("primitives version %d does not match the host's %d", response.Primitives, primitivesVersion)
	}

	// 插件写清单文件的原文、去掉开头的 BOM（handshake.md §3 第 3 条）
	if strings.HasPrefix(response.Manifest, "\uFEFF") {
		return errors.New("the manifest in the handshake must not start with a UTF-8 BOM")
	}
	declared, declaredRaw, err := parseManifest([]byte(response.Manifest))
	if err != nil {
		return fmt.Errorf("the manifest in the handshake is invalid: %w", err)
	}
	if declared.Kind != "plugin" || declared.Isolation != "out-of-process" {
		return fmt.Errorf("the manifest in the handshake does not describe an out-of-process plugin")
	}
	if field, same := sameManifest(r.raw, declaredRaw); !same {
		return fmt.Errorf("the manifest in the handshake differs from the manifest file in %q", field)
	}

	seen := map[string]bool{}
	for _, operation := range response.Operations {
		if !rules.IsOperationID(operation.Id) {
			return fmt.Errorf("operation ID %q is not valid", operation.Id)
		}
		if !slices.Contains(r.manifest.Contributes.Operations, operation.Id) {
			return fmt.Errorf("operation %q is not declared in contributes.operations", operation.Id)
		}
		if seen[operation.Id] {
			return fmt.Errorf("operation %q is declared more than once", operation.Id)
		}
		seen[operation.Id] = true
		if !slices.Contains(stabilities, operation.Stability) {
			return fmt.Errorf("operation %q has an invalid stability %q", operation.Id, operation.Stability)
		}
		if operation.Permission != "" && operation.Policy != "" {
			return fmt.Errorf("operation %q declares both a permission and a policy", operation.Id)
		}
	}

	subscriptions := map[string]bool{}
	for _, subscription := range response.Subscriptions {
		if !rules.IsTopic(subscription.Topic) {
			return fmt.Errorf("subscription topic %q is not valid", subscription.Topic)
		}
		key := subscription.Topic + "\x00" + subscription.MessageType
		if subscriptions[key] {
			return fmt.Errorf("subscription %q is declared more than once", subscription.Topic)
		}
		subscriptions[key] = true
	}

	for _, catalog := range response.Catalogs {
		if len(catalog.Locales) == 0 {
			return fmt.Errorf("message catalog %q has no locales", catalog.Name)
		}
		for _, locale := range catalog.Locales {
			if locale.Locale == "" {
				return fmt.Errorf("message catalog %q has a locale without a name", catalog.Name)
			}
		}
	}
	return nil
}
