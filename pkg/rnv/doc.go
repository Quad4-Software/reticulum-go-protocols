// SPDX-License-Identifier: 0BSD

// Package rnv implements Reticulum Native Video protocol version 1.
//
// Destination is rnv.media. Sessions use encrypted RNS Links. Stills and clips
// may use RNS Resources for bulk. Live streams use droppable link frames.
//
// What this is:
//   - P2P still image transfer
//   - Short opaque clip transfer
//   - Low-rate MJPEG video stream with optional Opus or Codec2 audio
//
// What this is not:
//   - LXST telephony (ringing, phonebook, busy). Use pkg/lxst for voice calls.
//   - Guaranteed lip-sync or HD video
//   - Group or broadcast fanout
//
// Stack choice:
//   - Voice-only UI -> LXST (RecommendStack VoiceOnly)
//   - Camera / A/V / stills / clips -> RNV (RecommendStack AV or Stills)
//
// Footguns:
//   - Announce is never automatic. Call Endpoint.Announce explicitly.
//   - SafeConfig defaults to profile Low (no video stream).
//   - OpenStream with video requires profile Medium or High.
//   - Parallel LXST + RNV audio to the same peer is blocked unless
//     AllowParallelLXST is set.
//   - Absolute size caps cannot be raised without DangerousRaiseLimits.
//   - A/V sync via media.Clock is best-effort only.
//
// Extensions:
//   - Register private codecs with proto.DefaultRegistry.
//   - Envelope extension map key 90 is skipped when unknown unless StrictExtensions.
package rnv
