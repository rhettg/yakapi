package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	wsName   = "GalaxyRVR"
	wsType   = "GalaxyRVR"
	wsCheck  = "SC"
	videoURL = ""
)

type CheckInfo struct {
	Name  string `json:"Name"`
	Type  string `json:"Type"`
	Check string `json:"Check"`
	Video string `json:"video"`
}

func generateCheckInfo() []byte {
	checkInfo := CheckInfo{
		Name:  wsName,
		Type:  wsType,
		Check: wsCheck,
		Video: videoURL,
	}
	jsonData, err := json.Marshal(checkInfo)
	if err != nil {
		log.Println("Error marshaling JSON:", err)
		return []byte{}
	}
	return jsonData
}

type wsMessage struct {
	Type    int
	Message []byte
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	defer conn.Close()

	log.Println("upgraded to websocket")

	checkInfo := generateCheckInfo()

	// Send check info twice with a small delay
	if err := conn.WriteMessage(websocket.TextMessage, checkInfo); err != nil {
		log.Println("Failed to send message:", err)
		return
	}
	time.Sleep(100 * time.Millisecond)
	if err := conn.WriteMessage(websocket.TextMessage, checkInfo); err != nil {
		log.Println("Failed to send message:", err)
		return
	}

	msgs := make(chan wsMessage)

	go func(ctx context.Context) {
		// Handle incoming messages
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
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
			log.Printf("Received message: (%d) %s", m.Type, m.Message)
			if m.Type == websocket.TextMessage {
				data := make(map[string]interface{})
				err := json.Unmarshal(m.Message, &data)
				if err != nil {
					log.Println("Error unmarshaling JSON:", err)
					continue
				}
				for k, v := range data {
					if k == "" {
						continue
					}
					if k == "Len" {
						continue
					}

					log.Printf("Key: %s, Value: %v", k, v)
				}
			}
			lastPong = time.Now()
		case <-time.After(20 * time.Millisecond):
			if time.Since(lastPong) > 200*time.Millisecond {
				lastPong = time.Now()
				log.Println("Sending ping")
				err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("ping %d", time.Now().Unix())))
				if err != nil {
					log.Println("Socket write error:", err)
					return
				}
			}
		case <-r.Context().Done():
			break
		}
	}
}

func handleVideo(w http.ResponseWriter, r *http.Request) {
	log.Println("Serving video")
	select {
	case <-r.Context().Done():
		return
	}
}

func main() {
	http.HandleFunc("/", handleWebSocket)
	http.HandleFunc("/mjpg", handleVideo)

	videoURL = "http://100.92.177.18:8765/mjpg"
	log.Println("WebSocket server starting on :8765")
	log.Fatal(http.ListenAndServe(":8765", nil))
}
