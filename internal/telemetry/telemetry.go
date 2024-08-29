package telemetry

import (
	"context"
	"log/slog"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Mutex for concurrent access to the metrics map
	metricsMutex sync.Mutex
	// Map to store dynamic gauges by key
	dynamicMetrics map[string]prometheus.Gauge
)

func init() {
	dynamicMetrics = make(map[string]prometheus.Gauge)
}

// Function to create or update a metric
func setMetric(key, value string) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	gauge, exists := dynamicMetrics[key]
	if !exists {
		slog.Info("establishing new telemetry metric", "name", key)
		// Metric does not exist, create it
		gauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "telemetry_" + key,
			Help: "",
		})
		prometheus.MustRegister(gauge)
		dynamicMetrics[key] = gauge
	}

	slog.Debug("setting metric for telemetry", key, value)
	if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
		gauge.Set(floatValue)
	}
}

type Data map[string]interface{}

func Run(ctx context.Context, source chan Data) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case data, ok := <-source:
			if !ok {
				slog.Warn("telemetry stream closed")
				return nil
			}

			slog.Debug("processing telemetry stream")
			for key, value := range data {
				valueStr, ok := value.(string)
				if ok {
					setMetric(key, valueStr)
				} else {
					slog.Warn("telemetry value is not a string", "key", key, "value", value)
				}
			}
		}
	}
}
