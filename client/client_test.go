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
			fmt.Fprintf(w, "%x\r\n%s\r\n", len(eventJSON), eventJSON)
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}

		// End the chunked response
		fmt.Fprintf(w, "0\r\n\r\n")
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
