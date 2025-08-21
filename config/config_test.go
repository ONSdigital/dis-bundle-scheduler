package config

import (
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConfig(t *testing.T) {
	Convey("Given an environment with no environment variables set", t, func() {
		os.Clearenv()
		cfg, err := Get()

		Convey("When the config values are retrieved", func() {
			Convey("Then there should be no error returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("The values should be set to the expected defaults", func() {
				So(cfg.BundlesAPIUrl, ShouldEqual, "http://localhost:29800")
				So(cfg.ServiceToken, ShouldEqual, "bundle-scheduler-test-auth-token")
			})
		})
	})
}

func TestSensitiveFieldsOmitted(t *testing.T) {
	Convey("Given an environment with no environment variables set", t, func() {
		os.Clearenv()
		cfg, err := Get()

		Convey("When string is called the service token is not returned", func() {
			obj := cfg.String()
			So(err, ShouldBeNil)
			So(obj, ShouldNotContainSubstring, cfg.ServiceToken)
		})
	})
}
