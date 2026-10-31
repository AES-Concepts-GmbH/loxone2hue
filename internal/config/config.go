package config

import (
	"crypto/subtle"
	"os"
	"path/filepath"
	"sync"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig     `yaml:"server" json:"server"`
	Auth     AuthConfig       `yaml:"auth" json:"auth"`
	Hue      HueConfig        `yaml:"hue" json:"hue"`
	Loxone   LoxoneConfig     `yaml:"loxone" json:"loxone"`
	Logging  LoggingConfig    `yaml:"logging" json:"logging"`
	Mappings []models.Mapping `yaml:"mappings" json:"mappings"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port int    `yaml:"port" json:"port"`
	Host string `yaml:"host" json:"host"`
}

// HueConfig holds HUE bridge settings
type HueConfig struct {
	BridgeIP       string `yaml:"bridge_ip" json:"bridge_ip"`
	ApplicationKey string `yaml:"application_key" json:"application_key"`
}

// MiniserverConfig holds settings for a single Loxone Miniserver
type MiniserverConfig struct {
	ID           string `yaml:"id" json:"id"`
	Name         string `yaml:"name" json:"name"`
	IP           string `yaml:"ip" json:"ip"`
	Port         int    `yaml:"port" json:"port"`
	UDPEnabled   bool   `yaml:"udp_enabled" json:"udp_enabled"`
	HTTPEnabled  bool   `yaml:"http_enabled" json:"http_enabled"`
	HTTPURL      string `yaml:"http_url" json:"http_url"`
	HTTPUser     string `yaml:"http_user" json:"http_user"`
	HTTPPassword string `yaml:"http_password" json:"http_password"`
	// HTTPVerifyTLS makes the HTTP sender check the Miniserver's certificate
	// when http_url is https. Off by default: Miniservers ship self-signed
	// certificates, and existing installations must keep working.
	HTTPVerifyTLS bool `yaml:"http_verify_tls,omitempty" json:"http_verify_tls"`
	SendAll       bool `yaml:"send_all" json:"send_all"`
}

// LoxoneConfig holds Loxone integration settings
type LoxoneConfig struct {
	Miniservers []MiniserverConfig `yaml:"miniservers" json:"miniservers"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
}

// AuthConfig holds the credentials guarding the web UI and API.
// No password disables authentication, which keeps existing installations
// — and the Home Assistant Ingress setup — working unchanged.
//
// Password is only ever read from a hand-edited file: Load hashes it into
// PasswordHash, clears it and writes the file back, so the plain text does
// not stay on disk or end up in backups. UpdateAuth does the same for a
// password set through the API.
type AuthConfig struct {
	Username     string `yaml:"username" json:"username"`
	Password     string `yaml:"password,omitempty" json:"-"`
	PasswordHash string `yaml:"password_hash,omitempty" json:"-"`
}

// Enabled reports whether authentication is configured.
func (a AuthConfig) Enabled() bool {
	return a.PasswordHash != "" || a.Password != ""
}

// Verify reports whether password matches the stored credential. It accepts
// the bcrypt hash and, until the first save, a not yet hashed plain text.
func (a AuthConfig) Verify(password string) bool {
	if a.PasswordHash != "" {
		return bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) == nil
	}
	if a.Password != "" {
		return subtle.ConstantTimeCompare([]byte(a.Password), []byte(password)) == 1
	}
	return false
}

// BcryptCost is the work factor for new password hashes. Cost 10 keeps a
// verification around 100 ms on a Raspberry Pi; the API caches verified
// credentials, so Loxone commands pay it once per credential, not per call.
// Tests lower it to bcrypt.MinCost.
var BcryptCost = bcrypt.DefaultCost

// HashPassword returns the bcrypt hash stored for a password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// hashPlaintext replaces a plain-text password by its hash, in place.
func (a *AuthConfig) hashPlaintext() error {
	if a.Password == "" {
		return nil
	}
	hash, err := HashPassword(a.Password)
	if err != nil {
		return err
	}
	a.PasswordHash = hash
	a.Password = ""
	return nil
}

