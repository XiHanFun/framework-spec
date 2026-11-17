package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xihanfun/framework-spec/conformance/internal/pluginv1"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

// errSkip 表示用例因插件未声明相关能力而跳过。
type errSkip struct{ reason string }

// Error 实现 error。
func (e errSkip) Error() string { return e.reason }

// servingStates 是 Start 之后健康应答里可接受的状态。
var servingStates = []pluginv1.LifecycleState{pluginv1.LifecycleState_LIFECYCLE_STATE_STARTED, pluginv1.LifecycleState_LIFECYCLE_STATE_READY, pluginv1.LifecycleState_LIFECYCLE_STATE_DEGRADED}

// traceParent 是用例调用携带的 W3C traceparent。
const traceParent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

// missingOperation 是用例里调用的未声明操作。
const missingOperation = "conformance.missing"

// wrongToken 是用例里带的错误会话令牌。
const wrongToken = "not-the-session-token"

// 用例核对的错误码、消息键与固定的 reason（invocation.md §8、plugin/plugin-messages.json）。
const (
	// codeUnavailable 是不在可服务状态时的错误码。
	codeUnavailable = "PLUGINS_UNAVAILABLE_001"
	// keyUnavailable 是不在可服务状态时的消息键，参数 id、state。
	keyUnavailable = "plugins.process.unavailable"
	// codeFailed 是插件一侧失败的错误码。
	codeFailed = "PLUGINS_UNEXPECTED_001"
	// keyFailed 是插件一侧失败的消息键，参数 id、phase、reason。
	keyFailed = "plugins.process.failed"
	// codeUnsubscribed 是投递找不到订阅时的错误码。
	codeUnsubscribed = "KERNEL_NOTFOUND_001"
	// keyUnsubscribed 是投递找不到订阅时的消息键，参数 id、topic、type、mode。
	keyUnsubscribed = "plugins.event.unsubscribed"
	// configurationReason 是配置节不是 JSON 对象时的 reason（lifecycle.md §7.2）。
	configurationReason = "the configuration is not a JSON object"
)

// caseHandshake：进程写出合法握手行、协商出宿主支持的版本、原语层版本相同、清单与文件一致、操作与订阅都在声明范围内；
// 另核对插件一侧的写法：primitives 取自清单、只声明一元操作、订阅的方式总是写出（handshake.md §3）。
func (r *runner) caseHandshake(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if s.handshake.ProtocolVersion != supportedVersions[len(supportedVersions)-1] {
		return fmt.Errorf("negotiated version %d, want %d", s.handshake.ProtocolVersion, supportedVersions[len(supportedVersions)-1])
	}
	// 插件写清单的 primitives（第 2 条）
	if primitives := strconv.FormatUint(uint64(s.handshake.Primitives), 10); primitives != r.manifest.Primitives {
		return fmt.Errorf("the handshake reports primitives %s, want the manifest's %q", primitives, r.manifest.Primitives)
	}
	// 流式操作等流式契约的 RFC，插件只声明一元操作（第 4 条）
	for _, operation := range s.handshake.Operations {
		if operation.Streaming {
			return fmt.Errorf("operation %q is declared with streaming: true; plugins declare only unary operations", operation.Id)
		}
	}
	// 插件按声明顺序写订阅，方式总是 competing 或 broadcast、不写空（第 5 条）
	for _, subscription := range s.handshake.Subscriptions {
		if subscription.Mode != "competing" && subscription.Mode != "broadcast" {
			return fmt.Errorf("the subscription to %q (type %q) has mode %q; want competing or broadcast", subscription.Topic, subscription.MessageType, subscription.Mode)
		}
	}
	return nil
}

