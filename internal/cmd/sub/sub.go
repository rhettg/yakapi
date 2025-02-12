package sub

import (
	"encoding/json"
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
		d, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", event.StreamName, d)
	}
	return nil
}
