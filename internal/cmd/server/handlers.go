package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rhettg/yakapi/internal/ci"
	"github.com/rhettg/yakapi/internal/gds"
	"github.com/rhettg/yakapi/internal/stream"
	"github.com/rhettg/yakapi/internal/telemetry"
	"tailscale.com/client/tailscale"
)

var (
	//go:embed assets/*
	assets        embed.FS
	streamManager *stream.Manager
	ciResults     ci.ResultCollector
)

type resource struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "/v1")
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func eyes(w http.ResponseWriter, r *http.Request) {
	content, err := assets.ReadFile("assets/index.html")
	if err != nil {
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(content)
	if err != nil {
		slog.Error("error writing response", "error", err)
		return
	}
}

func errorResponse(w http.ResponseWriter, respErr error, statusCode int) {
	resp := struct {
		Error string `json:"error"`
	}{Error: respErr.Error()}

	err := sendResponse(w, resp, statusCode)
	if err != nil {
		slog.Error("error sending response", "error", err)
		return
	}
}

func sendResponse(w http.ResponseWriter, resp interface{}, statusCode int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		return err
	}

	return nil
}

func me(w http.ResponseWriter, r *http.Request) {
	lc := tailscale.LocalClient{}
	whois, err := lc.WhoIs(r.Context(), r.RemoteAddr)
	if err != nil {
		errorResponse(w, errors.New("unknown"), http.StatusInternalServerError)
		slog.Error("whois failure", "error", err)
		return
	}

	resp := struct {
		Name   string `json:"name"`
		Login  string `json:"login"`
		Device string `json:"device"`
	}{
		Name:   whois.UserProfile.DisplayName,
		Login:  whois.UserProfile.LoginName,
		Device: whois.Node.Hostinfo.Hostname(),
	}

	err = sendResponse(w, &resp, http.StatusOK)
	if err != nil {
		slog.Error("error sending response", "err", err)
		return
	}
}

