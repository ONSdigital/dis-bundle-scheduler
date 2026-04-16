package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ONSdigital/dis-bundle-api/models"
	"github.com/ONSdigital/dis-bundle-api/sdk"
	"github.com/ONSdigital/dis-bundle-api/sdk/errors"
	"github.com/ONSdigital/dis-bundle-scheduler/config"

	"github.com/ONSdigital/log.go/v2/log"
)

type PublishBundleResult struct {
	BundleID string
	Success  bool
	Error    errors.Error
}

type PublishResult struct {
	Results []PublishBundleResult
	Success bool
}

// ClientList is a struct obj of all the clients the service is dependent on
type ClientList struct {
	BundleClient sdk.Clienter
}

// NewClientList returns a new ClientList obj with all available clients
func NewClientList(bundle sdk.Clienter) *ClientList {
	return &ClientList{
		BundleClient: bundle,
	}
}

type BundlePublisher interface {
	Run(ctx context.Context) (*PublishResult, error)
}

// Publisher is the main logic/orchestrator of the application.
type Publisher struct {
	bundlesClient ClientList
	config        *config.Configuration
}

func CreatePublisher(cfg *config.Configuration, clientList ClientList) (BundlePublisher, error) {
	return &Publisher{
		bundlesClient: clientList,
		config:        cfg,
	}, nil
}

func (p *Publisher) runPublicationProcess(ctx context.Context, headers sdk.Headers, bundleId string, ch chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	logData := log.Data{"bundle_id": bundleId}
	/// GetBundles does not return the etags for the bundles as it is returned in the header value, so a GetBundle request is required
	bundle, err := p.bundlesClient.BundleClient.GetBundle(ctx, headers, bundleId)
	if err != nil {
		log.Error(ctx, "Error getting bundle info, moving to next item", err, logData)
		return
	} else {
		var bundleObj models.Bundle
		err := json.Unmarshal(bundle.Body, &bundleObj)
		if err != nil {
			log.Error(ctx, "Error unmarshalling bundle info, moving to next item", err, logData)
			return
		} else if bundleObj.State == models.BundleStateApproved {
			// Ensure the bundle is in the approved state
			headers.IfMatch = bundle.Headers.Get("Etag")
			_, err := p.bundlesClient.BundleClient.PutBundleState(ctx, headers, bundleId, models.BundleStatePublished)
			if err != nil {
				log.Error(ctx, "Error publishing bundle, moving to next item", err, logData)
				return
			}

			log.Info(ctx, "Successfully processed bundle:", logData)
			ch <- bundleId
		}
	}
}

// Run is the main logic of the app. It gets bundles scheduled for release and then attempts to publish them one by one.
func (p *Publisher) Run(ctx context.Context) (*PublishResult, error) {
	// The time to check for scheduled publication, this is rounded to the nearest minute as publication on the minute
	// is what is provided to users to enter.  Validation is carried out below to ensure publications are not made early
	now := time.Now().UTC()
	// get difference between now and next minute - to the whole minute to improve accuracy
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	logData := log.Data{"publish_date": nextMinute}

	cfg, err := config.Get()
	if err != nil {
		log.Error(ctx, "Error getting configuration", err, logData)
		return &PublishResult{}, err
	}

	headers := sdk.Headers{
		ServiceAuthToken: cfg.ServiceToken,
	}

	log.Info(ctx, "Retrieving bundles scheduled for release", logData)

	getScheduledBundlesResult, err := p.bundlesClient.BundleClient.GetBundles(ctx, headers, &nextMinute, nil)
	if getScheduledBundlesResult.Count == 0 && strings.Contains(fmt.Sprint(err), "404") {
		log.Info(ctx, "No bundles ready for publication", logData)
		return &PublishResult{Success: true}, nil
	} else if err != nil {
		log.Error(ctx, "Error getting scheduled bundles", err, logData)
		return &PublishResult{Success: false}, err
	}

	log.Info(ctx, "There are "+strconv.Itoa(getScheduledBundlesResult.Count)+" bundles to publish", logData)
	var bundles []string
	for r := range getScheduledBundlesResult.Items {
		bundles = append(bundles, getScheduledBundlesResult.Items[r].ID)
	}

	logData = log.Data{"bundle_ids": bundles}
	log.Info(ctx, "Bundle list to publish", logData)

	// Get the time difference between the minute submitted in the query and the current time as specified above
	publishCheck := nextMinute.Sub(now)

	// Check to ensure bundles are not published early - sleeps the process for the amount of time between current time
	// above and publication time
	time.Sleep(publishCheck)

	var wg sync.WaitGroup
	ch := make(chan string, len(getScheduledBundlesResult.Items))

	for i := range getScheduledBundlesResult.Items {
		bundleId := getScheduledBundlesResult.Items[i].ID
		wg.Add(1)
		go p.runPublicationProcess(ctx, headers, bundleId, ch, &wg)
	}
	wg.Wait()

	return &PublishResult{
		Success: true,
	}, nil
}
