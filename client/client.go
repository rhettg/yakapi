package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Client represents a YakAPI client
type Client struct {
	BaseURL string
}

// Event represents a YakAPI event
type Event struct {
	StreamName string
	Data       map[string]interface{}
}

// NewClient creates a new YakAPI client
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL}
}

// Subscribe subscribes to the specified streams and returns a channel of events
func (c *Client) Subscribe(streamNames []string) (<-chan Event, error) {
	eventChan := make(chan Event)
	var wg sync.WaitGroup

	for _, streamName := range streamNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := c.subscribeToStream(name, eventChan)
			if err != nil {
				fmt.Printf("Error subscribing to stream %s: %v\n", name, err)
			}
		}(streamName)
	}

	go func() {
		wg.Wait()
		close(eventChan)
	}()

	return eventChan, nil
}

func (c *Client) subscribeToStream(streamName string, eventChan chan<- Event) error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %v", err)
	}

	u.Scheme = "ws"
	u.Path = fmt.Sprintf("/v1/stream/%s", streamName)

	log.Printf("Connecting to WebSocket URL: %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial error: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to WebSocket for stream: %s", streamName)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("error reading WebSocket message: %v", err)
		}

		log.Printf("Received raw message from stream %s: %s", streamName, string(message))

		// Check if the message is a ping
		if strings.HasPrefix(string(message), "ping") {
			log.Printf("Received ping message from stream %s: %s", streamName, string(message))
			continue // Skip processing for ping messages
		}

		var data map[string]interface{}
		if err := json.Unmarshal(message, &data); err != nil {
			log.Printf("Error unmarshaling message: %v. Raw message: %s", err, string(message))
			continue // Skip this message and continue with the next one
		}

		log.Printf("Unmarshaled data for stream %s: %+v", streamName, data)

		eventChan <- Event{StreamName: streamName, Data: data}
	}
}

// Publish posts a single event to the specified stream
func (c *Client) Publish(streamName string, data []byte) error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %v", err)
	}

	u.Path = fmt.Sprintf("/v1/stream/%s", streamName)

	resp, err := http.Post(u.String(), "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("error posting event: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