// User returns the configured username, defaulting to "admin".
func (a AuthConfig) User() string {
	if a.Username == "" {
		return "admin"
	}
	return a.Username
}

var (
	cfg        *Config
	cfgOnce    sync.Once
	cfgPath    string
	mu         sync.RWMutex
	appVersion string
)

// SetVersion sets the application version for use in backups
func SetVersion(v string) {
	appVersion = v
}

// GetVersion returns the application version
func GetVersion() string {
	if appVersion == "" {
		return "dev"
	}
	return appVersion
}

// GetPath returns the current config file path
func GetPath() string {
	mu.RLock()
	defer mu.RUnlock()
	if cfgPath == "" {
		return "configs/config.yaml"
	}
	return cfgPath
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Hue: HueConfig{
			BridgeIP:       "",
			ApplicationKey: "",
		},
		Loxone: LoxoneConfig{
			Miniservers: []MiniserverConfig{},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Mappings: []models.Mapping{},
	}
}

// legacyLoxoneConfig is used to detect and migrate old config format
type legacyLoxoneConfig struct {
	Enabled      *bool  `yaml:"enabled"`
	MiniserverIP string `yaml:"miniserver_ip"`
	UDPFeedback  *struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
		SendAll bool `yaml:"send_all"`
	} `yaml:"udp_feedback"`
	Miniservers []MiniserverConfig `yaml:"miniservers"`
}

type legacyConfig struct {
	Loxone legacyLoxoneConfig `yaml:"loxone"`
}

// migrateConfig checks for old config format and migrates to new format
func migrateConfig(data []byte, cfg *Config) {
	var legacy legacyConfig
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return
	}

	// If new format already has miniservers, no migration needed
	if len(legacy.Loxone.Miniservers) > 0 {
		return
	}

	// Check if old format fields exist
	if legacy.Loxone.MiniserverIP == "" && legacy.Loxone.UDPFeedback == nil {
		return
	}

	log.Info().Msg("Migrating legacy Loxone config to multi-miniserver format")

	ip := legacy.Loxone.MiniserverIP
	port := 7777
	udpEnabled := false
	sendAll := false

	if legacy.Loxone.UDPFeedback != nil {
		udpEnabled = legacy.Loxone.UDPFeedback.Enabled
		if legacy.Loxone.UDPFeedback.Port > 0 {
			port = legacy.Loxone.UDPFeedback.Port
		}
		sendAll = legacy.Loxone.UDPFeedback.SendAll
	}

	if ip != "" {
		cfg.Loxone.Miniservers = []MiniserverConfig{
			{
				ID:         uuid.New().String(),
				Name:       "Miniserver",
				IP:         ip,
				Port:       port,
				UDPEnabled: udpEnabled,
				SendAll:    sendAll,
			},
		}
	}
}

// legacyMapping matches the old YAML field names (without yaml tags, go-yaml lowercases field names)
type legacyMapping struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	LoxoneID     string `yaml:"loxoneid"`
	HueID        string `yaml:"hueid"`
	HueType      string `yaml:"huetype"`
	Enabled      bool   `yaml:"enabled"`
	Description  string `yaml:"description"`
	MiniserverID string `yaml:"miniserverid"`
	// New format fields (for detection)
	LoxoneIDNew string `yaml:"loxone_id"`
	HueIDNew    string `yaml:"hue_id"`
	HueTypeNew  string `yaml:"hue_type"`
}

type legacyMappingsConfig struct {
	Mappings []legacyMapping `yaml:"mappings"`
}

