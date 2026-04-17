package realtime

import (
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

var testNATSConn *nats.Conn

func TestMain(m *testing.M) {
	opts := &server.Options{Port: -1}
	srv, err := server.NewServer(opts)
	if err != nil {
		panic("nats server: " + err.Error())
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		panic("nats server not ready")
	}

	testNATSConn, err = nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		panic("nats connect: " + err.Error())
	}

	code := m.Run()

	testNATSConn.Close()
	srv.Shutdown()
	os.Exit(code)
}
