package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/xihanfun/framework-spec/conformance/rules"
)

// testCase 是用例表的一项：用例名与它的实现。
type testCase struct {
	name string
	run  func(*runner, context.Context) error
}

// caseTable 是用例表，按执行顺序：AllCases 与 Run 都从这张表取。新增用例在 options.go 加用例名、在 cases.go 实现、在这里登记一行。
var caseTable = []testCase{
	{CaseHandshake, (*runner).caseHandshake},
	{CaseRefusal, (*runner).caseRefusal},
	{CaseSession, (*runner).caseSession},
	{CaseLifecycle, (*runner).caseLifecycle},
	{CaseOrder, (*runner).caseOrder},
	{CaseUnknownOperation, (*runner).caseUnknownOperation},
	{CaseDeclaredOperations, (*runner).caseDeclaredOperations},
	{CaseUnavailable, (*runner).caseUnavailable},
	{CaseEvent, (*runner).caseEvent},
	{CaseEventEnvelope, (*runner).caseEventEnvelope},
	{CaseUnsubscribed, (*runner).caseUnsubscribed},
	{CaseStream, (*runner).caseStream},
	{CaseConfiguration, (*runner).caseConfiguration},
	{CaseStopEarly, (*runner).caseStopEarly},
	{CaseStdin, (*runner).caseStdin},
}

// caseNames 按执行顺序列出用例表里的用例名。
func caseNames() []string {
	names := make([]string, 0, len(caseTable))
	for _, c := range caseTable {
		names = append(names, c.name)
	}
	return names
}

// runner 持有一次运行的状态。
type runner struct {
	opts     Options
	manifest Manifest
	raw      map[string]any
	// hostAddresses 是交给插件的回调端点列表（handshake.md §4），一次运行共用。
	hostAddresses []rules.Endpoint
	token         string
}

// Run 对一个插件目录运行选定的用例，每个用例都单独启动一次插件进程。
// 返回的错误只表示运行器本身无法开始（目录或清单不可用、用例名未知）；用例失败在 Result 里。
func Run(ctx context.Context, opts Options) ([]Result, error) {
	if opts.Directory == "" {
		return nil, errors.New("plugin: the plugin directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Configuration == "" {
		opts.Configuration = "{}"
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	manifest, raw, err := readManifest(opts.Directory)
	if err != nil {
		return nil, fmt.Errorf("plugin: %w", err)
	}
	if manifest.Kind != "plugin" || manifest.Isolation != "out-of-process" {
		return nil, fmt.Errorf("plugin: the manifest %q does not describe an out-of-process plugin", manifest.ID)
	}

	names := AllCases
	if len(opts.Cases) > 0 {
		names = opts.Cases
	}
	selected := make([]testCase, 0, len(names))
	for _, name := range names {
		index := slices.IndexFunc(caseTable, func(c testCase) bool { return c.name == name })
		if index < 0 {
			return nil, fmt.Errorf("plugin: unknown case %q", name)
		}
		selected = append(selected, caseTable[index])
	}

	hostAddresses, err := newHostAddresses()
	if err != nil {
		return nil, fmt.Errorf("plugin: %w", err)
	}
	r := &runner{opts: opts, manifest: manifest, raw: raw, hostAddresses: hostAddresses, token: newSessionToken()}

	results := make([]Result, 0, len(selected))
	for _, c := range selected {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		started := time.Now()
		err := c.run(r, ctx)
		result := Result{Case: c.name, DurationMillis: time.Since(started).Milliseconds()}
		var skip errSkip
		switch {
		case err == nil:
			result.Passed = true
		case errors.As(err, &skip):
			result.Passed = true
			result.Skipped = true
			result.Message = skip.reason
		default:
			result.Message = err.Error()
		}
		results = append(results, result)
	}
	return results, nil
}
