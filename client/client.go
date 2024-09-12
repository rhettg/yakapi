package client

import (
	"encoding/json"
	"fmt"
	"net/url"
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

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial error: %v", err)
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("error reading WebSocket message: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(message, &data); err != nil {
			return fmt.Errorf("error unmarshaling message: %v", err)
		}

		eventChan <- Event{StreamName: streamName, Data: data}
	}
}
