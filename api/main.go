package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
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

func homev1(w http.ResponseWriter, r *http.Request) {
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
		},
	}

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error encoding response: %v\n", err)
		return
	}
}

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/v1", homev1)

	http.ListenAndServe(":8090", nil)
}
