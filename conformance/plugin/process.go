package plugin

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/xihanfun/framework-spec/conformance/internal/pluginv1"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

// inheritedNames 是原样传给插件的环境变量：PATH、用户目录、临时目录、区域与时区，以及操作系统运行所需的系统变量（handshake.md §1）。
var inheritedNames = []string{
	"PATH", "HOME", "USERPROFILE", "TMP", "TEMP", "TMPDIR", "LANG", "LC_ALL", "TZ",
	"SystemRoot", "windir", "ComSpec", "PATHEXT", "SystemDrive", "APPDATA", "LOCALAPPDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)", "ProgramW6432",
}

// inheritedPrefixes 是按前缀原样传给插件的环境变量。
var inheritedPrefixes = []string{"DOTNET_", "GO"}

// launch 是一次启动的参数（handshake.md §1 的环境变量）。
type launch struct {
	versions     []uint32
	primitives   uint32
	sessionToken string
	hostEndpoint rules.Endpoint
	instanceID   string
	locale       string
}

// process 是一个已启动并连上的插件进程。
type process struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	endpoint rules.Endpoint
	conn     *grpc.ClientConn
	client   pluginv1.PluginClient
	exited   chan struct{}
	exitErr  error
	closeOne sync.Once
}

// environment 组装插件进程的环境：白名单继承 + 协议变量。
func environment(l launch) []string {
	var env []string
	for _, pair := range os.Environ() {
		name, _, ok := strings.Cut(pair, "=")
		if ok && inherited(name) {
			env = append(env, pair)
		}
	}
	versions := make([]string, 0, len(l.versions))
	for _, version := range l.versions {
		versions = append(versions, strconv.FormatUint(uint64(version), 10))
	}
	return append(env,
		"XIHAN_PLUGIN_PROTOCOL_VERSIONS="+strings.Join(versions, ","),
		"XIHAN_PLUGIN_PRIMITIVES="+strconv.FormatUint(uint64(l.primitives), 10),
		"XIHAN_PLUGIN_SESSION="+l.sessionToken,
		"XIHAN_PLUGIN_HOST_ADDRESS="+l.hostEndpoint.Network+" "+l.hostEndpoint.Address,
		"XIHAN_PLUGIN_INSTANCE="+l.instanceID,
		"XIHAN_PLUGIN_LOCALE="+l.locale,
	)
}

// inherited 判断一个环境变量是否在白名单里；变量名不区分大小写。
func inherited(name string) bool {
	for _, allowed := range inheritedNames {
		if strings.EqualFold(allowed, name) {
			return true
		}
	}
	upper := strings.ToUpper(name)
	for _, prefix := range inheritedPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

// resolveExecutable 定位清单 entry 指向的可执行文件；Windows 上没有扩展名时补 .exe。
func resolveExecutable(directory, entry string) (string, error) {
	if entry == "" {
		return "", errors.New("the manifest has no entry")
	}
	path := filepath.Join(directory, filepath.FromSlash(entry))
	if runtime.GOOS == "windows" && filepath.Ext(path) == "" {
		if _, err := os.Stat(path + ".exe"); err == nil {
			return path + ".exe", nil
		}
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("the plugin executable %q was not found: %w", entry, err)
	}
	return path, nil
}

// start 启动插件进程、读握手行并连上端点；stdin 保持打开直到 close。
func start(ctx context.Context, directory string, manifest Manifest, l launch, timeout time.Duration, log io.Writer) (*process, error) {
	// 先把目录转成绝对路径：可执行文件路径相对调用方的工作目录解析，不能依赖 cmd.Dir
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	executable, err := resolveExecutable(absolute, manifest.Entry)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(executable)
	cmd.Dir = absolute
	cmd.Env = environment(l)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutReader, stdoutWriter := io.Pipe()
	cmd.Stdout = stdoutWriter
	cmd.Stderr = newLineWriter(log, "[plugin stderr] ")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %q: %w", executable, err)
	}

	p := &process{cmd: cmd, stdin: stdin, exited: make(chan struct{})}
	go func() {
		// Wait 结束后关掉管道写端，让读 stdout 的协程收到 EOF
		p.exitErr = cmd.Wait()
		_ = stdoutWriter.Close()
		close(p.exited)
	}()

	lines := make(chan string, 1)
	readErrors := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stdoutReader)
		line, err := reader.ReadString('\n')
		if err != nil {
			readErrors <- err
			_, _ = io.Copy(io.Discard, stdoutReader)
			return
		}
		lines <- strings.TrimRight(line, "\r\n")
		// 握手完成后的 stdout 按信息记录（handshake.md §1）
		_, _ = io.Copy(newLineWriter(log, "[plugin stdout] "), reader)
	}()

	var line string
	select {
	case line = <-lines:
	case err := <-readErrors:
		p.close()
		return nil, fmt.Errorf("the plugin exited before writing a handshake line: %v (exit: %v)", err, p.exitErr)
	case <-time.After(timeout):
		p.close()
		return nil, fmt.Errorf("no handshake line within %s", timeout)
	case <-ctx.Done():
		p.close()
		return nil, ctx.Err()
	}

	endpoint, ok := rules.ParseHandshakeLine(line)
	if !ok {
		p.close()
		return nil, fmt.Errorf("invalid handshake line %q", line)
	}
	p.endpoint = endpoint

	conn, err := dial(endpoint)
	if err != nil {
		p.close()
		return nil, err
	}
	p.conn = conn
	p.client = pluginv1.NewPluginClient(conn)
	return p, nil
}

// waitExit 等待进程退出；超时返回假。
func (p *process) waitExit(timeout time.Duration) bool {
	select {
	case <-p.exited:
		return true
	case <-time.After(timeout):
		return false
	}
}

// closeStdin 关闭插件的 stdin（协议要求插件随后自行退出）。
func (p *process) closeStdin() error {
	return p.stdin.Close()
}

// close 断开连接、关闭 stdin，必要时强制终止进程。
func (p *process) close() {
	p.closeOne.Do(func() {
		if p.conn != nil {
			_ = p.conn.Close()
		}
		_ = p.stdin.Close()
		if !p.waitExit(2 * time.Second) {
			_ = p.cmd.Process.Kill()
			p.waitExit(5 * time.Second)
		}
	})
}

// lineWriter 把写入的字节按行转发到目标，每行加前缀；进程退出时残留的半行也写出。
type lineWriter struct {
	target io.Writer
	prefix string
	mu     sync.Mutex
	buffer []byte
}

// newLineWriter 创建行转发器；target 为 nil 时丢弃。
func newLineWriter(target io.Writer, prefix string) *lineWriter {
	if target == nil {
		target = io.Discard
	}
	return &lineWriter{target: target, prefix: prefix}
}

// Write 实现 io.Writer。
func (w *lineWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer = append(w.buffer, data...)
	for {
		index := strings.IndexByte(string(w.buffer), '\n')
		if index < 0 {
			break
		}
		line := strings.TrimRight(string(w.buffer[:index]), "\r")
		w.buffer = w.buffer[index+1:]
		_, _ = fmt.Fprintf(w.target, "%s%s\n", w.prefix, line)
	}
	return len(data), nil
}
