package plugin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"

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

// placeholderHostEndpoint 是占位的宿主回调端点：运行器不提供回调服务，只验证插件侧。
func placeholderHostEndpoint() rules.Endpoint {
	if runtime.GOOS == "windows" {
		return rules.Endpoint{Network: "npipe", Address: `\\.\pipe\xihan-conformance-host`}
	}
	return rules.Endpoint{Network: "unix", Address: filepath.Join(os.TempDir(), "xihan-conformance-host.sock")}
}

// launchProcess 以宿主身份启动插件进程并读握手行。
func (r *runner) launchProcess(ctx context.Context) (*process, error) {
	l := launch{
		versions:     supportedVersions,
		primitives:   primitivesVersion,
		sessionToken: r.token,
		hostEndpoint: r.hostEndpoint,
		instanceID:   r.manifest.ID + "#conformance",
		locale:       "en-US",
	}
	return start(ctx, r.opts.Directory, r.manifest, l, r.opts.Timeout, r.opts.Stderr)
}

// handshakeRequest 组装握手请求。
func (r *runner) handshakeRequest(versions []uint32) *pluginv1.HandshakeRequest {
	return &pluginv1.HandshakeRequest{
		ProtocolVersions: versions,
		Primitives:       primitivesVersion,
		InstanceId:       "conformance",
		Host:             &pluginv1.HostInfo{FrameworkVersion: "spec", Sdk: "xihan-conformance", SdkVersion: Version, Application: "conformance"},
		HostEndpoint:     &pluginv1.Endpoint{Network: r.hostEndpoint.Network, Address: r.hostEndpoint.Address},
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
