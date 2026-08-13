// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"context"
	"time"
)

func generateStampWithLog(messageID []byte, stampCost int) ([]byte, int, error) {
	Info("generating lxmf delivery stamp", "cost", stampCost)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	stamp, value, err := GenerateStamp(ctx, messageID, stampCost, WorkblockExpandRounds)
	if err != nil {
		return nil, 0, err
	}
	Verbose("lxmf delivery stamp ready", "value", value)
	return stamp, value, nil
}
