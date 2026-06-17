package ws

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/tent-of-trials/market/matching"
	"github.com/tent-of-trials/market/orderbook"
	"github.com/tent-of-trials/market/types"
	"go.uber.org/zap"
)

func TestWriteAndRecoverOrderBookSnapshot(t *testing.T) {
	server, book := newSnapshotTestServer(t)
	addSnapshotTestOrder(t, book, "snap-1")

	before, err := server.engine.SnapshotOrderBooks()
	if err != nil {
		t.Fatalf("snapshot before write: %v", err)
	}
	if err := server.writeOrderBookSnapshot(); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	if err := server.writeOrderBookSnapshot(); err != nil {
		t.Fatalf("rewrite snapshot: %v", err)
	}

	written, err := os.ReadFile(server.snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(before) != string(written) {
		t.Fatalf("written snapshot mismatch:\n%s\n%s", before, written)
	}

	checksum, err := os.ReadFile(server.checksumPath)
	if err != nil {
		t.Fatalf("read checksum: %v", err)
	}
	sum := sha256.Sum256(written)
	if strings.TrimSpace(string(checksum)) != fmt.Sprintf("%x", sum[:]) {
		t.Fatalf("checksum did not match snapshot body")
	}

	recoveredServer, _ := newSnapshotTestServerWithPaths(t, server.snapshotPath, server.checksumPath)
	if err := recoveredServer.recoverOrderBookSnapshot(); err != nil {
		t.Fatalf("recover snapshot: %v", err)
	}
	after, err := recoveredServer.engine.SnapshotOrderBooks()
	if err != nil {
		t.Fatalf("snapshot after recover: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("recovered snapshot mismatch:\n%s\n%s", before, after)
	}
}

func TestRecoverRejectsCorruptedSnapshot(t *testing.T) {
	server, book := newSnapshotTestServer(t)
	addSnapshotTestOrder(t, book, "snap-2")
	if err := server.writeOrderBookSnapshot(); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	if err := os.WriteFile(server.snapshotPath, []byte(`{"corrupted":true}`), 0644); err != nil {
		t.Fatalf("corrupt snapshot: %v", err)
	}

	recoveredServer, _ := newSnapshotTestServerWithPaths(t, server.snapshotPath, server.checksumPath)
	if err := recoveredServer.recoverOrderBookSnapshot(); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestAdminSnapshotEndpoint(t *testing.T) {
	server, book := newSnapshotTestServer(t)
	addSnapshotTestOrder(t, book, "snap-3")

	getRecorder := httptest.NewRecorder()
	server.handleOrderBookSnapshot(getRecorder, httptest.NewRequest(http.MethodGet, "/admin/orderbook/snapshot", nil))
	if getRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", getRecorder.Code, http.StatusMethodNotAllowed)
	}

	postRecorder := httptest.NewRecorder()
	server.handleOrderBookSnapshot(postRecorder, httptest.NewRequest(http.MethodPost, "/admin/orderbook/snapshot", nil))
	if postRecorder.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body %q", postRecorder.Code, postRecorder.Body.String())
	}
	if _, err := os.Stat(server.snapshotPath); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}
	if _, err := os.Stat(server.checksumPath); err != nil {
		t.Fatalf("checksum file missing: %v", err)
	}
}

func newSnapshotTestServer(t *testing.T) (*Server, *orderbook.OrderBook) {
	t.Helper()
	tempDir := t.TempDir()
	return newSnapshotTestServerWithPaths(
		t,
		filepath.Join(tempDir, "orderbook_snapshot.json"),
		filepath.Join(tempDir, "orderbook_snapshot.sha256"),
	)
}

func newSnapshotTestServerWithPaths(t *testing.T, snapshotPath, checksumPath string) (*Server, *orderbook.OrderBook) {
	t.Helper()
	book := orderbook.NewOrderBook(types.Symbol("BTC-USD"), orderbook.Config{MaxDepth: 10, PriceDecimals: 8, VolumeDecimals: 8})
	books := map[types.Symbol]*orderbook.OrderBook{
		types.Symbol("BTC-USD"): book,
	}
	engine := matching.NewMatchingEngine(matching.EngineConfig{EnableShorting: true}, books)
	logger := zap.NewNop()
	server := NewServer(NewHub(logger), engine, logger, 0)
	server.snapshotPath = snapshotPath
	server.checksumPath = checksumPath
	server.snapshotInterval = 0
	return server, book
}

func addSnapshotTestOrder(t *testing.T, book *orderbook.OrderBook, id string) {
	t.Helper()
	_, err := book.AddOrder(&types.Order{
		ID:           id,
		Symbol:       types.Symbol("BTC-USD"),
		Side:         types.Buy,
		Type:         types.Limit,
		Price:        decimal.RequireFromString("101.50"),
		Quantity:     decimal.RequireFromString("1.25"),
		RemainingQty: decimal.RequireFromString("1.25"),
	})
	if err != nil {
		t.Fatalf("add order: %v", err)
	}
}
