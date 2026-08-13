# reticulum-go-protocols

Go implementations of application protocols that run on [Reticulum-Go](https://github.com/Quad4-Software/Reticulum-Go). All three packages share one transport instance.

| Package | What it is |
| --- | --- |
| [`pkg/mf`](pkg/mf) | Compact native format: 16-byte sender hash plus UTF-8 text |
| [`pkg/lxmf`](pkg/lxmf) | [LXMF](https://github.com/markqvist/LXMF) 1.1.0 pack, stamp, paper URI, and delivery messenger |
| [`pkg/rrc`](pkg/rrc) | [RRC](https://rrc.kc1awv.net/) protocol version 1 (spec 0.1.3) hub and client |

## Requirements

- Go 1.26.5
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

Python round-trips (`TestInterop`) need `uv` and `LXMF-ref`. Run them with `task test:lxmf:interop`.

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

Python codec checks use [rrcd](https://github.com/kc1awv/rrcd) via `uv`. Clone it to `RRC-ref`, then `task test:rrc:interop`. The live UDP case `TestInterop_Live_PythonClientGoHub` is skipped under `-short`.

Slash commands, IRC-style room modes, and RNS Resource blob transfer are rrcd hub extras. This package implements the envelope and hub policy for type 50, not the blob transfer itself.

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
| MF two-way messenger | `task test:messenger` |
| Benchmarks | `task bench` |
| Fuzz | `task fuzz` |
| Refresh `vendor/` | `task deps` |

`task --list` prints the rest.

## License

[0BSD](LICENSE). Copyright 2026 Quad4.io.
