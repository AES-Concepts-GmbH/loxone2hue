package hue

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sbeyeler/loxone2hue/internal/models"
)

// sensorTypes lists HUE resource types that are sensors
var sensorTypes = map[string]bool{
	"motion":          true,
	"temperature":     true,
	"light_level":     true,
	"button":          true,
	"contact":         true,
	"relative_rotary": true,
	"device_power":    true,
}

// IsSensorType returns true if the resource type is a sensor
func IsSensorType(resourceType string) bool {
	return sensorTypes[resourceType]
}

// reconnectDelay is how long to wait before re-opening the event stream.
const reconnectDelay = 5 * time.Second

// RunEventStream supervises the HUE Bridge SSE event stream for the lifetime of
// ctx. It blocks, so call it in its own goroutine.
//
// Unlike a one-shot connect, this survives the states the gateway actually goes
// through: it waits when no bridge is configured yet (pairing happens later via
// the Web UI), reconnects after errors, and drops the current connection as
// soon as Configure points the client at a different bridge.
func (c *Client) RunEventStream(ctx context.Context) {
	for {
		if !c.IsConfigured() {
			log.Debug().Msg("HUE Bridge not configured, event stream idle")
			if !c.waitForRestart(ctx, 0) {
				return
			}
			continue
		}

		streamCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- c.connectEventStream(streamCtx) }()

		select {
		case <-ctx.Done():
			cancel()
			<-done
			return

		case <-c.stopChan:
			cancel()
			<-done
			return

		case <-c.restart:
			// Configuration changed: abandon this connection and start over
			// against the new bridge.
			log.Info().Msg("HUE Bridge configuration changed, restarting event stream")
			cancel()
			<-done

		case err := <-done:
			cancel()
			if err != nil {
				log.Error().Err(err).Msg("Event stream error, reconnecting...")
			} else {
				log.Info().Msg("Event stream closed by bridge, reconnecting...")
			}
			// Back off on a clean end too, otherwise a bridge that closes the
			// stream immediately spins this loop at full speed.
			if !c.waitForRestart(ctx, reconnectDelay) {
				return
			}
		}
	}
}

// waitForRestart blocks until the configuration changes, delay elapses, or the
// client shuts down. A delay of 0 waits indefinitely for a configuration
// change. It reports whether the caller should keep running.
func (c *Client) waitForRestart(ctx context.Context, delay time.Duration) bool {
	var timeout <-chan time.Time
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case <-ctx.Done():
		return false
	case <-c.stopChan:
		return false
	case <-c.restart:
		return true
	case <-timeout:
		return true
	}
}

func (c *Client) connectEventStream(ctx context.Context) error {
	baseURL, applicationKey := c.endpoint()
	url := fmt.Sprintf("%s/eventstream/clip/v2", baseURL)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("hue-application-key", applicationKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	log.Info().Str("bridge", c.BridgeIP()).Msg("Connected to HUE event stream")

	scanner := bufio.NewScanner(resp.Body)
	var eventData strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		case <-c.stopChan:
			return nil
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			eventData.WriteString(strings.TrimPrefix(line, "data: "))
		} else if line == "" && eventData.Len() > 0 {
			// End of event
			c.processEvent(eventData.String())
			eventData.Reset()
		}
	}

	return scanner.Err()
}

