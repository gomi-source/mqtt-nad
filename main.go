// Command mqtt-nad bridges a NAD M33 amplifier's proprietary RS-232
// telemetry/command stream - relayed as raw MQTT messages by an
// existing serial<->MQTT bridge - to and from granular per-metric MQTT
// topics.
//
//	tele/m33/raw  --(parse)-->  tele/m33/{metric}
//	cmd/m33/{metric}  --(format)-->  cmd/m33/raw
//
// All topics and the broker connection are configurable via
// environment variables; see config.Load.
package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"subsoma.com/services/mqtt-nad/config"
	"subsoma.com/services/mqtt-nad/mqtt"
	"subsoma.com/services/mqtt-nad/nad"
)

func main() {
	cfg := config.Load()

	client, err := mqtt.Connect(mqtt.Options{
		BrokerURL: cfg.BrokerURL,
		ClientID:  cfg.ClientID,
		Username:  cfg.Username,
		Password:  cfg.Password,
	})
	if err != nil {
		log.Fatalf("could not connect to mqtt broker %s: %v", cfg.BrokerURL, err)
	}
	defer client.Disconnect()

	if err := subscribeTelemetry(client, cfg); err != nil {
		log.Fatalf("%v", err)
	}
	if err := subscribeCommands(client, cfg); err != nil {
		log.Fatalf("%v", err)
	}

	log.Printf("mqtt-nad running: %s -> %s/{metric}, %s/{metric} -> %s",
		cfg.RawTelemetryTopic, cfg.TelemetryTopicBase, cfg.CommandTopicBase, cfg.RawCommandTopic)

	waitForShutdown()
	log.Println("shutting down")
}

// subscribeTelemetry parses raw telemetry from the amplifier and fans
// it out to granular tele/m33/{metric} topics.
func subscribeTelemetry(client *mqtt.Client, cfg config.Config) error {
	handler := func(_ string, payload string) {
		for _, reading := range nad.ParseTelemetry(payload) {
			if !reading.Recognized {
				log.Printf("unhandled telemetry: %s=%s", reading.Key, reading.Value)
				continue
			}

			topic := cfg.TelemetryTopicBase + "/" + reading.Metric
			if err := client.Publish(topic, 0, false, reading.Value); err != nil {
				log.Printf("failed to publish telemetry to %s: %v", topic, err)
				continue
			}
			log.Printf("telemetry: %s = %s", topic, reading.Value)
		}
	}

	if err := client.Subscribe(cfg.RawTelemetryTopic, 0, handler); err != nil {
		return err
	}
	return nil
}

// subscribeCommands formats granular cmd/m33/{metric} commands into
// the amplifier's raw wire format and publishes them to cmd/m33/raw.
func subscribeCommands(client *mqtt.Client, cfg config.Config) error {
	wildcard := cfg.CommandTopicBase + "/+"
	prefix := cfg.CommandTopicBase + "/"

	handler := func(topic string, payload string) {
		if topic == cfg.RawCommandTopic {
			// The wildcard subscription also matches our own raw
			// command topic; ignore anything published there so we
			// never try to reinterpret our own output as a command.
			return
		}

		metric := strings.TrimPrefix(topic, prefix)
		if metric == topic {
			// Doesn't match our own base topic; ignore.
			return
		}

		command, ok := nad.FormatCommand(metric, payload)
		if !ok {
			log.Printf("ignoring command for unknown metric %q (from %s)", metric, topic)
			return
		}
		raw := nad.Frame(command)

		if err := client.Publish(cfg.RawCommandTopic, 0, false, raw); err != nil {
			log.Printf("failed to publish command to %s: %v", cfg.RawCommandTopic, err)
			return
		}
		log.Printf("command: %s (from %s)", command, topic)
	}

	if err := client.Subscribe(wildcard, 0, handler); err != nil {
		return err
	}
	return nil
}

func waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
