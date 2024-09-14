package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
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
	url := fmt.Sprintf("%s/v1/stream/%s", c.BaseURL, streamName)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buf := make([]byte, 16*1024)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			return fmt.Errorf("error reading chunk: %v", err)
		}
		if n == len(buf) {
			return fmt.Errorf("chunk too large")
		}
		s := 0
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				s = i + 1
				break
			}
		}

		var data map[string]interface{}
		if err := json.Unmarshal(buf[s:n], &data); err != nil {
			return fmt.Errorf("error unmarshaling chunk: %v", err)
		}

		log.Printf("Unmarshaled data for stream %s: %+v", streamName, data)

		eventChan <- Event{StreamName: streamName, Data: data}
	}
}
