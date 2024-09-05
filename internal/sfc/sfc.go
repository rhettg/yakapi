package sfc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Define device information
var (
	wsName  = "GalaxyRVR"
	wsType  = "GalaxyRVR"
	wsCheck = "SC"
)

type CheckInfo struct {
	Name  string `json:"Name"`
	Type  string `json:"Type"`
	Check string `json:"Check"`
	Video string `json:"video"`
}

func generateCheckInfo(r *http.Request) []byte {
	checkInfo := CheckInfo{
		Name:  wsName,
		Type:  wsType,
		Check: wsCheck,
		Video: fmt.Sprintf("http://%s:%d/mjpg", r.Host, 8765),
	}
	jsonData, err := json.Marshal(checkInfo)
	if err != nil {
		slog.Error("Error marshaling JSON:", err)
		return []byte{}
	}
	return jsonData
}

type wsMessage struct {
	Type    int
	Message []byte
}

type ControlValue struct {
	Region string
	Value  interface{}
}

func HandleWebSocket(cvc chan ControlValue, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade connection:", err)
		return
	}
	defer conn.Close()

	slog.Debug("upgraded to websocket")

	checkInfo := generateCheckInfo(r)

	// Send check info twice with a small delay
	if err := conn.WriteMessage(websocket.TextMessage, checkInfo); err != nil {
		slog.Error("Failed to send message:", err)
		return
	}
	time.Sleep(100 * time.Millisecond)
	if err := conn.WriteMessage(websocket.TextMessage, checkInfo); err != nil {
		slog.Error("Failed to send message:", err)
		return
	}

	msgs := make(chan wsMessage)

	go func(ctx context.Context) {
		// Handle incoming messages
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				slog.Error("WebSocket read error:", err)
				break
			}
			msgs <- wsMessage{Type: mt, Message: message}
			if ctx.Err() != nil {
				break
			}
		}
	}(r.Context())

	lastPong := time.Now()
	for {
		select {
		case m := <-msgs:
			slog.Debug("Received message", "type", m.Type, "message", string(m.Message))
			if m.Type == websocket.TextMessage {
				data := make(map[string]interface{})
				err := json.Unmarshal(m.Message, &data)
				if err != nil {
					slog.Error("Error unmarshaling JSON:", err)
					continue
				}
				for k, v := range data {
					if k == "" {
						continue
					}
					if k == "Len" {
						continue
					}

					slog.Debug("Control Value", "region", k, "value", v)
					cvc <- ControlValue{Region: k, Value: v}
				}
			}
			lastPong = time.Now()
		case <-time.After(20 * time.Millisecond):
			if time.Since(lastPong) > 200*time.Millisecond {
				lastPong = time.Now()
				slog.Debug("Sending ping")
				err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("ping %d", time.Now().Unix())))
				if err != nil {
					slog.Error("Socket write error:", err)
					return
				}
			}
		case <-r.Context().Done():
			break
		}
	}
}

func HandleVideo(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Serving video")
	select {
	case <-r.Context().Done():
		return
	}
}
