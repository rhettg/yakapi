package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
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