// caseRefusal：宿主没有共同版本时握手必须以 FailedPrecondition 拒绝，而不是返回宿主不支持的版本。
func (r *runner) caseRefusal(ctx context.Context) error {
	p, err := r.launchProcess(ctx)
	if err != nil {
		return err
	}
	defer p.close()
	response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandshakeResponse, error) {
		return p.client.Handshake(c, r.handshakeRequest([]uint32{math.MaxUint32}))
	})
	if err == nil {
		return fmt.Errorf("the handshake must be refused, got protocol version %d", response.ProtocolVersion)
	}
	return expectCode(err, codes.FailedPrecondition, "a handshake without a common version")
}

// caseSession：宿主的调用不带会话令牌、令牌不对或带了多个值时，插件必须以 Unauthenticated 拒绝，带对了照常握手（handshake.md §4）。
func (r *runner) caseSession(ctx context.Context) error {
	p, err := r.launchProcess(ctx)
	if err != nil {
		return err
	}
	defer p.close()
	// 元数据里必须恰好有一个值：两个值即使有一个对（不论先后）、两个都对也按不匹配拒绝
	attempts := []struct {
		tokens []string
		what   string
	}{
		{nil, "a handshake without the session token"},
		{[]string{wrongToken}, "a handshake with a wrong session token"},
		{[]string{r.token, wrongToken}, "a handshake with two session token values (the right one, then a wrong one)"},
		{[]string{wrongToken, r.token}, "a handshake with two session token values (a wrong one, then the right one)"},
		{[]string{r.token, r.token}, "a handshake with the right session token twice"},
	}
	for _, attempt := range attempts {
		conn, err := dial(p.endpoint, attempt.tokens...)
		if err != nil {
			return err
		}
		_, err = call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandshakeResponse, error) {
			return pluginv1.NewPluginClient(conn).Handshake(c, r.handshakeRequest(supportedVersions))
		})
		_ = conn.Close()
		if err := expectCode(err, codes.Unauthenticated, attempt.what); err != nil {
			return err
		}
	}
	if _, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandshakeResponse, error) {
		return p.client.Handshake(c, r.handshakeRequest(supportedVersions))
	}); err != nil {
		return fmt.Errorf("a handshake with the session token must succeed: %v", err)
	}
	return nil
}

// caseLifecycle：Configure 与 Start 成功；Health 与 WatchHealth 报可服务状态；Stop 成功后进程在宽限内退出。
func (r *runner) caseLifecycle(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if err := r.startPlugin(ctx, s); err != nil {
		return err
	}

	callCtx, cancel := r.deadline(ctx)
	health, err := s.client().Health(callCtx, &pluginv1.HealthRequest{})
	cancel()
	if err != nil {
		return fmt.Errorf("health: %w", err)
	}
	if !slices.Contains(servingStates, health.State) {
		return fmt.Errorf("health reported %s after Start; want Started, Ready or Degraded", health.State)
	}
	for _, item := range health.Items {
		if !slices.Contains(r.manifest.Contributes.Health, item.Name) {
			return fmt.Errorf("health item %q is not declared in contributes.health", item.Name)
		}
	}

	watchCtx, cancelWatch := r.deadline(ctx)
	defer cancelWatch()
	watch, err := s.client().WatchHealth(watchCtx, &pluginv1.HealthRequest{})
	if err != nil {
		return fmt.Errorf("watch health: %w", err)
	}
	first, err := watch.Recv()
	if err != nil {
		return fmt.Errorf("WatchHealth must push the current state first: %w", err)
	}
	if !slices.Contains(servingStates, first.State) {
		return fmt.Errorf("WatchHealth pushed %s; want Started, Ready or Degraded", first.State)
	}
	cancelWatch()

	stopCtx, cancelStop := r.deadline(ctx)
	defer cancelStop()
	stopped, err := s.client().Stop(stopCtx, &pluginv1.StopRequest{GraceSeconds: 5, Reason: "unload"})
	if err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	if stopped.Status == nil || !stopped.Status.Ok {
		return fmt.Errorf("Stop must succeed, got %s", describeStatus(stopped.Status))
	}
	if !s.process.waitExit(5 * time.Second) {
		return errors.New("the plugin must exit within the grace period after Stop")
	}
	return nil
}

