package nad

import (
	"reflect"
	"testing"
)

func TestParseTelemetry(t *testing.T) {
	// A batch of readings as they'd arrive from the amplifier, using
	// the datasheet's <CR>/<LF> framing, covering every known metric
	// plus a query echo (ignored, no value) and an unknown/unpublished
	// variable (recognized=false).
	raw := "Main.Power=On\r\nMain.Volume=-3\rMain.VolumePercent=60\n" +
		"Main.Balance=-3L\r\nMain.Mute=On\r\nMain.Treble=2\r\nMain.Bass=-1\r\n" +
		"Main.Brightness=50\r\nMain.Source=3\r\nMain.Dirac=1\r\n" +
		"Main.Model?\r\nMain.Sources=4\r\nSource1.Input=Wii\r\n\n"

	got := ParseTelemetry(raw)
	want := []Reading{
		{Key: "Main.Power", Value: "On", Metric: "power", Recognized: true},
		{Key: "Main.Volume", Value: "-3", Metric: "volume", Recognized: true},
		{Key: "Main.VolumePercent", Value: "60", Metric: "volume_percent", Recognized: true},
		{Key: "Main.Balance", Value: "-3L", Metric: "balance", Recognized: true},
		{Key: "Main.Mute", Value: "On", Metric: "mute", Recognized: true},
		{Key: "Main.Treble", Value: "2", Metric: "treble", Recognized: true},
		{Key: "Main.Bass", Value: "-1", Metric: "bass", Recognized: true},
		{Key: "Main.Brightness", Value: "50", Metric: "brightness", Recognized: true},
		{Key: "Main.Source", Value: "3", Metric: "source", Recognized: true},
		{Key: "Main.Dirac", Value: "1", Metric: "dirac", Recognized: true},
		{Key: "Main.Sources", Value: "4", Metric: "", Recognized: false},
		{Key: "Source1.Input", Value: "Wii", Metric: "", Recognized: false},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseTelemetry() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestParseTelemetryIgnoresMalformedLines(t *testing.T) {
	raw := "not a valid line\r\n=novalue\r\nMain.Volume=-3\r\n"

	got := ParseTelemetry(raw)
	want := []Reading{
		{Key: "Main.Volume", Value: "-3", Metric: "volume", Recognized: true},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseTelemetry() = %+v, want %+v", got, want)
	}
}

func TestFormatCommand(t *testing.T) {
	cases := []struct {
		metric  string
		payload string
		want    string
	}{
		{"volume", "-3", "Main.Volume=-3"},
		{"volume", "", "Main.Volume?"},
		{"volume", "+", "Main.Volume+"},
		{"volume", "-", "Main.Volume-"},
		{"mute", "On", "Main.Mute=On"},
		{"balance", "-3L", "Main.Balance=-3L"},
		{"dirac", "1", "Main.Dirac=1"},
	}
	for _, c := range cases {
		got, ok := FormatCommand(c.metric, c.payload)
		if !ok {
			t.Errorf("FormatCommand(%q, %q) reported ok=false, want true", c.metric, c.payload)
			continue
		}
		if got != c.want {
			t.Errorf("FormatCommand(%q, %q) = %q, want %q", c.metric, c.payload, got, c.want)
		}
	}
}

func TestFormatCommandRejectsUnknownMetric(t *testing.T) {
	if _, ok := FormatCommand("source_1/input", "Wii"); ok {
		t.Errorf("FormatCommand for an unknown metric should report ok=false")
	}
}

func TestFrame(t *testing.T) {
	want := "\r\nMain.Volume=-3\r\n"
	if got := Frame("Main.Volume=-3"); got != want {
		t.Errorf("Frame() = %q, want %q", got, want)
	}
}
