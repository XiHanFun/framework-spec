package plugin

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/xihanfun/framework-spec/conformance/rules"
)

// sessionMetadataKey 是会话令牌的元数据键（handshake.md §4）。
const sessionMetadataKey = "xihan-plugin-session"

// dial 为握手行里的端点创建 gRPC 连接：unix 与 tcp 直接拨号，npipe 只在 Windows 上可用。
// token 非空时每次调用都在元数据里带它（handshake.md §4）；为空时不带，用来核对插件拒绝不带令牌的调用。
func dial(endpoint rules.Endpoint, token string) (*grpc.ClientConn, error) {
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
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return dialer(ctx) }),
	}
	if token != "" {
		options = append(options,
			grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				return invoker(metadata.AppendToOutgoingContext(ctx, sessionMetadataKey, token), method, req, reply, cc, opts...)
			}),
			grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				return streamer(metadata.AppendToOutgoingContext(ctx, sessionMetadataKey, token), desc, cc, method, opts...)
			}),
		)
	}
	return grpc.NewClient("passthrough:///plugin", options...)
}
