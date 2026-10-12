package hue

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sbeyeler/loxone2hue/internal/models"
)

// Client represents a HUE Bridge API client
type Client struct {
	// cfgMu guards the bridge coordinates. It is deliberately separate from mu:
	// cache readers such as GetSensors hold mu while issuing requests, and a
	// single mutex for both would deadlock.
	cfgMu          sync.RWMutex
	bridgeIP       string
	applicationKey string
	baseURL        string

	httpClient *http.Client

	lights  map[string]*models.Light
	groups  map[string]*models.Group
	scenes  map[string]*models.Scene
	sensors map[string]*models.Sensor
	// groupedLights maps a grouped_light service ID to the room or zone that
	// owns it, so aggregated group events can be attributed to the group.
	groupedLights map[string]string
	mu            sync.RWMutex

	eventChan chan Event
	stopChan  chan struct{}
	closeOnce sync.Once

	// droppedEvents counts events that found eventChan full. lastDropLog
	// (unix nanos) throttles the warning so a stalled consumer does not flood
	// the log with one line per event.
	droppedEvents atomic.Int64
	lastDropLog   atomic.Int64

	// restart carries a nudge to the event stream supervisor whenever the
	// bridge configuration changes. Buffered so Configure never blocks.
	restart chan struct{}
}

// Event represents a HUE event from the SSE stream
type Event struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	IDV1      string      `json:"id_v1,omitempty"`
	Data      interface{} `json:"data"`
	CreatedAt time.Time   `json:"creationtime"`
}

// NewClient creates a new HUE Bridge client
func NewClient(bridgeIP, applicationKey string) *Client {
	// HUE Bridge uses self-signed certificates
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &Client{
		bridgeIP:       bridgeIP,
		applicationKey: applicationKey,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
		},
		baseURL:       fmt.Sprintf("https://%s", bridgeIP),
		lights:        make(map[string]*models.Light),
		groups:        make(map[string]*models.Group),
		scenes:        make(map[string]*models.Scene),
		sensors:       make(map[string]*models.Sensor),
		groupedLights: make(map[string]string),
		eventChan:     make(chan Event, 100),
		stopChan:      make(chan struct{}),
		restart:       make(chan struct{}, 1),
	}
}

// SetBridgeIP updates the bridge IP address.
// Used during pairing, before an application key exists.
func (c *Client) SetBridgeIP(ip string) {
	c.cfgMu.Lock()
	defer c.cfgMu.Unlock()

	c.bridgeIP = ip
	c.baseURL = fmt.Sprintf("https://%s", ip)
}

// SetApplicationKey updates the application key
func (c *Client) SetApplicationKey(key string) {
	c.cfgMu.Lock()
	defer c.cfgMu.Unlock()

	c.applicationKey = key
}

// Configure points the client at a bridge and makes the event stream pick up
// the change. Call it after pairing or after restoring a backup, otherwise the
// stream keeps talking to the previously configured bridge.
func (c *Client) Configure(bridgeIP, applicationKey string) {
	c.cfgMu.Lock()
	c.bridgeIP = bridgeIP
	c.applicationKey = applicationKey
	c.baseURL = fmt.Sprintf("https://%s", bridgeIP)
	c.cfgMu.Unlock()

	select {
	case c.restart <- struct{}{}:
	default: // a restart is already pending
	}
}

// endpoint returns the current base URL and application key.
func (c *Client) endpoint() (baseURL, applicationKey string) {
	c.cfgMu.RLock()
	defer c.cfgMu.RUnlock()
	return c.baseURL, c.applicationKey
}

// BridgeIP returns the configured bridge address
func (c *Client) BridgeIP() string {
	c.cfgMu.RLock()
	defer c.cfgMu.RUnlock()
	return c.bridgeIP
}

// IsConfigured returns true if the client has bridge IP and application key
func (c *Client) IsConfigured() bool {
	c.cfgMu.RLock()
	defer c.cfgMu.RUnlock()
	return c.bridgeIP != "" && c.applicationKey != ""
}

