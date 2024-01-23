package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/rhettg/agent"
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
