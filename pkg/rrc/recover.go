// SPDX-License-Identifier: 0BSD
package rrc

import "log"

func recoverLog() {
	if r := recover(); r != nil {
		log.Printf("rrc: recovered: %v", r)
	}
}
