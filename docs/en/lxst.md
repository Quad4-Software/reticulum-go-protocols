# pkg/lxst

LXST 0.5.1 telephony over Reticulum links. Wire format uses msgpack maps for signalling (`{0: [...]}`) and audio frames (`{1: [codec, payload]}`). Destination is lxst.telephony. Opus is the default codec, with Codec2 for low bandwidth.

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

| Package | Role |
| --- | --- |
| [pkg/lxst/proto](../../pkg/lxst/proto) | Wire codec |
| [pkg/lxst/call](../../pkg/lxst/call) | State machine and signalling |
| [pkg/lxst/media](../../pkg/lxst/media) | Jitter buffer and adaptive bitrate |
| [pkg/lxst/audio](../../pkg/lxst/audio) | Opus, Codec2, device I/O |
| [pkg/lxst/rnsnode](../../pkg/lxst/rnsnode) | Shared Reticulum instance, UDP hop, or config directory |

| Binary | Purpose |
| --- | --- |
| [cmd/rnphone](../../cmd/rnphone) | Interactive phone (phonebook, ringtone, announce, history) |
| [cmd/rgesp-dial](../../cmd/rgesp-dial) | Scriptable dialer for tests and automation |

Build with CGO for real audio:

```bash
task rnphone
./bin/rnphone-$(go env GOOS)-$(go env GOARCH)
```

Without CGO, pkg/lxst still compiles: codecs use stubs so unit tests and the wire codec run in CI without native libraries.

```bash
make liblxst
task build-liblxst
task test-codec
```

Python round-trips need `pip install lxst==0.5.1`.

```bash
task test:lxst
task test:lxst:interop
go test -v -count=1 ./pkg/lxst/...
task example:lxst
task example:lxst:call
```

Live mesh call tests (`TestGoGo*`, `TestE2E_*`, `TestAcceptance_*`) are skipped under `-short`.
