package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/rhettg/yakapi/internal/sfc"
)

type sfcValue struct {
	Value interface{} `json:"value"`
}

func sfcWriteControlValues(ctx context.Context, cv chan sfc.ControlValue) error {
	for {
		select {
		case cv, ok := <-cv:
			if !ok {
				return nil
			}

			slog.Debug("sfc control value", "region", cv.Region, "value", cv.Value)
			streamName := fmt.Sprintf("sfc-control:%s", cv.Region)
			s := streamManager.GetWriter(streamName)
			value, err := json.Marshal(sfcValue{Value: cv.Value})
			if err != nil {
				slog.Error("error marshaling sfc control value", "error", err)
				continue
			}
			s <- value
			streamManager.ReturnWriter(streamName)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func sfcReadControlValues(ctx context.Context, cv chan sfc.ControlValue) error {
	var wg sync.WaitGroup

	for _, r := range sfc.AllRegions {
		wg.Add(1)
		go func(region sfc.Region) {
			defer wg.Done()
			ch := streamManager.GetReader(fmt.Sprintf("sfc-control-set:%s", region))
			defer streamManager.ReturnReader(fmt.Sprintf("sfc-control-set:%s", region), ch)

			for {
				select {
				case data, ok := <-ch:
					if !ok {
						slog.Warn("Channel closed", "region", region)
						return
					}
					slog.Debug("Received control value", "region", region, "data", string(data))
					fv, err := strconv.ParseFloat(string(data), 64)
					if err != nil {
						slog.Debug("not a float")
						cv <- sfc.ControlValue{Region: region, Value: data}
						continue
					}

					cv <- sfc.ControlValue{Region: region, Value: fv}
				case <-ctx.Done():
					return
				}
			}
		}(r)
	}

	wg.Wait()
	return ctx.Err()
}

func setupSFCserver() *http.ServeMux {
	mux := http.NewServeMux()

	/*
		var wrapper func(http.Handler) http.Handler
		logmw := mw.NewLoggerMiddleware(slog.Default())
		wrapper = func(next http.Handler) http.Handler {
			return logmw(next)
		}
	*/

	cvIn := make(chan sfc.ControlValue)
	go func() {
		if err := sfcReadControlValues(context.Background(), cvIn); err != nil {
			slog.Error("Error in sfcReadControlValues", "error", err)
		}
	}()

	cvOut := make(chan sfc.ControlValue)
	go func() {
		if err := sfcWriteControlValues(context.Background(), cvOut); err != nil {
			slog.Error("Error in sfcWriteControlValues", "error", err)
		}
	}()

	handleWebSocket := func(w http.ResponseWriter, r *http.Request) {
		sfc.HandleWebSocket(cvIn, cvOut, w, r)
	}

	mux.Handle("/", http.HandlerFunc(handleWebSocket))
	mux.Handle("/mjpg", http.HandlerFunc(sfc.HandleVideo))

	return mux
}
