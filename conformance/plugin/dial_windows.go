//go:build windows

package plugin

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// namedPipeDialer 连接 Windows 命名管道。
func namedPipeDialer(address string) (func(context.Context) (net.Conn, error), error) {
	return func(ctx context.Context) (net.Conn, error) {
		return winio.DialPipeContext(ctx, address)
	}, nil
}
