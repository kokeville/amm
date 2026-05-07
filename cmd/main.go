package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-vgo/robotgo"
	"github.com/kirsle/configdir"
	"github.com/prashantgupta24/automatic-mouse-mover/assets/icon"
	"github.com/prashantgupta24/automatic-mouse-mover/pkg/mousemover"
	log "github.com/sirupsen/logrus"
)

type AppSettings struct {
	Icon string `json:"icon"`
}

var configPath = configdir.LocalConfig("amm")
var configFile = filepath.Join(configPath, "settings.json")

func main() {
	systray.Run(onReady, onExit)
}

func setIcon(iconName string, configFile string) {
	switch {
	case iconName == "mouse":
		systray.SetIcon(icon.Data)
	case iconName == "cloud":
		systray.SetIcon(icon.CloudIcon)
	case iconName == "man":
		systray.SetIcon(icon.ManIcon)
	case iconName == "geometric":
		systray.SetIcon(icon.GeometricIcon)
	default:
		systray.SetIcon(icon.Data)
	}
	if configFile != "" {
		var settings AppSettings
		settings = AppSettings{iconName}
		fh, _ := os.Create(configFile)
		defer fh.Close()

		encoder := json.NewEncoder(fh)
		encoder.Encode(settings)
	}
}

func onReady() {
	go func() {
		err := configdir.MakePath(configPath)
		if err != nil {
			log.Errorf("could not create config directory: %v", err)
		}
		var settings AppSettings
		settings = AppSettings{"geometric"}

		if _, err = os.Stat(configFile); os.IsNotExist(err) {
			fh, err := os.Create(configFile)
			if err != nil {
				log.Errorf("could not create config file: %v", err)
			} else {
				defer fh.Close()
				encoder := json.NewEncoder(fh)
				encoder.Encode(settings)
			}
		} else {
			fh, err := os.Open(configFile)
			if err != nil {
				log.Errorf("could not open config file: %v", err)
			} else {
				defer fh.Close()
				decoder := json.NewDecoder(fh)
				if err := decoder.Decode(&settings); err != nil {
					log.Errorf("could not decode config file: %v", err)
					settings = AppSettings{"geometric"}
				}
			}
		}
		setIcon(settings.Icon, "")

		about := systray.AddMenuItem("About AMM", "Information about the app")
		systray.AddSeparator()
		ammStart := systray.AddMenuItem("Start", "start the app")
		ammStop := systray.AddMenuItem("Stop", "stop the app")

		systray.AddSeparator()
		ammTimer := systray.AddMenuItem("Start for...", "start the app for a set duration")
		timer30m := ammTimer.AddSubMenuItem("30 minutes", "run for 30 minutes")
		timer1h := ammTimer.AddSubMenuItem("1 hour", "run for 1 hour")
		timer2h := ammTimer.AddSubMenuItem("2 hours", "run for 2 hours")
		timer4h := ammTimer.AddSubMenuItem("4 hours", "run for 4 hours")
		timer8h := ammTimer.AddSubMenuItem("8 hours", "run for 8 hours")
		timerCustom := ammTimer.AddSubMenuItem("Custom...", "run until a specific date and time")

		systray.AddSeparator()
		icons := systray.AddMenuItem("Icons", "icon of the app")
		mouse := icons.AddSubMenuItem("Mouse", "Mouse icon")
		mouse.SetIcon(icon.Data)
		cloud := icons.AddSubMenuItem("Cloud", "Cloud icon")
		cloud.SetIcon(icon.CloudIcon)
		man := icons.AddSubMenuItem("Man", "Man icon")
		man.SetIcon(icon.ManIcon)
		geometric := icons.AddSubMenuItem("Geometric", "Geometric")
		geometric.SetIcon(icon.GeometricIcon)

		ammStop.Disable()
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
		mouseMover := mousemover.GetInstance()

		startUI := func() {
			ammStart.Disable()
			ammStop.Enable()
			ammTimer.Disable()
			timer30m.Disable()
			timer1h.Disable()
			timer2h.Disable()
			timer4h.Disable()
			timer8h.Disable()
			timerCustom.Disable()
		}
		stopUI := func() {
			ammStart.Enable()
			ammStop.Disable()
			ammTimer.Enable()
			timer30m.Enable()
			timer1h.Enable()
			timer2h.Enable()
			timer4h.Enable()
			timer8h.Enable()
			timerCustom.Enable()
		}

		tryStart := func(start func()) {
			if !mousemover.IsAccessibilityGranted() {
				go openAccessibilitySettings()
				return
			}
			start()
			startUI()
		}

		mouseMover.OnStop = stopUI
		tryStart(mouseMover.Start)

		for {
			select {
			case <-ammStart.ClickedCh:
				log.Infof("starting the app")
				tryStart(mouseMover.Start)

			case <-ammStop.ClickedCh:
				log.Infof("stopping the app")
				mouseMover.Quit() // blocks until fully stopped; OnStop(stopUI) called inside

			case <-timer30m.ClickedCh:
				log.Infof("starting the app for 30 minutes")
				tryStart(func() { mouseMover.StartWithDuration(30 * time.Minute) })
			case <-timer1h.ClickedCh:
				log.Infof("starting the app for 1 hour")
				tryStart(func() { mouseMover.StartWithDuration(1 * time.Hour) })
			case <-timer2h.ClickedCh:
				log.Infof("starting the app for 2 hours")
				tryStart(func() { mouseMover.StartWithDuration(2 * time.Hour) })
			case <-timer4h.ClickedCh:
				log.Infof("starting the app for 4 hours")
				tryStart(func() { mouseMover.StartWithDuration(4 * time.Hour) })
			case <-timer8h.ClickedCh:
				log.Infof("starting the app for 8 hours")
				tryStart(func() { mouseMover.StartWithDuration(8 * time.Hour) })
			case <-timerCustom.ClickedCh:
				log.Infof("requesting custom stop time")
				if !mousemover.IsAccessibilityGranted() {
					go openAccessibilitySettings()
					break
				}
				stopTime, err := promptCustomStopTime()
				if err != nil {
					log.Infof("custom time cancelled or invalid: %v", err)
					break
				}
				if err := mouseMover.StartUntil(stopTime); err != nil {
					log.Errorf("could not start with custom time: %v", err)
					robotgo.Alert("Invalid stop time", err.Error(), "OK", "")
					break
				}
				log.Infof("starting the app until %v", stopTime)
				startUI()

			case <-mQuit.ClickedCh:
				log.Infof("Requesting quit")
				mouseMover.Quit()
				systray.Quit()
				return
			case <-mouse.ClickedCh:
				setIcon("mouse", configFile)
			case <-cloud.ClickedCh:
				setIcon("cloud", configFile)
			case <-man.ClickedCh:
				setIcon("man", configFile)
			case <-geometric.ClickedCh:
				setIcon("geometric", configFile)
			case <-about.ClickedCh:
				log.Infof("Requesting about")
				robotgo.Alert("Automatic-mouse-mover app v1.3.0", "Developed by Prashant Gupta. \n\nMore info at: https://github.com/prashantgupta24/automatic-mouse-mover", "OK", "")
			}
		}

	}()
}

