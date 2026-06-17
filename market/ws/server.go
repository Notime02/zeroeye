package ws

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tent-of-trials/market/matching"
	"github.com/tent-of-trials/market/types"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	subs   map[types.Symbol]struct{}
	remote string
	mu     sync.Mutex
}

type Hub struct {
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	logger     *zap.Logger
	mu         sync.RWMutex
}

type Server struct {
	hub              *Hub
	engine           *matching.MatchingEngine
	logger           *zap.Logger
	port             int
	srv              *http.Server
	snapshotPath     string
	checksumPath     string
	snapshotInterval time.Duration
	snapshotStop     chan struct{}
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
			h.logger.Info("client connected",
				zap.String("remote", client.remote),
				zap.Int("total", len(h.clients)),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Info("client disconnected",
				zap.String("remote", client.remote),
				zap.Int("total", len(h.clients)),
			)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func NewServer(hub *Hub, engine *matching.MatchingEngine, logger *zap.Logger, port int) *Server {
	return &Server{
		hub:              hub,
		engine:           engine,
		logger:           logger,
		port:             port,
		snapshotPath:     filepath.Join("data", "orderbook_snapshot.json"),
		checksumPath:     filepath.Join("data", "orderbook_snapshot.sha256"),
		snapshotInterval: snapshotIntervalFromEnv(),
		snapshotStop:     make(chan struct{}),
	}
}

func (s *Server) Start() error {
	if err := s.recoverOrderBookSnapshot(); err != nil {
		return err
	}
	s.startSnapshotLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/trades", s.handleGetTrades)
	mux.HandleFunc("/api/v1/depth", s.handleGetDepth)
	mux.HandleFunc("/admin/orderbook/snapshot", s.handleOrderBookSnapshot)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.srv.ListenAndServe()
}

func (s *Server) Stop() {
	select {
	case <-s.snapshotStop:
	default:
		close(s.snapshotStop)
	}
	if s.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	client := &Client{
		hub:    s.hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		subs:   make(map[types.Symbol]struct{}),
		remote: r.RemoteAddr,
	}

	s.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"service": "tent-market",
		"time":    time.Now().Unix(),
	})
}

func (s *Server) handleGetTrades(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	trades := s.engine.GetRecentTrades(100)
	json.NewEncoder(w).Encode(trades)
}

func (s *Server) handleGetDepth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "depth endpoint"})
}

func (s *Server) handleOrderBookSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.writeOrderBookSnapshot(); err != nil {
		s.logger.Error("order book snapshot failed", zap.Error(err))
		http.Error(w, "snapshot failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"path":   s.snapshotPath,
	})
}

func (s *Server) startSnapshotLoop() {
	if s.snapshotInterval <= 0 {
		return
	}
	ticker := time.NewTicker(s.snapshotInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.writeOrderBookSnapshot(); err != nil {
					s.logger.Error("periodic order book snapshot failed", zap.Error(err))
				}
			case <-s.snapshotStop:
				return
			}
		}
	}()
}

func (s *Server) writeOrderBookSnapshot() error {
	data, err := s.engine.SnapshotOrderBooks()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.snapshotPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.checksumPath), 0755); err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	checksum := fmt.Sprintf("%x", sum[:])

	if err := writeFileAtomic(s.snapshotPath, data, 0644); err != nil {
		return err
	}
	if err := writeFileAtomic(s.checksumPath, []byte(checksum+"\n"), 0644); err != nil {
		return err
	}
	return nil
}

func (s *Server) recoverOrderBookSnapshot() error {
	data, err := os.ReadFile(s.snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	checksumData, err := os.ReadFile(s.checksumPath)
	if err != nil {
		return fmt.Errorf("read order book snapshot checksum: %w", err)
	}

	expected := strings.TrimSpace(string(checksumData))
	actualBytes := sha256.Sum256(data)
	actual := fmt.Sprintf("%x", actualBytes[:])
	if expected != actual {
		return fmt.Errorf("order book snapshot checksum mismatch")
	}

	if err := s.engine.RecoverOrderBooks(data); err != nil {
		return err
	}
	s.logger.Info("order book snapshot recovered", zap.String("path", s.snapshotPath))
	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(tmpPath, path)
	}
	return nil
}

func snapshotIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("OB_SNAPSHOT_INTERVAL_SECS"))
	if raw == "" {
		return 60 * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		return 60 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(65536)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		c.mu.Lock()

		c.mu.Unlock()
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
