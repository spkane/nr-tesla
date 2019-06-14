package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atrox/homedir"
	newrelic "github.com/newrelic/go-agent"
	nrlog "github.com/newrelic/go-agent/_integrations/nrlogrus"
	log "github.com/sirupsen/logrus"
	tesla "github.com/spkane/tesla"
	yaml "gopkg.in/yaml.v2"
)

// conf contains the application configuration options
type conf struct {
	Options  map[string]float64 `yaml:"options"`
	NewRelic map[string]string  `yaml:"new_relic"`
	Tesla    map[string]string  `yaml:"tesla"`
}

// getConf reads the configuration from disk
func (c *conf) getConf() *conf {

	dir, err := homedir.Dir()
	if err != nil {
		log.Printf("could not determine user's home directory (#%v)", err)
	}

        log.Printf("attempting to read config from: %s/.nr-tesla/config.yaml", dir)

	yamlFile, err := ioutil.ReadFile(dir + "/.nr-tesla/config.yaml")
	if err != nil {
		log.Printf("ensure that you have created and properly setup: %s/.nr-tesla/config.yaml (#%v)", dir, err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return c
}

// getVehicles grabs the full list of vehicles from the Tesla API
func getVehicles(client *tesla.Client, nr newrelic.Application) (tesla.Vehicles, error) {
	txn := nr.StartTransaction("getVehicles", nil, nil)
	defer txn.End()
	err := txn.AddAttribute("teslaEmail", client.Auth.Email)
	if err != nil {
		return nil, err
	}
	es := newrelic.StartExternalSegment(txn, nil)
	defer es.End()
	vehicles, err := client.Vehicles()
	if err != nil {
		nr.RecordCustomEvent("tesla_api_call", map[string]interface{}{
			"call":  "Vehicles",
			"error": true,
		})
		txn.NoticeError(err)
		return nil, err
	}
	nr.RecordCustomMetric("tesla_vehicle_count", float64(len(vehicles)))
	nr.RecordCustomEvent("tesla_api_call", map[string]interface{}{
		"call":  "Vehicles",
		"error": false,
	})
	return vehicles, nil
}

// checkState pulls and reports all the vehicle state data
func checkState(vehicle struct{ *tesla.Vehicle }, nr newrelic.Application, awake float64) error {
	log.Debug("Checking all vehicle states")

	// Record vehicle data
	err := nr.RecordCustomEvent("tesla_state", map[string]interface{}{
		"DisplayName":          vehicle.DisplayName,
		"ID":                   vehicle.ID,
		"OptionCodes":          vehicle.OptionCodes,
		"VehicleID":            vehicle.VehicleID,
		"Vin":                  vehicle.Vin,
		"State":                vehicle.State,
		"IDS":                  vehicle.IDS,
		"RemoteStartEnabled":   vehicle.RemoteStartEnabled,
		"CalendarEnabled":      vehicle.CalendarEnabled,
		"NotificationsEnabled": vehicle.NotificationsEnabled,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "RecordCustomEvent",
			"type": "tesla_state",
		}).Warn(err)
		return err
	}

	// Get vehicle charge data
	var charge *tesla.ChargeState
	err = retry(5, 2*time.Second, func() (err error) {
		charge, err = vehicle.ChargeState()
		return
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "ChargeState",
		}).Warn(err)
		return err
	}

	err = nr.RecordCustomEvent("tesla_charge_state", map[string]interface{}{
		"ChargingState":               charge.ChargingState,
		"ChargeLimitSoc":              charge.ChargeLimitSoc,
		"ChargeLimitSocStd":           charge.ChargeLimitSocStd,
		"ChargeLimitSocMin":           charge.ChargeLimitSocMin,
		"ChargeLimitSocMax":           charge.ChargeLimitSocMax,
		"ChargeToMaxRange":            charge.ChargeToMaxRange,
		"BatteryHeaterOn":             charge.BatteryHeaterOn,
		"NotEnoughPowerToHeat":        charge.NotEnoughPowerToHeat,
		"MaxRangeChargeCounter":       charge.MaxRangeChargeCounter,
		"FastChargerPresent":          charge.FastChargerPresent,
		"FastChargerType":             charge.FastChargerType,
		"BatteryRange":                charge.BatteryRange,
		"EstBatteryRange":             charge.EstBatteryRange,
		"IdealBatteryRange":           charge.IdealBatteryRange,
		"BatteryLevel":                charge.BatteryLevel,
		"UsableBatteryLevel":          charge.UsableBatteryLevel,
		"ChargeEnergyAdded":           charge.ChargeEnergyAdded,
		"ChargeMilesAddedRated":       charge.ChargeMilesAddedRated,
		"ChargeMilesAddedIdeal":       charge.ChargeMilesAddedIdeal,
		"TimeToFullCharge":            charge.TimeToFullCharge,
		"ChargeRate":                  charge.ChargeRate,
		"ChargePortDoorOpen":          charge.ChargePortDoorOpen,
		"MotorizedChargePort":         charge.MotorizedChargePort,
		"ScheduledChargingPending":    charge.ScheduledChargingPending,
		"ChargeEnableRequest":         charge.ChargeEnableRequest,
		"EuVehicle":                   charge.EuVehicle,
		"ChargePortLatch":             charge.ChargePortLatch,
		"ChargeCurrentRequest":        charge.ChargeCurrentRequest,
		"ChargeCurrentRequestMax":     charge.ChargeCurrentRequestMax,
		"ManagedChargingActive":       charge.ManagedChargingActive,
		"ManagedChargingUserCanceled": charge.ManagedChargingUserCanceled,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "RecordCustomEvent",
			"type": "tesla_charge_state",
		}).Warn(err)
		return err
	}

	// Get vehicle climate data
	var climate *tesla.ClimateState
	err = retry(5, 2*time.Second, func() (err error) {
		climate, err = vehicle.ClimateState()
		return
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "ClimateState",
		}).Warn(err)
		return err
	}

	err = nr.RecordCustomEvent("tesla_climate_state", map[string]interface{}{
		"InsideTemp":              climate.InsideTemp,
		"OutsideTemp":             climate.OutsideTemp,
		"DriverTempSetting":       climate.DriverTempSetting,
		"PassengerTempSetting":    climate.PassengerTempSetting,
		"LeftTempDirection":       climate.LeftTempDirection,
		"RightTempDirection":      climate.RightTempDirection,
		"IsAutoConditioningOn":    climate.IsAutoConditioningOn,
		"IsFrontDefrosterOn":      climate.IsFrontDefrosterOn,
		"IsRearDefrosterOn":       climate.IsRearDefrosterOn,
		"IsClimateOn":             climate.IsClimateOn,
		"MinAvailTemp":            climate.MinAvailTemp,
		"MaxAvailTemp":            climate.MaxAvailTemp,
		"SeatHeaterLeft":          climate.SeatHeaterLeft,
		"SeatHeaterRight":         climate.SeatHeaterRight,
		"SeatHeaterRearLeft":      climate.SeatHeaterRearLeft,
		"SeatHeaterRearRight":     climate.SeatHeaterRearRight,
		"SeatHeaterRearCenter":    climate.SeatHeaterRearCenter,
		"SeatHeaterRearRightBack": climate.SeatHeaterRearRightBack,
		"SeatHeaterRearLeftBack":  climate.SeatHeaterRearLeftBack,
		"SmartPreconditioning":    climate.SmartPreconditioning,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "RecordCustomEvent",
			"type": "tesla_climate_state",
		}).Warn(err)
		return err
	}

	// Get vehicle drive data
	var drive *tesla.DriveState
	err = retry(5, 2*time.Second, func() (err error) {
		drive, err = vehicle.DriveState()
		return
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "DriveState",
		}).Warn(err)
		return err
	}

	err = nr.RecordCustomEvent("tesla_drive_state", map[string]interface{}{
		"Speed":     drive.Speed,
		"Latitude":  drive.Latitude,
		"Longitude": drive.Longitude,
		"Heading":   drive.Heading,
		"GpsAsOf":   drive.GpsAsOf,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "RecordCustomEvent",
			"type": "tesla_drive_state",
		}).Warn(err)
		return err
	}

	// Get vehicle gui data
	var gui *tesla.GuiSettings
	err = retry(5, 2*time.Second, func() (err error) {
		gui, err = vehicle.GuiSettings()
		return
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "GuiSettings",
		}).Warn(err)
		return err
	}

	err = nr.RecordCustomEvent("tesla_gui_state", map[string]interface{}{
		"GuiDistanceUnits":    gui.GuiDistanceUnits,
		"GuiTemperatureUnits": gui.GuiTemperatureUnits,
		"GuiChargeRateUnits":  gui.GuiChargeRateUnits,
		"Gui24HourTime":       gui.Gui24HourTime,
		"GuiRangeDisplay":     gui.GuiRangeDisplay,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "RecordCustomEvent",
			"type": "tesla_gui_state",
		}).Warn(err)
		return err
	}

	// Get vehicle state data
	var vstate *tesla.VehicleState
	err = retry(5, 2*time.Second, func() (err error) {
		vstate, err = vehicle.VehicleState()
		return
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "VehicleState",
		}).Warn(err)
		return err
	}

	err = nr.RecordCustomEvent("tesla_vehicle_state", map[string]interface{}{
		"APIVersion":              vstate.APIVersion,
		"AutoParkState":           vstate.AutoParkState,
		"AutoParkStateV2":         vstate.AutoParkStateV2,
		"CalendarSupported":       vstate.CalendarSupported,
		"CarType":                 vstate.CarType,
		"CarVersion":              vstate.CarVersion,
		"CenterDisplayState":      vstate.CenterDisplayState,
		"DarkRims":                vstate.DarkRims,
		"Df":                      vstate.Df,
		"Dr":                      vstate.Dr,
		"ExteriorColor":           vstate.ExteriorColor,
		"Ft":                      vstate.Ft,
		"HasSpoiler":              vstate.HasSpoiler,
		"Locked":                  vstate.Locked,
		"NotificationsSupported":  vstate.NotificationsSupported,
		"Odometer":                vstate.Odometer,
		"ParsedCalendarSupported": vstate.ParsedCalendarSupported,
		"PerfConfig":              vstate.PerfConfig,
		"Pf":                      vstate.Pf,
		"Pr":                      vstate.Pr,
		"RearSeatHeaters":         vstate.RearSeatHeaters,
		"RemoteStart":             vstate.RemoteStart,
		"RemoteStartSupported":    vstate.RemoteStartSupported,
		"Rhd":                     vstate.Rhd,
		"RoofColor":               vstate.RoofColor,
		"Rt":                      vstate.Rt,
		"SeatType":                vstate.SeatType,
		"SpoilerType":             vstate.SpoilerType,
		"SunRoofInstalled":        vstate.SunRoofInstalled,
		"SunRoofPercentOpen":      vstate.SunRoofPercentOpen,
		"SunRoofState":            vstate.SunRoofState,
		"ThirdRowSeats":           vstate.ThirdRowSeats,
		"ValetMode":               vstate.ValetMode,
		"VehicleName":             vstate.VehicleName,
		"WheelType":               vstate.WheelType,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"call": "RecordCustomEvent",
			"type": "tesla_vehicle_state",
		}).Warn(err)
		return err
	}

	if (drive.Speed == 0) && (charge.ChargingState == "Complete") {
		log.WithFields(log.Fields{
			"drive_speed": drive.Speed,
			"charging_state": charge.ChargingState,
		}).Info("Vehicle in rest state...pausing checks")

		time.Sleep(time.Duration(awake*60000000000))
	}

	return nil

}

