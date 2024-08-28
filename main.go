package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"gitlab.com/greyxor/slogor"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rhettg/batteries/yakapi/internal/ci"
	"github.com/rhettg/batteries/yakapi/internal/gds"
	"github.com/rhettg/batteries/yakapi/internal/mw"
	"github.com/rhettg/batteries/yakapi/internal/stream"
	"github.com/rhettg/batteries/yakapi/internal/telemetry"
	"tailscale.com/client/tailscale"
)

var (
	//go:embed assets/*
	assets        embed.FS
	rdb           *redis.Client
	startTime     time.Time
	revision      = "unknown"
	streamManager *stream.Manager
)

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
	w.Write(content)
}

type resource struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

func init() {
	startTime = time.Now()
}

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

func errorResponse(w http.ResponseWriter, respErr error, statusCode int) error {
	resp := struct {
		Error string `json:"error"`
	}{Error: respErr.Error()}

	return sendResponse(w, resp, statusCode)
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
	whois, err := tailscale.WhoIs(r.Context(), r.RemoteAddr)
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

	msgID, err := ci.Accept(r.Context(), rdb, req.Command)
	if err != nil {
		slog.Error("failed accepting ci command", "error", err)
		errorResponse(w, err, http.StatusBadRequest)
		return
	}

	var result, errorResult string
	if waitForResult {
		waitCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		result, err = ci.FetchResult(waitCtx, rdb, msgID)
		cancel()

		if err != nil {
			slog.Error("failed fetching ci command result", "error", err)
			errorResult = err.Error()
		}
	}

	resp := struct {
		ID     string `json:"id"`
		Result string `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}{
		ID:     string(msgID),
		Result: result,
		Error:  errorResult,
	}

	err = sendResponse(w, resp, http.StatusAccepted)
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
		msgID, err := ci.Accept(ctx, rdb, req.Command)
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
	w.Write(content)
}

func homev1(w http.ResponseWriter, r *http.Request) {
	resp := struct {
		Name      string     `json:"name"`
		Revision  string     `json:"revision"`
		UpTime    int64      `json:"uptime"`
		Resources []resource `json:"resources"`
	}{
		Name:     "YakAPI Server",
		Revision: revision,
		UpTime:   int64(time.Since(startTime).Seconds()),
		Resources: []resource{
			{Name: "metrics", Ref: "/metrics"},
			{Name: "ci", Ref: "/v1/ci"},
			{Name: "cam", Ref: "/v1/cam/capture"},
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

func fetchTelemetryData(ctx context.Context, rdb *redis.Client) (map[string]interface{}, error) {
	// Fetch the telemetry data from Redis
	result, err := rdb.XRange(ctx, "yakapi:telemetry", "-", "+").Result()
	if err != nil {
		return nil, err
	}

	// Convert the result to a map[string]interface{}
	telemetryData := make(map[string]interface{})
	for _, message := range result {
		for key, value := range message.Values {
			telemetryData[key] = value
		}
	}

	return telemetryData, nil
}

func parseStreamPath(path string) string {
	remaining, found := strings.CutPrefix(path, "/stream/")
	if found {
		return remaining
	}
	return ""
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	streamName := parseStreamPath(r.URL.Path)
	if streamName == "" {
		http.Error(w, "Invalid stream path", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		slog.Info("stream out", "stream", streamName)
		w.Header().Set("Transfer-Encoding", "chunked")
		err := stream.StreamOut(r.Context(), w, streamName, streamManager)
		if err != nil {
			http.Error(w, "Error streaming out", http.StatusInternalServerError)
			return
		}
		slog.Info("stream out complete", "stream", streamName)
	case http.MethodPost:
		slog.Info("stream in", "stream", streamName)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}

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

func main() {
	slog.SetDefault(slog.New(slogor.NewHandler(os.Stderr, slogor.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.Stamp,
	})))

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "yakapi_uptime_seconds",
		Help: "The uptime of the yakapi service",
	}, func() float64 {
		return float64(time.Since(startTime).Seconds())
	})

	loadDotEnv()

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yakapi_requests_total",
			Help: "A counter for requests to the server.",
		},
		[]string{"code", "method"},
	)

	prometheus.MustRegister(counter)

	var wrapper func(http.Handler) http.Handler
	logmw := mw.NewLoggerMiddleware(slog.Default())
	wrapper = func(next http.Handler) http.Handler {
		return promhttp.InstrumentHandlerCounter(counter, logmw(next))
	}

	http.Handle("/", wrapper(http.HandlerFunc(home)))
	http.Handle("/v1", wrapper(http.HandlerFunc(homev1)))
	http.Handle("/v1/me", wrapper(http.HandlerFunc(me)))
	http.Handle("/v1/ci", wrapper(http.HandlerFunc(handleCI)))
	http.Handle("/v1/cam/capture", wrapper(http.HandlerFunc(handleCamCapture)))
	http.Handle("/metrics", wrapper(promhttp.Handler()))
	http.Handle("/eyes", wrapper(http.HandlerFunc(eyes)))
	http.Handle("/stream/", wrapper(http.HandlerFunc(handleStream)))

	port := os.Getenv("YAKAPI_PORT")
	if port == "" {
		port = "8080"
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		slog.Error("failed loading build info")
	}

	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
			break
		}
	}

	redis_url := "127.0.0.1:6379"
	if os.Getenv("YAKAPI_REDIS_URL") != "" {
		redis_url = os.Getenv("YAKAPI_REDIS_URL")
	}

	slog.Info("configuring redis", "url", redis_url)
	rdb = redis.NewClient(&redis.Options{
		Addr: redis_url,
	})

	ping := rdb.Ping(context.Background())
	if err := ping.Err(); err != nil {
		slog.Error("failed connecting to redis", "error", err)
		rdb = nil
	} else {
		slog.Info("redis connected")
	}

	streamManager = stream.NewManager()

	if os.Getenv("YAKAPI_GDS_API_URL") != "" {
		go func() {
			c := gds.New(os.Getenv("YAKAPI_GDS_API_URL"))
			for {
				err := doGDSCI(context.Background(), c)
				if err != nil {
					slog.Error("error running GDS CI", "error", err)
				}
				time.Sleep(10 * time.Second)
			}
		}()

		go func() {
			c := gds.New(os.Getenv("YAKAPI_GDS_API_URL"))

			cachedTd := make(map[string]interface{})

			var lastSSB time.Time

			for {
				td, err := fetchTelemetryData(context.Background(), rdb)
				if err != nil {
					slog.Error("error fetching telemetry data from Redis", "error", err)
					time.Sleep(10 * time.Second)
					continue
				}

				for key, value := range td {
					if cachedTd[key] != value {
						cachedTd[key] = value
					} else {
						delete(td, key)
					}
				}

				if time.Since(lastSSB) > 10*time.Second {
					td["seconds_since_boot"] = int(time.Since(startTime).Seconds())
					lastSSB = time.Now()
				}

				if len(td) > 0 {
					err = c.SendTelemetry(context.Background(), td)
					if err != nil {
						slog.Error("error uploading telemetry to GDS", "error", err)
					} else {
						slog.Info("uploaded telemetry to GDS", "data", td)
					}
				}

				time.Sleep(1 * time.Second)
			}
		}()
	}

	go func() {
		if rdb == nil {
			return
		}
		err := telemetry.Run(context.Background(), rdb, "yakapi:telemetry")
		if err != nil {
			slog.Error("error running telemetry", "error", err)
		}
	}()

	slog.Info("starting", "version", "1.0.0", "port", port, "build", revision)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		slog.Error("error from ListenAndServer", "error", err)
	}
}
