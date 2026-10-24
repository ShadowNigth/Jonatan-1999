package swayipc

import (
	"net"
	"sync"
	"testing"
	"time"
)

type readSignalConn struct {
	net.Conn
	started chan struct{}
	once    sync.Once
}

func (c *readSignalConn) Read(p []byte) (int, error) {
	c.once.Do(func() { close(c.started) })
	return c.Conn.Read(p)
}

func TestCloseUnblocksNext(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	conn := newConn(&readSignalConn{Conn: client, started: make(chan struct{})})
	conn.SetTimeout(0)

	readDone := make(chan error, 1)
	go func() {
		_, err := conn.Next()
		readDone <- err
	}()

	select {
	case <-conn.nc.(*readSignalConn).started:
	case <-time.After(time.Second):
		t.Fatal("Next no comenzó a leer")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-readDone:
		if err == nil {
			t.Fatal("Next terminó sin error después de cerrar el socket")
		}
	case <-time.After(time.Second):
		t.Fatal("Close no desbloqueó Next")
	}
}
