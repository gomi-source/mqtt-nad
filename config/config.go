// Package config loads mqtt-nad's runtime configuration from
// environment variables, so topics and broker connection details can
// be set per-environment without a code change.
package config

import "os"

// Config holds all runtime configuration for mqtt-nad.
type Config struct {
	// MQTT broker connection.
	BrokerURL string // e.g. "tcp://10.10.1.30:1883"
	ClientID  string
	Username  string
	Password  string

	// RawTelemetryTopic is the legacy topic the amplifier's telemetry
	// arrives on, in NAD's proprietary wire format (e.g. "tele/m33/raw").
	RawTelemetryTopic string

	// TelemetryTopicBase is the base under which granular telemetry is
	// published, one metric per subtopic (e.g. "tele/m33" ->
	// "tele/m33/volume", "tele/m33/mute", ...).
	TelemetryTopicBase string

	// CommandTopicBase is the base subscribed to for granular commands,
	// one metric per subtopic (e.g. "cmd/m33" -> "cmd/m33/volume",
	// "cmd/m33/source_1/input", ...).
	CommandTopicBase string

	// RawCommandTopic is the legacy topic formatted commands are
	// published to, in NAD's proprietary wire format (e.g.
	// "cmd/m33/raw").
	RawCommandTopic string
}

// Load reads Config from the environment, falling back to sensible
// defaults for the currently-deployed NAD M33 amplifier.
func Load() Config {
	return Config{
		BrokerURL: getEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		ClientID:  getEnv("MQTT_CLIENT_ID", "mqtt-nad"),
		Username:  os.Getenv("MQTT_USERNAME"),
		Password:  os.Getenv("MQTT_PASSWORD"),

		RawTelemetryTopic:  getEnv("RAW_TELEMETRY_TOPIC", "tele/m33/raw"),
		TelemetryTopicBase: getEnv("TELEMETRY_TOPIC_BASE", "tele/m33"),
		CommandTopicBase:   getEnv("COMMAND_TOPIC_BASE", "cmd/m33"),
		RawCommandTopic:    getEnv("RAW_COMMAND_TOPIC", "cmd/m33/raw"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
