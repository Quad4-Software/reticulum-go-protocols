# pkg/rnv

Go-only Reticulum Native Video (protocol version 1). Destination is rnv.media. Sessions use encrypted RNS Links. Stills and clips may use RNS Resources for bulk. Live streams use droppable binary frames (`0xF1` video, `0xF2` audio).

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

Footguns:

- `Announce` is never automatic
- Video streams require profile Medium or High
- Parallel LXST + RNV audio to the same peer is blocked unless `AllowParallelLXST`
- Absolute size caps cannot rise without `DangerousRaiseLimits`
- Extensions and private codecs (`0xE0`–`0xFE`) register via `proto.DefaultRegistry`

```bash
task test:rnv
task test:rnv:short
task test:rnv:live
go test -v -count=1 ./pkg/rnv/...
go test -short -v -count=1 ./pkg/rnv/...
```

Live coverage:

- Go↔Go UDP: stills, clips, video+audio stream, capacity reject (`TestLive*`)
- Go↔Go over a Python Reticulum shared-instance hub (`TestLivePythonSharedInstanceStillStream`). Needs `import RNS`. Set `REQUIRE_RNV_PYTHON=1` to fail instead of skip under `-short`.

There is no Python RNV application peer yet (protocol is Go-only). "Python node" here means the RNS transport hub, not a Python RNV endpoint.
