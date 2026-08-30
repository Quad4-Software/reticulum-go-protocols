# reticulum-go-protocols

Go implementations of application protocols that run on [Reticulum-Go](https://github.com/Quad4-Software/Reticulum-Go). All packages share one transport instance. This is a monorepo.

| Package | What it is |
| --- | --- |
| [`pkg/mf`](pkg/mf) | Compact native format: 16-byte sender hash plus UTF-8 text |
| [`pkg/lxmf`](pkg/lxmf) | [LXMF](https://github.com/markqvist/LXMF) 1.1.0 pack, stamp, paper URI, and delivery messenger |
| [`pkg/rrc`](pkg/rrc) | [RRC](https://rrc.kc1awv.net/) protocol version 1 (spec 0.1.3) hub and client |
| [`pkg/lxst`](pkg/lxst) | [LXST](https://pypi.org/project/lxst/) 0.5.1 telephony: encrypted voice calls over Reticulum links |
| [`pkg/rnv`](pkg/rnv) | Reticulum Native Video: stills, clips, and low-rate A/V streams over Reticulum links |

## Requirements

- Go 1.26.6
- [Reticulum-Go](https://github.com/Quad4-Software/Reticulum-Go)
- [Task](https://taskfile.dev/) if you use the `task` targets below

Optional, for Go/Python interop tests:

- [`uv`](https://docs.astral.sh/uv/)
- `LXMF-ref` clone of upstream LXMF
- `RRC-ref` clone of [rrcd](https://github.com/kc1awv/rrcd)

If your Task binary is named `go-task`, alias it: `alias task='go-task'`.

## Install

The module path is `quad4/reticulum-go-protocols`. It is not on the public Go module proxy.

```go
require quad4/reticulum-go-protocols v0.0.0

replace quad4/reticulum-go-protocols => github.com/Quad4-Software/reticulum-go-protocols master
```

Then `go mod tidy`. Pin a commit or tag instead of `master` for a reproducible build.

For a local checkout:

```go
replace quad4/reticulum-go-protocols => ../reticulum-go-protocols
```

Copy the `replace` lines for `quad4/reticulum-go` and the other `quad4/*` modules from this repo's `go.mod`, or set `GOPRIVATE=quad4/*`.

## pkg/mf

Wire layout is `[16-byte sender hash][UTF-8 text]`. An optional group hash tags fan-out on a 1:1 destination. `mf.Messenger` sends and receives that format over an existing transport.

```go
msg := &mf.Message{SenderHash: senderHash, Text: "hello"}
packed, err := msg.Pack()
if err != nil {
	log.Fatal(err)
}
got, err := mf.Unpack(packed)
if err != nil {
	log.Fatal(err)
}
_ = got.Text
```

```go
messenger := mf.NewMessenger(tr, dest)
if err := messenger.SendMessage(remoteHash, "hello"); err != nil {
	log.Fatal(err)
}
```

Runnable samples: `task example` (`examples/example.go`) and `task example:messenger` (`examples/messenger.go`).

## pkg/lxmf

Matches LXMF 1.1.0 on the wire: pack and Ed25519 sign, opportunistic and direct delivery, proof-of-work stamps, `lxm://` paper URIs, packed containers, and announce app-data. `NewDeliveryMessenger` builds an inbound `lxmf.delivery` destination so the hash matches what Python LXMF derives for the same identity.

```go
messenger, err := lxmf.NewDeliveryMessenger(id, tr)
if err != nil {
	log.Fatal(err)
}
messenger.SetMessageHandler(func(m *lxmf.LXMessage, _ common.NetworkInterface) {
	log.Printf("%s: %q", m.FormatHash(), m.ContentString())
})
_ = messenger.Destination().Announce(false, nil, nil)
```

Other entry points:

- `lxmf.NewMessage` and `LXMessage.Pack` to assemble and sign
- `lxmf.Unpack` / `UnpackFromBytes` for bytes that did not arrive through `Messenger`
- `DisplayNameFromAppData`, `StampCostFromAppData`, and `EncodeAnnounceAppData*` for announce metadata

Python round-trips (`TestInterop`) need `uv` and `LXMF-ref`. Run them with `task test:lxmf:interop`. Live UDP (`TestInterop_Live_*`) is skipped under `-short` and is required when `CI=1`.

See `examples/lxmf_send.go` for a two-node UDP sender.

## pkg/rrc

CBOR envelopes on a Reticulum Link to destination `rrc.hub`. Spec is [0.1.3](https://rrc.kc1awv.net/). The Go hub and client cover HELLO/WELCOME, JOIN/PART, MSG/NOTICE/ACTION, PING/PONG, ERROR, plus rrcd extensions: envelope dest key 8, resource envelope type 50, and WELCOME capability flags.

```go
dest, err := rrc.NewHubDestination(id, tr)
if err != nil {
	log.Fatal(err)
}
hub, err := rrc.NewHub(tr, dest, rrc.HubConfig{Name: "go-hub", Version: "0.1.0"})
if err != nil {
	log.Fatal(err)
}
hub.Start()
_ = dest.Announce(false, nil, nil)
```

```go
client, err := rrc.Dial(tr, id, hubHash, rrc.ClientConfig{
	Nick: "alice",
	Name: "my-client",
	Handlers: rrc.ClientHandlers{
		OnMsg: func(env *rrc.Envelope) {
			text, _ := rrc.BodyAsString(env.Body)
			log.Printf("[%s] %s", env.Room, text)
		},
	},
})
if err != nil {
	log.Fatal(err)
}
defer client.Close()
if err := client.Join("#lobby"); err != nil {
	log.Fatal(err)
}
```

Wait for `OnJoined` before `SendMsg`. Tests: `task test:rrc` or `go test ./pkg/rrc/...`.

Python codec checks use [rrcd](https://github.com/kc1awv/rrcd) via `uv`. Clone it to `RRC-ref`, then `task test:rrc:interop`. Live UDP cases (`TestInterop_Live_PythonClientGoHub`, `TestInterop_Live_GoClientPythonHub`, `TestInterop_Live_PythonClientGoDaemon`) are skipped under `-short` and are required when `CI_REQUIRE_INTEROP=1`.

The interop package also includes a Python RRC client (`pkg/rrc/interoptest/client.py`) used to drive gorrcd: HELLO/WELCOME, JOIN/MSG, `/list` `/who`, unknown slash commands, PING/PONG, and ACTION.

Go-to-Go hub and client mesh tests: `task test:live`.

`pkg/rrc.Hub` is the wire protocol core. Optional `HubPolicy` is used by `cmd/gorrcd` for slash commands, IRC-style room modes, trusted/banned lists, MOTD, hub ping, and RNS Resource blob transfer.

## pkg/lxst

LXST 0.5.1 telephony over Reticulum links. Wire format uses msgpack maps for signalling (`{0: [...]}`) and audio frames (`{1: [codec, payload]}`). Destination is `lxst.telephony`. Opus is the default codec, with Codec2 for low bandwidth.

```go
caller := call.NewCall(tr, call.Config{
	Identity: id,
	UseAudio: false,
})
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := caller.Dial(ctx, remoteIdentity); err != nil {
	log.Fatal(err)
}
```

The call stack is split into `pkg/lxst/proto` (wire codec), `pkg/lxst/call` (state machine and signalling), `pkg/lxst/media` (jitter buffer and adaptive bitrate), and `pkg/lxst/audio` (Opus, Codec2, device I/O). `pkg/lxst/rnsnode` attaches to a shared Reticulum instance, a UDP hop, or a config directory.

CLIs:

- `cmd/rnphone` interactive phone (phonebook, ringtone, announce, history)
- `cmd/rgesp-dial` scriptable dialer for tests and automation

Build with CGO for real audio:

```bash
task rnphone
./bin/rnphone-$(go env GOOS)-$(go env GOARCH)
```

Without CGO, `pkg/lxst` still compiles: codecs use stubs so unit tests and the wire codec run in CI without native libraries.

C ABI for the wire codec only: `task build-liblxst` builds `bin/liblxst.so` and `include/lxst.h`. Python, C, and C++ bindings live under `bindings/` (`task test-codec`).

Python round-trips need `pip install lxst==0.5.1`. Run them with `task test:lxst:interop`. Live mesh call tests (`TestGoGo*`, `TestE2E_*`, `TestAcceptance_*`) are skipped under `-short`.

Go examples: `task example:lxst` (wire codec), `task example:lxst:call` (minimal two-node call).

## pkg/rnv

Go-only Reticulum Native Video (protocol version 1). Destination is `rnv.media`. Sessions use encrypted RNS Links. Stills and clips may use RNS Resources for bulk. Live streams use droppable binary frames (`0xF1` video, `0xF2` audio).

**What it is:** P2P still transfer, short opaque clips, low-rate MJPEG video with optional Opus/Codec2 audio on the same link.

**What it is not:** LXST telephony (ringing, phonebook, busy). Guaranteed lip-sync or HD video. Group/broadcast fanout.

| Use case | Stack |
| --- | --- |
| Voice-only call UI | LXST (`rnv.RecommendStack(UseCaseVoiceOnly)`) |
| Stills / clips / A/V | RNV |

```go
cfg := session.SafeConfig() // prefers profile Low, no auto-announce
ep, err := session.Bind(tr, id, cfg)
if err != nil {
	log.Fatal(err)
}
if err := ep.Announce(); err != nil {
	log.Fatal(err)
}
conn, err := ep.Dial(peerHash)
if err != nil {
	log.Fatal(err)
}
defer conn.Close()
_ = conn.SendStill(ctx, jpegBytes, proto.StillMeta{})
sc, err := conn.OpenStream(ctx, proto.StreamOffer{
	Profile: proto.ProfileMedium,
	Tracks:  proto.TrackVideo | proto.TrackAudio,
	Video:   proto.CodecJPEG,
	Audio:   proto.CodecOpus,
})
```

Footguns: `Announce` is never automatic. Video streams require profile Medium or High. Parallel LXST + RNV audio to the same peer is blocked unless `AllowParallelLXST`. Absolute size caps cannot rise without `DangerousRaiseLimits`. Extensions and private codecs (`0xE0`–`0xFE`) register via `proto.DefaultRegistry`.

Tests: `task test:rnv` or `task test:rnv:short`. Live UDP mesh cases skip under `-short`.

## cmd/gorrcd

Go RRC hub daemon with operator feature parity against [rrcd](https://github.com/kc1awv/rrcd). State lives in `GORRCD_HOME` or `~/.gorrcd/` (`gorrcd.toml`, `hub_identity`, `rooms.toml`) so it does not collide with Python `~/.rrcd`. First run writes defaults and exits 0.

```bash
task gorrcd
./bin/gorrcd-$(go env GOOS)-$(go env GOARCH)
```

Attach to a running Reticulum shared instance when available, otherwise start AutoInterface. For loopback tests, `--udp-listen` and `--udp-forward` bind a UDP pair and skip AutoInterface. `--ready-file` writes the hub destination hash when the daemon is up.

Trusted identities may use `/stats`, `/reload`, and `/kline`. Room founders and ops may use `/kick`, `/topic`, `/mode`, `/op`, `/deop`, `/voice`, `/devoice`, `/ban`, and `/invite`. Anyone may `/list` and `/who`.

Tagged `v*` pushes and Sunday preview snapshots publish cross-compiled `gorrcd` binaries (Linux including 32-bit, armv6, riscv64, ppc64le, Darwin Intel and Apple silicon, Windows, FreeBSD, OpenBSD including ppc64 and riscv64, and NetBSD). CI compiles the same matrix on every push and runs `--version` under qemu-user for linux/386, arm, riscv64, and ppc64le.

## Tests

```bash
task test
task test-short
task check
```

`task check` runs `go vet`, revive, short tests, and gosec. `task test-race` turns on the race detector. `task coverage` writes `coverage.out`.

| Task | Command |
| --- | --- |
| Format | `task fmt` |
| Vet | `task vet` |
| Lint | `task lint` |
| Gosec | `task scan` |
| LXMF tests | `task test:lxmf` |
| RRC tests | `task test:rrc` |
| LXST tests | `task test:lxst` |
| LXST Python interop | `task test:lxst:interop` |
| RNV tests | `task test:rnv` |
| RNV short | `task test:rnv:short` |
| Codec bindings | `task test-codec` |
| MF two-way messenger | `task test:messenger` |
| Benchmarks | `task bench` |
| Live Go mesh | `task test:live` |
| Fuzz | `task fuzz` |
| Refresh `vendor/` | `task deps` |

`task --list` prints the rest.

## License

[0BSD](LICENSE). Copyright 2026 Quad4.io.
