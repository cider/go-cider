package echo

import (
	// Stdlib
	"runtime"
	"testing"
	"time"

	// Cider broker
	"github.com/cider/cider/broker"
	"github.com/cider/cider/broker/exchanges/rpc/roundrobin"
	//"github.com/cider/cider/broker/log"
	wsb "github.com/cider/cider/broker/transports/websocket/rpc"

	// Cider client
	"github.com/cider/go-cider/cider/services/rpc"
	wsc "github.com/cider/go-cider/cider/transports/websocket/rpc"

	// Others
	"github.com/cihub/seelog"
)

func BenchmarkTransport_______WebSocket(b *testing.B) {
	// Enable logging.
	// log.UseLogger(seelog.Default)

	// Disable logging.
	seelog.ReplaceLogger(seelog.Disabled)

	// Run in parallel.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Instantiate a new broker and RPC exchange.
	var (
		bro      = broker.New()
		balancer = roundrobin.NewBalancer()
	)

	// Set up WebSocket endpoint using a random local port.
	var endpoint *wsb.Endpoint
	bro.RegisterEndpointFactory("websocket_rpc", func() (broker.Endpoint, error) {
		factory := wsb.NewEndpointFactory()
		factory.Addr = "127.0.0.1:0"
		endpoint = factory.NewEndpoint(balancer)
		return endpoint, nil
	})
	bro.ListenAndServe()
	defer bro.Terminate()

	// Give broker some time to spawn the WebSocket endpoint. It must be running
	// before the client attempts to connect, otherwise it's connection refused.
	time.Sleep(time.Second)

	// Set up an RPC service instance using the WebSocket transport.
	srv, err := rpc.NewService(func() (rpc.Transport, error) {
		factory := wsc.NewTransportFactory()
		factory.Server = "ws://" + endpoint.Addr().String()
		factory.Origin = "http://localhost"
		return factory.NewTransport("Echo#" + mustRandomString())
	})
	if err != nil {
		b.Fatal(err)
	}
	defer srv.Close()

	// Export Echo method.
	if err := srv.RegisterMethod("Echo", func(req rpc.RemoteRequest) {
		var payload []byte
		if err := req.UnmarshalArgs(&payload); err != nil {
			req.Resolve(2, err.Error())
			return
		}
		req.Resolve(0, payload)
	}); err != nil {
		b.Fatal(err)
	}

	payload := []byte("payload")
	calls := make([]*rpc.RemoteCall, b.N)

	// Start shooting requests.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calls[i] = srv.NewRemoteCall("Echo", payload).GoExecute()
	}

	// Wait for all the responses to come back.
	for i := 0; i < b.N; i++ {
		if err := calls[i].Wait(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	// Set GOMAXPROCS to the original value.
	runtime.GOMAXPROCS(1)
}
