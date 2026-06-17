package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	messageTypeChat   = "chat"
	messageTypeSystem = "system"
	messageTypeError  = "error"

	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

type clientMessage struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Content  string `json:"content"`
}

type serverMessage struct {
	Type      string    `json:"type"`
	Username  string    `json:"username,omitempty"`
	Content   string    `json:"content"`
	ClientID  int64     `json:"client_id,omitempty"`
	Online    int       `json:"online,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	nextID  int64
}

func newHub() *hub {
	return &hub{
		clients: make(map[*client]struct{}),
	}
}

func (h *hub) add(conn *websocket.Conn) *client {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextID++
	c := &client{
		id:   h.nextID,
		hub:  h,
		conn: conn,
		send: make(chan serverMessage, 16),
	}
	h.clients[c] = struct{}{}
	return c
}

func (h *hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *hub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *hub) broadcast(msg serverMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			go c.closeSlowClient()
		}
	}
}

type client struct {
	id   int64
	hub  *hub
	conn *websocket.Conn
	send chan serverMessage
}

func (c *client) readPump() {
	defer func() {
		c.hub.remove(c)
		c.conn.Close()
		c.hub.broadcast(systemMessage(fmt.Sprintf("client %d disconnected", c.id), c.hub.count()))
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var incoming clientMessage
		if err := c.conn.ReadJSON(&incoming); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("client %d read: %v", c.id, err)
			}
			return
		}

		msg, err := normalizeClientMessage(c.id, incoming)
		if err != nil {
			c.send <- errorMessage(err.Error())
			continue
		}
		c.hub.broadcast(msg)
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("client %d write: %v", c.id, err)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) closeSlowClient() {
	c.hub.remove(c)
	_ = c.conn.Close()
}

type server struct {
	hub      *hub
	upgrader websocket.Upgrader
	static   http.Handler
}

func newServer(staticDir string) *server {
	return &server{
		hub:    newHub(),
		static: http.FileServer(http.Dir(staticDir)),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return strings.Contains(origin, r.Host)
			},
		},
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.Handle("/", s.static)
	return mux
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"online": s.hub.count(),
	})
}

func (s *server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}

	c := s.hub.add(conn)
	c.send <- systemMessage(fmt.Sprintf("connected as client %d", c.id), s.hub.count())
	s.hub.broadcast(systemMessage(fmt.Sprintf("client %d joined", c.id), s.hub.count()))

	go c.writePump()
	c.readPump()
}

func normalizeClientMessage(clientID int64, incoming clientMessage) (serverMessage, error) {
	username := strings.TrimSpace(incoming.Username)
	content := strings.TrimSpace(incoming.Content)
	if username == "" {
		return serverMessage{}, errors.New("username is required")
	}
	if content == "" {
		return serverMessage{}, errors.New("message content is required")
	}
	if len(username) > 32 {
		return serverMessage{}, errors.New("username must be 32 characters or fewer")
	}
	if len(content) > maxMessageSize {
		return serverMessage{}, errors.New("message content is too long")
	}

	return serverMessage{
		Type:      messageTypeChat,
		Username:  username,
		Content:   content,
		ClientID:  clientID,
		Timestamp: time.Now().UTC(),
	}, nil
}

func systemMessage(content string, online int) serverMessage {
	return serverMessage{
		Type:      messageTypeSystem,
		Content:   content,
		Online:    online,
		Timestamp: time.Now().UTC(),
	}
}

func errorMessage(content string) serverMessage {
	return serverMessage{
		Type:      messageTypeError,
		Content:   content,
		Timestamp: time.Now().UTC(),
	}
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	staticDir := flag.String("static", "phase1/public", "static web client directory")
	flag.Parse()

	s := newServer(*staticDir)
	log.Printf("phase 1 websocket chat listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, s.routes()); err != nil {
		log.Fatal(err)
	}
}