// openAccessibilitySettings opens System Settings to the Accessibility pane and shows
// a one-time, non-blocking alert. Called in a goroutine so it never blocks the menu loop.
// If AMM already appears in the list (old binary), the user must remove it and re-add
// this version — after doing so, clicking Start works immediately without restarting.
func openAccessibilitySettings() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Run()
	robotgo.Alert("Accessibility Access Required",
		"AMM needs accessibility access to move the mouse.\n\nIn System Settings → Privacy & Security → Accessibility:\n• If AMM is already listed, remove it (–) and re-add it (+).\n• Toggle the switch ON.\n\nThen click Start — no restart needed.", "OK", "")
}

func onExit() {
	// clean up here
	log.Infof("Finished quitting")
}

// promptCustomStopTime shows osascript dialogs to collect a stop date and time from the user.
func promptCustomStopTime() (time.Time, error) {
	now := time.Now()
	defaultDate := now.Format("2006-01-02")
	defaultTime := now.Add(time.Hour).Format("15:04")

	dateScript := fmt.Sprintf(
		`display dialog "Enter stop date:" default answer "%s" buttons {"Cancel", "OK"} default button "OK"`,
		defaultDate,
	)
	dateOut, err := exec.Command("osascript", "-e", dateScript).Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("cancelled")
	}
	dateStr := parseOsascriptText(string(dateOut))

	timeScript := fmt.Sprintf(
		`display dialog "Enter stop time (HH:MM):" default answer "%s" buttons {"Cancel", "OK"} default button "OK"`,
		defaultTime,
	)
	timeOut, err := exec.Command("osascript", "-e", timeScript).Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("cancelled")
	}
	timeStr := parseOsascriptText(string(timeOut))

	stopTime, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+timeStr, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse date/time %q %q: %v", dateStr, timeStr, err)
	}
	return stopTime, nil
}

// parseOsascriptText extracts the "text returned" value from osascript dialog output.
func parseOsascriptText(output string) string {
	parts := strings.SplitN(strings.TrimSpace(output), "text returned:", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
