package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ONSdigital/dis-bundle-api/sdk"
	"github.com/ONSdigital/dis-bundle-scheduler/config"
	"github.com/ONSdigital/dis-bundle-scheduler/publisher"
	"github.com/ONSdigital/log.go/v2/log"
	"github.com/pkg/errors"
)

const serviceName = "dis-bundle-scheduler"

var (
	// BuildTime represents the time in which the service was built
	BuildTime string
	// GitCommit represents the commit (SHA-1) hash of the service that is running
	GitCommit string
	// Version represents the version of the service that is running
	Version string
)

func main() {
	log.Namespace = serviceName
	ctx := context.Background()

	if err := run(ctx); err != nil {
		log.Fatal(ctx, "fatal runtime error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	// Read config
	cfg, err := config.Get()
	if err != nil {
		return errors.Wrap(err, "unable to retrieve service configuration")
	}
	log.Info(ctx, "config on startup", log.Data{"config": cfg, "build_time": BuildTime, "git-commit": GitCommit})

	// Create services
	bundleAPIClient := sdk.New(cfg.BundlesAPIUrl)
	clList := publisher.NewClientList(bundleAPIClient)
	publish, err := publisher.CreatePublisher(cfg, *clList)

	if err != nil {
		return errors.Wrap(err, "unable to instantiate publisher")
	}

	// Run the publisher in the background, using a result channel and an error channel for fatal errors
	errChan := make(chan error, 1)
	resultChan := make(chan *publisher.PublishResult, 1)
	go func() {
		result, err := publish.Run(ctx)
		if err != nil {
			errChan <- err
		}
		resultChan <- result
	}()

	// blocks until completion, an os interrupt or a fatal error occurs
	select {
	case err := <-errChan:
		log.Error(ctx, "publisher error received", err)
		return err
	case sig := <-signals:
		log.Info(ctx, "os signal received", log.Data{"signal": sig})
	case result := <-resultChan:
		log.Info(ctx, "publish scheduled bundles result", log.Data{"Result": result})
		if !result.Success {
			if err != nil {
				log.Error(ctx, "unable to send notification of result", err)
				return err
			}
		}
		log.Info(ctx, "publish scheduled bundles complete")
	}
	return nil // TODO close down the checker and confirm task completion state (err or nil)
}
