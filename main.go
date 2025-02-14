package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.com/greyxor/slogor"

	"github.com/rhettg/yakapi/internal/cmd/pub"
	"github.com/rhettg/yakapi/internal/cmd/server"
	"github.com/rhettg/yakapi/internal/cmd/sub"
)

func loadDotEnv() error {
	// Open .env file
	f, err := os.Open(".env")
	if err != nil {
		if os.IsNotExist(err) {
			// .env file does not exist, ignore
			return nil
		}
		return err
	}
	defer f.Close()

	// Read lines
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Parse key/value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid line: %s", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if strings.HasPrefix(key, "#") {
			// Skip comments
			continue
		}

		// Remove surrounding quotes
		re := regexp.MustCompile(`^["'](.*)["']$`)

		if re.MatchString(value) {
			value = re.ReplaceAllString(value, `$1`)
		}

		fmt.Printf("Setting environment variable: %s=%s\n", key, value)

		// Set environment variable
		err := os.Setenv(key, value)
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func main() {
	var logLevel string
	var serverURL string

	rootCmd := &cobra.Command{
		Use:   "yakapi",
		Short: "YakAPI - A versatile API server",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if logLevel == "debug" {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slogor.NewHandler(os.Stderr, slogor.Options{
				Level:      level,
				TimeFormat: time.Stamp,
			})))
		},
	}

	serverURLDefault := os.Getenv("YAKAPI_SERVER")
	if serverURLDefault == "" {
		serverURLDefault = "http://localhost:8080"
	}

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set the logging level (info or debug)")
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", serverURLDefault, "Server URL to connect to")

	err := loadDotEnv()
	if err != nil {
		slog.Error("error loading .env file", "error", err)
		return
	}

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the YakAPI server",
		Run:   server.DoServer,
	}

	helloCmd := &cobra.Command{
		Use:   "hello",
		Short: "Print a greeting",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("hello world")
		},
	}

	subCmd := &cobra.Command{
		Use:   "sub [streams...]",
		Short: "Subscribe to events from specified streams",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("Please specify at least one stream name")
				return
			}

			err := sub.DoSub(serverURL, args)
			if err != nil {
				slog.Error("Error subscribing to streams", "error", err)
				return
			}
		},
	}

	pubCmd := &cobra.Command{
		Use:   "pub",
		Short: "Publish an event to a stream",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				fmt.Println("Please specify a stream name")
				return
			}

			err := pub.DoPub(serverURL, args[0])
			if err != nil {
				slog.Error("Error publishing event", "error", err)
				return
			}
		},
	}

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(helloCmd)
	rootCmd.AddCommand(subCmd)
	rootCmd.AddCommand(pubCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error executing root command", "error", err)
		os.Exit(1)
	}
}
