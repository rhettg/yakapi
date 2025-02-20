package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rhettg/yakapi/internal/gds"
	"github.com/rhettg/yakapi/internal/mw"
	"github.com/rhettg/yakapi/internal/stream"
	"github.com/rhettg/yakapi/internal/telemetry"
	"github.com/spf13/cobra"
)

var startTime time.Time

func init() {
	startTime = time.Now()
}

func DoServer(cmd *cobra.Command, args []string) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "yakapi_uptime_seconds",
		Help: "The uptime of the yakapi service",
	}, func() float64 {
		return float64(time.Since(startTime).Seconds())
	})

	port := os.Getenv("YAKAPI_PORT")
	if port == "" {
		port = "8080"
	}

	mux := setupServer()

	streamManager = stream.NewManager()

	if os.Getenv("YAKAPI_GDS_API_URL") != "" {
		go func() {
			c := gds.New(os.Getenv("YAKAPI_GDS_API_URL"))
			for {
				err := doGDSCI(context.Background(), c, streamManager)
				if err != nil {
					slog.Error("error running GDS CI", "error", err)
				}
				time.Sleep(10 * time.Second)
			}
		}()

		source := make(chan telemetry.Data, 10)
		go func() {
			err := fetchTelemetryData(context.Background(), source)
			if err != nil {
				slog.Error("error fetching telemetry data from Stream", "error", err)
			}
		}()

		go func() {
			c := gds.New(os.Getenv("YAKAPI_GDS_API_URL"))

			cachedTd := make(map[string]interface{})

			var lastSSB time.Time
			var lastSent time.Time

			for {
				var td telemetry.Data
				select {
				case td = <-source:
				case <-time.After(1 * time.Second):
					td = telemetry.Data{}
				}

				for key, value := range td {
					// Type assert both values before comparison
					switch v := value.(type) {
					case string, int, int64, float64, bool:
						if cachedValue, ok := cachedTd[key]; ok && cachedValue == v {
							delete(td, key)
						} else {
							cachedTd[key] = v
						}
					default:
						// Just skip caching complex types
					}
				}

				if time.Since(lastSSB) > 10*time.Second {
					td["seconds_since_boot"] = int(time.Since(startTime).Seconds())
					lastSSB = time.Now()
				}

				if len(td) > 0 {
					// Throttle
					if !lastSent.IsZero() && time.Since(lastSent) < 1*time.Second {
						time.Sleep(1 * time.Second)
					}

					err := c.SendTelemetry(context.Background(), td)
					if err != nil {
						slog.Error("error uploading telemetry to GDS", "error", err)
					} else {
						slog.Info("uploaded telemetry to GDS", "data", td)
					}

					lastSent = time.Now()
				}
			}
		}()
	}

	telemetrySource := make(chan telemetry.Data)
	go func() {
		err := fetchTelemetryData(context.Background(), telemetrySource)
		if err != nil {
			slog.Error("error fetching telemetry data from Stream", "error", err)
		}
	}()

	go func() {
		err := telemetry.Run(context.Background(), telemetrySource)
		if err != nil {
			slog.Error("error running telemetry", "error", err)
		}
	}()

	go func() {
		port := 8765
		sfcMux := setupSFCserver()
		slog.Info("starting sfc", "version", "1.0.0", "port", port, "build", Revision)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), sfcMux)
		if err != nil {
			slog.Error("error from sfc ListenAndServe", "error", err)
		}
	}()

	slog.Info("starting", "version", "1.0.0", "port", port, "build", Revision)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
	if err != nil {
		slog.Error("error from ListenAndServe", "error", err)
	}
}

func setupServer() *http.ServeMux {
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

	mux := http.NewServeMux()
	mux.Handle("/", wrapper(http.HandlerFunc(home)))
	mux.Handle("/v1", wrapper(http.HandlerFunc(homev1)))
	mux.Handle("/v1/me", wrapper(http.HandlerFunc(me)))
	mux.Handle("/v1/eyes/", http.HandlerFunc(handleStreamEyes))
	mux.Handle("/v1/stream/", wrapper(http.HandlerFunc(handleStream)))
	mux.Handle("/metrics", wrapper(promhttp.Handler()))
	mux.Handle("/eyes", wrapper(http.HandlerFunc(eyes)))

	return mux
}
