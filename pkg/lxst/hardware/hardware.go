// SPDX-License-Identifier: Apache-2.0
package hardware

import "strings"

// Keypad is a 4x4 matrix used by rnphone on Raspberry Pi.
type Keypad struct {
	Enabled bool
}

func NewKeypad() *Keypad {
	return &Keypad{}
}

func (k *Keypad) Enable(driver string) {
	k.Enabled = strings.EqualFold(strings.TrimSpace(driver), "gpio_4x4")
}

func (*Keypad) Read() (rune, bool) {
	return 0, false
}

// LCD is a 16x2 I2C display used by rnphone.
type LCD struct {
	Enabled bool
}

func NewLCD() *LCD {
	return &LCD{}
}

func (d *LCD) Enable(driver string) {
	d.Enabled = strings.EqualFold(strings.TrimSpace(driver), "i2c_lcd1602")
}

func (*LCD) Write(_, _ string) {}

func (*LCD) Clear() {}