// caseOrder：不在次序表里的调用以 FailedPrecondition 拒绝、状态不变；Start 之前的 Health 报本地状态、没有健康项；
// Start 应答时已经求过一次健康，之后是 Ready 或 Degraded（lifecycle.md §7.1、§7.3、§7.4）。
func (r *runner) caseOrder(ctx context.Context) error {
	p, err := r.launchProcess(ctx)
	if err != nil {
		return err
	}
	defer p.close()
	client := p.client

	// 握手之前只接受 Handshake（与 Health、调用、投递）
	if err := r.expectConfigureRefused(ctx, client, "Configure before Handshake"); err != nil {
		return err
	}
	_, err = call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.StopResponse, error) {
		return client.Stop(c, &pluginv1.StopRequest{GraceSeconds: 5, Reason: "unload"})
	})
	if err := expectCode(err, codes.FailedPrecondition, "Stop before Handshake"); err != nil {
		return err
	}

	handshake, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandshakeResponse, error) {
		return client.Handshake(c, r.handshakeRequest(supportedVersions))
	})
	if err != nil {
		return fmt.Errorf("handshake after the refused calls: %w", err)
	}
	if err := r.validateHandshake(handshake); err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	_, err = call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandshakeResponse, error) {
		return client.Handshake(c, r.handshakeRequest(supportedVersions))
	})
	if err := expectCode(err, codes.FailedPrecondition, "a second Handshake"); err != nil {
		return err
	}
	if err := r.expectStartRefused(ctx, client, "Start without Configure"); err != nil {
		return err
	}
	if err := r.expectHealth(ctx, client, "after Handshake", true, pluginv1.LifecycleState_LIFECYCLE_STATE_RESOLVED); err != nil {
		return err
	}

	if err := r.configure(ctx, client); err != nil {
		return err
	}
	if err := r.expectHealth(ctx, client, "after Configure", true, pluginv1.LifecycleState_LIFECYCLE_STATE_CONFIGURED); err != nil {
		return err
	}
	if err := r.expectConfigureRefused(ctx, client, "a second Configure"); err != nil {
		return err
	}

	if err := r.start(ctx, client); err != nil {
		return err
	}
	if err := r.expectStartRefused(ctx, client, "a second Start"); err != nil {
		return err
	}
	// Start 应答之前插件已经按健康回调的结果进入 Ready、Degraded 或 Failed；Failed 不可服务，不算通过
	return r.expectHealth(ctx, client, "after Start", false, pluginv1.LifecycleState_LIFECYCLE_STATE_READY, pluginv1.LifecycleState_LIFECYCLE_STATE_DEGRADED)
}

// caseUnknownOperation：未声明的操作以 gRPC Unimplemented 结束，而不是业务失败。
func (r *runner) caseUnknownOperation(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if err := r.startPlugin(ctx, s); err != nil {
		return err
	}
	response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.InvokeResponse, error) {
		return s.client().Invoke(c, &pluginv1.InvokeRequest{Operation: missingOperation, Payload: []byte("{}")})
	})
	if err == nil {
		return fmt.Errorf("an unknown operation must end with Unimplemented, got %s", describeStatus(response.Status))
	}
	if code := status.Code(err); code != codes.Unimplemented {
		return fmt.Errorf("an unknown operation must end with Unimplemented, got %s: %v", code, err)
	}
	return nil
}

