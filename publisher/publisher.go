package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

func (p *Publisher) runPublicationProcess(ctx context.Context, headers sdk.Headers, bundleId string, logData log.Data, ch chan PublishResult, wg *sync.WaitGroup) {
	// GetBundles does not return the etags for the bundles as it is returned in the header value, so a GetBundle request is required
	fmt.Println("IN THE BUNDLE PUBLICATION PROCESS")
	defer wg.Done()
	var publicationList PublishResult
	bundle, err := p.bundlesClient.BundleClient.GetBundle(ctx, headers, bundleId)
	if err != nil {
		fmt.Println("THERE WAS AN ERROR")
		fmt.Println(err)
		// Do not fail and return if there is an issue as the process needs to continue
		log.Error(ctx, "Error getting bundle info, moving to next item", err, logData)
	} else {
		fmt.Println("FOUND THE BUNDLE")
		var bundleObj models.Bundle
		err := json.Unmarshal(bundle.Body, &bundleObj)
		if err != nil {
			// Do not fail and return if there is an issue as the process needs to continue
			log.Error(ctx, "Error unmarshalling bundle info, moving to next item", err, logData)
		} else if bundleObj.State == "APPROVED" {
			// Ensure the bundle is in the approved state
			var publishedBundle PublishBundleResult
			headers.IfMatch = bundle.Headers.Get("Etag")
			updatedBundle, err := p.bundlesClient.BundleClient.PutBundleState(ctx, headers, bundleId, models.BundleStatePublished)
			if err != nil {
				// Do not fail and return if there is an issue as the process needs to continue
				log.Error(ctx, "Error publishing bundle, moving to next item", err, logData)
				publishedBundle = PublishBundleResult{BundleID: bundleId, Success: false, Error: nil}
			} else {
				publishedBundle = PublishBundleResult{BundleID: updatedBundle.ID, Success: true, Error: nil}
			}
			fmt.Println(publishedBundle)
			// defer resp.Body.Close()

			publicationList.Results = append(publicationList.Results, publishedBundle)
		}
	}
	ch <- PublishResult{publicationList.Results, true}

	// Close the channel (optional since program ends here)

	fmt.Println("JUST EXITING")
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
		return &PublishResult{}, err
	}

	log.Info(ctx, "There are "+strconv.Itoa(getScheduledBundlesResult.Count)+" bundles to publish", logData)

	var publicationList PublishResult

	// Get the time difference between the minute submitted in the query and the current time as specified above
	publishCheck := nextMinute.Sub(now)

	// Check to ensure bundles are not published early - sleeps the process for the amount of time between current time
	// above and publication time
	time.Sleep(publishCheck)
	fmt.Println("Before starting goroutines:", runtime.NumGoroutine())
	fmt.Println("Starting go routines at: ", time.Now().String())

	// cores := runtime.NumCPU()
	// runtime.GOMAXPROCS(cores)
	var wg sync.WaitGroup
	// numWorkers := len(getScheduledBundlesResult.Items)
	// wg.Add(numWorkers)
	ch := make(chan PublishResult)

	for i := range getScheduledBundlesResult.Items {
		bundleId := getScheduledBundlesResult.Items[i].ID
		wg.Add(1)
		fmt.Println("Starting loop: "+strconv.Itoa(i)+"at: ", time.Now().String())
		go p.runPublicationProcess(ctx, headers, bundleId, logData, ch, &wg)
		fmt.Println("Ending loop: "+strconv.Itoa(i)+"at: ", time.Now().String())
		// GetBundles does not return the etags for the bundles as it is returned in the header value, so a GetBundle request is required
		// bundle, err := p.bundlesClient.BundleClient.GetBundle(ctx, headers, getScheduledBundlesResult.Items[i].ID)
		// if err != nil {
		// 	// Do not fail and return if there is an issue as the process needs to continue
		// 	log.Error(ctx, "Error getting bundle info, moving to next item", err, logData)
		// } else {
		// 	var bundleObj models.Bundle
		// 	err := json.Unmarshal(bundle.Body, &bundleObj)
		// 	if err != nil {
		// 		// Do not fail and return if there is an issue as the process needs to continue
		// 		log.Error(ctx, "Error unmarshalling bundle info, moving to next item", err, logData)
		// 	} else if bundleObj.State == "APPROVED" {
		// 		// Ensure the bundle is in the approved state
		// 		var publishedBundle PublishBundleResult
		// 		headers.IfMatch = bundle.Headers.Get("Etag")
		// 		updatedBundle, err := p.bundlesClient.BundleClient.PutBundleState(ctx, headers, getScheduledBundlesResult.Items[i].ID, models.BundleStatePublished)
		// 		if err != nil {
		// 			// Do not fail and return if there is an issue as the process needs to continue
		// 			log.Error(ctx, "Error publishing bundle, moving to next item", err, logData)
		// 			publishedBundle = PublishBundleResult{BundleID: getScheduledBundlesResult.Items[i].ID, Success: false, Error: nil}
		// 		} else {
		// 			publishedBundle = PublishBundleResult{BundleID: updatedBundle.ID, Success: true, Error: nil}
		// 		}
		// 		publicationList.Results = append(publicationList.Results, publishedBundle)
		// 	}
		// }
	}
	//close(ch)
	<-ch
	wg.Wait()
	// Collect responses
	// for i := 0; i < len(getScheduledBundlesResult.Items); i++ {
	// 	resp := <-ch
	// 	if resp.Results != nil {
	// 		fmt.Printf("%s", fmt.Sprint(resp))
	// 	}
	// 	// } else {
	// 	//     fmt.Printf("Successfully fetched %s: %s\n", resp.url, resp.status)
	// 	// }
	// }

	fmt.Println("After goroutines launched:", runtime.NumGoroutine())
	return &PublishResult{
		Success: true,
		Results: publicationList.Results,
	}, nil
}