// migrateMappingFields migrates mappings from legacy YAML field names to new format
// Old format (no yaml tags): loxoneid, hueid, huetype
// New format (with yaml tags): loxone_id, hue_id, hue_type
func migrateMappingFields(data []byte, cfg *Config) {
	var legacy legacyMappingsConfig
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return
	}

	if len(legacy.Mappings) == 0 || len(legacy.Mappings) != len(cfg.Mappings) {
		return
	}

	migrated := false
	for i, lm := range legacy.Mappings {
		// If new format fields are empty but old format fields have data, migrate
		if cfg.Mappings[i].LoxoneID == "" && lm.LoxoneID != "" {
			cfg.Mappings[i].LoxoneID = lm.LoxoneID
			migrated = true
		}
		if cfg.Mappings[i].HueID == "" && lm.HueID != "" {
			cfg.Mappings[i].HueID = lm.HueID
			migrated = true
		}
		if cfg.Mappings[i].HueType == "" && lm.HueType != "" {
			cfg.Mappings[i].HueType = lm.HueType
			migrated = true
		}
	}

	if migrated {
		log.Info().Msg("Migrated mapping fields from legacy YAML format (loxoneid -> loxone_id, etc.)")
	}
}

// Load reads the configuration from a YAML file
func Load(path string) (*Config, error) {
	cfgOnce.Do(func() {
		cfg = DefaultConfig()
	})

	mu.Lock()
	defer mu.Unlock()

	cfgPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Str("path", path).Msg("Config file not found, using defaults")
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Migrate legacy config if needed
	migrateConfig(data, cfg)

	// Ensure miniservers is never nil
	if cfg.Loxone.Miniservers == nil {
		cfg.Loxone.Miniservers = []MiniserverConfig{}
	}

	// Migrate mappings from legacy YAML field names (loxoneid -> loxone_id, etc.)
	migrateMappingFields(data, cfg)

	// Migrate mappings: assign first miniserver to mappings without miniserver_id
	if len(cfg.Loxone.Miniservers) > 0 {
		firstID := cfg.Loxone.Miniservers[0].ID
		for i := range cfg.Mappings {
			if cfg.Mappings[i].MiniserverID == "" {
				cfg.Mappings[i].MiniserverID = firstID
			}
		}
	}

	// A password typed into the file is hashed on first load and the file
	// rewritten, so the plain text does not stay on disk. A failed write only
	// logs: the password still works, it is just hashed again next start.
	if cfg.Auth.Password != "" {
		if err := cfg.Auth.hashPlaintext(); err != nil {
			return nil, err
		}
		if err := saveLocked(); err != nil {
			log.Warn().Err(err).Msg("Login password hashed in memory, but the configuration could not be rewritten")
		} else {
			log.Info().Msg("Login password replaced by its hash in the configuration file")
		}
	}

	log.Info().Str("path", path).Msg("Configuration loaded")
	return cfg, nil
}

// Snapshot returns a deep copy of the current configuration.
//
// There is deliberately no accessor that hands out the live *Config: callers
// used to read it without the lock while UpdateMappings, UpdateLoxone and
// RestoreFromBackup wrote under it, which is a data race. The copy costs a few
// allocations per API call and makes every read self-contained.
func Snapshot() Config {
	mu.RLock()
	defer mu.RUnlock()

	snap := *cfg
	snap.Loxone.Miniservers = append([]MiniserverConfig(nil), cfg.Loxone.Miniservers...)
	if snap.Loxone.Miniservers == nil {
		snap.Loxone.Miniservers = []MiniserverConfig{}
	}
	snap.Mappings = append([]models.Mapping(nil), cfg.Mappings...)
	return snap
}

// GetServer returns a copy of the HTTP server settings.
func GetServer() ServerConfig {
	mu.RLock()
	defer mu.RUnlock()
	return cfg.Server
}

// GetHue returns a copy of the HUE bridge settings, including the key.
func GetHue() HueConfig {
	mu.RLock()
	defer mu.RUnlock()
	return cfg.Hue
}

// Save writes the current configuration to the file.
//
// The write goes to a temporary file in the same directory and is then renamed
// into place, so an interrupted write cannot leave a truncated config behind.
// A full write lock is held because two concurrent saves would otherwise race
// for the same destination.
func Save() error {
	mu.Lock()
	defer mu.Unlock()
	return saveLocked()
}

