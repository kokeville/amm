package mousemover

import (
	"fmt"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/prashantgupta24/activity-tracker/pkg/activity"
	"github.com/prashantgupta24/activity-tracker/pkg/tracker"
)

var instance *MouseMover

const (
	timeout     = 100 //ms
	logDir      = "log"
	logFileName = "logFile-amm-5"
)

// Start the main app
func (m *MouseMover) Start() {
	if m.state.isRunning() {
		return
	}
	// Cancel any pending timer before starting fresh
	if m.timerQuit != nil {
		select {
		case m.timerQuit <- struct{}{}:
		default:
		}
		m.timerQuit = nil
	}
	m.state = &state{}
	m.quit = make(chan struct{}, 1) // buffered so Quit() never deadlocks during the nested mouse-move select
	m.done = make(chan struct{})

	heartbeatInterval := 60 //value always in seconds
	workerInterval := 10

	activityTracker := &tracker.Instance{
		HeartbeatInterval: heartbeatInterval,
		WorkerInterval:    workerInterval,
		// LogLevel:          "debug", //if we want verbose logging
	}

	heartbeatCh := activityTracker.Start()
	m.run(heartbeatCh, activityTracker)
}

func (m *MouseMover) run(heartbeatCh chan *tracker.Heartbeat, activityTracker *tracker.Instance) {
	go func() {
		state := m.state
		if state == nil || state.isRunning() {
			return
		}
		state.updateRunningStatus(true)

		logger := getLogger(m, false, logFileName) //set writeToFile=true only for debugging
		movePixel := 10
		for {
			select {
			case heartbeat := <-heartbeatCh:
				if heartbeat == nil {
					continue
				}
				if !heartbeat.WasAnyActivity {
					if state.isSystemSleeping() {
						logger.Infof("system sleeping")
						continue
					}
					mouseMoveSuccessCh := make(chan bool)
					go moveAndCheck(state, movePixel, mouseMoveSuccessCh)
					select {
					case wasMouseMoveSuccess := <-mouseMoveSuccessCh:
						if wasMouseMoveSuccess {
							state.updateLastMouseMovedTime(time.Now())
							logger.Infof("Is system sleeping? : %v : moved mouse at : %v\n\n", state.isSystemSleeping(), state.getLastMouseMovedTime())
							movePixel *= -1
							state.updateDidNotMoveCount(0)
						} else {
							didNotMoveCount := state.getDidNotMoveCount()
							state.updateDidNotMoveCount(didNotMoveCount + 1)
							state.updateLastErrorTime(time.Now())
							msg := fmt.Sprintf("Mouse pointer cannot be moved at %v. Last moved at %v. Happened %v times. (Only notifies once every 24 hours.) See README for details.",
								time.Now(), state.getLastMouseMovedTime(), state.getDidNotMoveCount())
							logger.Errorf(msg)
							if state.getDidNotMoveCount() >= 10 && (time.Since(state.lastErrorTime).Hours() > 24) { //show only 1 error in a 24 hour window
								go func() {
									robotgo.Alert("Error with Automatic Mouse Mover", msg)
								}()
							}
						}
					case <-time.After(timeout * time.Millisecond):
						//timeout, do nothing
						logger.Errorf("timeout happened after %vms while trying to move mouse", timeout)
					}
				} else {
					logger.Infof("activity detected in the last %v seconds.", int(activityTracker.HeartbeatInterval))
					logger.Infof("Activity type:\n")
					for activityType, times := range heartbeat.ActivityMap {
						logger.Infof("activityType : %v times: %v\n", activityType, len(times))
						if activityType == activity.MachineSleep {
							state.updateMachineSleepStatus(true)
							logger.Infof("system sleep registered. Is system sleeping? : %v", state.isSystemSleeping())
							break
						} else {
							state.updateMachineSleepStatus(false)
						}
					}
					logger.Infof("\n\n\n")
				}
			case <-m.quit:
				logger.Infof("stopping mouse mover")
				state.updateRunningStatus(false)
				activityTracker.Quit()
				if m.OnStop != nil {
					m.OnStop()
				}
				close(m.done)
				return
			}
		}
	}()
}

// Quit the app
func (m *MouseMover) Quit() {
	//making it idempotent
	if m != nil && m.state != nil && m.state.isRunning() {
		done := m.done
		select {
		case m.quit <- struct{}{}:
		default:
		}
		if done != nil {
			<-done // wait for the run goroutine to finish and call OnStop
		}
	}
	// Cancel any pending auto-stop timer
	if m.timerQuit != nil {
		select {
		case m.timerQuit <- struct{}{}:
		default:
		}
		m.timerQuit = nil
	}
	if m.logFile != nil {
		m.logFile.Close()
	}
}

// StartWithDuration starts the app and automatically stops it after the given duration.
func (m *MouseMover) StartWithDuration(duration time.Duration) {
	m.Start()
	tq := make(chan struct{}, 1)
	m.timerQuit = tq
	go func() {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-timer.C:
			m.Quit()
		case <-tq:
			// Cancelled by manual quit or new start
		}
	}()
}

// StartUntil starts the app and automatically stops it at the given time.
// Returns an error if stopTime is in the past.
func (m *MouseMover) StartUntil(stopTime time.Time) error {
	duration := time.Until(stopTime)
	if duration <= 0 {
		return fmt.Errorf("stop time %v is in the past", stopTime)
	}
	m.StartWithDuration(duration)
	return nil
}

// GetInstance gets the singleton instance for mouse mover app
func GetInstance() *MouseMover {
	if instance == nil {
		instance = &MouseMover{
			state: &state{},
		}
	}
	return instance
}