// request performs an HTTP request to the HUE Bridge API
func (c *Client) request(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	baseURL, applicationKey := c.endpoint()

	url := fmt.Sprintf("%s%s", baseURL, path)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if applicationKey != "" {
		req.Header.Set("hue-application-key", applicationKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HUE API error: %s - %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

// Pair attempts to create a new application key by pressing the bridge button
func (c *Client) Pair(appName, instanceName string) (string, error) {
	body := map[string]interface{}{
		"devicetype":        fmt.Sprintf("%s#%s", appName, instanceName),
		"generateclientkey": true,
	}

	log.Info().Str("bridge_ip", c.BridgeIP()).Msg("Attempting to pair with HUE Bridge")

	resp, err := c.request("POST", "/api", body)
	if err != nil {
		log.Error().Err(err).Str("bridge_ip", c.BridgeIP()).Msg("Failed to connect to HUE Bridge")
		return "", fmt.Errorf("connection to bridge failed: %v", err)
	}

	log.Debug().Str("response", string(resp)).Msg("Pairing response from bridge")

	var result []map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("invalid response from bridge: %v", err)
	}

	if len(result) == 0 {
		return "", fmt.Errorf("empty response from bridge")
	}

	// The response comes from whatever host bridge_ip points at, so every field
	// is untrusted: assert types instead of assuming the HUE schema.
	if errInfo, ok := result[0]["error"]; ok {
		errMap, _ := errInfo.(map[string]interface{})
		errType := 0
		if t, ok := errMap["type"].(float64); ok {
			errType = int(t)
		}
		errDesc := "unknown error"
		if d, ok := errMap["description"].(string); ok {
			errDesc = d
		}
		log.Warn().Int("error_type", errType).Str("description", errDesc).Msg("Bridge pairing error")
		// Error type 101 = link button not pressed
		if errType == 101 {
			return "", fmt.Errorf("link button not pressed - please press the link button on your HUE Bridge and try again within 30 seconds")
		}
		return "", fmt.Errorf("pairing error: %s", errDesc)
	}

	if success, ok := result[0]["success"]; ok {
		if successMap, ok := success.(map[string]interface{}); ok {
			if username, ok := successMap["username"].(string); ok && username != "" {
				c.SetApplicationKey(username)
				log.Info().Msg("Successfully paired with HUE Bridge")
				return username, nil
			}
		}
	}

	return "", fmt.Errorf("unexpected response: %s", string(resp))
}

// GetBridgeInfo returns information about the bridge
func (c *Client) GetBridgeInfo() (map[string]interface{}, error) {
	resp, err := c.request("GET", "/clip/v2/resource/bridge", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetLights fetches all lights from the bridge
func (c *Client) GetLights() ([]*models.Light, error) {
	resp, err := c.request("GET", "/clip/v2/resource/light", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []hueLight `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	lights := make([]*models.Light, 0, len(result.Data))
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, hl := range result.Data {
		light := convertHueLight(hl)
		c.lights[light.ID] = light
		lights = append(lights, cloneLight(light))
	}

	log.Debug().Int("count", len(lights)).Msg("Fetched lights from bridge")
	return lights, nil
}

// GetLight fetches a single light by ID
func (c *Client) GetLight(id string) (*models.Light, error) {
	c.mu.RLock()
	if light, ok := c.lights[id]; ok {
		c.mu.RUnlock()
		return cloneLight(light), nil
	}
	c.mu.RUnlock()

	resp, err := c.request("GET", fmt.Sprintf("/clip/v2/resource/light/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []hueLight `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("light not found: %s", id)
	}

	light := convertHueLight(result.Data[0])
	c.mu.Lock()
	c.lights[light.ID] = light
	c.mu.Unlock()

	return cloneLight(light), nil
}

// SetLightState updates the state of a light
func (c *Client) SetLightState(id string, cmd models.DeviceCommand) error {
	body := make(map[string]interface{})

	if cmd.On != nil {
		body["on"] = map[string]bool{"on": *cmd.On}
	}
	if cmd.Brightness != nil {
		body["dimming"] = map[string]float64{"brightness": *cmd.Brightness}
	}
	if cmd.ColorTemp != nil {
		body["color_temperature"] = map[string]int{"mirek": *cmd.ColorTemp}
	}
	if cmd.Color != nil {
		body["color"] = map[string]interface{}{
			"xy": map[string]float64{
				"x": cmd.Color.XY[0],
				"y": cmd.Color.XY[1],
			},
		}
	}

	_, err := c.request("PUT", fmt.Sprintf("/clip/v2/resource/light/%s", id), body)
	if err != nil {
		return err
	}

	log.Debug().Str("id", id).Interface("command", cmd).Msg("Light state updated")
	return nil
}

// GetGroups fetches all rooms and zones from the bridge
func (c *Client) GetGroups() ([]*models.Group, error) {
	groups := make([]*models.Group, 0)

	// First fetch lights to build device-to-light mapping
	deviceToLightID := make(map[string]string)
	lightsResp, err := c.request("GET", "/clip/v2/resource/light", nil)
	if err == nil {
		var lightsResult struct {
			Data []hueLight `json:"data"`
		}
		if err := json.Unmarshal(lightsResp, &lightsResult); err == nil {
			for _, hl := range lightsResult.Data {
				if hl.Owner != nil && hl.Owner.RType == "device" {
					deviceToLightID[hl.Owner.RID] = hl.ID
				}
			}
		}
	}
	log.Debug().Int("mappings", len(deviceToLightID)).Msg("Built device-to-light mapping")

	// Fetch grouped_light states
	groupedLightStates := make(map[string]models.GroupState)
	glResp, err := c.request("GET", "/clip/v2/resource/grouped_light", nil)
	if err == nil {
		var glResult struct {
			Data []struct {
				ID    string `json:"id"`
				Owner struct {
					RID string `json:"rid"`
				} `json:"owner"`
				On *struct {
					On bool `json:"on"`
				} `json:"on"`
				Dimming *struct {
					Brightness float64 `json:"brightness"`
				} `json:"dimming"`
			} `json:"data"`
		}
		if err := json.Unmarshal(glResp, &glResult); err == nil {
			for _, gl := range glResult.Data {
				state := models.GroupState{}
				if gl.On != nil {
					state.AnyOn = gl.On.On
					state.AllOn = gl.On.On
				}
				if gl.Dimming != nil {
					state.Brightness = gl.Dimming.Brightness
				}
				groupedLightStates[gl.Owner.RID] = state
			}
		}
	}

	// Fetch rooms
	roomsResp, err := c.request("GET", "/clip/v2/resource/room", nil)
	if err != nil {
		return nil, err
	}

	var roomsResult struct {
		Data []hueRoom `json:"data"`
	}
	if err := json.Unmarshal(roomsResp, &roomsResult); err != nil {
		return nil, err
	}

	c.mu.Lock()
	for _, hr := range roomsResult.Data {
		group := convertHueRoom(hr, deviceToLightID)
		// Apply state from grouped_light
		if state, ok := groupedLightStates[group.ID]; ok {
			group.State = state
		}
		c.storeGroupLocked(group)
		groups = append(groups, cloneGroup(group))
	}
	c.mu.Unlock()

	// Fetch zones
	zonesResp, err := c.request("GET", "/clip/v2/resource/zone", nil)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch zones")
	} else {
		var zonesResult struct {
			Data []hueRoom `json:"data"`
		}
		if err := json.Unmarshal(zonesResp, &zonesResult); err == nil {
			c.mu.Lock()
			for _, hz := range zonesResult.Data {
				group := convertHueRoom(hz, deviceToLightID)
				group.Type = "zone"
				// Apply state from grouped_light
				if state, ok := groupedLightStates[group.ID]; ok {
					group.State = state
				}
				c.storeGroupLocked(group)
				groups = append(groups, cloneGroup(group))
			}
			c.mu.Unlock()
		}
	}

	log.Debug().Int("count", len(groups)).Msg("Fetched groups from bridge")
	return groups, nil
}

// SetGroupState updates the state of all lights in a group
func (c *Client) SetGroupState(id string, cmd models.DeviceCommand) error {
	log.Debug().Str("group_id", id).Interface("command", cmd).Msg("SetGroupState called")

	// The room's services list already names its grouped_light; only fall
	// back to listing every grouped_light when the cache does not know it.
	c.mu.RLock()
	var groupedLightID string
	if g, ok := c.groups[id]; ok {
		groupedLightID = g.GroupedLightID
	}
	c.mu.RUnlock()

	if groupedLightID == "" {
		var err error
		groupedLightID, err = c.lookupGroupedLight(id)
		if err != nil {
			return err
		}
	}

	body := make(map[string]interface{})
	if cmd.On != nil {
		body["on"] = map[string]bool{"on": *cmd.On}
	}
	if cmd.Brightness != nil {
		body["dimming"] = map[string]float64{"brightness": *cmd.Brightness}
	}

	log.Debug().Str("grouped_light_id", groupedLightID).Interface("body", body).Msg("Sending PUT request")

	if _, err := c.request("PUT", fmt.Sprintf("/clip/v2/resource/grouped_light/%s", groupedLightID), body); err != nil {
		log.Error().Err(err).Msg("Failed to update grouped_light")
		return err
	}

	log.Debug().Str("group_id", id).Msg("Group state updated successfully")
	return nil
}

// lookupGroupedLight asks the bridge which grouped_light belongs to a room or
// zone and remembers the answer in the cache.
func (c *Client) lookupGroupedLight(groupID string) (string, error) {
	resp, err := c.request("GET", "/clip/v2/resource/grouped_light", nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch grouped_lights")
		return "", err
	}

	var result struct {
		Data []struct {
			ID    string `json:"id"`
			Owner struct {
				RID string `json:"rid"`
			} `json:"owner"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Error().Err(err).Msg("Failed to parse grouped_lights")
		return "", err
	}

	for _, gl := range result.Data {
		if gl.Owner.RID != groupID {
			continue
		}
		c.mu.Lock()
		if g, ok := c.groups[groupID]; ok {
			g.GroupedLightID = gl.ID
		}
		c.groupedLights[gl.ID] = groupID
		c.mu.Unlock()
		return gl.ID, nil
	}

	log.Error().Str("group_id", groupID).Msg("grouped_light not found for group")
	return "", fmt.Errorf("grouped_light not found for group: %s", groupID)
}

// GetScenes fetches all scenes from the bridge
func (c *Client) GetScenes() ([]*models.Scene, error) {
	resp, err := c.request("GET", "/clip/v2/resource/scene", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []hueScene `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	scenes := make([]*models.Scene, 0, len(result.Data))
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, hs := range result.Data {
		scene := convertHueScene(hs)
		c.scenes[scene.ID] = scene
		scenes = append(scenes, scene)
	}

	log.Debug().Int("count", len(scenes)).Msg("Fetched scenes from bridge")
	return scenes, nil
}

// ActivateScene activates a scene
func (c *Client) ActivateScene(id string) error {
	body := map[string]interface{}{
		"recall": map[string]string{
			"action": "active",
		},
	}

	_, err := c.request("PUT", fmt.Sprintf("/clip/v2/resource/scene/%s", id), body)
	if err != nil {
		return err
	}

	log.Debug().Str("id", id).Msg("Scene activated")
	return nil
}

// storeGroupLocked puts a group into the cache and indexes its grouped_light.
// The caller holds c.mu.
func (c *Client) storeGroupLocked(group *models.Group) {
	if old, ok := c.groups[group.ID]; ok && old.GroupedLightID != "" && old.GroupedLightID != group.GroupedLightID {
		delete(c.groupedLights, old.GroupedLightID)
	}
	c.groups[group.ID] = group
	if group.GroupedLightID != "" {
		c.groupedLights[group.GroupedLightID] = group.ID
	}
}

// GroupIDForGroupedLight returns the room or zone that owns a grouped_light
// service, or "" if the cache does not know it.
func (c *Client) GroupIDForGroupedLight(groupedLightID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.groupedLights[groupedLightID]
}

// LightSnapshot returns a copy of a cached light. It never talks to the
// bridge, so it is safe to call from the event loop, and the copy can be
// marshalled without racing against event updates to the cache.
func (c *Client) LightSnapshot(id string) (models.Light, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	light, ok := c.lights[id]
	if !ok {
		return models.Light{}, false
	}
	return *cloneLight(light), true
}

// GroupSnapshot returns a copy of a cached room or zone, see LightSnapshot.
func (c *Client) GroupSnapshot(id string) (models.Group, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	group, ok := c.groups[id]
	if !ok {
		return models.Group{}, false
	}
	return *cloneGroup(group), true
}

// The cache hands out copies, never its own objects: handlers JSON-encode what
// they get while updateFromEvent mutates the cached originals under c.mu.

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneLight(l *models.Light) *models.Light {
	c := *l
	c.State.Color = clonePtr(l.State.Color)
	return &c
}

func cloneGroup(g *models.Group) *models.Group {
	c := *g
	c.Lights = append([]string(nil), g.Lights...)
	c.Scenes = append([]models.Scene(nil), g.Scenes...)
	return &c
}

func cloneSensorState(s models.SensorState) models.SensorState {
	s.Motion = clonePtr(s.Motion)
	s.Temperature = clonePtr(s.Temperature)
	s.LightLevel = clonePtr(s.LightLevel)
	s.ButtonEvent = clonePtr(s.ButtonEvent)
	s.ControlID = clonePtr(s.ControlID)
	s.ContactState = clonePtr(s.ContactState)
	s.RotaryAction = clonePtr(s.RotaryAction)
	s.RotarySteps = clonePtr(s.RotarySteps)
	s.BatteryLevel = clonePtr(s.BatteryLevel)
	s.BatteryState = clonePtr(s.BatteryState)
	return s
}

func cloneSensor(sn *models.Sensor) *models.Sensor {
	c := *sn
	c.State = cloneSensorState(sn.State)
	return &c
}

// DroppedEvents reports how many bridge events were discarded because the
// event consumer could not keep up.
func (c *Client) DroppedEvents() int64 {
	return c.droppedEvents.Load()
}

// GetGroupName returns the name of a group by ID, or empty string if not found
func (c *Client) GetGroupName(groupID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if g, ok := c.groups[groupID]; ok {
		return g.Name
	}
	return ""
}

// GetSensors fetches all sensors (motion, temperature, light_level, button, contact, rotary, device_power) from the bridge
func (c *Client) GetSensors() ([]*models.Sensor, error) {
	// First build device ID → name mapping via /clip/v2/resource/device
	deviceNames := make(map[string]string)
	devResp, err := c.request("GET", "/clip/v2/resource/device", nil)
	if err == nil {
		var devResult struct {
			Data []struct {
				ID       string `json:"id"`
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
			} `json:"data"`
		}
		if err := json.Unmarshal(devResp, &devResult); err == nil {
			for _, d := range devResult.Data {
				deviceNames[d.ID] = d.Metadata.Name
			}
		}
	}

	sensors := make([]*models.Sensor, 0)

	type sensorEndpoint struct {
		path       string
		sensorType string
	}

	endpoints := []sensorEndpoint{
		{"/clip/v2/resource/motion", "motion"},
		{"/clip/v2/resource/temperature", "temperature"},
		{"/clip/v2/resource/light_level", "light_level"},
		{"/clip/v2/resource/button", "button"},
		{"/clip/v2/resource/contact", "contact"},
		{"/clip/v2/resource/relative_rotary", "relative_rotary"},
		{"/clip/v2/resource/device_power", "device_power"},
	}

	// Fetch everything before touching the cache. Holding the write lock across
	// these seven sequential requests blocked every reader — GetSensorName and
	// friends are called from the event stream — for as long as the bridge took
	// to answer, up to the full request timeout per endpoint.
	for _, ep := range endpoints {
		resp, err := c.request("GET", ep.path, nil)
		if err != nil {
			log.Debug().Err(err).Str("type", ep.sensorType).Msg("Sensor type not available on this bridge")
			continue
		}

		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			continue
		}

		for _, raw := range result.Data {
			if sensor := convertHueSensor(raw, ep.sensorType, deviceNames); sensor != nil {
				sensors = append(sensors, sensor)
			}
		}
	}

	result := make([]*models.Sensor, 0, len(sensors))
	c.mu.Lock()
	for _, sensor := range sensors {
		c.sensors[sensor.ID] = sensor
		result = append(result, cloneSensor(sensor))
	}
	c.mu.Unlock()

	log.Debug().Int("count", len(result)).Msg("Fetched sensors from bridge")
	return result, nil
}

// GetSensor returns a cached sensor by ID
func (c *Client) GetSensor(id string) *models.Sensor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s, ok := c.sensors[id]; ok {
		return cloneSensor(s)
	}
	return nil
}

// GetSensorName returns the name of a sensor by ID, or empty string
func (c *Client) GetSensorName(sensorID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s, ok := c.sensors[sensorID]; ok {
		return s.Name
	}
	return ""
}

// Events returns the event channel for SSE events
func (c *Client) Events() <-chan Event {
	return c.eventChan
}

// Close stops the client and closes all connections. Safe to call more than once.
func (c *Client) Close() {
	c.closeOnce.Do(func() { close(c.stopChan) })
}

// Internal HUE API response types
type hueLight struct {
	ID    string `json:"id"`
	Owner *struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"owner,omitempty"`
	Metadata struct {
		Name      string `json:"name"`
		Archetype string `json:"archetype"`
	} `json:"metadata"`
	ProductData struct {
		ModelID     string `json:"model_id"`
		ProductName string `json:"product_name"`
	} `json:"product_data"`
	On struct {
		On bool `json:"on"`
	} `json:"on"`
	Dimming *struct {
		Brightness float64 `json:"brightness"`
	} `json:"dimming,omitempty"`
	ColorTemperature *struct {
		Mirek      int  `json:"mirek"`
		MirekValid bool `json:"mirek_valid"`
	} `json:"color_temperature,omitempty"`
	Color *struct {
		XY struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
		Gamut struct {
			Red   struct{ X, Y float64 } `json:"red"`
			Green struct{ X, Y float64 } `json:"green"`
			Blue  struct{ X, Y float64 } `json:"blue"`
		} `json:"gamut"`
		GamutType string `json:"gamut_type"`
	} `json:"color,omitempty"`
}

type hueRoom struct {
	ID       string `json:"id"`
	Metadata struct {
		Name      string `json:"name"`
		Archetype string `json:"archetype"`
	} `json:"metadata"`
	Children []struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"children"`
	Services []struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"services"`
}

type hueScene struct {
	ID       string `json:"id"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Group struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"group"`
}

func convertHueLight(hl hueLight) *models.Light {
	light := &models.Light{
		ID:          hl.ID,
		Name:        hl.Metadata.Name,
		Type:        hl.Metadata.Archetype,
		ModelID:     hl.ProductData.ModelID,
		ProductName: hl.ProductData.ProductName,
		State: models.LightState{
			On:        hl.On.On,
			Reachable: true,
		},
		Capabilities: models.Capabilities{},
	}

	if hl.Dimming != nil {
		light.State.Brightness = hl.Dimming.Brightness
		light.Capabilities.SupportsDimming = true
	}

	if hl.ColorTemperature != nil && hl.ColorTemperature.MirekValid {
		light.State.ColorTemp = hl.ColorTemperature.Mirek
		light.Capabilities.SupportsColorTemp = true
	}

	if hl.Color != nil {
		light.State.Color = &models.Color{
			XY:    [2]float64{hl.Color.XY.X, hl.Color.XY.Y},
			Gamut: hl.Color.GamutType,
		}
		light.Capabilities.SupportsColor = true
	}

	return light
}

func convertHueRoom(hr hueRoom, deviceToLightID map[string]string) *models.Group {
	group := &models.Group{
		ID:     hr.ID,
		Name:   hr.Metadata.Name,
		Type:   "room",
		Lights: make([]string, 0),
	}

	for _, child := range hr.Children {
		if child.RType == "device" {
			// Map device ID to light ID
			if lightID, ok := deviceToLightID[child.RID]; ok {
				group.Lights = append(group.Lights, lightID)
			}
		}
	}

	for _, svc := range hr.Services {
		if svc.RType == "grouped_light" {
			group.GroupedLightID = svc.RID
			break
		}
	}

	return group
}

func convertHueScene(hs hueScene) *models.Scene {
	return &models.Scene{
		ID:      hs.ID,
		Name:    hs.Metadata.Name,
		GroupID: hs.Group.RID,
		Type:    hs.Group.RType,
	}
}

// convertHueSensor converts raw JSON sensor data into a Sensor model
func convertHueSensor(raw json.RawMessage, sensorType string, deviceNames map[string]string) *models.Sensor {
	// Common fields present in all sensor types
	var base struct {
		ID    string `json:"id"`
		Owner *struct {
			RID   string `json:"rid"`
			RType string `json:"rtype"`
		} `json:"owner,omitempty"`
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil
	}

	sensor := &models.Sensor{
		ID:   base.ID,
		Type: sensorType,
		State: models.SensorState{
			Enabled: base.Enabled,
		},
	}

	if base.Owner != nil {
		sensor.DeviceID = base.Owner.RID
		if name, ok := deviceNames[base.Owner.RID]; ok {
			sensor.Owner = name
			sensor.Name = name
		}
	}

	switch sensorType {
	case "motion":
		var m struct {
			Motion struct {
				Motion       bool `json:"motion"`
				MotionValid  bool `json:"motion_valid"`
				MotionReport *struct {
					Changed string `json:"changed"`
					Motion  bool   `json:"motion"`
				} `json:"motion_report,omitempty"`
			} `json:"motion"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &m); err == nil {
			motion := m.Motion.Motion
			sensor.State.Motion = &motion
			sensor.State.Enabled = m.Enabled
			if m.Motion.MotionReport != nil {
				sensor.State.LastUpdated = m.Motion.MotionReport.Changed
			}
			sensor.Name = sensor.Name + " Motion"
		}

	case "temperature":
		var t struct {
			Temperature struct {
				Temperature       float64 `json:"temperature"`
				TemperatureValid  bool    `json:"temperature_valid"`
				TemperatureReport *struct {
					Changed     string  `json:"changed"`
					Temperature float64 `json:"temperature"`
				} `json:"temperature_report,omitempty"`
			} `json:"temperature"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &t); err == nil {
			temp := t.Temperature.Temperature
			sensor.State.Temperature = &temp
			sensor.State.Enabled = t.Enabled
			if t.Temperature.TemperatureReport != nil {
				sensor.State.LastUpdated = t.Temperature.TemperatureReport.Changed
			}
			sensor.Name = sensor.Name + " Temperature"
		}

	case "light_level":
		var l struct {
			Light struct {
				LightLevel       int  `json:"light_level"`
				LightLevelValid  bool `json:"light_level_valid"`
				LightLevelReport *struct {
					Changed    string `json:"changed"`
					LightLevel int    `json:"light_level"`
				} `json:"light_level_report,omitempty"`
			} `json:"light"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &l); err == nil {
			level := l.Light.LightLevel
			sensor.State.LightLevel = &level
			sensor.State.Enabled = l.Enabled
			if l.Light.LightLevelReport != nil {
				sensor.State.LastUpdated = l.Light.LightLevelReport.Changed
			}
			sensor.Name = sensor.Name + " Light Level"
		}

	case "button":
		var b struct {
			Metadata struct {
				ControlID int `json:"control_id"`
			} `json:"metadata"`
			Button struct {
				ButtonReport *struct {
					Updated string `json:"updated"`
					Event   string `json:"event"`
				} `json:"button_report,omitempty"`
				LastEvent string `json:"last_event"`
			} `json:"button"`
		}
		if err := json.Unmarshal(raw, &b); err == nil {
			controlID := b.Metadata.ControlID
			sensor.State.ControlID = &controlID
			event := b.Button.LastEvent
			if b.Button.ButtonReport != nil {
				event = b.Button.ButtonReport.Event
				sensor.State.LastUpdated = b.Button.ButtonReport.Updated
			}
			if event != "" {
				sensor.State.ButtonEvent = &event
			}
			sensor.Name = fmt.Sprintf("%s Button %d", sensor.Name, controlID)
		}

	case "contact":
		var ct struct {
			ContactReport *struct {
				Changed string `json:"changed"`
				State   string `json:"state"`
			} `json:"contact_report,omitempty"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &ct); err == nil {
			sensor.State.Enabled = ct.Enabled
			if ct.ContactReport != nil {
				sensor.State.ContactState = &ct.ContactReport.State
				sensor.State.LastUpdated = ct.ContactReport.Changed
			}
			sensor.Name = sensor.Name + " Contact"
		}

	case "relative_rotary":
		var rr struct {
			RelativeRotary struct {
				RotaryReport *struct {
					Updated  string `json:"updated"`
					Action   string `json:"action"`
					Rotation struct {
						Direction string `json:"direction"`
						Steps     int    `json:"steps"`
						Duration  int    `json:"duration"`
					} `json:"rotation"`
				} `json:"rotary_report,omitempty"`
				LastEvent *struct {
					Action   string `json:"action"`
					Rotation struct {
						Direction string `json:"direction"`
						Steps     int    `json:"steps"`
					} `json:"rotation"`
				} `json:"last_event,omitempty"`
			} `json:"relative_rotary"`
		}
		if err := json.Unmarshal(raw, &rr); err == nil {
			if rr.RelativeRotary.RotaryReport != nil {
				sensor.State.RotaryAction = &rr.RelativeRotary.RotaryReport.Action
				steps := rr.RelativeRotary.RotaryReport.Rotation.Steps
				if rr.RelativeRotary.RotaryReport.Rotation.Direction == "counter_clockwise" {
					steps = -steps
				}
				sensor.State.RotarySteps = &steps
				sensor.State.LastUpdated = rr.RelativeRotary.RotaryReport.Updated
			}
			sensor.Name = sensor.Name + " Rotary"
		}

	case "device_power":
		var dp struct {
			PowerState struct {
				BatteryLevel int    `json:"battery_level"`
				BatteryState string `json:"battery_state"`
			} `json:"power_state"`
		}
		if err := json.Unmarshal(raw, &dp); err == nil {
			sensor.State.BatteryLevel = &dp.PowerState.BatteryLevel
			sensor.State.BatteryState = &dp.PowerState.BatteryState
			sensor.Name = sensor.Name + " Battery"
		}
	}

	// Fallback name if device not found
	if sensor.Name == "" {
		shortID := sensor.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		sensor.Name = fmt.Sprintf("Sensor %s", shortID)
	}

	return sensor
}