// caseDeclaredOperations：每个声明的一元操作都返回带 status 的应答；成功时载荷是 JSON，失败时错误码合法；gRPC 状态保持 OK。
func (r *runner) caseDeclaredOperations(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if err := r.startPlugin(ctx, s); err != nil {
		return err
	}
	for _, operation := range s.handshake.Operations {
		if operation.Streaming || r.opts.Exclude[operation.Id] {
			continue
		}
		response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.InvokeResponse, error) {
			return s.client().Invoke(c, r.invokeRequest(operation.Id))
		})
		if err != nil {
			return fmt.Errorf("operation %q: %w", operation.Id, err)
		}
		if response.Status == nil {
			return fmt.Errorf("operation %q must return a status", operation.Id)
		}
		if response.Status.Ok {
			if len(response.Payload) > 0 && !json.Valid(response.Payload) {
				return fmt.Errorf("operation %q returned a payload that is not JSON", operation.Id)
			}
			continue
		}
		if response.Status.Error == nil {
			return fmt.Errorf("operation %q failed without an error", operation.Id)
		}
		if !rules.IsErrorCode(response.Status.Error.Code) {
			return fmt.Errorf("operation %q returned an invalid error code %q", operation.Id, response.Status.Error.Code)
		}
	}
	return nil
}

// caseUnavailable：Configure 之后、Start 之前，未声明的操作仍先报 gRPC Unimplemented；声明的操作与第一个订阅的投递
// 都以 PLUGINS_UNAVAILABLE_001 + plugins.process.unavailable 失败，state 为 Configured（invocation.md §1、§4，lifecycle.md §7.3）。
func (r *runner) caseUnavailable(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if err := r.configure(ctx, s.client()); err != nil {
		return err
	}
	// 操作是否声明先于就绪核对
	_, err = call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.InvokeResponse, error) {
		return s.client().Invoke(c, &pluginv1.InvokeRequest{Operation: missingOperation, Payload: []byte("{}")})
	})
	if err := expectCode(err, codes.Unimplemented, "invoking an undeclared operation before Start"); err != nil {
		return err
	}

	details := map[string]string{"id": r.manifest.ID, "state": "Configured"}
	for _, operation := range s.handshake.Operations {
		if r.opts.Exclude[operation.Id] {
			continue
		}
		what := fmt.Sprintf("operation %q invoked before Start", operation.Id)
		response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.InvokeResponse, error) {
			return s.client().Invoke(c, r.invokeRequest(operation.Id))
		})
		if err != nil {
			return fmt.Errorf("%s must return a status, got %w", what, err)
		}
		if err := expectFailure(response.Status, codeUnavailable, keyUnavailable, details, what); err != nil {
			return err
		}
	}

	if len(s.handshake.Subscriptions) > 0 {
		subscription := s.handshake.Subscriptions[0]
		what := fmt.Sprintf("a delivery to the subscription %q before Start", subscription.Topic)
		response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandleEventResponse, error) {
			return s.client().HandleEvent(c, eventRequest(subscription.Topic, subscriptionType(subscription), deliveryMode(subscription.Mode)))
		})
		if err != nil {
			return fmt.Errorf("%s must return a status, got %w", what, err)
		}
		if err := expectFailure(response.Status, codeUnavailable, keyUnavailable, details, what); err != nil {
			return err
		}
	}
	return nil
}

// caseEvent：向声明的订阅投递一条消息必须得到确认；没有订阅时跳过。
func (r *runner) caseEvent(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if len(s.handshake.Subscriptions) == 0 {
		return errSkip{"the plugin declares no subscriptions"}
	}
	subscription := s.handshake.Subscriptions[0]
	if err := r.startPlugin(ctx, s); err != nil {
		return err
	}
	// 与宿主一样按订阅声明的方式投递：插件只把消息交给以这种方式订阅的处理器（invocation.md §4）
	response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandleEventResponse, error) {
		return s.client().HandleEvent(c, eventRequest(subscription.Topic, subscriptionType(subscription), deliveryMode(subscription.Mode)))
	})
	if err != nil {
		return fmt.Errorf("handle event: %w", err)
	}
	if response.Status == nil || !response.Status.Ok {
		return fmt.Errorf("the plugin must acknowledge a message on a declared subscription, got %s", describeStatus(response.Status))
	}
	return nil
}

