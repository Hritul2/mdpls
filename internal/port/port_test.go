package port

import (
	"net"
	"strconv"
	"testing"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

func TestListenUsesStartPortWhenFree(t *testing.T) {
	p := freePort(t)
	l, err := Listen(p, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if got := l.Addr().(*net.TCPAddr).Port; got != p {
		t.Fatalf("got port %d, want %d", got, p)
	}
}

func TestListenSkipsTakenPort(t *testing.T) {
	// Something else holds the port on the wildcard address: we must not
	// bind next to it (that would silently shadow it), we must move on.
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	p := busy.Addr().(*net.TCPAddr).Port

	l, err := Listen(p, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	got := l.Addr().(*net.TCPAddr).Port
	if got == p {
		t.Fatalf("bound the taken port %d", p)
	}
	if host, _, _ := net.SplitHostPort(l.Addr().String()); host != "127.0.0.1" {
		t.Fatalf("must bind loopback only, got %s", host)
	}
}

func TestTaken(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	if !Taken(p) {
		t.Fatalf("port %s should be taken", strconv.Itoa(p))
	}
	l.Close()
	if Taken(p) {
		t.Fatalf("port %d should be free after close", p)
	}
}
