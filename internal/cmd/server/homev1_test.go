package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHomeV1Handler(t *testing.T) {
	// Save the original environment variables
	originalName := os.Getenv("YAKAPI_NAME")
	originalProject := os.Getenv("YAKAPI_PROJECT")
	originalOperator := os.Getenv("YAKAPI_OPERATOR")

	// Set up test environment variables
	os.Setenv("YAKAPI_NAME", "Test YakAPI")
	os.Setenv("YAKAPI_PROJECT", "https://test-project.com")
	os.Setenv("YAKAPI_OPERATOR", "https://test-operator.com")

	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", "/v1", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Call the handler function directly
	handler := http.HandlerFunc(homev1)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")

	// Check the Content-Type header
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "handler returned wrong Content-Type")

	// Parse the response body
	var response struct {
		Name      string     `json:"name"`
		Revision  string     `json:"revision"`
		UpTime    int64      `json:"uptime"`
		Resources []resource `json:"resources"`
	}

	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err, "error parsing response body")

	// Check the response body
	assert.Equal(t, "Test YakAPI", response.Name)
	assert.NotEmpty(t, response.Revision)
	assert.GreaterOrEqual(t, response.UpTime, int64(0))

	// Check the resources
	expectedResources := []resource{
		{Name: "metrics", Ref: "/metrics"},
		{Name: "eyes", Ref: "/eyes"},
		{Name: "eyes-api", Ref: "/v1/eyes/"},
		{Name: "stream", Ref: "/v1/stream/"},
		{Name: "project", Ref: "https://test-project.com"},
		{Name: "operator", Ref: "https://test-operator.com"},
	}
	assert.ElementsMatch(t, expectedResources, response.Resources)

	// Restore original environment variables
	os.Setenv("YAKAPI_NAME", originalName)
	os.Setenv("YAKAPI_PROJECT", originalProject)
	os.Setenv("YAKAPI_OPERATOR", originalOperator)
}
