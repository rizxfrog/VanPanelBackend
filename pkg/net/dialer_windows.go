//go:build windows
// +build windows

package net

import (
	"net"
	"syscall"
)

func CheckDialer() *net.Dialer {
	return &net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				linger := &syscall.Linger{
					Onoff:  1,
					Linger: 1,
				}
				_ = syscall.SetsockoptLinger(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, linger)
			})
		},
	}
}