// caseUnsubscribed：Start 之后向没人订阅的主题投递，以 KERNEL_NOTFOUND_001 + plugins.event.unsubscribed 失败，方式为空时按 competing；
// 未知的投递方式与没有信封的投递以 gRPC InvalidArgument 拒绝（invocation.md §4）。
func (r *runner) caseUnsubscribed(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if err := r.startPlugin(ctx, s); err != nil {
		return err
	}
	// 随机主题不会碰上插件的订阅
	topic := "conformance.unsubscribed-" + randomID()[:12]
	const messageType = "conformance.event"
	details := map[string]string{"id": r.manifest.ID, "topic": topic, "type": messageType, "mode": "competing"}
	for _, mode := range []string{"competing", ""} {
		what := fmt.Sprintf("a delivery to the unsubscribed topic %q with mode %q", topic, mode)
		response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandleEventResponse, error) {
			return s.client().HandleEvent(c, eventRequest(topic, messageType, mode))
		})
		if err != nil {
			return fmt.Errorf("%s must return a status, got %w", what, err)
		}
		if err := expectFailure(response.Status, codeUnsubscribed, keyUnsubscribed, details, what); err != nil {
			return err
		}
	}

	_, err = call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandleEventResponse, error) {
		return s.client().HandleEvent(c, eventRequest(topic, messageType, "bogus"))
	})
	if err := expectCode(err, codes.InvalidArgument, "a delivery with mode 'bogus'"); err != nil {
		return err
	}
	withoutEnvelope := eventRequest(topic, messageType, "competing")
	withoutEnvelope.Envelope = nil
	_, err = call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HandleEventResponse, error) {
		return s.client().HandleEvent(c, withoutEnvelope)
	})
	return expectCode(err, codes.InvalidArgument, "a delivery without an envelope")
}

// caseStream：插件在流式契约之前不实现流式调用（invocation.md §1）：第一帧之前结束或第一帧不是 start 时以 gRPC InvalidArgument
// 结束流，第一帧是 start 时以 gRPC Unimplemented 结束流，不论操作是否声明。
func (r *runner) caseStream(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if err := r.startPlugin(ctx, s); err != nil {
		return err
	}
	chunk := &pluginv1.StreamFrame{Frame: &pluginv1.StreamFrame_Chunk{Chunk: []byte("{}")}}
	if err := r.expectStream(ctx, s.client(), chunk, codes.InvalidArgument, "an InvokeStream whose first frame is a chunk"); err != nil {
		return err
	}
	if err := r.expectStream(ctx, s.client(), nil, codes.InvalidArgument, "an InvokeStream closed before its first frame"); err != nil {
		return err
	}
	operations := []string{missingOperation}
	for _, operation := range s.handshake.Operations {
		if !r.opts.Exclude[operation.Id] {
			operations = append(operations, operation.Id)
			break
		}
	}
	for _, operation := range operations {
		frame := &pluginv1.StreamFrame{Frame: &pluginv1.StreamFrame_Start{Start: r.invokeRequest(operation)}}
		if err := r.expectStream(ctx, s.client(), frame, codes.Unimplemented, fmt.Sprintf("an InvokeStream that starts operation %q", operation)); err != nil {
			return err
		}
	}
	return nil
}

// caseConfiguration：配置节不是 JSON 对象（数组、语法错误）时 Configure 以 PLUGINS_UNEXPECTED_001 + plugins.process.failed 失败，
// 之后 Health 报 Failed，Stop 应答 ok 且进程在宽限内以退出码 0 退出（lifecycle.md §7.2、§7.5）；每种写法各开一个进程。
func (r *runner) caseConfiguration(ctx context.Context) error {
	for _, configuration := range []string{"[]", `{"a":`} {
		if err := r.expectConfigurationRejected(ctx, configuration); err != nil {
			return err
		}
	}
	return nil
}

