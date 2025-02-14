package client

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	Data       []byte
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

		// HTTP chunk may end with a newline, so we need to trim it
		s := n
		if buf[s-1] == '\n' {
			s--
		}

		eventData := make([]byte, s)
		copy(eventData, buf[:s])

		eventChan <- Event{StreamName: streamName, Data: eventData}
	}
}

func (c *Client) Publish(streamName string, b []byte, contentType string) error {
	url := fmt.Sprintf("%s/v1/stream/%s", c.BaseURL, streamName)

	buf := bytes.NewBuffer(b)

	resp, err := http.Post(url, contentType, buf)
	if err != nil {
		return fmt.Errorf("HTTP POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) PublishJSON(streamName string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling data: %v", err)
	}

	return c.Publish(streamName, payload, "application/json")
}
