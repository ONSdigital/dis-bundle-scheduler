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

func (p *Publisher) runPublicationProcess(ctx context.Context, headers sdk.Headers, bundleId string, logData log.Data, ch chan string, wg *sync.WaitGroup) {
	// GetBundles does not return the etags for the bundles as it is returned in the header value, so a GetBundle request is required
	fmt.Println("IN THE BUNDLE PUBLICATION PROCESS")
	defer wg.Done()

	//var publicationList PublishResult
	fmt.Println("before getbundle", bundleId)
	bundle, err := p.bundlesClient.BundleClient.GetBundle(ctx, headers, bundleId)
	fmt.Println("after getbundle", bundleId)
	if err != nil {
		fmt.Println("THERE WAS AN ERROR")
		fmt.Println(err)
		// Do not fail and return if there is an issue as the process needs to continue
		log.Error(ctx, "Error getting bundle info, moving to next item", err, logData)
		//ch <- PublishResult{Success: false}
		return
	} else {
		fmt.Println("FOUND THE BUNDLE")
		var bundleObj models.Bundle
		err := json.Unmarshal(bundle.Body, &bundleObj)
		if err != nil {
			// Do not fail and return if there is an issue as the process needs to continue
			log.Error(ctx, "Error unmarshalling bundle info, moving to next item", err, logData)
			//ch <- PublishResult{Success: false}
			return
		} else if bundleObj.State == "APPROVED" {
			// Ensure the bundle is in the approved state
			var publishedBundle PublishBundleResult

			headers.IfMatch = bundle.Headers.Get("Etag")
			//headers.IfMatch = bundleObj.ETag
			fmt.Println("before putbundlestate", bundleId)
			_, err := p.bundlesClient.BundleClient.PutBundleState(ctx, headers, bundleId, models.BundleStatePublished)
			fmt.Println("after putbundlestate", bundleId)
			if err != nil {
				// Do not fail and return if there is an issue as the process needs to continue
				log.Error(ctx, "Error publishing bundle, moving to next item", err, logData)
				publishedBundle = PublishBundleResult{BundleID: bundleId, Success: false, Error: nil}
			} else {
				//ch <- PublishResult{Success: true}
				//return
				fmt.Println("SUCCESS")
			}
			fmt.Println("About to send result for ", bundleId)
			publishedBundle.BundleID = bundleId
			ch <- bundleId
			fmt.Println("Send result for ", bundleId)
			// defer resp.Body.Close()

			//publicationList.Results = append(publicationList.Results, publishedBundle)
		}
	}

	// select {
	// case <-ctx.Done():

	// 	fmt.Println("IN CONTEXT.DONE")
	// 	fmt.Println(ctx)
	// 	// Access the specific cancellation cause
	// 	if cause := context.Cause(ctx); cause != nil {
	// 		fmt.Printf("Worker cancelled due to: %v\n", cause)
	// 	}
	// 	ch <- PublishResult{nil, false}
	// case data := <-ch:
	// 	fmt.Println("DOING FMT.PRINTLN DATA")
	// 	fmt.Println(data)
	// 	time.Sleep(30 * time.Second)
	// 	return

	// }

	// select {
	// case <-ctx.Done():
	// 	fmt.Println("Worker exiting")
	fmt.Println("Working...")
	//time.Sleep(1 * time.Second)
	//ch <- PublishResult{nil, true}
	//close(ch)
	//return
	// default:
	// 	fmt.Println("Working...")
	// 	time.Sleep(1 * time.Second)
	// }

	// Close the channel (optional since program ends here)

	fmt.Println("JUST EXITING")
	//return
	//wg.Done()
}

// Run is the main logic of the app. It gets bundles scheduled for release and then attempts to publish them one by one.
func (p *Publisher) Run(ctx context.Context) (*PublishResult, error) {

	// Correctly pass the request context
	//ctx, cancel := context.WithTimeout(ctx, 100*time.Second)

	//defer cancel()
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
	fmt.Println("THE RESPONSE IS")
	fmt.Println(getScheduledBundlesResult)
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
	ch := make(chan string, len(getScheduledBundlesResult.Items))

	for i := range getScheduledBundlesResult.Items {
		bundleId := getScheduledBundlesResult.Items[i].ID
		//bgCtx := context.WithoutCancel(ctx)
		wg.Add(1)
		fmt.Println("Starting loop: "+strconv.Itoa(i)+"at: ", time.Now().String())
		fmt.Println("Number of goroutines on startup", runtime.NumGoroutine())
		go p.runPublicationProcess(ctx, headers, bundleId, logData, ch, &wg)
		fmt.Println("Ending loop: "+strconv.Itoa(i)+"at: ", time.Now().String())
		fmt.Println("Number of goroutines after", runtime.NumGoroutine())
	}
	fmt.Println("Waiting...")
	wg.Wait()

	fmt.Println("After goroutines launched:", runtime.NumGoroutine())
	return &PublishResult{
		Success: true,
		Results: publicationList.Results,
	}, nil
}
