# Testing

Adaptive Limiter is monitor-only through milestone 2. It reads interface and
dummynet state and sends ICMP echo requests, but it has no code path that
changes a pipe.

## Automated checks

```sh
make fmt-check
make test
make vet
make build-freebsd
```

The raw-socket integration test is opt-in and requires `CAP_NET_RAW`:

```sh
docker run --rm --cap-add=NET_RAW \
  -e ADAPTIVE_LIMITER_ICMP_TEST=1 \
  -v "$PWD:/src" -w /src golang:1.26-bookworm \
  go test -run TestICMPIntegration -v ./internal/probe
```

## Generic pfSense VM

A pfSense 2.8.1 VM can validate package installation, service lifecycle,
WebUI configuration, the dashboard widget, interface counters, raw ICMP, and
dummynet parsing. It needs:

- Internet access on its WAN;
- a configured FQ_CODEL upload/download limiter pair;
- a firewall rule applying that pair;
- the actual VM WAN device selected instead of `pppoe0`;
- monitor-only mode enabled.

After installation, verify:

```sh
service adaptive_limiter status
cat /usr/local/etc/adaptive-limiter/config.json
cat /var/run/adaptive-limiter/status.json
netstat -I <wan-device> -b -n
dnctl pipe show
```

Generate download and upload traffic through the VM and confirm the status
JSON, full status page, and dashboard widget report the same direction and
approximately the same rates.

## PPPoE-specific validation

A generic WAN validates most of milestone 2, but not PPPoE address changes or
the exact `pppoe0` counter format. Exact testing requires either:

- a second VM acting as a PPPoE access concentrator on the WAN segment; or
- a monitor-only installation on the real pfSense appliance.

For a real appliance test, first capture this output so its columns can be
compared with the parser fixture:

```sh
netstat -I pppoe0 -b -n
```

Then reconnect PPPoE once and verify that probes resume automatically after
the public IPv4 address changes. No limiter rates should change during any
milestone 2 test.
