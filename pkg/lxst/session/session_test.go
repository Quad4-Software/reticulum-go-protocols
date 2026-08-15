// SPDX-License-Identifier: Apache-2.0
package session_test

import (
	"context"
	"encoding/hex"
	stdio "io"
	"net"
	"sync"
	"testing"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/audio/io"
	"quad4/reticulum-go-protocols/pkg/lxst/call"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
	"quad4/reticulum-go-protocols/pkg/lxst/session"
	"quad4/reticulum-go/pkg/common"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/transport"
)

type pairIface struct {
	common.BaseInterface
	peer *pairIface
}

func newPairIface(name string) *pairIface {
	p := &pairIface{BaseInterface: common.NewBaseInterface(name, common.IFTypeAuto, true)}
	p.MTU = common.DefaultMTU
	p.Bitrate = 1_000_000
	p.In = true
	p.Out = true
	p.Enable()
	return p
}

func (p *pairIface) Send(data []byte, _ string) error {
	if p.peer == nil {
		return nil
	}
	cp := append([]byte(nil), data...)
	go p.peer.ProcessIncoming(cp)
	return nil
}

func (p *pairIface) ProcessOutgoing(data []byte) error { return p.Send(data, "") }

func isolatedConfig(t *testing.T) *common.ReticulumConfig {
	t.Helper()
	cfg := common.DefaultConfig()
	cfg.ShareInstance = false
	cfg.InMemoryPathTable = true
	cfg.InMemoryKnownDestinations = true
	cfg.ConfigPath = t.TempDir() + "/config"
	return cfg
}

func answerWhenRinging(ctx context.Context, s *session.Session) {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return
		}
		c := s.Active()
		if c != nil && c.Incoming() && c.State() == call.StateRinging {
			_ = s.Answer(ctx)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func pairedSessions(t *testing.T) (*session.Session, *session.Session, *identity.Identity) {
	t.Helper()
	tA := transport.NewTransport(isolatedConfig(t))
	tB := transport.NewTransport(isolatedConfig(t))
	if err := tA.Start(); err != nil {
		t.Fatal(err)
	}
	if err := tB.Start(); err != nil {
		t.Fatal(err)
	}
	ifA := newPairIface("a")
	ifB := newPairIface("b")
	ifA.peer = ifB
	ifB.peer = ifA
	if err := tA.RegisterInterface("a", ifA); err != nil {
		t.Fatal(err)
	}
	if err := tB.RegisterInterface("b", ifB); err != nil {
		t.Fatal(err)
	}
	idA, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	idB, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	gotA, err := session.Open(session.Config{Transport: tA, Identity: idA})
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := session.Open(session.Config{Transport: tB, Identity: idB})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = gotA.Close()
		_ = gotB.Close()
	})
	if err := gotB.Announce(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	return gotA, gotB, idB
}

func TestParseHash(t *testing.T) {
	raw := make([]byte, proto.IdentityHashLen)
	raw[0] = 0xab
	raw[1] = 0xcd
	hexs := hex.EncodeToString(raw)
	got, err := session.ParseHash(" <" + hexs[:4] + " " + hexs[4:] + "> ")
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(got) != hexs {
		t.Fatalf("got %x", got)
	}
}

func TestParseHashRejectsShort(t *testing.T) {
	if _, err := session.ParseHash("abcd"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenRequiresTransport(t *testing.T) {
	id, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Open(session.Config{Identity: id}); err != session.ErrNoTransport {
		t.Fatalf("got %v", err)
	}
}

func TestDialAnswerHangup(t *testing.T) {
	a, b, idB := pairedSessions(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go answerWhenRinging(ctx, b)
	outgoing, err := a.Dial(ctx, idB)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if outgoing.State() != call.StateActive {
		t.Fatalf("state %v", outgoing.State())
	}
	if err := a.Hangup(); err != nil {
		t.Fatal(err)
	}
}

func TestHostPCMSurvivesHangup(t *testing.T) {
	a, b, idB := pairedSessions(t)
	host := a.Host()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go answerWhenRinging(ctx, b)
	if _, err := a.Dial(ctx, idB); err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := a.Hangup(); err != nil {
		t.Fatal(err)
	}
	if host.Closed() {
		t.Fatal("host closed after hangup")
	}
	if err := a.PushPCM([]int16{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
}

func TestPCMRoundTrip(t *testing.T) {
	a, b, idB := pairedSessions(t)
	got := make(chan struct{}, 1)
	b.Host().SetPlaybackHandler(func(pcm []int16) {
		for _, s := range pcm {
			if s != 0 {
				select {
				case got <- struct{}{}:
				default:
				}
				return
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go answerWhenRinging(ctx, b)
	if _, err := a.Dial(ctx, idB); err != nil {
		t.Fatalf("dial: %v", err)
	}
	tone := make([]int16, io.DefaultFrameSize)
	for i := range tone {
		tone[i] = 12000
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = a.PushPCM(tone)
		select {
		case <-got:
			return
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	t.Fatal("no playback pcm")
}

func TestAttachStream(t *testing.T) {
	h := io.NewHost()
	id, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	tr := transport.NewTransport(isolatedConfig(t))
	if err := tr.Start(); err != nil {
		t.Fatal(err)
	}
	s, err := session.Open(session.Config{Transport: tr, Identity: id, Device: h})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	c1, c2 := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- s.Attach(ctx, c1) }()
	raw := io.PCM16LE([]int16{7, 8, 9})
	hdr := []byte{byte(len(raw)), 0, 0, 0}
	if _, err := c2.Write(append(hdr, raw...)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	var captured []int16
	for time.Now().Before(deadline) {
		frame, err := h.ReadPCM()
		if err != nil {
			t.Fatal(err)
		}
		if len(frame) == 3 && frame[0] == 7 {
			captured = frame
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(captured) != 3 {
		t.Fatal("capture not pushed")
	}
	if err := h.WritePCM([]int16{3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := c2.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	lenbuf := make([]byte, 4)
	if _, err := stdio.ReadFull(c2, lenbuf); err != nil {
		t.Fatal(err)
	}
	n := int(lenbuf[0]) | int(lenbuf[1])<<8 | int(lenbuf[2])<<16 | int(lenbuf[3])<<24
	buf := make([]byte, n)
	if _, err := stdio.ReadFull(c2, buf); err != nil {
		t.Fatal(err)
	}
	pcm, err := io.FromPCM16LE(buf)
	if err != nil || len(pcm) != 2 || pcm[0] != 3 {
		t.Fatalf("playback %v %v", pcm, err)
	}
	cancel()
	_ = c1.Close()
	_ = c2.Close()
	select {
	case <-errCh:
	case <-time.After(time.Second):
		t.Fatal("attach did not return")
	}
}

func TestPushWithoutHost(t *testing.T) {
	dev := io.NewNullDevice()
	id, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	tr := transport.NewTransport(isolatedConfig(t))
	if err := tr.Start(); err != nil {
		t.Fatal(err)
	}
	s, err := session.Open(session.Config{Transport: tr, Identity: id, Device: dev})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.PushPCM([]int16{1}); err != session.ErrNoHost {
		t.Fatalf("got %v", err)
	}
}

func TestRaceSessionPCM(t *testing.T) {
	a, _, _ := pairedSessions(t)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for range 100 {
			_ = a.PushPCM([]int16{1, 2, 3, 4})
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			a.PullPCM()
		}
	}()
	go func() {
		defer wg.Done()
		pcm := make([]int16, io.DefaultFrameSize)
		for range 100 {
			_ = a.Host().WritePCM(pcm)
		}
	}()
	wg.Wait()
}
