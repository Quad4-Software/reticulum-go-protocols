// SPDX-License-Identifier: 0BSD

//go:build (!linux && !darwin && !windows) || lxstamp_nogpu

package lxmf

import "context"

var gpuEngine *gpuEngineState

type gpuEngineState struct {
	vendor string
	name   string
}

func openGPUEngine() (*gpuEngineState, error) {
	return nil, ErrGPUUnavailable
}

func (e *gpuEngineState) generate(ctx context.Context, workblock []byte, stampCost int) ([]byte, int, error) {
	return nil, 0, ErrGPUUnavailable
}
