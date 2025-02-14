package pub

import (
	"bufio"
	"log/slog"
	"os"

	"github.com/rhettg/yakapi/client"
)

func DoPub(serverURL string, stream string) error {
	c := client.NewClient(serverURL)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		err := c.Publish(stream, []byte(line), "text/plain")
		if err != nil {
			return err
		}
		slog.Debug("published event", "content", line)
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
