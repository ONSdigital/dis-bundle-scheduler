package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ONSdigital/dis-bundle-api/models"
	"github.com/ONSdigital/dis-bundle-api/sdk"
	"github.com/ONSdigital/dis-bundle-scheduler/config"

	"github.com/ONSdigital/log.go/v2/log"
)

type PublishBundleResult struct {
	BundleID string
	Success  bool
	Error    *string
}

type PublishResult struct {
	// need to make an object for here
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

// Run is the main logic of the app. It gets bundles scheduled for release and then attempts to publish them one by one.
func (p *Publisher) Run(ctx context.Context) (*PublishResult, error) {
	// The time to check for scheduled publication, this is rounded to the nearest minute as publication on the minute
	// is what is provided to users to enter.  Validation is carried out below to ensure publications are not made early
	now := time.Now().UTC().Round(time.Minute)
	logData := log.Data{"publish_date": now}

	cfg, err := config.Get()
	if err != nil {
		log.Error(ctx, "Error getting configuration", err, logData)
		return &PublishResult{}, err
	}

	headers := sdk.Headers{
		ServiceAuthToken: cfg.ServiceToken,
	}

	log.Info(ctx, "Retrieving bundles scheduled for release", logData)

	getScheduledBundlesResult, err := p.bundlesClient.BundleClient.GetBundles(ctx, headers, &now, nil)

	if getScheduledBundlesResult.Count == 0 && strings.Contains(fmt.Sprint(err), "404") {
		log.Info(ctx, "No bundles ready for publication", logData)
		return &PublishResult{Success: true}, nil
	} else if err != nil {
		log.Error(ctx, "Error getting scheduled bundles", err, logData)
		return &PublishResult{}, err
	}

	log.Info(ctx, "There are "+strconv.Itoa(getScheduledBundlesResult.Count)+" bundles to publish", logData)

	var publicationList PublishResult

	for i := range getScheduledBundlesResult.Items {
		// GetBundles does not return the etags for the bundles as it is returned in the header value, so a GetBundle request is required
		bundle, err := p.bundlesClient.BundleClient.GetBundle(ctx, headers, getScheduledBundlesResult.Items[i].ID)
		if err != nil {
			// Do not fail and return if there is an issue as the process needs to continue
			log.Error(ctx, "Error getting bundle info, moving to next item", err, logData)
		} else {
			var bundleObj models.Bundle
			err := json.Unmarshal(bundle.Body, &bundleObj)
			if err != nil {
				// Do not fail and return if there is an issue as the process needs to continue
				log.Error(ctx, "Error unmarshalling bundle info, moving to next item", err, logData)
			} else {
				publishCheck := time.Now().UTC()
				// Check to ensure bundles are not published early
				if bundleObj.ScheduledAt.Before(publishCheck) || bundleObj.ScheduledAt.Equal(publishCheck) {
					// Ensure the bundle is in the approved state
					if bundleObj.State == "APPROVED" {
						var publishedBundle PublishBundleResult
						headers.IfMatch = bundle.Headers.Get("Etag")
						updatedBundle, err := p.bundlesClient.BundleClient.PutBundleState(ctx, headers, getScheduledBundlesResult.Items[i].ID, models.BundleStatePublished)
						if err != nil {
							// Do not fail and return if there is an issue as the process needs to continue
							log.Error(ctx, "Error publishing bundle, moving to next item", err, logData)
							publishedBundle = PublishBundleResult{BundleID: getScheduledBundlesResult.Items[i].ID, Success: false, Error: nil}
						} else {
							publishedBundle = PublishBundleResult{BundleID: updatedBundle.ID, Success: true, Error: nil}
						}
						publicationList.Results = append(publicationList.Results, publishedBundle)
					}
				}
			}
		}
	}

	return &PublishResult{
		Success: true,
		Results: publicationList.Results,
	}, nil
}
