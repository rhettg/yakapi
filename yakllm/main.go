package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/rhettg/agent"
	"github.com/rhettg/agent/agentset"
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

	as := agentset.New()
	as.Add("eyes", EyesAgentStartFunc(eyesP))

	ts := tools.New()
	ts.AddTools(as.Tools())
	ts.Add(forwardCommand, forwardHelp, forwardSchema, forward)
	ts.Add(backwardCommand, backwardHelp, backwardSchema, backward)
	ts.Add(rightCommand, rightHelp, rightSchema, right)
	ts.Add(leftCommand, leftHelp, leftSchema, left)

	a := agent.New(p, tools.WithTools(ts), agentset.WithAgentSet(as))
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

			if msg.Role == agent.RoleAssistant && msg.FunctionCallName == "" && as.Idle() {
				fmt.Println(response)
				break
			}
		}
	}
}

func grabImage(ctx context.Context) ([]byte, error) {
	url := "http://bni/v1/cam/capture"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Create a client
	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "image/jpeg" {
		return nil, fmt.Errorf("bad content type: %s", resp.Header.Get("Content-Type"))
	}

	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func overlay(jpgIn []byte) ([]byte, error) {
	// Load the overlay image
	// TODO: no reason to load it over and over
	overlayFile, err := os.Open("overlay.png")
	if err != nil {
		return nil, err
	}
	defer overlayFile.Close()

	overlayImg, err := png.Decode(overlayFile)
	if err != nil {
		return nil, err
	}

	jpgReader := bytes.NewReader(jpgIn)
	jpgImg, err := jpeg.Decode(jpgReader)
	if err != nil {
		return nil, err
	}

	// Create a new image for the composited result
	result := image.NewRGBA(overlayImg.Bounds())

	draw.Draw(result, result.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)

	// Draw the captured image onto the new image
	dst := image.Rectangle{
		Min: image.Point{
			X: 0,
			Y: 112,
		},
		Max: image.Point{
			X: 512,
			Y: 112 + 288,
		},
	}
	draw.Draw(result, dst, jpgImg, image.Point{0, 0}, draw.Src)

	// Draw the overlay onto the new image
	draw.Draw(result, overlayImg.Bounds().Add(image.Point{X: 0, Y: 0}), overlayImg, image.Point{}, draw.Over)

	buf := bytes.Buffer{}

	err = jpeg.Encode(&buf, result, &jpeg.Options{Quality: 100})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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

func forward(ctx context.Context, arguments string) (string, error) {
	args := struct {
		Distance int `json:"distance"`
	}{}
	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", err
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
