package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/gorilla/websocket"
)

func newTestHub() *WebSocketHub {
	return NewWebSocketHub(
		hue.NewClient("", ""),
		loxone.NewMappingManager(),
		loxone.NewUDPSender(),
		loxone.NewHTTPSender(),
	)
}

// TestHubDropsSlowClientWhileItWrites covers the teardown race in the hub:
// a client that never reads fills its send buffer and gets dropped by the
// broadcast loop, while its own readPump is still producing error replies for
// the same channel. Closing send from the hub made that a "send on closed
// channel" panic, and deleting from h.clients under RLock raced with
// register/unregister.
//
// Run with -race.
func TestHubDropsSlowClientWhileItWrites(t *testing.T) {
	hub := newTestHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	const clients = 8
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < clients; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()

		// Deliberately never read from the connection, so the send buffer fills
		// and the hub drops this client mid-flight.
		wg.Add(1)
		go func(c *websocket.Conn) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				// Unparseable, so the server answers via sendError -> c.send
				if err := c.WriteMessage(websocket.TextMessage, []byte("NOT A COMMAND")); err != nil {
					return
				}
			}
		}(conn)
	}

	// Flood broadcasts so the 256-slot buffers overflow and clients get dropped.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50000; i++ {
			select {
			case <-stop:
				return
			default:
			}
			hub.BroadcastStatus("light-1", map[string]interface{}{"on": true})
		}
	}()

	time.Sleep(time.Second)
	close(stop)
	wg.Wait()
}

// TestHubRegisterUnregisterChurn hammers the register/unregister paths against a
// running broadcast loop to shake out map races.
func TestHubRegisterUnregisterChurn(t *testing.T) {
	hub := newTestHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			hub.BroadcastStatus("light-1", map[string]interface{}{"on": true})
		}
	}()

	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				conn, _, err := websocket.DefaultDialer.Dial(url, nil)
				if err != nil {
					return
				}
				// Bounded, so a client that connects after the broadcaster has
				// stopped does not park here forever.
				conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				conn.ReadMessage()
				conn.Close()
			}
		}()
	}

	time.Sleep(time.Second)
	close(stop)
	wg.Wait()
}
