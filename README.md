# mqtt-nad

Go middleware that bridges a NAD M33 amplifier's legacy raw MQTT topics
to and from granular, per-metric topics:

```
tele/m33/raw      --(parse)-->   tele/m33/{metric}
cmd/m33/{metric}  --(format)-->  cmd/m33/raw
```

`tele/m33/raw` and `cmd/m33/raw` carry the amplifier's proprietary
RS-232 wire format (see `RS-232 Protocol for NAD Products v2.02`),
relayed as-is over MQTT by an existing serial<->MQTT bridge. This
service does not talk to the serial port itself - it only translates
between that raw format and granular topics.

The existing bridge, that is wired to the RS232 port and translates
raw commands from/to MQTT is a Waveshare RS323/485/422 to RJ45 device.

## Supported metrics

This mirrors the metrics the existing NAD amplifier integration
(`p-nad-v1`) already exposes. A metric not in this list is not
published from telemetry, and a command for it is rejected:

| metric           | wire variable        |
|------------------|-----------------------|
| `power`          | `Main.Power`          |
| `volume`         | `Main.Volume`         |
| `volume_percent` | `Main.VolumePercent`  |
| `balance`        | `Main.Balance`        |
| `mute`           | `Main.Mute`           |
| `treble`         | `Main.Treble`         |
| `bass`           | `Main.Bass`           |
| `brightness`     | `Main.Brightness`     |
| `source`         | `Main.Source`         |
| `dirac`          | `Main.Dirac`          |

Telemetry values are republished exactly as received from the
amplifier (e.g. `balance` stays in NAD's own `-3L`/`-3R`/`0` form). A
command's payload is used the same way: to set `balance`, publish that
same wire-format value (e.g. `-3L`) to `cmd/m33/balance`.

Anything else on the raw telemetry topic (per-source detail like
`Source1.Input`, the `Main.Sources` count, or any future/unknown
variable) is logged but intentionally not fanned out to a topic,
matching the existing integration. Extending the supported metric set
is a one-line addition to the map in `nad/protocol.go`.

### Commands

A command topic's payload selects the operator, per the RS-232
protocol:

- empty payload -> query, e.g. `cmd/m33/volume` with an empty payload
  sends `Main.Volume?`
- `+` or `-` -> step/toggle, e.g. `Main.Volume+`
- anything else -> set that value, e.g. `Main.Volume=-3`

## Known quirks

### `source = 9` is ambiguous (BluOS vs HDMI/ARC)

The M33 reports `Main.Source=9` both when the active source is its
BluOS streaming module and when it's actually set to HDMI/ARC
passthrough - the amp's RS-232 interface can't tell the two apart on
its own. A previous integration resolved this with an extra HTTP call
to the amp's BluOS API (`GET http://<amp-host>:<port>/Status`, checking
whether `<inputTypeIndex>` equals `arc-1`) and republished an overridden
value (`99`) when that was the case.

This bridge deliberately does not do that: it only translates the
RS-232 wire protocol over MQTT and has no HTTP dependency on the
amplifier's BluOS side. If you need to tell BluOS and HDMI/ARC apart,
do the same `/Status` check from whatever consumes `tele/m33/source`
when you see a value of `9`. For what it's worth, BluOS's own
capture-source listing (`GET .../RadioBrowse?service=Capture`)
identifies this same HDMI/ARC capture input as id `0` - worth using as
your override value instead of the old integration's arbitrary `99`,
if you want it to line up with that convention.

## Configuration

All configuration is via environment variables:

| Variable                | Default               | Purpose                                   |
|--------------------------|-----------------------|--------------------------------------------|
| `MQTT_BROKER_URL`         | `tcp://localhost:1883`| Broker to connect to                       |
| `MQTT_CLIENT_ID`          | `mqtt-nad`            | MQTT client id                             |
| `MQTT_USERNAME`           | *(none)*              | Broker username, if required               |
| `MQTT_PASSWORD`           | *(none)*              | Broker password, if required               |
| `RAW_TELEMETRY_TOPIC`     | `tele/m33/raw`        | Legacy topic telemetry arrives on          |
| `TELEMETRY_TOPIC_BASE`    | `tele/m33`            | Base for granular telemetry topics         |
| `COMMAND_TOPIC_BASE`      | `cmd/m33`             | Base subscribed to for granular commands   |
| `RAW_COMMAND_TOPIC`       | `cmd/m33/raw`         | Legacy topic formatted commands are sent to|

## Running

```bash
go build -o mqtt-nad .
MQTT_BROKER_URL=tcp://broker:1883 ./mqtt-nad
```

Or via Docker:

```bash
docker build -t mqtt-nad .
docker run -e MQTT_BROKER_URL=tcp://broker:1883 mqtt-nad
```

`docker-compose.yml` spins up a local mosquitto broker alongside
mqtt-nad for quick manual testing:

```bash
docker compose up --build
mosquitto_pub -h localhost -t tele/m33/raw -m $'Main.Volume=-3\r\n'
mosquitto_sub -h localhost -t 'tele/m33/#' -v
```

## Tests

```bash
go test ./...
```

`nad/protocol_test.go` covers parsing and command formatting, including
the exact wire quirks (mixed `<CR>`/`<LF>` framing, the `Main.Balance`
`-3L`/`-3R` encoding, unknown/unpublished variables).
