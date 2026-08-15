// SPDX-License-Identifier: Apache-2.0
package hardware_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/lxst/hardware"
)

func TestEnableKnownDrivers(t *testing.T) {
	k := hardware.NewKeypad()
	k.Enable("gpio_4x4")
	if !k.Enabled {
		t.Fatal("keypad")
	}
	k.Enable("unknown")
	if k.Enabled {
		t.Fatal("unknown keypad")
	}
	d := hardware.NewLCD()
	d.Enable("i2c_lcd1602")
	if !d.Enabled {
		t.Fatal("lcd")
	}
	d.Enable("")
	if d.Enabled {
		t.Fatal("empty lcd")
	}
}
