# New Relic Reporting for Tesla Vehicles

## The Basics

* Copy `nr-tesla-config-example.yaml` to `${HOME}/.nr-tesla/config.yaml` and then edit.
  * Add your Tesla client id, client secret, username, and password.
  * Add your New Relic license key.
* Build
  * `go build .`
* Run
  * `./nr-tesla`
* Debug
  * `TESLA_DEBUG=true ./nr-tesla`

