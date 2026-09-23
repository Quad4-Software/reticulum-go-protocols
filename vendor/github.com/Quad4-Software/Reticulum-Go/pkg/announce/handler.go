// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2024-2026 Quad4.io

package announce

type Handler interface {
	AspectFilter() []string
	ReceivedAnnounce(destHash []byte, identity any, appData []byte, hops uint8) error
	ReceivePathResponses() bool
}

// PathAwareHandler is an optional extension of Handler. When a
// registered handler also implements it, the transport calls
// ReceivedAnnouncePathResponse instead of ReceivedAnnounce so the
// handler can tell regular announces from path responses, matching
// the is_path_response flag in Python Reticulum announce handlers.
type PathAwareHandler interface {
	ReceivedAnnouncePathResponse(destHash []byte, identity any, appData []byte, hops uint8, isPathResponse bool) error
}
