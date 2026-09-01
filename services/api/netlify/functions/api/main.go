package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"

	"lifehub/services/api/internal/application"
	"lifehub/services/api/internal/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

var (
	initializationMu sync.Mutex
	apiProxy         *httpadapter.HandlerAdapter
)

func initialize() (*httpadapter.HandlerAdapter, error) {
	initializationMu.Lock()
	defer initializationMu.Unlock()
	if apiProxy != nil {
		return apiProxy, nil
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	settings, err := config.Load()
	if err != nil {
		logger.Error("API configuration invalid")
		return nil, err
	}
	app, err := application.New(context.Background(), settings, logger)
	if err != nil {
		// Production logs expose only the stage. Database URLs and auth material
		// must never be serialized by an upstream parser error.
		logger.Error("API initialization failed", "stage", application.Stage(err))
		return nil, err
	}
	apiProxy = httpadapter.New(app.Handler)
	return apiProxy, nil
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	proxy, err := initialize()
	if err != nil {
		return unavailableResponse(), nil
	}
	request.Path = normalizePath(request.Path)
	return proxy.ProxyWithContext(ctx, request)
}

func normalizePath(path string) string {
	const functionPrefix = "/.netlify/functions/api"
	if path == functionPrefix {
		return "/"
	}
	if strings.HasPrefix(path, functionPrefix+"/") {
		return strings.TrimPrefix(path, functionPrefix)
	}
	return path
}

func unavailableResponse() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: 503,
		Headers: map[string]string{
			"Cache-Control": "no-store",
			"Content-Type":  "application/json; charset=utf-8",
			"Retry-After":   "5",
		},
		Body: `{"error":{"code":"SERVICE_UNAVAILABLE","message":"Layanan sedang memulai. Coba lagi sebentar."}}`,
	}
}

func main() {
	lambda.Start(handler)
}
