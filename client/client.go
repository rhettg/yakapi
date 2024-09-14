package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

	decoder := json.NewDecoder(resp.Body)
	for {
		var data map[string]interface{}
		if err := decoder.Decode(&data); err != nil {
			return fmt.Errorf("error decoding JSON: %v", err)
		}

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
