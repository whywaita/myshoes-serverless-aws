package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"

	"github.com/aws/aws-lambda-go/events"

	"github.com/whywaita/myshoes/pkg/datastore"

	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/starter"
	"github.com/whywaita/myshoes/pkg/starter/safety/unlimited"

	"github.com/aws/aws-lambda-go/lambda"
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

func HandleRequest(ctx context.Context, event events.SQSEvent) error {
	queueURL, region, err := sqs.LoadSQSQueueURL()
	if err != nil {
		return fmt.Errorf("failed to sqs.LoadSQSQueueURL: %w", err)
	}
	sqsService, err := sqs.NewSQS(ctx, region, queueURL)
	if err != nil {
		return fmt.Errorf("failed to create sqs.NewSQS: %w", err)
	}

	s := starter.New(sqsService, unlimited.Unlimited{}, config.Config.RunnerVersion, make(chan struct{}, 1))

	wg := sync.WaitGroup{}

	for _, record := range event.Records {
		wg.Add(1)
		j, err := sqs.ConvertJob([]byte(record.Body))
		if err != nil {
			log.Printf("failed to sqs.ConvertJob (message id: %s): %+v", record.MessageId, err)
			continue
		}

		go func(job datastore.Job) {
			defer wg.Done()
			if err := s.ProcessJob(ctx, job); err != nil {
				log.Printf("failed to s.ProcessJob: %+v", err)
			}
		}(*j)
	}

	wg.Wait()

	return nil
}

func main() {
	lambda.Start(HandleRequest)
}
