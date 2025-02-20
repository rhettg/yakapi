package server

import (
	"context"
	"embed"
	"encoding/base64"
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

func parseCamPath(path string) string {
	remaining, found := strings.CutPrefix(path, "/v1/eyes/")
	if found {
		return remaining
	}
	return ""
}

func hijackStream(w http.ResponseWriter, r *http.Request, s stream.StreamChan) {
	// At the start of the handler, get the underlying hijacked connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		slog.Error("webserver doesn't support hijacking")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		slog.Error("failed to hijack connection", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Write headers
	fmt.Fprintf(bufrw, "HTTP/1.1 200 OK\r\n")
	fmt.Fprintf(bufrw, "Content-Type: multipart/x-mixed-replace; boundary=YAKFRAME\r\n")
	fmt.Fprintf(bufrw, "Cache-Control: no-cache\r\n")
	fmt.Fprintf(bufrw, "Connection: keep-alive\r\n")
	fmt.Fprintf(bufrw, "Pragma: no-cache\r\n\r\n")

	//fmt.Fprintf(bufrw, "--YAKFRAME\r\n\r\n")
	fmt.Fprintf(bufrw, "--YAKFRAME\r\n")
	bufrw.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-s:
			if !ok {
				return
			}

			imageData, err := base64.StdEncoding.DecodeString(string(data))
			if err != nil {
				slog.Error("failed to decode base64 image", "error", err)
				continue
			}
			fmt.Fprintf(bufrw, "Content-Type: image/jpeg\r\n")
			fmt.Fprintf(bufrw, "Content-Length: %d\r\n", len(imageData))
			fmt.Fprintf(bufrw, "\r\n")
			bufrw.Write(imageData)

			fmt.Fprintf(bufrw, "\r\n--YAKFRAME\r\n")

			// This is crazy, but sending the image twice seems to be the only
			// way to get browsers to show the image immediately. Otherwise, it
			// seems to want to wait until the next one is available.
			fmt.Fprintf(bufrw, "Content-Type: image/jpeg\r\n")
			fmt.Fprintf(bufrw, "Content-Length: %d\r\n", len(imageData))
			fmt.Fprintf(bufrw, "\r\n")
			bufrw.Write(imageData)
			fmt.Fprintf(bufrw, "\r\n--YAKFRAME\r\n")

			bufrw.Flush()
			slog.Info("eyes frame sent", "frame_size", len(imageData))
		}
	}
}

func handleStreamEyes(w http.ResponseWriter, r *http.Request) {
	streamName := parseCamPath(r.URL.Path)
	if streamName == "" {
		slog.Warn("invalid camera stream path", "path", r.URL.Path)
		http.Error(w, "Invalid camera stream path", http.StatusBadRequest)
		return
	}

	stream := streamManager.GetReader(streamName)
	defer streamManager.ReturnReader(streamName, stream)

	hijackStream(w, r, stream)

	/*
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Pragma", "no-cache")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		mw := multipart.NewWriter(w)
		mw.SetBoundary("frame")
		defer mw.Close()

		header := make(textproto.MIMEHeader)
		header.Add("Content-Type", "image/jpeg")

		for {
			select {
			case <-r.Context().Done():
				return
			case data, ok := <-stream:
				if !ok {
					return
				}

				imageData, err := base64.StdEncoding.DecodeString(string(data))
				if err != nil {
					slog.Error("failed to decode base64 image", "error", err)
					continue
				}

				pw, _ := mw.CreatePart(header)
				_, err = pw.Write(imageData)
				if err != nil {
					slog.Error("failed to write image data", "error", err)
					continue
				}

				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				} else {
					slog.Warn("flusher not supported")
				}
				slog.Info("eyes frame sent", "stream", streamName, "frame_size", len(imageData))
			}
		}
	*/
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
