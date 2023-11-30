package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os/exec"

	"github.com/aws/aws-lambda-go/events"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/web"

	"github.com/whywaita/myshoes-serverless-aws/pkg/sqs"
)

func init() {
	if err := exec.Command("cp", "-a", "./shoes-ecs-task", "/tmp/shoes-ecs-task").Run(); err != nil {
		log.Panicf("failed to copy shoes-ecs-task: %+v", err)
	}
	config.Load()
	if err := gh.InitializeCache(config.Config.GitHub.AppID, config.Config.GitHub.PEMByte); err != nil {
		log.Panicf("failed to create a cache: %+v", err)
	}
}

func HandleRequest(ctx context.Context, req events.LambdaFunctionURLRequest) (*events.LambdaFunctionURLResponse, error) {
	queueURL, region, err := sqs.LoadSQSQueueURL()
	if err != nil {
		return nil, fmt.Errorf("failed to sqs.LoadSQSQueueURL: %w", err)
	}
	sqsService, err := sqs.NewSQS(ctx, region, queueURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqs.NewSQS: %w", err)
	}

	newReq, err := http.NewRequest(http.MethodPost, "/dummy-path", bytes.NewBufferString(req.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create http.Request: %w", err)
	}

	for key, value := range req.Headers {
		newReq.Header.Set(revertOriginalHeader(key), value)
	}

	rec := httptest.NewRecorder()
	web.HandleGitHubEvent(rec, newReq, sqsService)

	return &events.LambdaFunctionURLResponse{
		StatusCode:      rec.Code,
		Headers:         nil,
		Body:            rec.Body.String(),
		IsBase64Encoded: false,
		Cookies:         nil,
	}, nil
}

func revertOriginalHeader(key string) string {
	switch key {
	case "x-hub-signature":
		return "X-Hub-Signature"
	case "x-hub-signature-256":
		return "X-Hub-Signature-256"
	case "x-github-event":
		return "X-Github-Event"
	case "content-type":
		return "Content-Type"
	}

	return key
}

func main() {
	lambda.Start(HandleRequest)
}