// EventItem represents a single item in the SSE event data array
type EventItem struct {
	ID    string `json:"id"`
	IDV1  string `json:"id_v1"`
	Type  string `json:"type"`
	Owner *struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"owner,omitempty"`
	// Light fields
	On *struct {
		On bool `json:"on"`
	} `json:"on,omitempty"`
	Dimming *struct {
		Brightness float64 `json:"brightness"`
	} `json:"dimming,omitempty"`
	ColorTemperature *struct {
		Mirek int `json:"mirek"`
	} `json:"color_temperature,omitempty"`
	Color *struct {
		XY struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
	} `json:"color,omitempty"`
	// Motion sensor
	Motion *struct {
		Motion       bool `json:"motion"`
		MotionValid  bool `json:"motion_valid"`
		MotionReport *struct {
			Changed string `json:"changed"`
			Motion  bool   `json:"motion"`
		} `json:"motion_report,omitempty"`
	} `json:"motion,omitempty"`
	// Temperature sensor
	Temperature *struct {
		Temperature       float64 `json:"temperature"`
		TemperatureValid  bool    `json:"temperature_valid"`
		TemperatureReport *struct {
			Changed     string  `json:"changed"`
			Temperature float64 `json:"temperature"`
		} `json:"temperature_report,omitempty"`
	} `json:"temperature,omitempty"`
	// Light level sensor
	LightSensor *struct {
		LightLevel       int  `json:"light_level"`
		LightLevelValid  bool `json:"light_level_valid"`
		LightLevelReport *struct {
			Changed    string `json:"changed"`
			LightLevel int    `json:"light_level"`
		} `json:"light_level_report,omitempty"`
	} `json:"light,omitempty"`
	// Button
	Button *struct {
		ButtonReport *struct {
			Updated string `json:"updated"`
			Event   string `json:"event"`
		} `json:"button_report,omitempty"`
		LastEvent string `json:"last_event"`
	} `json:"button,omitempty"`
	Metadata *struct {
		ControlID int `json:"control_id"`
	} `json:"metadata,omitempty"`
	// Contact
	ContactReport *struct {
		Changed string `json:"changed"`
		State   string `json:"state"`
	} `json:"contact_report,omitempty"`
	// Relative rotary
	RelativeRotary *struct {
		RotaryReport *struct {
			Updated  string `json:"updated"`
			Action   string `json:"action"`
			Rotation struct {
				Direction string `json:"direction"`
				Steps     int    `json:"steps"`
				Duration  int    `json:"duration"`
			} `json:"rotation"`
		} `json:"rotary_report,omitempty"`
	} `json:"relative_rotary,omitempty"`
	// Device power
	PowerState *struct {
		BatteryLevel int    `json:"battery_level"`
		BatteryState string `json:"battery_state"`
	} `json:"power_state,omitempty"`
}

func (c *Client) processEvent(data string) {
	var events []struct {
		CreationTime time.Time   `json:"creationtime"`
		Data         []EventItem `json:"data"`
		Type         string      `json:"type"`
	}

	if err := json.Unmarshal([]byte(data), &events); err != nil {
		log.Warn().Err(err).Msg("Failed to parse event data")
		return
	}

	for _, event := range events {
		for _, item := range event.Data {
			// Update internal state
			c.updateFromEvent(item)

			// Send event to channel
			select {
			case c.eventChan <- Event{
				Type:      item.Type,
				ID:        item.ID,
				IDV1:      item.IDV1,
				Data:      item,
				CreatedAt: event.CreationTime,
			}:
			default:
				c.noteDroppedEvent(item)
			}
		}
	}
}

// dropLogInterval bounds how often a full event channel is reported.
const dropLogInterval = 10 * time.Second

// noteDroppedEvent counts an event that found eventChan full and warns about
// it at most once per dropLogInterval. The cache is already up to date at this
// point; what is lost is the notification to Loxone and the Web UI.
func (c *Client) noteDroppedEvent(item EventItem) {
	total := c.droppedEvents.Add(1)

	now := time.Now().UnixNano()
	last := c.lastDropLog.Load()
	if now-last < int64(dropLogInterval) || !c.lastDropLog.CompareAndSwap(last, now) {
		return
	}

	log.Warn().
		Str("id", item.ID).
		Str("type", item.Type).
		Int64("dropped_total", total).
		Msg("Event channel full, dropping bridge event (feedback to Loxone lost)")
}

func (c *Client) updateFromEvent(item EventItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case item.Type == "light":
		light, ok := c.lights[item.ID]
		if !ok {
			return
		}
		if item.On != nil {
			light.State.On = item.On.On
		}
		if item.Dimming != nil {
			light.State.Brightness = item.Dimming.Brightness
		}
		if item.ColorTemperature != nil {
			light.State.ColorTemp = item.ColorTemperature.Mirek
		}
		if item.Color != nil {
			if light.State.Color == nil {
				light.State.Color = &models.Color{}
			}
			light.State.Color.XY = [2]float64{item.Color.XY.X, item.Color.XY.Y}
		}
		log.Debug().Str("id", item.ID).Msg("Light state updated from event")

	case item.Type == "grouped_light":
		groupID, ok := c.groupedLights[item.ID]
		if !ok {
			return
		}
		group, ok := c.groups[groupID]
		if !ok {
			return
		}
		if item.On != nil {
			// The bridge reports any_on for a grouped_light; all_on is not part
			// of the event and tracks it as an approximation.
			group.State.AnyOn = item.On.On
			group.State.AllOn = item.On.On
		}
		if item.Dimming != nil {
			group.State.Brightness = item.Dimming.Brightness
		}
		log.Debug().Str("id", item.ID).Str("group_id", groupID).Msg("Group state updated from event")

	case IsSensorType(item.Type):
		sensor, ok := c.sensors[item.ID]
		if !ok {
			return
		}
		c.updateSensorFromEvent(sensor, item)
		log.Debug().Str("id", item.ID).Str("type", item.Type).Msg("Sensor state updated from event")
	}
}

func (c *Client) updateSensorFromEvent(sensor *models.Sensor, item EventItem) {
	now := time.Now().UTC().Format(time.RFC3339)

	switch item.Type {
	case "motion":
		if item.Motion != nil {
			motion := item.Motion.Motion
			if item.Motion.MotionReport != nil {
				motion = item.Motion.MotionReport.Motion
				sensor.State.LastUpdated = item.Motion.MotionReport.Changed
			} else {
				sensor.State.LastUpdated = now
			}
			sensor.State.Motion = &motion
		}

	case "temperature":
		if item.Temperature != nil {
			temp := item.Temperature.Temperature
			if item.Temperature.TemperatureReport != nil {
				temp = item.Temperature.TemperatureReport.Temperature
				sensor.State.LastUpdated = item.Temperature.TemperatureReport.Changed
			} else {
				sensor.State.LastUpdated = now
			}
			sensor.State.Temperature = &temp
		}

	case "light_level":
		if item.LightSensor != nil {
			level := item.LightSensor.LightLevel
			if item.LightSensor.LightLevelReport != nil {
				level = item.LightSensor.LightLevelReport.LightLevel
				sensor.State.LastUpdated = item.LightSensor.LightLevelReport.Changed
			} else {
				sensor.State.LastUpdated = now
			}
			sensor.State.LightLevel = &level
		}

	case "button":
		if item.Button != nil {
			event := item.Button.LastEvent
			if item.Button.ButtonReport != nil {
				event = item.Button.ButtonReport.Event
				sensor.State.LastUpdated = item.Button.ButtonReport.Updated
			} else {
				sensor.State.LastUpdated = now
			}
			if event != "" {
				sensor.State.ButtonEvent = &event
			}
		}

	case "contact":
		if item.ContactReport != nil {
			sensor.State.ContactState = &item.ContactReport.State
			sensor.State.LastUpdated = item.ContactReport.Changed
		}

	case "relative_rotary":
		if item.RelativeRotary != nil && item.RelativeRotary.RotaryReport != nil {
			sensor.State.RotaryAction = &item.RelativeRotary.RotaryReport.Action
			steps := item.RelativeRotary.RotaryReport.Rotation.Steps
			if item.RelativeRotary.RotaryReport.Rotation.Direction == "counter_clockwise" {
				steps = -steps
			}
			sensor.State.RotarySteps = &steps
			sensor.State.LastUpdated = item.RelativeRotary.RotaryReport.Updated
		}

	case "device_power":
		if item.PowerState != nil {
			sensor.State.BatteryLevel = &item.PowerState.BatteryLevel
			sensor.State.BatteryState = &item.PowerState.BatteryState
			sensor.State.LastUpdated = now
		}
	}
}
