# New Relic Reporting for Tesla Vehicles

## The Basics

* Copy `nr-tesla-config-example.yaml` to `${HOME}/.nr-tesla/config.yaml` and then edit.
  * Add your Tesla client id, client secret, username, and password.
  * Add your New Relic license key.
* Build
  * `go build .` or `docker build -t nr-tesla:latest .`
* Run
  * `./nr-tesla` or `docker run -v ${HOME}/.nr-tesla:/root/.nr-tesla nr-tesla:latest`
* Debug
  * `TESLA_DEBUG=true ./nr-tesla` or `docker run -e TESLA_DEBUG=true -v ${HOME}/.nr-tesla:/root/.nr-tesla nr-tesla:latest`

