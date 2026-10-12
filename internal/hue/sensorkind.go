package hue

// Sensor kinds group HUE resource types by the payload they carry. The bridge
// reports motion and light level both per device and aggregated per room or
// home ("grouped_*"); the aggregated resources use the same JSON as the device
// ones, so everything that parses a sensor switches on the kind, not the type.
const (
	KindMotion      = "motion"
	KindTemperature = "temperature"
	KindLightLevel  = "light_level"
	KindButton      = "button"
	KindContact     = "contact"
	KindRotary      = "relative_rotary"
	KindPower       = "device_power"
)

// sensorKinds maps every supported sensor resource type to its kind.
var sensorKinds = map[string]string{
	"motion":              KindMotion,
	"grouped_motion":      KindMotion,
	"temperature":         KindTemperature,
	"light_level":         KindLightLevel,
	"grouped_light_level": KindLightLevel,
	"button":              KindButton,
	"contact":             KindContact,
	"relative_rotary":     KindRotary,
	"device_power":        KindPower,
}

// SensorKind returns the kind of a sensor resource type, or "" for types that
// are not sensors.
func SensorKind(resourceType string) string {
	return sensorKinds[resourceType]
}

// IsSensorType returns true if the resource type is a sensor
func IsSensorType(resourceType string) bool {
	_, ok := sensorKinds[resourceType]
	return ok
}
