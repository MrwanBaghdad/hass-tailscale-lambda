package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Mock HTTP server to simulate the backend API
func mockServer(responseCode int, responseBody map[string]interface{}) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(responseCode)
		json.NewEncoder(w).Encode(responseBody)
	})
	return httptest.NewServer(handler)
}

// Test for HandleRequest with Alexa Discovery event
func TestHandleRequest_Discovery(t *testing.T) {
	// Set up environment variables
	os.Setenv("BASE_URL", "http://localhost")
	os.Setenv("DEBUG", "true")
	os.Setenv("LONG_LIVED_ACCESS_TOKEN", "mock-token")
	os.Setenv("NOT_VERIFY_SSL", "true")

	// Mock response from backend API
	mockResponse := map[string]interface{}{
		"event": map[string]interface{}{
			"header": map[string]interface{}{
				"namespace":      "Alexa.Discovery",
				"name":           "Discover.Response",
				"payloadVersion": "3",
				"messageId":      "1bd5d003-31b9-476f-ad03-71d471922820",
			},
			"payload": map[string]interface{}{
				"endpoints": []map[string]interface{}{
					{
						"endpointId":        "device-001",
						"friendlyName":      "Sample Device",
						"description":       "A sample smart device",
						"displayCategories": []string{"SWITCH"},
						"capabilities": []map[string]interface{}{
							{
								"type":      "AlexaInterface",
								"interface": "Alexa.PowerController",
								"version":   "3",
								"properties": map[string]interface{}{
									"supported": []map[string]string{
										{"name": "powerState"},
									},
									"proactivelyReported": true,
									"retrievable":         true,
								},
							},
						},
					},
				},
			},
		},
	}
	mockServer := mockServer(http.StatusOK, mockResponse)
	defer mockServer.Close()

	// Update BASE_URL to point to the mock server
	os.Setenv("BASE_URL", mockServer.URL)

	// Initialize the handler
	handler := NewLambdaHandler()

	// Define the Discovery event
	event := map[string]interface{}{
		"directive": map[string]interface{}{
			"header": map[string]interface{}{
				"namespace":      "Alexa.Discovery",
				"name":           "Discover",
				"payloadVersion": "3",
				"messageId":      "1bd5d003-31b9-476f-ad03-71d471922820",
			},
			"payload": map[string]interface{}{
				"scope": map[string]interface{}{
					"type": "BearerToken",
				},
			},
		},
	}

	// Invoke the handler
	response, err := handler.HandleRequest(context.Background(), event)
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	// Validate the response
	responseJSON, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to serialize response: %v", err)
	}

	// Extract the actual and expected endpoints for comparison
	expectedEndpoints := mockResponse["event"].(map[string]interface{})["payload"].(map[string]interface{})["endpoints"].([]map[string]interface{})
	actualEndpoints := response["event"].(map[string]interface{})["payload"].(map[string]interface{})["endpoints"].([]interface{})

	if len(actualEndpoints) != len(expectedEndpoints) {
		t.Errorf("Expected %d endpoints, got %d", len(expectedEndpoints), len(actualEndpoints))
	}

	t.Logf("Response: %s", responseJSON)
}
