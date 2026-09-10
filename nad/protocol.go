// Package nad implements the wire format used by NAD's RS-232 protocol
// v2.x (see the "RS-232 Protocol for NAD Products" datasheet), as
// relayed over MQTT by an existing serial<->MQTT bridge.
//
// Every command and response has the form:
//
//	<Prefix>.<Variable><Operator><Value><CR>(and/or)<LF>
//
// e.g. "Main.Volume=-3", "Main.Mute=On" (a response/telemetry line
// always uses the "=" operator). A command may instead use "?" to
// query a variable (no value), or "+"/"-" to toggle/step it by one
// (also no value).
//
// This package only knows about the fixed set of Main.* variables the
// M33 integration actually uses (mirroring the reference NAD amplifier
// integration this middleware replaces): Power, Volume, VolumePercent,
// Balance, Mute, Treble, Bass, Brightness, Source and Dirac. Telemetry
// for any other variable (including per-source Source{N}.* detail and
// the Main.Sources count) is intentionally left unpublished, exactly
// as in that reference implementation - it's read but not fanned out
// to a granular topic. Commands for a metric outside this set are
// rejected rather than guessed at.
package nad

import "strings"

// metricsByKey maps a wire-format key to the granular metric name
// (MQTT topic suffix) it's published/accepted under. keysByMetric is
// built from it at init time for the reverse direction.
var metricsByKey = map[string]string{
	"Main.Power":         "power",
	"Main.Volume":        "volume",
	"Main.VolumePercent": "volume_percent",
	"Main.Balance":       "balance",
	"Main.Mute":          "mute",
	"Main.Treble":        "treble",
	"Main.Bass":          "bass",
	"Main.Brightness":    "brightness",
	"Main.Source":        "source",
	"Main.Dirac":         "dirac",
}

var keysByMetric = func() map[string]string {
	m := make(map[string]string, len(metricsByKey))
	for key, metric := range metricsByKey {
		m[metric] = key
	}
	return m
}()

// Reading is one parsed telemetry line.
//
// Key and Value are exactly as received on the wire. Metric and
// Recognized are set when Key is one of the known variables above; for
// any other key, Recognized is false and Metric is empty, meaning the
// reading should be logged but not published, matching the reference
// implementation.
type Reading struct {
	Key        string
	Value      string
	Metric     string
	Recognized bool
}

// ParseTelemetry parses a raw message received on the amplifier's raw
// telemetry topic. A message may contain one or more lines separated
// by <CR> and/or <LF>. Lines that don't have the documented
// "Key=Value" form (including "?"/"+"/"-" operators, which carry no
// value) are silently dropped, per the datasheet: "Any data received
// which does not follow this format should be ignored."
func ParseTelemetry(raw string) []Reading {
	lines := splitLines(raw)

	readings := make([]Reading, 0, len(lines))
	for _, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}

		metric, recognized := metricsByKey[key]
		readings = append(readings, Reading{
			Key:        key,
			Value:      value,
			Metric:     metric,
			Recognized: recognized,
		})
	}
	return readings
}

// FormatCommand builds a wire-format command for the given metric and
// payload, and reports whether the metric is one this package knows
// how to command:
//   - an empty payload produces a query, e.g. "Main.Volume?"
//   - a payload of exactly "+" or "-" produces a toggle/step command,
//     e.g. "Main.Volume+"
//   - any other payload sets the value, e.g. "Main.Volume=-3"
func FormatCommand(metric, payload string) (command string, ok bool) {
	key, ok := keysByMetric[metric]
	if !ok {
		return "", false
	}

	switch payload {
	case "":
		return key + "?", true
	case "+", "-":
		return key + payload, true
	default:
		return key + "=" + payload, true
	}
}

// Frame wraps a command in the framing the amplifier's serial bridge
// expects on the raw command topic: a leading and trailing <CR><LF> to
// flush any noise ahead of the command (see protocol note 2).
func Frame(command string) string {
	return "\r\n" + command + "\r\n"
}

// splitLines normalizes <CR>/<LF>/<CR><LF> line endings and drops
// blank lines and surrounding whitespace.
func splitLines(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
