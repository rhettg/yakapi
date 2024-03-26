package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/rhettg/agent"
	"github.com/sashabaranov/go-openai/jsonschema"
)

//go:embed eyes.txt
var eyesSystemPrompt string

//go:embed eyes_welcome.txt
var eyesWelcomePrompt string

func insertEyesImage(nextStep agent.CompletionFunc) agent.CompletionFunc {
	return func(ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef) (*agent.Message, error) {
		if len(msgs) == 0 {
			return nextStep(ctx, msgs, tdfs)
		}

		msg := msgs[len(msgs)-1]
		if msg.Role != agent.RoleUser {
			return nextStep(ctx, msgs, tdfs)
		}

		for {
			imgData, err := grabImage(context.Background())
			if err != nil {
				return nil, err
			}

			slog.Info("grabbed image", "size", len(imgData))

			// Sometimes we get a bad image
			if len(imgData) <= 4096 {
				slog.Warn("truncated image, retrying")
				time.Sleep(200 * time.Millisecond)
				continue
			}

			overlayImgData, err := overlay(imgData)
			if err != nil {
				return nil, err
			}

			slog.Info("overlayed image", "size", len(overlayImgData))

			displayImage(overlayImgData)
			msg.AddImage("eyes.jpg", overlayImgData)
			return nextStep(ctx, msgs, tdfs)
		}
	}
}

func displayImage(d []byte) {
	if os.Getenv("ITERM_SESSION_ID") == "" {
		return
	}

	encoded := base64.StdEncoding.EncodeToString(d)

	// iTerm2's proprietary escape code
	fmt.Printf("\033]1337;File=inline=1:%s\a\n", encoded)
}

func EyesAgentStartFunc(c agent.CompletionFunc) func() (*agent.Agent, string) {
	return func() (*agent.Agent, string) {

		a := agent.New(insertEyesImage(c))
		a.Add(agent.RoleSystem, eyesSystemPrompt)

		return a, eyesWelcomePrompt
	}
}

var eyesCommand = "eyes"
var eyesHelp = `view eyes in front of rover`
var eyesSchema = jsonschema.Definition{
	Type: "object",
	Properties: map[string]jsonschema.Definition{
		"prompt": {
			Type:        "string",
			Description: "ask eyes for visual data about the scene in front of you.",
		},
	},
}

func askEyes(p agent.CompletionFunc) func(ctx context.Context, arguments string) (string, error) {
	return func(ctx context.Context, arguments string) (string, error) {
		args := struct {
			Prompt string `json:"prompt"`
		}{}

		err := json.Unmarshal([]byte(arguments), &args)
		if err != nil {
			return "", err
		}

		slog.Info("asking eyes", "prompt", args.Prompt)

		a := agent.New(p)
		a.Add(agent.RoleSystem, eyesSystemPrompt)

		var overlayImgData []byte
		for {
			imgData, err := grabImage(context.Background())
			if err != nil {
				return "", err
			}

			slog.Info("grabbed image", "size", len(imgData))

			// Sometimes we get a bad image
			if len(imgData) <= 4096 {
				slog.Warn("truncated image, retrying")
				time.Sleep(200 * time.Millisecond)
				continue
			}

			overlayImgData, err = overlay(imgData)
			if err != nil {
				return "", err
			}
			break
		}

		slog.Info("overlayed image", "size", len(overlayImgData))
		displayImage(overlayImgData)

		msg := agent.NewContentMessage(agent.RoleUser, args.Prompt)
		msg.AddImage("eyes.jpg", overlayImgData)
		a.AddMessage(msg)

		r, err := a.Step(ctx)
		if err != nil {
			return "", err
		}

		c, err := r.Content(ctx)
		if err != nil {
			return "", err
		}

		return c, nil
	}
}
