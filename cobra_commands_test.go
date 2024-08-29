package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	rootCmd := &cobra.Command{Use: "yakapi"}
	rootCmd.PersistentFlags().String("log-level", "info", "Set the logging level (info or debug)")

	_, err := executeCommand(rootCmd)
	assert.NoError(t, err)
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("log-level"))
}

func TestServerCommand(t *testing.T) {
	rootCmd := &cobra.Command{Use: "yakapi"}
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the YakAPI server",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	rootCmd.AddCommand(serverCmd)

	_, err := executeCommand(rootCmd, "server")
	assert.NoError(t, err)
	assert.NotNil(t, rootCmd.Commands())
	assert.Contains(t, rootCmd.Commands(), serverCmd)
}

func TestHelloCommand(t *testing.T) {
	rootCmd := &cobra.Command{Use: "yakapi"}
	helloCmd := &cobra.Command{
		Use:   "hello",
		Short: "Print a greeting",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Print("hello world")
		},
	}
	rootCmd.AddCommand(helloCmd)

	output, err := executeCommand(rootCmd, "hello")
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
}

func TestLogLevelFlag(t *testing.T) {
	rootCmd := &cobra.Command{Use: "yakapi"}
	rootCmd.PersistentFlags().String("log-level", "info", "Set the logging level (info or debug)")

	_, err := executeCommand(rootCmd, "--log-level", "debug")
	assert.NoError(t, err)

	logLevel, err := rootCmd.Flags().GetString("log-level")
	assert.NoError(t, err)
	assert.Equal(t, "debug", logLevel)
}

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()

	return buf.String(), err
}
