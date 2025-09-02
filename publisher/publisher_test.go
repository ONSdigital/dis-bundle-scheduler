package publisher_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ONSdigital/dis-bundle-api/models"
	"github.com/ONSdigital/dis-bundle-api/sdk"
	"github.com/ONSdigital/dis-bundle-api/sdk/errors"
	bundleMocks "github.com/ONSdigital/dis-bundle-api/sdk/mocks"
	"github.com/ONSdigital/dis-bundle-scheduler/config"
	"github.com/ONSdigital/dis-bundle-scheduler/publisher"
	. "github.com/smartystreets/goconvey/convey"
)

var (
	now            = time.Now().UTC()
	oneMinuteLater = now.Add(1 * time.Minute)
	testBundle     = models.Bundle{
		ID:            "bundle1",
		BundleType:    models.BundleTypeScheduled,
		CreatedBy:     &models.User{Email: "creator@example.com"},
		CreatedAt:     &now,
		LastUpdatedBy: &models.User{Email: "updater@example.com"},
		PreviewTeams:  []models.PreviewTeam{{ID: "team1"}, {ID: "team2"}},
		ScheduledAt:   &oneMinuteLater,
		State:         models.BundleStateApproved,
		Title:         "Scheduled Bundle 1",
		UpdatedAt:     &now,
		ManagedBy:     models.ManagedByDataAdmin,
	}

	testBundleIncorrectState = models.Bundle{
		ID:            "bundle5",
		BundleType:    models.BundleTypeScheduled,
		CreatedBy:     &models.User{Email: "creator@example.com"},
		CreatedAt:     &now,
		LastUpdatedBy: &models.User{Email: "updater@example.com"},
		PreviewTeams:  []models.PreviewTeam{{ID: "team1"}, {ID: "team2"}},
		ScheduledAt:   &now,
		State:         models.BundleStateDraft,
		Title:         "Scheduled Bundle 5",
		UpdatedAt:     &now,
		ManagedBy:     models.ManagedByDataAdmin,
	}

	testBundleForPublish = models.Bundle{
		ID:            "bundle2",
		BundleType:    models.BundleTypeScheduled,
		CreatedBy:     &models.User{Email: "creator@example.com"},
		CreatedAt:     &now,
		LastUpdatedBy: &models.User{Email: "updater@example.com"},
		PreviewTeams:  []models.PreviewTeam{{ID: "team1"}, {ID: "team2"}},
		ScheduledAt:   &now,
		State:         models.BundleStateApproved,
		Title:         "Scheduled Bundle 2",
		UpdatedAt:     &now,
		ManagedBy:     models.ManagedByDataAdmin,
	}

	testBundleItems = []models.Bundle{testBundle}

	testBundles = sdk.BundlesList{
		Count:  2,
		Items:  testBundleItems,
		Offset: 0,
		Limit:  20,
	}
)

