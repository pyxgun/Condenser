package websocket

import (
	"condenser/internal/env"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func NewRequestHandler() *Handler {
	return &Handler{
		Resolver: StaticResolver{
			ContainerRoot: env.ContainerRootDir,
			SockName:      "tty.sock",
		},
		Upgrader: websocket.Upgrader{},
	}
}

type SockResolver interface {
	// containerId -> path to droplet shim's unix socket (tty.sock)
	ConsoleSockPath(containerId string) (string, error)
}

type StaticResolver struct {
	ContainerRoot string // e.g. "/etc/raind/container"
	SockName      string // e.g. "tty.sock"
}

func (r StaticResolver) ConsoleSockPath(containerId string) (string, error) {
	name := r.SockName
	if name == "" {
		name = "tty.sock"
	}
	return filepath.Join(r.ContainerRoot, containerId, name), nil
}

type Handler struct {
	Resolver SockResolver
	Upgrader websocket.Upgrader
}

// ServeHTTP handles GET /containers/{id}/attach (WebSocket)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		http.Error(w, "missing container id", http.StatusBadRequest)
		return
	}

	up := h.Upgrader
	if up.CheckOrigin == nil {
		up.CheckOrigin = func(r *http.Request) bool { return true }
	}

	ws, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	sockPath, err := h.Resolver.ConsoleSockPath(containerId)
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("resolve sock path failed: %v", err)))
		return
	}

	unixConn, err := net.Dial("unix", sockPath)
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("dial unix sock failed: %v", err)))
		return
	}
	defer unixConn.Close()

	wsr := newWSBinaryStreamReader(ws) // WS(binary messages) -> stream reader
	wsw := newWSBinaryStreamWriter(ws) // stream writer -> WS(binary messages)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// WS -> unix (client->ptmx framed bytes)
	go func() {
		defer wg.Done()
		_, e := io.Copy(unixConn, wsr)
		errCh <- e
	}()

	// unix -> WS (ptmx raw bytes)
	go func() {
		defer wg.Done()
		_, e := io.Copy(wsw, unixConn)
		errCh <- e
	}()

	// close both session
	// close both session
	e := <-errCh
	_ = unixConn.Close()

	// send CloseMessage
	_ = ws.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "stream closed"),
		time.Now().Add(1*time.Second),
	)

	_ = ws.Close()
	wg.Wait()

	_ = e
}

// --- WebSocket stream adapters ---
type wsBinaryStreamReader struct {
	ws  *websocket.Conn
	cur io.Reader
}

func newWSBinaryStreamReader(ws *websocket.Conn) *wsBinaryStreamReader {
	return &wsBinaryStreamReader{ws: ws}
}

func (r *wsBinaryStreamReader) Read(p []byte) (int, error) {
	for {
		if r.cur != nil {
			n, err := r.cur.Read(p)
			if err == io.EOF {
				r.cur = nil
				continue
			}
			return n, err
		}

		mt, rd, err := r.ws.NextReader()
		if err != nil {
			return 0, err
		}
		if mt != websocket.BinaryMessage {
			continue
		}
		r.cur = rd
	}
}

// wsBinaryStreamWriter writes stream bytes as binary WS messages.
type wsBinaryStreamWriter struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func newWSBinaryStreamWriter(ws *websocket.Conn) *wsBinaryStreamWriter {
	return &wsBinaryStreamWriter{ws: ws}
}

func (w *wsBinaryStreamWriter) Write(p []byte) (int, error) {
	const chunk = 32 * 1024
	total := 0

	w.mu.Lock()
	defer w.mu.Unlock()

	for len(p) > 0 {
		n := len(p)
		if n > chunk {
			n = chunk
		}

		wr, err := w.ws.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return total, err
		}
		if _, err := wr.Write(p[:n]); err != nil {
			_ = wr.Close()
			return total, err
		}
		if err := wr.Close(); err != nil {
			return total, err
		}

		total += n
		p = p[n:]
	}
	return total, nil
}
