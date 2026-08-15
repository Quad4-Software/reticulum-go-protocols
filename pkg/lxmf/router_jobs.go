// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"time"
)

const (
	jobProcessingInterval = 4 * time.Second
	jobTransientInterval  = 60 * time.Second
	jobStoreInterval      = 120 * time.Second
	jobPeerSyncInterval   = 6 * time.Second
)

func (r *Router) jobLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(jobProcessingInterval)
	defer ticker.Stop()

	var tickCount int64
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			tickCount++
			r.runJobs(tickCount)
		}
	}
}

func (r *Router) runJobs(tick int64) {
	defer func() {
		if rec := recover(); rec != nil {
			Error("router job panic", "error", rec)
		}
	}()

	if tick%int64(jobTransientInterval/jobProcessingInterval) == 0 {
		r.cleanTransientCaches()
	}

	if r.propagationEnabled {
		if tick%int64(jobStoreInterval/jobProcessingInterval) == 0 {
			if r.store != nil {
				r.store.CleanExpired()
				r.store.CleanToLimit()
			}
		}
		if tick%int64(jobPeerSyncInterval/jobProcessingInterval) == 0 {
			r.flushPeerDistribution()
			r.syncPeers()
			r.savePeers()
			r.saveNodeStats()
		}
	}

	if tick%15 == 0 {
		r.saveLocallyDelivered()
		r.saveLocallyProcessed()
	}

	if r.cfg.LXMF.AnnounceIntervalMinutes > 0 && tick%int64(time.Duration(r.cfg.LXMF.AnnounceIntervalMinutes)*time.Minute/jobProcessingInterval) == 0 {
		r.announceDelivery()
	}
	if r.propagationEnabled && r.cfg.Propagation.AnnounceIntervalMinutes > 0 &&
		tick%int64(time.Duration(r.cfg.Propagation.AnnounceIntervalMinutes)*time.Minute/jobProcessingInterval) == 0 {
		go r.announcePropagationNode()
	}
}
