package plugin

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/xihanfun/framework-spec/conformance/rules"
)

// dial 为握手行里的端点创建 gRPC 连接：unix 与 tcp 直接拨号，npipe 只在 Windows 上可用。
func dial(endpoint rules.Endpoint) (*grpc.ClientConn, error) {
	var dialer func(context.Context) (net.Conn, error)
	switch endpoint.Network {
	case "unix", "tcp":
		network := endpoint.Network
		dialer = func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, endpoint.Address)
		}
	case "npipe":
		pipe, err := namedPipeDialer(endpoint.Address)
		if err != nil {
			return nil, err
		}
		dialer = pipe
	default:
		return nil, fmt.Errorf("unsupported endpoint network %q", endpoint.Network)
	}

	// passthrough 解析器把目标原样交给拨号器；真正的地址来自端点
	return grpc.NewClient(
		"passthrough:///plugin",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return dialer(ctx) }),
	)
}
