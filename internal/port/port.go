// Package port picks a loopback port: a fixed one that only moves when taken.
package port

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Taken reports whether something already accepts connections on
// 127.0.0.1:port. A real connection is used because binding alone can
// succeed next to a service listening on the wildcard address, silently
// shadowing it.
func Taken(port int) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// Listen binds 127.0.0.1 on start, or the next free port, trying up to tries
// ports. If all are busy it lets the OS choose.
func Listen(start, tries int) (net.Listener, error) {
	for p := start; p < start+tries && p <= 65535; p++ {
		if Taken(p) {
			continue
		}
		l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
		if err == nil {
			return l, nil
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("no free port from %d: %w", start, err)
	}
	return l, nil
}
