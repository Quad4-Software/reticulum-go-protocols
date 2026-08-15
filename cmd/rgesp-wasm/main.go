//go:build js && wasm

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"syscall/js"

	"quad4/reticulum-go-protocols/pkg/lxst/call"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

func main() {
	js.Global().Set("rgespReady", js.ValueOf(true))
	js.Global().Set("rgespApp", js.ValueOf(proto.AppName+"."+proto.AspectName))
	js.Global().Set("rgespDefaultProfile", js.ValueOf(call.DefaultConfig().Profile))
	fmt.Println("rgesp-wasm ready")
	select {}
}
