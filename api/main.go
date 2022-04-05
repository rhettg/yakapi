package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	mw "github.com/rhettg/batteries/api/internal/mw"
)

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "/v1")
	w.WriteHeader(http.StatusTemporaryRedirect)
}

type resource struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

var startTime time.Time

func init() {
	startTime = time.Now()
}

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "batteries_processed_ops_total",
		Help: "The total number of processed requests",
	})
)

func homev1(w http.ResponseWriter, r *http.Request) {
	opsProcessed.Inc()

	w.Header().Set("Content-Type", "application/json")
	resp := struct {
		Name      string     `json:"name"`
		UpTime    int64      `json:"uptime"`
		Resources []resource `json:"resources"`
	}{
		Name:   "Batteries Not Included",
		UpTime: int64(time.Since(startTime).Seconds()),
		Resources: []resource{
			{Name: "operator", Ref: "https://t.me/rhettg"},
			{Name: "project", Ref: "https://github.com/rhettg/batteries"},
			{Name: "metrics", Ref: "/metrics"},
		},
	}

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error encoding response: %v\n", err)
		return
	}
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync() // flushes buffer, if any
	log := logger.Sugar()
	log.Infow("starting", "version", "1.0.0")

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "batteries_uptime_seconds",
		Help: "The uptime of the batteries service",
	}, func() float64 {
		return float64(time.Since(startTime).Seconds())
	})

	logmw := mw.New(logger)

	http.Handle("/", logmw(http.HandlerFunc(home)))
	http.Handle("/v1", logmw(http.HandlerFunc(homev1)))
	http.Handle("/metrics", logmw(promhttp.Handler()))

	http.ListenAndServe(":8080", nil)
}
