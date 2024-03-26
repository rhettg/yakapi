package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/rhettg/agent"
	"github.com/rhettg/agent/provider/openaichat"
	"github.com/rhettg/agent/tools"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"gitlab.com/greyxor/slogor"
)

//go:embed system.txt
var systemMessage string

func errorf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}

func main() {
	envFile, err := loadEnvFile(".env")
	if err != nil && !os.IsNotExist(err) {
		errorf("failed to load .env file: %v\n", err)
	}

	for k, v := range envFile {
		_ = os.Setenv(k, v)
	}

	slog.SetDefault(slog.New(slogor.NewHandler(os.Stderr, &slogor.Options{
		TimeFormat: time.Stamp,
		Level:      slog.LevelDebug,
		ShowSource: false,
	})))

	slog.Info("booting up")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		errorf("OPENAI_API_KEY is not set")
	}

	client := openai.NewClient(apiKey)

	eyesP := openaichat.New(client, "gpt-4-vision-preview",
		openaichat.WithMaxTokens(512),
		openaichat.WithMiddleware(openaichat.Logger(slog.Default())),
	)
	p := openaichat.New(client, "gpt-4-1106-preview",
		openaichat.WithMiddleware(openaichat.Logger(slog.Default())),
	)

	ts := tools.New()
	ts.Add(forwardCommand, forwardHelp, forwardSchema, forward)
	ts.Add(backwardCommand, backwardHelp, backwardSchema, backward)
	ts.Add(rightCommand, rightHelp, rightSchema, right)
	ts.Add(leftCommand, leftHelp, leftSchema, left)
	ts.Add(eyesCommand, eyesHelp, eyesSchema, askEyes(eyesP))

	a := agent.New(p, tools.WithTools(ts))
	a.Add(agent.RoleSystem, systemMessage)

	for {
		var prompt string
		text := huh.NewText().
			Title("operator").
			Placeholder("Where are you?").
			Value(&prompt)

		form := huh.NewForm(huh.NewGroup(text))
		err = form.Run()
		if err != nil {
			errorf("error getting prompt: %v\n", err)
		}

		fmt.Println("operator: " + prompt)
		a.Add(agent.RoleUser, prompt)

		slog.Info("asking llm")

		for {
			slog.Debug("stepping")
			msg, err := a.Step(context.Background())
			if err != nil {
				errorf("error getting message: %v\n", err)
			}

			if msg == nil {
				continue
			}

			response, err := msg.Content(context.Background())
			if err != nil {
				errorf("error getting response: %v\n", err)
			}

			slog.Debug("step", "role", msg.Role, "function", msg.FunctionCallName, "content", response)

			if msg.Role == agent.RoleAssistant && msg.FunctionCallName == "" {
				fmt.Println(response)
				break
			}
		}
	}
}

var forwardCommand = "fwd"
var forwardHelp = `Move foward`
var forwardSchema = jsonschema.Definition{
	Type: "object",
	Properties: map[string]jsonschema.Definition{
		"distance": {
			Type:        "integer",
			Description: "distance to move forward. 100 is 6-inches.",
		},
	},
}

var backwardCommand = "back"
var backwardHelp = `Move backward`
var backwardSchema = jsonschema.Definition{
	Type: "object",
	Properties: map[string]jsonschema.Definition{
		"distance": {
			Type:        "integer",
			Description: "distance to move forward. 100 is 6-inches.",
		},
	},
}

var rightCommand = "rt"
var rightHelp = `rotate right`
var rightSchema = jsonschema.Definition{
	Type: "object",
	Properties: map[string]jsonschema.Definition{
		"angle": {
			Type:        "integer",
			Description: "angle to move right. 90 is a quarter turn.",
		},
	},
}

var leftCommand = "lt"
var leftHelp = `rotate left`
var leftSchema = jsonschema.Definition{
	Type: "object",
	Properties: map[string]jsonschema.Definition{
		"angle": {
			Type:        "integer",
			Description: "angle to move left. 90 is a quarter turn.",
		},
	},
}

const (
	maxDistance = 500
	minDistance = 10
	maxAngle    = 180
	minAngle    = 0
)

func forward(ctx context.Context, arguments string) (string, error) {
	args := struct {
		Distance int `json:"distance"`
	}{}
	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", err
	}

	if args.Distance < minDistance {
		return fmt.Sprintf("min distance is %d", minDistance), nil
	}
	if args.Distance > maxDistance {
		return fmt.Sprintf("max distance is %d", maxDistance), nil
	}

	err = sendCommand(ctx, fmt.Sprintf("fwd %d", args.Distance))
	if err != nil {
		return "", err
	}

	slog.Info("forward", "distance", args.Distance)
	return "OK", nil
}

func backward(ctx context.Context, arguments string) (string, error) {
	args := struct {
		Distance int `json:"distance"`
	}{}
	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", err
	}

	if args.Distance < minDistance {
		return fmt.Sprintf("min distance is %d", minDistance), nil
	}
	if args.Distance > maxDistance {
		return fmt.Sprintf("max distance is %d", maxDistance), nil
	}

	err = sendCommand(ctx, fmt.Sprintf("bck %d", args.Distance))
	if err != nil {
		return "", err
	}

	slog.Info("backward", "distance", args.Distance)
	return "OK", nil
}

func left(ctx context.Context, arguments string) (string, error) {
	args := struct {
		Angle int `json:"angle"`
	}{}
	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", err
	}

	if args.Angle < minAngle {
		return fmt.Sprintf("min angle is %d", minAngle), nil
	}
	if args.Angle > maxAngle {
		return fmt.Sprintf("max angle is %d", maxAngle), nil
	}

	err = sendCommand(ctx, fmt.Sprintf("lt %d", args.Angle))
	if err != nil {
		return "", err
	}

	slog.Info("turn left", "angle", args.Angle)
	return "OK", nil
}

func right(ctx context.Context, arguments string) (string, error) {
	args := struct {
		Angle int `json:"angle"`
	}{}
	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", err
	}

	if args.Angle < minAngle {
		return fmt.Sprintf("min angle is %d", minAngle), nil
	}
	if args.Angle > maxAngle {
		return fmt.Sprintf("max angle is %d", maxAngle), nil
	}

	err = sendCommand(ctx, fmt.Sprintf("rt %d", args.Angle))
	if err != nil {
		return "", err
	}

	slog.Info("turn right", "angle", args.Angle)
	return "OK", nil
}

func sendCommand(ctx context.Context, cmd string) error {
	url := "http://bni/v1/ci"

	reqData := struct {
		Command string `json:"command"`
	}{
		Command: cmd,
	}

	data, err := json.Marshal(reqData)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		return fmt.Errorf("bad content type: %s", resp.Header.Get("Content-Type"))
	}

	defer resp.Body.Close()

	result := struct {
		Result string `json:"result"`
	}{}

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(respData, &result)
	if err != nil {
		return err
	}

	slog.Info("command sent", "command", cmd, "result", result.Result)

	return nil
}
