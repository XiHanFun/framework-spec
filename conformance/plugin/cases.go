package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
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

// caseHandshake：进程写出合法握手行、协商出宿主支持的版本、原语层版本相同、清单与文件一致、操作与订阅都在声明范围内。
func (r *runner) caseHandshake(ctx context.Context) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	if s.handshake.ProtocolVersion != supportedVersions[len(supportedVersions)-1] {
		return fmt.Errorf("negotiated version %d, want %d", s.handshake.ProtocolVersion, supportedVersions[len(supportedVersions)-1])
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
	callCtx, cancel := r.deadline(ctx)
	defer cancel()
	response, err := p.client.Handshake(callCtx, r.handshakeRequest([]uint32{math.MaxUint32}))
	if err == nil {
		return fmt.Errorf("the handshake must be refused, got protocol version %d", response.ProtocolVersion)
	}
	if code := status.Code(err); code != codes.FailedPrecondition {
		return fmt.Errorf("the handshake must be refused with FailedPrecondition, got %s: %v", code, err)
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
	callCtx, cancel := r.deadline(ctx)
	defer cancel()
	response, err := s.client().Invoke(callCtx, &pluginv1.InvokeRequest{Operation: "conformance.missing", Payload: []byte("{}")})
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
		payload := "{}"
		if sample, ok := r.opts.Samples[operation.Id]; ok {
			payload = sample
		}
		callCtx, cancel := r.deadline(ctx)
		response, err := s.client().Invoke(callCtx, &pluginv1.InvokeRequest{
			Operation: operation.Id,
			Payload:   []byte(payload),
			Metadata:  &pluginv1.InvocationMetadata{TraceParent: traceParent, Baggage: map[string]string{"xihan.correlation": "conformance"}},
		})
		cancel()
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
	messageType := subscription.MessageType
	if messageType == "" {
		messageType = "conformance.event"
	}
	callCtx, cancel := r.deadline(ctx)
	defer cancel()
	response, err := s.client().HandleEvent(callCtx, &pluginv1.HandleEventRequest{
		Envelope: &pluginv1.Envelope{
			Topic:           subscription.Topic,
			Id:              randomID(),
			Source:          "urn:xihan:conformance",
			Type:            messageType,
			SpecVersion:     "1.0",
			DataContentType: "application/json",
			Data:            []byte("{}"),
		},
		Metadata: &pluginv1.InvocationMetadata{TraceParent: traceParent},
		Attempt:  1,
	})
	if err != nil {
		return fmt.Errorf("handle event: %w", err)
	}
	if response.Status == nil || !response.Status.Ok {
		return fmt.Errorf("the plugin must acknowledge a message on a declared subscription, got %s", describeStatus(response.Status))
	}
	return nil
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
	workDirectory := filepath.Join(os.TempDir(), "xihan-conformance", r.manifest.ID)
	if err := os.MkdirAll(workDirectory, 0o700); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	configureCtx, cancelConfigure := r.deadline(ctx)
	defer cancelConfigure()
	configured, err := s.client().Configure(configureCtx, &pluginv1.ConfigureRequest{
		Configuration:      r.opts.Configuration,
		GrantedPermissions: r.manifest.Permissions,
		WorkDirectory:      workDirectory,
	})
	if err != nil {
		return fmt.Errorf("configure: %w", err)
	}
	if configured.Status == nil || !configured.Status.Ok {
		return fmt.Errorf("Configure must succeed, got %s", describeStatus(configured.Status))
	}

	startCtx, cancelStart := r.deadline(ctx)
	defer cancelStart()
	started, err := s.client().Start(startCtx, &pluginv1.StartRequest{})
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if started.Status == nil || !started.Status.Ok {
		return fmt.Errorf("Start must succeed, got %s", describeStatus(started.Status))
	}
	return nil
}

// deadline 给一次调用加截止时间。
func (r *runner) deadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.opts.Timeout)
}

// describeStatus 把一个 Status 描述成可读文本。
func describeStatus(s *pluginv1.Status) string {
	switch {
	case s == nil:
		return "no status"
	case s.Ok:
		return "ok"
	case s.Error == nil:
		return "failure without error"
	default:
		return fmt.Sprintf("%s (%s)", s.Error.Code, s.Error.Text)
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
