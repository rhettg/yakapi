package sub

import (
	"fmt"

	"github.com/rhettg/yakapi/client"
)

func DoSub(serverURL string, streams []string) error {
	c := client.NewClient(serverURL)
	eventChan, err := c.Subscribe(streams)
	if err != nil {
		return err
	}

	for event := range eventChan {
		fmt.Printf("%s: %s\n", event.StreamName, string(event.Data))
	}
	return nil
}