// expectConfigurationRejected 以一份不是对象的配置节启动一个进程，核对 Configure 的失败、Failed 状态与停止。
func (r *runner) expectConfigurationRejected(ctx context.Context, configuration string) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	what := fmt.Sprintf("Configure with the configuration %s", configuration)
	response, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.ConfigureResponse, error) {
		return s.client().Configure(c, r.configureRequest(configuration))
	})
	if err != nil {
		return fmt.Errorf("%s must return a status, got %w", what, err)
	}
	details := map[string]string{"id": r.manifest.ID, "phase": "configure", "reason": configurationReason}
	if err := expectFailure(response.Status, codeFailed, keyFailed, details, what); err != nil {
		return err
	}
	when := "after " + what
	if err := r.expectHealth(ctx, s.client(), when, false, pluginv1.LifecycleState_LIFECYCLE_STATE_FAILED); err != nil {
		return err
	}
	return r.stopAndExit(ctx, s, 5, when)
}

// caseStopEarly：握手之后直接 Stop（grace_seconds 为 0）应答 ok，进程在 5 秒内以退出码 0 退出（lifecycle.md §7.5）。
func (r *runner) caseStopEarly(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	return r.stopAndExit(ctx, s, 0, "right after the handshake")
}

// caseStdin：宿主关闭 stdin 后插件必须自行退出，不留孤儿进程。
func (r *runner) caseStdin(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if err := s.process.closeStdin(); err != nil {
		return fmt.Errorf("close stdin: %w", err)
	}
	if !s.process.waitExit(r.opts.Timeout) {
		return errors.New("the plugin must exit after its stdin is closed")
	}
	return nil
}

// startPlugin 执行 Configure 与 Start，两步都必须成功（lifecycle.md §2）。
func (r *runner) startPlugin(ctx context.Context, s *session) error {
	if err := r.configure(ctx, s.client()); err != nil {
		return err
	}
	return r.start(ctx, s.client())
}

// configureRequest 组装 Configure 请求：配置节、清单声明的权限与可写的工作目录。
func (r *runner) configureRequest(configuration string) *pluginv1.ConfigureRequest {
	return &pluginv1.ConfigureRequest{
		Configuration:      configuration,
		GrantedPermissions: r.manifest.Permissions,
		WorkDirectory:      filepath.Join(os.TempDir(), "xihan-conformance", r.manifest.ID),
	}
}

// configure 以 --config 的配置节执行 Configure，必须成功。
func (r *runner) configure(ctx context.Context, client pluginv1.PluginClient) error {
	request := r.configureRequest(r.opts.Configuration)
	if err := os.MkdirAll(request.WorkDirectory, 0o700); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	configured, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.ConfigureResponse, error) {
		return client.Configure(c, request)
	})
	if err != nil {
		return fmt.Errorf("configure: %w", err)
	}
	if configured.Status == nil || !configured.Status.Ok {
		return fmt.Errorf("Configure must succeed, got %s", describeStatus(configured.Status))
	}
	return nil
}

// start 执行 Start，必须成功。
func (r *runner) start(ctx context.Context, client pluginv1.PluginClient) error {
	started, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.StartResponse, error) {
		return client.Start(c, &pluginv1.StartRequest{})
	})
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if started.Status == nil || !started.Status.Ok {
		return fmt.Errorf("Start must succeed, got %s", describeStatus(started.Status))
	}
	return nil
}

// expectConfigureRefused 核对这次 Configure 以 FailedPrecondition 拒绝。
func (r *runner) expectConfigureRefused(ctx context.Context, client pluginv1.PluginClient, what string) error {
	_, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.ConfigureResponse, error) {
		return client.Configure(c, r.configureRequest(r.opts.Configuration))
	})
	return expectCode(err, codes.FailedPrecondition, what)
}

// expectStartRefused 核对这次 Start 以 FailedPrecondition 拒绝。
func (r *runner) expectStartRefused(ctx context.Context, client pluginv1.PluginClient, what string) error {
	_, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.StartResponse, error) {
		return client.Start(c, &pluginv1.StartRequest{})
	})
	return expectCode(err, codes.FailedPrecondition, what)
}

