// SPDX-License-Identifier: Apache-2.0
package integration_test

import (
	"testing"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/call"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/interfaces"
	"quad4/reticulum-go/pkg/transport"
)

func TestSharedInstanceSwitchboard(t *testing.T) {
	if testing.Short() {
		t.Skip("live mesh integration test")
	}
	port := freeTCP(t)
	hubCfg := isolatedTransportConfig(t)
	hubCfg.EnableTransport = true
	hub := transport.NewTransport(hubCfg)
	if err := hub.Start(); err != nil {
		t.Fatal(err)
	}
	spawn := func(client *interfaces.LocalClientInterface) {
		client.In = true
		client.Out = true
		if err := hub.RegisterInterface(client.GetName(), client); err != nil {
			t.Errorf("register: %v", err)
		}
	}
	srv, err := interfaces.NewLocalServerInterface(port, "", false, spawn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Stop() }()

	tA, idA := startGoLocal(t, port)
	tB, idB := startGoLocal(t, port)
	destB, err := destination.New(idB, destination.In, destination.Single, proto.AppName, tB, proto.AspectName)
	if err != nil {
		t.Fatal(err)
	}
	sb := call.NewSwitchboard(tB, call.Config{Identity: idB, UseAudio: false}, nil)
	sb.Bind(destB)
	for range 8 {
		_ = destB.Announce(false, nil, nil)
		time.Sleep(80 * time.Millisecond)
	}
	if err := waitPath(hub, destB.GetHash(), 5*time.Second); err != nil {
		t.Fatal(err)
	}
	_ = tA
	_ = idA
	if sb.Active() != nil && sb.Active().State() == call.StateEnded {
		t.Fatal("stale active call")
	}
}
