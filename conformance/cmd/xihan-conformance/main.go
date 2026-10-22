// xihan-conformance 是 XiHan Framework Spec 的一致性运行器命令行。
//
//	xihan-conformance plugin --dir <插件目录> [--timeout 15s] [--config '{}'] [--sample op=json]... [--exclude op]... [--case name]... [--json]
//	xihan-conformance vectors [--json]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/plugin"
)

// multiFlag 收集可重复的命令行选项。
type multiFlag []string

// String 实现 flag.Value。
func (m *multiFlag) String() string { return strings.Join(*m, ",") }

// Set 实现 flag.Value。
func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "plugin":
		os.Exit(runPlugin(os.Args[2:]))
	case "vectors":
		os.Exit(runVectors(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println("xihan-conformance " + plugin.Version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

// usage 打印用法。
func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  xihan-conformance plugin --dir <directory> [--timeout 15s] [--config '{}'] [--sample op=json]... [--exclude op]... [--case name]... [--json] [--quiet]")
	fmt.Fprintln(os.Stderr, "  xihan-conformance vectors [--json]")
	fmt.Fprintln(os.Stderr, "cases: "+strings.Join(plugin.AllCases, ", "))
}

// runPlugin 执行 plugin 子命令，返回进程退出码：0 全部通过，1 有失败，2 参数错误。
func runPlugin(args []string) int {
	flags := flag.NewFlagSet("plugin", flag.ContinueOnError)
	directory := flags.String("dir", "", "plugin directory containing xihan.manifest.json")
	timeout := flags.Duration("timeout", plugin.DefaultTimeout, "deadline for each step")
	configuration := flags.String("config", "{}", "JSON object passed to Configure")
	asJSON := flags.Bool("json", false, "print results as JSON")
	quiet := flags.Bool("quiet", false, "do not forward the plugin's stderr")
	var samples, excludes, cases multiFlag
	flags.Var(&samples, "sample", "request JSON for an operation, as op=json (repeatable)")
	flags.Var(&excludes, "exclude", "operation not to invoke (repeatable)")
	flags.Var(&cases, "case", "case to run (repeatable); default all")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *directory == "" {
		fmt.Fprintln(os.Stderr, "--dir is required")
		return 2
	}

	opts := plugin.Options{
		Directory:     *directory,
		Timeout:       *timeout,
		Configuration: *configuration,
		Samples:       map[string]string{},
		Exclude:       map[string]bool{},
		Cases:         cases,
		Stderr:        os.Stderr,
	}
	if *quiet {
		opts.Stderr = nil
	}
	for _, sample := range samples {
		operation, payload, ok := strings.Cut(sample, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "--sample must be op=json, got %q\n", sample)
			return 2
		}
		opts.Samples[operation] = payload
	}
	for _, operation := range excludes {
		opts.Exclude[operation] = true
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	started := time.Now()
	results, err := plugin.Run(ctx, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(results)
	} else {
		passed, failed, skipped := 0, 0, 0
		for _, result := range results {
			switch {
			case result.Skipped:
				skipped++
				fmt.Printf("SKIP  %-20s %s\n", result.Case, result.Message)
			case result.Passed:
				passed++
				fmt.Printf("PASS  %-20s %dms\n", result.Case, result.DurationMillis)
			default:
				failed++
				fmt.Printf("FAIL  %-20s %s\n", result.Case, result.Message)
			}
		}
		fmt.Printf("\n%d passed, %d failed, %d skipped in %s\n", passed, failed, skipped, time.Since(started).Round(time.Millisecond))
	}
	if plugin.Failed(results) {
		return 1
	}
	return 0
}

// runVectors 执行 vectors 子命令：每个向量文件都必须能按其类型解析。
func runVectors(args []string) int {
	flags := flag.NewFlagSet("vectors", flag.ContinueOnError)
	asJSON := flags.Bool("json", false, "print the vector names as JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	names, err := conformance.Names()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	checks := []struct {
		name string
		load func() error
	}{
		{"plugin/handshake-lines.json", func() error { _, err := conformance.HandshakeLines(); return err }},
		{"plugin/protocol-versions.json", func() error { _, err := conformance.ProtocolVersions(); return err }},
		{"plugin/wasm-exports.json", func() error { _, err := conformance.WasmExports(); return err }},
		{"primitives/error-codes.json", func() error { _, err := conformance.ErrorCodes(); return err }},
		{"primitives/baggage-keys.json", func() error { _, err := conformance.BaggageKeys(); return err }},
		{"primitives/capability-requirements.json", func() error { _, err := conformance.CapabilityRequirements(); return err }},
		{"primitives/lifecycle-transitions.json", func() error { _, err := conformance.LifecycleTransitions(); return err }},
	}
	code := 0
	for _, check := range checks {
		if err := check.load(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(names)
	} else {
		for _, name := range names {
			fmt.Println(name)
		}
		fmt.Printf("\n%d vector files\n", len(names))
	}
	return code
}
