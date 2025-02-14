package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	// Create a mock HTTP server with chunked encoding
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to be an http.Flusher")
		}

		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)

		// Send two events
		events := []map[string]interface{}{
			{"message": "Event 1"},
			{"message": "Event 2"},
		}

		for _, event := range events {
			eventJSON, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}
			fmt.Fprint(w, string(eventJSON))
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}

		flusher.Flush()
	}))
	defer server.Close()

	// Create a new client with the server URL
	client := NewClient(server.URL)

	// Subscribe to a stream
	eventChan, err := client.Subscribe([]string{"test-stream"})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Collect events
	receivedEvents := make([]Event, 0)
	timeout := time.After(2 * time.Second)

collectLoop:
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				break collectLoop
			}
			fmt.Printf("Received event: %s\n", string(event.Data))
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			t.Fatal("Test timed out waiting for events")
		}
	}

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

		var data map[string]interface{}

		err := json.Unmarshal(event.Data, &data)
		if err != nil {
			t.Errorf("Event %d: failed to unmarshal event data: %v", i, err)
			continue
		}

		if msg, ok := data["message"].(string); !ok || msg != expectedMessages[i] {
			t.Errorf("Event %d: expected message '%s', got '%v'", i, expectedMessages[i], data["message"])
		}
	}
}