// saveLocked writes the configuration; the caller holds mu.
func saveLocked() error { return saveValueLocked(cfg) }

// Commit persists a candidate before publishing it to readers. Callers serialize
// read-modify-write operations so candidates cannot overwrite newer changes.
func Commit(next Config) error {
	mu.Lock()
	defer mu.Unlock()
	if err := saveValueLocked(&next); err != nil {
		return err
	}
	cfg = &next
	return nil
}

func saveValueLocked(value *Config) error {
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}

	// The config holds the HUE application key and Miniserver passwords, so
	// CreateTemp's 0600 is exactly what the final file should have too.
	tmp, err := os.CreateTemp(filepath.Dir(cfgPath), ".config-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, cfgPath)
}

// UpdateHue updates the HUE configuration
func UpdateHue(bridgeIP, applicationKey string) {
	mu.Lock()
	defer mu.Unlock()

	cfg.Hue.BridgeIP = bridgeIP
	cfg.Hue.ApplicationKey = applicationKey
}

// GetAuth returns the authentication settings.
func GetAuth() AuthConfig {
	mu.RLock()
	defer mu.RUnlock()
	return cfg.Auth
}

// UpdateAuth replaces the authentication settings. A plain-text Password is
// hashed before it is stored; an AuthConfig without password disables the
// login.
func UpdateAuth(auth AuthConfig) error {
	if err := auth.hashPlaintext(); err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	cfg.Auth = auth
	return nil
}

// GetLoxone returns a copy of the Loxone configuration, including passwords.
func GetLoxone() LoxoneConfig {
	mu.RLock()
	defer mu.RUnlock()

	miniservers := make([]MiniserverConfig, len(cfg.Loxone.Miniservers))
	copy(miniservers, cfg.Loxone.Miniservers)
	return LoxoneConfig{Miniservers: miniservers}
}

// MaskedPassword stands in for a stored Loxone HTTP password whenever the
// configuration is handed to a client. Sending it back means "keep unchanged".
const MaskedPassword = "__unchanged__"

// UpdateLoxone replaces the Loxone configuration.
//
// A miniserver whose http_password is the MaskedPassword sentinel keeps the
// password already stored for that ID, so a plain round-trip through the UI
// does not wipe credentials. The merge happens under the write lock: reading
// the stored passwords in the caller and assigning afterwards left a window in
// which a concurrent save could observe a half-updated configuration.
func UpdateLoxone(loxone LoxoneConfig) {
	mu.Lock()
	defer mu.Unlock()

	stored := make(map[string]string, len(cfg.Loxone.Miniservers))
	for _, ms := range cfg.Loxone.Miniservers {
		stored[ms.ID] = ms.HTTPPassword
	}

	if loxone.Miniservers == nil {
		loxone.Miniservers = []MiniserverConfig{}
	}
	for i := range loxone.Miniservers {
		if loxone.Miniservers[i].HTTPPassword == MaskedPassword {
			loxone.Miniservers[i].HTTPPassword = stored[loxone.Miniservers[i].ID]
		}
	}

	cfg.Loxone = loxone
}

// UpdateMappings updates the mappings configuration
func UpdateMappings(mappings []models.Mapping) {
	mu.Lock()
	defer mu.Unlock()

	cfg.Mappings = mappings
}

// GetMappings returns a copy of current mappings
func GetMappings() []models.Mapping {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]models.Mapping, len(cfg.Mappings))
	copy(result, cfg.Mappings)
	return result
}

// RestoreFromBackup restores all configuration from a backup
func RestoreFromBackup(server ServerConfig, bridgeIP, appKey string, loxone LoxoneConfig, logging LoggingConfig, mappings []models.Mapping) {
	mu.Lock()
	defer mu.Unlock()

	cfg.Server = server
	cfg.Hue.BridgeIP = bridgeIP
	cfg.Hue.ApplicationKey = appKey
	cfg.Loxone = loxone
	cfg.Logging = logging
	cfg.Mappings = mappings
}