// expectHealth 求一次 Health，核对状态是 states 之一；noItems 为真时另核对没有健康项（Start 之前报本地状态，lifecycle.md §7.4）。
func (r *runner) expectHealth(ctx context.Context, client pluginv1.PluginClient, when string, noItems bool, states ...pluginv1.LifecycleState) error {
	health, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.HealthResponse, error) {
		return client.Health(c, &pluginv1.HealthRequest{})
	})
	if err != nil {
		return fmt.Errorf("Health %s: %w", when, err)
	}
	if !slices.Contains(states, health.State) {
		names := make([]string, 0, len(states))
		for _, state := range states {
			names = append(names, state.String())
		}
		return fmt.Errorf("Health %s reported %s, want %s", when, health.State, strings.Join(names, " or "))
	}
	if noItems && len(health.Items) > 0 {
		return fmt.Errorf("Health %s must report no health items, got %d", when, len(health.Items))
	}
	return nil
}

// stopAndExit 以 grace 秒的宽限调 Stop，核对应答 ok、进程在 5 秒内以退出码 0 退出（lifecycle.md §7.5）。
func (r *runner) stopAndExit(ctx context.Context, s *session, grace uint32, when string) error {
	stopped, err := call(ctx, r.opts.Timeout, func(c context.Context) (*pluginv1.StopResponse, error) {
		return s.client().Stop(c, &pluginv1.StopRequest{GraceSeconds: grace, Reason: "unload"})
	})
	if err != nil {
		return fmt.Errorf("Stop %s: %w", when, err)
	}
	if stopped.Status == nil || !stopped.Status.Ok {
		return fmt.Errorf("Stop %s must succeed, got %s", when, describeStatus(stopped.Status))
	}
	if !s.process.waitExit(5 * time.Second) {
		return fmt.Errorf("the plugin must exit within 5 seconds after Stop %s", when)
	}
	if code := s.process.exitCode(); code != 0 {
		return fmt.Errorf("the plugin must exit with code 0 after Stop %s, got %d", when, code)
	}
	return nil
}

// expectStream 打开一个 InvokeStream、发出 first（nil 表示不发帧就关闭发送方向），核对流以 want 结束。
// 发出第一帧后不关闭发送方向：插件读到第一帧就要作出判定，不等调用方结束。
func (r *runner) expectStream(ctx context.Context, client pluginv1.PluginClient, first *pluginv1.StreamFrame, want codes.Code, what string) error {
	streamCtx, cancel := r.deadline(ctx)
	defer cancel()
	stream, err := client.InvokeStream(streamCtx)
	if err != nil {
		return fmt.Errorf("%s: open the stream: %w", what, err)
	}
	if first == nil {
		if err := stream.CloseSend(); err != nil {
			return fmt.Errorf("%s: close the stream: %w", what, err)
		}
	} else if err := stream.Send(first); err != nil && !errors.Is(err, io.EOF) {
		// io.EOF 表示服务端已经结束了流，结果在 Recv 里
		return fmt.Errorf("%s: send the first frame: %w", what, err)
	}
	frame, err := stream.Recv()
	switch {
	case err == nil:
		return fmt.Errorf("%s must end with %s, got a frame %v", what, want, frame)
	case errors.Is(err, io.EOF):
		return fmt.Errorf("%s must end with %s, got OK", what, want)
	}
	return expectCode(err, want, what)
}

// invokeRequest 组装一次调用：载荷取 --sample 的样例（没有时为 {}），带 traceparent 与已登记的 Baggage。
func (r *runner) invokeRequest(operation string) *pluginv1.InvokeRequest {
	payload := "{}"
	if sample, ok := r.opts.Samples[operation]; ok {
		payload = sample
	}
	return &pluginv1.InvokeRequest{
		Operation: operation,
		Payload:   []byte(payload),
		Metadata:  &pluginv1.InvocationMetadata{TraceParent: traceParent, Baggage: map[string]string{"xihan.correlation": "conformance"}},
	}
}

