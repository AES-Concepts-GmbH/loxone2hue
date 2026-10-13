package main

import (
	"context"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/api"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/logging"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		log.Info().Str("version", version).Str("build_time", buildTime).Msg("Loxone2HUE Gateway")
		os.Exit(0)
	}

	// Set version for backup metadata
	config.SetVersion(version)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", *configPath).Msg("Failed to load configuration")
	}

	// Setup logging
	setupLogging(cfg.Logging.Level, cfg.Logging.Format)

	log.Info().
		Str("version", version).
		Str("config", *configPath).
		Msg("Starting Loxone2HUE Gateway")

	// Create HUE client
	hueClient := hue.NewClient(cfg.Hue.BridgeIP, cfg.Hue.ApplicationKey)

	// Create mapping manager
	mappingManager := loxone.NewMappingManager()
	mappingManager.Load(cfg.Mappings)

	// Create UDP sender for Loxone status feedback
	udpSender := loxone.NewUDPSender()
	udpSender.Configure(cfg.Loxone.Miniservers)

	// Create HTTP sender for Loxone status feedback
	httpSender := loxone.NewHTTPSender()
	httpSender.Configure(cfg.Loxone.Miniservers)

	// Create API server
	server := api.NewServer(hueClient, mappingManager, udpSender, httpSender)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Supervise the HUE event stream for the whole process lifetime. It idles
	// until a bridge is configured, so pairing through the Web UI takes effect
	// without a restart.
	if hueClient.IsConfigured() {
		log.Info().Str("bridge_ip", cfg.Hue.BridgeIP).Msg("HUE Bridge configured, starting event stream")
	} else {
		log.Info().Msg("HUE Bridge not configured, waiting for pairing via Web UI")
	}
	go hueClient.RunEventStream(ctx)

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
	}()

	// Start HTTP server
	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Msg("Starting HTTP server")

	if err := server.Start(ctx, cfg.Server.Host, cfg.Server.Port); err != nil {
		log.Error().Err(err).Msg("Server error")
	}

	// Cleanup
	udpSender.Close()
	httpSender.Close()
	hueClient.Close()
	log.Info().Msg("Loxone2HUE Gateway stopped")
}

func setupLogging(level, format string) {
	// Set log level
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// The log buffer always receives JSON, so we create a JSON writer for it.
	// For stdout, we use the configured format.
	var stdoutWriter io.Writer = os.Stdout
	if format == "console" || format == "text" {
		stdoutWriter = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	// MultiWriter: stdout (human-readable or JSON) + in-memory buffer (always JSON)
	multi := io.MultiWriter(stdoutWriter, logging.Buffer)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()
}