// fatalError logs an error and then cleanly shuts down the application
func fatalError(err error, code int, nr newrelic.Application) {
	log.WithFields(log.Fields{
		"exitCode": code,
	}).Error(err)
	cleanupExit(code, nr)
}

// cleanupExit cleanly shuts down the application
func cleanupExit(code int, nr newrelic.Application) {
	nr.Shutdown(10 * time.Second)
	os.Exit(code)
}

// init configures debug mode if enabled
func init() {
	if os.Getenv("TESLA_DEBUG") == "true" {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

// retry will attempt to retry a function with backoff if an error is detected
func retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep * time.Duration((i + 1)))

		log.Println("retrying after error:", err)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

// checkLoop is the main loop that checks vehicle status on a schedule
func checkLoop(cnf conf, nr newrelic.Application, client *tesla.Client) {
	awake := cnf.Options["awake_check_period"]
	active := cnf.Options["active_check_period"]
	log.WithFields(log.Fields{
		"awake_check_period":  awake,
		"active_check_period": active,
	}).Debug("Config Options")

	var vehicles tesla.Vehicles
	for {

		err := retry(5, 2*time.Second, func() (err error) {
			vehicles, err = getVehicles(client, nr)
			return
		})
		if err != nil {
			fatalError(err, 1, nr)
			return
		}

		vehicle := vehicles[0]
		_, err = vehicle.MobileEnabled()
		if err != nil {
			fatalError(err, 1, nr)
		}

		log.WithFields(log.Fields{
			"vehicle_state": vehicle.State,
		}).Info("Vehicle State")

		if (vehicle.State != "asleep") && (vehicle.State != "offline") {
			err = checkState(vehicle, nr, awake)
			if err != nil {
				fatalError(err, 1, nr)
			}
			time.Sleep(60 * time.Second)
		} else {
			log.WithFields(log.Fields{
				"vehicle_state": vehicle.State,
			}).Info("Vehicle asleep or offline...pausing checks")
			time.Sleep(time.Duration(awake*60000000000))
		}
	}
}

// main is the entrypoint for the application
func main() {

	var cnf conf
	cnf.getConf()

	cfg := newrelic.NewConfig(cnf.NewRelic["app_name"], cnf.NewRelic["license_key"])
	cfg.DistributedTracer.Enabled = true
	cfg.Logger = nrlog.StandardLogger()
	nr, err := newrelic.NewApplication(cfg)
	if err != nil {
		fatalError(err, 1, nr)
	}

	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigchan
		fatalError(fmt.Errorf("the application received an interrupt or SIGTERM"), 2, nr)
	}()

	// Wait for the application to connect.
	if err = nr.WaitForConnection(5 * time.Second); nil != err {
		log.Warn(err)
	}

	client, err := tesla.NewClient(
		&tesla.Auth{
			ClientID:     cnf.Tesla["client_id"],
			ClientSecret: cnf.Tesla["client_secret"],
			Email:        cnf.Tesla["username"],
			Password:     cnf.Tesla["password"],
		})
	if err != nil {
		fatalError(err, 1, nr)
	}

	vehicles, err := getVehicles(client, nr)
	if err != nil {
		fatalError(err, 1, nr)
	}

	if len(vehicles) > 0 {
		checkLoop(cnf, nr, client)
	} else {
		fatalError(fmt.Errorf("there are no vehicles registered to this Tesla account"), 2, nr)
	}

	// Shut down the application to flush data to New Relic.
	nr.Shutdown(10 * time.Second)
}