func handleCI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		errorResponse(w, errors.New("POST required"), http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("content-type") != "application/json" {
		errorResponse(w, errors.New("application/json required"), http.StatusUnsupportedMediaType)
		return
	}

	waitForResult := false
	if r.URL.Query().Get("wait") == "1" {
		waitForResult = true
	}

	req := struct {
		Command string `json:"command"`
	}{}

	err := json.NewDecoder(r.Body).Decode(&req)
	defer r.Body.Close()

	if err != nil {
		slog.Error("failed parsing body", "error", err)
		errorResponse(w, errors.New("failed parsing body"), http.StatusBadRequest)
		return
	}

	msgID, err := ci.Accept(r.Context(), streamManager, req.Command)
	if err != nil {
		slog.Error("failed accepting ci command", "error", err)
		errorResponse(w, err, http.StatusBadRequest)
		return
	}

	var result ci.Result
	result.ID = msgID

	if waitForResult {
		waitCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		for {
			result = ciResults.FetchResult(msgID)
			if result.ID != "" {
				break
			}
			select {
			case <-waitCtx.Done():
				err = waitCtx.Err()
				cancel()
				slog.Error("failed fetching ci command result", "error", err)
				errorResponse(w, err, http.StatusServiceUnavailable)
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
		cancel()
	}

	err = sendResponse(w, result, http.StatusAccepted)
	if err != nil {
		slog.Error("error sending response", "error", err)
		return
	}
}

func doGDSCI(ctx context.Context, c *gds.Client) error {
	startTime := time.Now()

	slog.Info("retrieving commands from GDS")

	notes, err := c.GetNotes(ctx)
	if err != nil {
		return fmt.Errorf("failed to retreive notes: %w", err)
	}

	req := struct {
		Command string `json:"command"`
	}{}

	for _, n := range notes {
		// Reset our command
		req.Command = ""

		slog.Info("processing note", "file", n.File, "note", n.Note, "created_at", n.CreatedAt)
		if n.File != "commands.qi" {
			continue
		}

		err = json.Unmarshal([]byte(n.Body), &req)
		if err != nil {
			slog.Error("failed unmarshaling note", "error", err, "note", n.Note)
			continue
		}

		if req.Command == "" {
			slog.Error("empty command", "note", n.Note)
			continue
		}

		slog.Info("accepting command", "command", req.Command, "note", n.Note)
		msgID, err := ci.Accept(ctx, streamManager, req.Command)
		if err != nil {
			slog.Error("failed accepting ci command", "error", err)
			continue
		}
		slog.Info("accepted command", "command", req.Command, "command_id", msgID)
	}

	slog.Info("finished processing notes", "note_count", len(notes), "elapsed", time.Since(startTime))

	return nil
}

func handleCamCapture(w http.ResponseWriter, r *http.Request) {
	captureFile := os.Getenv("YAKAPI_CAM_CAPTURE_PATH")
	if captureFile == "" {
		err := errors.New("YAKAPI_CAM_CAPTURE_PATH not configured")
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	content, err := os.ReadFile(captureFile)
	if err != nil {
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(content)
	if err != nil {
		slog.Error("error writing response", "error", err)
		return
	}
}

func homev1(w http.ResponseWriter, r *http.Request) {
	resp := struct {
		Name      string     `json:"name"`
		Revision  string     `json:"revision"`
		UpTime    int64      `json:"uptime"`
		Resources []resource `json:"resources"`
	}{
		Name:     "YakAPI Server",
		Revision: Revision,
		UpTime:   int64(time.Since(startTime).Seconds()),
		Resources: []resource{
			{Name: "metrics", Ref: "/metrics"},
			{Name: "ci", Ref: "/v1/ci"},
			{Name: "cam", Ref: "/v1/cam/capture"},
			{Name: "stream", Ref: "/v1/stream/"},
		},
	}

	name := os.Getenv("YAKAPI_NAME")
	if name != "" {
		resp.Name = name
	}

	project := os.Getenv("YAKAPI_PROJECT")
	if project != "" {
		resp.Resources = append(resp.Resources, resource{Name: "project", Ref: project})
	}

	operator := os.Getenv("YAKAPI_OPERATOR")
	if operator != "" {
		resp.Resources = append(resp.Resources, resource{Name: "operator", Ref: operator})
	}

	err := sendResponse(w, resp, http.StatusOK)
	if err != nil {
		slog.Error("error sending response", "error", err)
		return
	}
}

func fetchTelemetryData(ctx context.Context, out chan telemetry.Data) error {
	stream := streamManager.GetReader("telemetry")
	defer streamManager.ReturnReader("telemetry", stream)

	allTelemetryData := make(telemetry.Data)

	for {
		select {
		case <-ctx.Done():
			return nil
		case data, ok := <-stream:
			if !ok {
				return errors.New("stream closed")
			}
			// Load json telemetry from data
			telemetryData := make(map[string]interface{})
			err := json.Unmarshal([]byte(data), &telemetryData)
			if err != nil {
				slog.Warn("failed to unmarshal telemetry data", "error", err)
				continue
			}

			for key, value := range telemetryData {
				slog.Info("telemetry data", "key", key, "value", value)
				allTelemetryData[key] = value
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case out <- allTelemetryData:
			slog.Debug("telemetry collected")
		default:
		}
	}
}

func parseStreamPath(path string) string {
	remaining, found := strings.CutPrefix(path, "/v1/stream/")
	if found {
		return remaining
	}
	return ""
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	streamName := parseStreamPath(r.URL.Path)
	if streamName == "" {
		slog.Warn("invalid stream path", "path", r.URL.Path)
		http.Error(w, "Invalid stream path", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		slog.Debug("stream out", "stream", streamName)
		w.Header().Set("Transfer-Encoding", "chunked")
		err := stream.StreamOut(r.Context(), w, streamName, streamManager)
		if err != nil {
			http.Error(w, "Error streaming out", http.StatusInternalServerError)
			return
		}
		slog.Info("stream out complete", "stream", streamName)
	case http.MethodPost:
		slog.Debug("stream in", "stream", streamName)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}

		slog.Debug("stream in", "stream", streamName, "body", string(body))

		err = stream.StreamIn(r.Context(), streamName, body, streamManager)
		if err != nil {
			http.Error(w, "Error streaming in", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
