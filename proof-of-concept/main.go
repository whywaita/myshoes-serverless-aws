package main

import (
	"context"
	"fmt"
	"log"

	"github.com/whywaita/myshoes-serverless-aws/pkg/sqs"

	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/starter"
	"github.com/whywaita/myshoes/pkg/starter/safety/unlimited"
	"github.com/whywaita/myshoes/pkg/web"

	"golang.org/x/sync/errgroup"
)

func init() {
	config.Load()
	if err := gh.InitializeCache(config.Config.GitHub.AppID, config.Config.GitHub.PEMByte); err != nil {
		log.Panicf("failed to create a cache: %+v", err)
	}
}

func main() {
	myshoes, err := newShoes()
	if err != nil {
		log.Fatalln(err)
	}

	if err := myshoes.Run(); err != nil {
		log.Fatalln(err)
	}
}

type myShoes struct {
	ds    datastore.Datastore
	start *starter.Starter
}

// newShoes create myshoes.
func newShoes() (*myShoes, error) {
	notifyEnqueueCh := make(chan struct{}, 1)

	queueURL, region, err := sqs.LoadSQSQueueURL()
	if err != nil {
		return nil, fmt.Errorf("failed to sqs.LoadSQSQueueURL: %w", err)
	}

	ds, err := sqs.NewSQS(context.Background(), region, queueURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqs.NewSQS: %w", err)
	}

	unlimit := unlimited.Unlimited{}
	s := starter.New(ds, unlimit, config.Config.RunnerVersion, notifyEnqueueCh)

	return &myShoes{
		ds:    ds,
		start: s,
	}, nil
}

// Run start services.
func (m *myShoes) Run() error {
	eg, ctx := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		if err := web.Serve(ctx, m.ds); err != nil {
			logger.Logf(false, "failed to web.Serve: %+v", err)
			return fmt.Errorf("failed to serve: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		if err := m.start.Loop(ctx); err != nil {
			logger.Logf(false, "failed to starter manager: %+v", err)
			return fmt.Errorf("failed to starter loop: %w", err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to wait errgroup: %w", err)
	}

	return nil
}
