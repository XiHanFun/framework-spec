//go:build !windows

package plugin

import (
	"context"
	"errors"
	"net"
)

// namedPipeDialer 在非 Windows 平台不可用：npipe 端点只存在于 Windows。
func namedPipeDialer(string) (func(context.Context) (net.Conn, error), error) {
	return nil, errors.New("npipe endpoints are only supported on Windows")
}