func TestRunScheduler(t *testing.T) {
	Convey("Given the scheduler runs and there are no bundles found to publish", t, func() {
		cfg, err := config.Get()
		So(err, ShouldBeNil)

		bundleAPIMock := &bundleMocks.ClienterMock{
			GetBundlesFunc: func(ctx context.Context, headers sdk.Headers, scheduledAt *time.Time, queryParams *sdk.QueryParams) (result *sdk.BundlesList, err errors.Error) {
				return &sdk.BundlesList{}, errors.StatusError{Code: 404, Err: fmt.Errorf("failed as unexpected code from bundle api: %v", 404)}
			},
		}
		pub, err := publisher.CreatePublisher(cfg, publisher.ClientList{bundleAPIMock})
		So(err, ShouldBeNil)
		Convey("When the the publisher is run", func(c C) {
			publishResult, err := pub.Run(context.Background())
			Convey("Then the count should be 0", func(c C) {
				So(err, ShouldBeNil)
				So(len(publishResult.Results), ShouldEqual, 0)
			})
		})
	})

	Convey("Given the scheduler runs and there are bundles found to publish", t, func() {
		cfg, err := config.Get()
		So(err, ShouldBeNil)

		body, err := json.Marshal(testBundleForPublish)
		if err != nil {
			fmt.Println(err)
		}

		bundleAPIMock := &bundleMocks.ClienterMock{
			GetBundlesFunc: func(ctx context.Context, headers sdk.Headers, scheduledAt *time.Time, queryParams *sdk.QueryParams) (result *sdk.BundlesList, err errors.Error) {
				return &testBundles, err
			},
			GetBundleFunc: func(ctx context.Context, headers sdk.Headers, id string) (response *sdk.ResponseInfo, err errors.Error) {
				return &sdk.ResponseInfo{Status: 200, Body: body}, err
			},
			PutBundleStateFunc: func(ctx context.Context, headers sdk.Headers, id string, state models.BundleState) (bundle *models.Bundle, err errors.Error) {
				return &testBundleForPublish, err
			},
		}
		pubPublish, err := publisher.CreatePublisher(cfg, publisher.ClientList{bundleAPIMock})
		fmt.Println(err)
		if pubPublish != nil {
			publishResult, err := pubPublish.Run(context.Background())
			So(err, ShouldBeNil)
			So(len(publishResult.Results), ShouldEqual, 1)
		}
	})

	Convey("Given the scheduler runs and there are bundles to publish in the incorrect state", t, func() {
		cfg, err := config.Get()
		So(err, ShouldBeNil)

		body, err := json.Marshal(testBundleIncorrectState)
		if err != nil {
			fmt.Println(err)
		}

		bundleAPIMock := &bundleMocks.ClienterMock{
			GetBundlesFunc: func(ctx context.Context, headers sdk.Headers, scheduledAt *time.Time, queryParams *sdk.QueryParams) (result *sdk.BundlesList, err errors.Error) {
				return &testBundles, err
			},
			GetBundleFunc: func(ctx context.Context, headers sdk.Headers, id string) (response *sdk.ResponseInfo, err errors.Error) {
				return &sdk.ResponseInfo{Status: 200, Body: body}, err
			},
			PutBundleStateFunc: func(ctx context.Context, headers sdk.Headers, id string, state models.BundleState) (bundle *models.Bundle, err errors.Error) {
				return nil, err
			},
		}
		pubPublish, err := publisher.CreatePublisher(cfg, publisher.ClientList{bundleAPIMock})
		fmt.Println(err)
		if pubPublish != nil {
			publishResult, err := pubPublish.Run(context.Background())
			So(err, ShouldBeNil)
			So(len(publishResult.Results), ShouldEqual, 0)
		}
	})

	Convey("Given the scheduler runs and there is an error retrieving the list of bundles", t, func() {
		cfg, err := config.Get()
		So(err, ShouldBeNil)
		bundleAPIMock := &bundleMocks.ClienterMock{
			GetBundlesFunc: func(ctx context.Context, headers sdk.Headers, scheduledAt *time.Time, queryParams *sdk.QueryParams) (result *sdk.BundlesList, err errors.Error) {
				return &testBundles, errors.StatusError{Code: 500, Err: fmt.Errorf("failed to unmarshal bundlesList response - error is: %v", 500)}
			},
		}
		pubPublish, err := publisher.CreatePublisher(cfg, publisher.ClientList{bundleAPIMock})
		fmt.Println(err)
		if pubPublish != nil {
			publishResult, err := pubPublish.Run(context.Background())
			So(err, ShouldNotBeNil)
			So(len(publishResult.Results), ShouldEqual, 0)
		}
	})

	Convey("Given the scheduler runs and there is an error retrieving specific bundle information", t, func() {
		cfg, err := config.Get()
		So(err, ShouldBeNil)
		bundleAPIMock := &bundleMocks.ClienterMock{
			GetBundlesFunc: func(ctx context.Context, headers sdk.Headers, scheduledAt *time.Time, queryParams *sdk.QueryParams) (result *sdk.BundlesList, err errors.Error) {
				return &testBundles, err
			},
			GetBundleFunc: func(ctx context.Context, headers sdk.Headers, id string) (response *sdk.ResponseInfo, err errors.Error) {
				return nil, errors.StatusError{Code: 500, Err: fmt.Errorf("failed to unmarshal bundleResponse - error is: %v", 500)}
			},
		}
		pubPublish, err := publisher.CreatePublisher(cfg, publisher.ClientList{bundleAPIMock})
		fmt.Println(err)
		if pubPublish != nil {
			publishResult, err := pubPublish.Run(context.Background())
			So(err, ShouldBeNil)
			So(len(publishResult.Results), ShouldEqual, 0)
		}
	})

	Convey("Given the scheduler runs and there is an error with publish", t, func() {
		cfg, err := config.Get()
		So(err, ShouldBeNil)

		body, err := json.Marshal(testBundleForPublish)
		if err != nil {
			fmt.Println(err)
		}
		bundleAPIMock := &bundleMocks.ClienterMock{
			GetBundlesFunc: func(ctx context.Context, headers sdk.Headers, scheduledAt *time.Time, queryParams *sdk.QueryParams) (result *sdk.BundlesList, err errors.Error) {
				return &testBundles, err
			},
			GetBundleFunc: func(ctx context.Context, headers sdk.Headers, id string) (response *sdk.ResponseInfo, err errors.Error) {
				return &sdk.ResponseInfo{Status: 200, Body: body}, err
			},
			PutBundleStateFunc: func(ctx context.Context, headers sdk.Headers, id string, state models.BundleState) (bundle *models.Bundle, err errors.Error) {
				return nil, errors.StatusError{Code: 500, Err: fmt.Errorf("failed to unmarshal bundleResponse - error is: %v", 500)}
			},
		}
		pubPublish, err := publisher.CreatePublisher(cfg, publisher.ClientList{bundleAPIMock})
		fmt.Println(err)
		if pubPublish != nil {
			publishResult, err := pubPublish.Run(context.Background())
			So(err, ShouldBeNil)
			So(len(publishResult.Results), ShouldEqual, 1)
			So(publishResult.Results[0].Success, ShouldBeFalse)
		}
	})
}
