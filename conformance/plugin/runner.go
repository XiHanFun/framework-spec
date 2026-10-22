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

// Version 是运行器的版本；握手时作为宿主 SDK 版本上报。
const Version = "0.1.0"

// runner 持有一次运行的状态。
type runner struct {
	opts         Options
	manifest     Manifest
	raw          map[string]any
	hostEndpoint rules.Endpoint
	token        string
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

	selected := AllCases
	if len(opts.Cases) > 0 {
		for _, name := range opts.Cases {
			if !slices.Contains(AllCases, name) {
				return nil, fmt.Errorf("plugin: unknown case %q", name)
			}
		}
		selected = opts.Cases
	}

	r := &runner{opts: opts, manifest: manifest, raw: raw, hostEndpoint: placeholderHostEndpoint(), token: newSessionToken()}
	cases := map[string]func(context.Context) error{
		CaseHandshake:          r.caseHandshake,
		CaseRefusal:            r.caseRefusal,
		CaseLifecycle:          r.caseLifecycle,
		CaseUnknownOperation:   r.caseUnknownOperation,
		CaseDeclaredOperations: r.caseDeclaredOperations,
		CaseEvent:              r.caseEvent,
		CaseStdin:              r.caseStdin,
	}

	results := make([]Result, 0, len(selected))
	for _, name := range selected {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		started := time.Now()
		err := cases[name](ctx)
		result := Result{Case: name, DurationMillis: time.Since(started).Milliseconds()}
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
