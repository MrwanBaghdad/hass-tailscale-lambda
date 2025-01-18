package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"tailscale.com/tsnet"

	"github.com/aws/aws-lambda-go/lambda"
)

type LambdaHandler struct {
	BaseURL        string
	Debug          bool
	LongLivedToken string
	VerifySSL      bool
	Logger         *zap.Logger
	TSNetServer    *tsnet.Server
}

func NewLambdaHandler(tsNetServer *tsnet.Server) *LambdaHandler {
	baseURL := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if baseURL == "" {
		panic("Please set BASE_URL environment variable")
	}

	debug := os.Getenv("DEBUG") == "true"
	logger, err := zap.NewProduction()
	if debug {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	longLivedToken := os.Getenv("LONG_LIVED_ACCESS_TOKEN")
	verifySSL := os.Getenv("NOT_VERIFY_SSL") != "true"

	h := &LambdaHandler{
		BaseURL:        baseURL,
		Debug:          debug,
		LongLivedToken: longLivedToken,
		VerifySSL:      verifySSL,
		Logger:         logger,
	}

	if tsNetServer != nil {
		h.TSNetServer = tsNetServer
	}
	return h
}

func (h *LambdaHandler) HandleRequest(ctx context.Context, event map[string]interface{}) (map[string]interface{}, error) {
	h.Logger.Sugar().Infof("Event: %+v", event)

	// Extract directive
	directive, ok := event["directive"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("malformatted request - missing directive")
	}

	header, ok := directive["header"].(map[string]interface{})
	if !ok || header["payloadVersion"] != "3" {
		return nil, fmt.Errorf("only support payloadVersion == 3")
	}

	// Extract scope
	scope := h.extractScope(directive)
	if scope == nil {
		return nil, fmt.Errorf("malformatted request - missing endpoint.scope")
	}

	scopeType, _ := scope["type"].(string)
	if scopeType != "BearerToken" {
		return nil, fmt.Errorf("only support BearerToken")
	}

	token, _ := scope["token"].(string)
	if token == "" && h.Debug {
		token = h.LongLivedToken
	}

	client := h.createHTTPClient()

	// Serialize event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		h.Logger.Sugar().Errorf("Error serializing event: %v", err)
		return nil, fmt.Errorf("failed to serialize event")
	}

	// Make HTTP request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/alexa/smart_home", h.BaseURL), bytes.NewBuffer(eventJSON))
	if err != nil {
		h.Logger.Sugar().Errorf("Error creating request: %v", err)
		return nil, fmt.Errorf("internal server error")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		h.Logger.Sugar().Errorf("Error making HTTP request: %v", err)
		return nil, fmt.Errorf("internal server error")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		message := fmt.Sprintf("status code: %d", resp.StatusCode)
		h.Logger.Sugar().Warnf("Error response: %s", message)
		return nil, fmt.Errorf(message)
	}

	var responseBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		h.Logger.Sugar().Errorf("Error decoding response: %v", err)
		return nil, fmt.Errorf("error decoding response")
	}
	h.Logger.Sugar().Infof("Response: %+v", responseBody)

	return responseBody, nil
}

func (h *LambdaHandler) extractScope(directive map[string]interface{}) map[string]interface{} {
	if endpoint, ok := directive["endpoint"].(map[string]interface{}); ok {
		if scope, ok := endpoint["scope"].(map[string]interface{}); ok {
			return scope
		}
	}
	if payload, ok := directive["payload"].(map[string]interface{}); ok {
		if scope, ok := payload["grantee"].(map[string]interface{}); ok {
			return scope
		}
		if scope, ok := payload["scope"].(map[string]interface{}); ok {
			return scope
		}
	}
	return nil
}

func (h *LambdaHandler) errorType(statusCode int) string {
	if statusCode == 401 || statusCode == 403 {
		return "INVALID_AUTHORIZATION_CREDENTIAL"
	}
	return "INTERNAL_ERROR"
}

func (h *LambdaHandler) createHTTPClient() *http.Client {
	var client *http.Client
	if h.TSNetServer != nil {
		client = h.TSNetServer.HTTPClient()
		return client
	} else {
		client = &http.Client{}

	}
	client.Timeout = 10 * time.Second

	if !h.VerifySSL {
		// Skip SSL verification
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = transport
	}

	return client
}

func main() {
	var tsNetServer *tsnet.Server = nil
	if v := os.Getenv("TS_AUTHKEY"); v != "" {
		tsNetServer = &tsnet.Server{
			AuthKey: v,
		}
	}
	handler := NewLambdaHandler(tsNetServer)
	lambda.Start(handler.HandleRequest)
}
