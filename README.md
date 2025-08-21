# dis-bundle-scheduler

dis-bundle-scheduler - a scheduler application for automating the publication of datasets linked to bundles.

## Getting started

* Run `make debug`

### Dependencies

* No further dependencies other than those defined in `go.mod`

### Configuration

| Environment variable          | Default                               | Description                                                |
|-------------------------------|---------------------------------------|------------------------------------------------------------|
| BUNDLES_API_SERVICE_TOKEN     | "bundle-scheduler-test-auth-token"    | The service auth token to connect to dis-bundle-api                               |
| BUNDLES_API_URL               | "http://localhost:29800"              | The URL for dis-bundle-api                                 |

## Contributing

See [CONTRIBUTING](CONTRIBUTING.md) for details.

## License

Copyright © 2025, Office for National Statistics (https://www.ons.gov.uk)

Released under MIT license, see [LICENSE](LICENSE.md) for details.
