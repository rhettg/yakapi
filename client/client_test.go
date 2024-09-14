package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClient(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// Create a mock WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
		}
		defer conn.Close()

		// Send two events
		events := []map[string]interface{}{
			{"message": "Event 1"},
			{"message": "Event 2"},
		}

		for _, event := range events {
			err := conn.WriteJSON(event)
			if err != nil {
				t.Fatalf("Failed to write event: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Close the connection after sending events
		if err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			t.Fatalf("Failed to close WebSocket connection: %v", err)
		}
		wg.Done()
	}))
	defer server.Close()

	// Create a new client with WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL)

	// Subscribe to a stream
	eventChan, err := client.Subscribe([]string{"test-stream"})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Collect events
	var receivedEvents []Event
	timeout := time.After(2 * time.Second)

collectLoop:
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				break collectLoop
			}
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			t.Fatal("Test timed out waiting for events")
		}
	}

	// Wait for the server to finish and close the connection
	wg.Wait()

	// Check if we received the expected number of events
	if len(receivedEvents) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(receivedEvents))
	}

	// Verify the content of the events
	expectedMessages := []string{"Event 1", "Event 2"}
	for i, event := range receivedEvents {
		if event.StreamName != "test-stream" {
			t.Errorf("Event %d: expected stream name 'test-stream', got '%s'", i, event.StreamName)
		}
		if msg, ok := event.Data["message"].(string); !ok || msg != expectedMessages[i] {
			t.Errorf("Event %d: expected message '%s', got '%v'", i, expectedMessages[i], event.Data["message"])
		}
	}
}

func TestPublish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/stream/") {
			t.Errorf("Invalid path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body) // Echo back the received data
	}))
	defer server.Close()

	client := NewClient(server.URL)

	t.Run("Publish with -d option", func(t *testing.T) {
		data := []byte(`{"message": "Test event"}`)
		err := client.Publish("test-stream", data)
		if err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}
	})

	t.Run("Publish with invalid URL", func(t *testing.T) {
		invalidClient := NewClient("http://invalid-url")
		err := invalidClient.Publish("test-stream", []byte(`{"message": "Test event"}`))
		if err == nil {
			t.Fatal("Expected error for invalid URL, got nil")
		}
	})

	t.Run("Publish with server error", func(t *testing.T) {
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer errorServer.Close()

		errorClient := NewClient(errorServer.URL)
		err := errorClient.Publish("test-stream", []byte(`{"message": "Test event"}`))
		if err == nil {
			t.Fatal("Expected error for server error, got nil")
		}
	})
}