// eventRequest 组装一次投递：信封带随机 ID 与空对象的数据，mode 是这次投递的方式。
func eventRequest(topic, messageType, mode string) *pluginv1.HandleEventRequest {
	return &pluginv1.HandleEventRequest{
		Envelope: &pluginv1.Envelope{
			Topic:           topic,
			Id:              randomID(),
			Source:          "urn:xihan:conformance",
			Type:            messageType,
			SpecVersion:     "1.0",
			DataContentType: "application/json",
			Data:            []byte("{}"),
		},
		Metadata: &pluginv1.InvocationMetadata{TraceParent: traceParent},
		Attempt:  1,
		Mode:     mode,
	}
}

// subscriptionType 是投递给订阅的消息类型：订阅没有限定类型时用一个任意的类型。
func subscriptionType(subscription *pluginv1.Subscription) string {
	if subscription.MessageType == "" {
		return "conformance.event"
	}
	return subscription.MessageType
}

// deliveryMode 把订阅声明的方式换成投递时的方式：宿主总是填写，声明为空即竞争消费。
func deliveryMode(declared string) string {
	if declared == "" {
		return "competing"
	}
	return declared
}

// call 以一步的截止时间执行一次一元调用。
func call[T any](ctx context.Context, timeout time.Duration, invoke func(context.Context) (T, error)) (T, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return invoke(callCtx)
}

// expectCode 核对一次调用以 gRPC 状态 want 结束；what 描述这次调用。
func expectCode(err error, want codes.Code, what string) error {
	if err == nil {
		return fmt.Errorf("%s must be refused with %s, got gRPC status OK", what, want)
	}
	if code := status.Code(err); code != want {
		return fmt.Errorf("%s must be refused with %s, got %s: %v", what, want, code, err)
	}
	return nil
}

// expectFailure 核对 Status 是带消息键的业务失败：码与键相等、text 为空（由宿主按目录翻译，invocation.md §3）、
// details 按 JSON 值等于 details。
func expectFailure(s *pluginv1.Status, code, key string, details map[string]string, what string) error {
	if s == nil || s.Ok || s.Error == nil {
		return fmt.Errorf("%s must fail with %s and message key %q, got %s", what, code, key, describeStatus(s))
	}
	failure := s.Error
	if failure.Code != code || failure.MessageKey != key {
		return fmt.Errorf("%s must fail with %s and message key %q, got %s and %q", what, code, key, failure.Code, failure.MessageKey)
	}
	if failure.Text != "" {
		return fmt.Errorf("%s: an error with a message key must leave text empty, got %q", what, failure.Text)
	}
	if !sameDetails(failure.Details, details) {
		expected, _ := json.Marshal(details)
		return fmt.Errorf("%s: details must equal %s as a JSON value, got %q", what, expected, failure.Details)
	}
	return nil
}

// sameDetails 按 JSON 值比较 details 文本与期望的对象：成员顺序与空白可以不同，重复成员与多余成员都不相等。
func sameDetails(text string, expected map[string]string) bool {
	if rules.CheckJSONText([]byte(text)) != nil {
		return false
	}
	var actual any
	if err := json.Unmarshal([]byte(text), &actual); err != nil {
		return false
	}
	want := make(map[string]any, len(expected))
	for name, value := range expected {
		want[name] = value
	}
	return reflect.DeepEqual(actual, want)
}

// deadline 给一次调用加截止时间。
func (r *runner) deadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.opts.Timeout)
}

// describeStatus 把一个 Status 描述成可读文本：错误码、消息键、文案与细节。
func describeStatus(s *pluginv1.Status) string {
	switch {
	case s == nil:
		return "no status"
	case s.Ok:
		return "ok"
	case s.Error == nil:
		return "failure without error"
	default:
		return fmt.Sprintf("%s (key %q, text %q, details %q)", s.Error.Code, s.Error.MessageKey, s.Error.Text, s.Error.Details)
	}
}

// randomID 生成 16 字节的随机十六进制 ID。
func randomID() string {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		panic(err)
	}
	return hex.EncodeToString(id)
}
