module quad4/reticulum-go-protocols

go 1.26.6

require (
	github.com/fxamacker/cbor/v2 v2.9.2
	github.com/landlock-lsm/go-landlock v0.9.0
	golang.org/x/sys v0.47.0
	golang.org/x/term v0.45.0
	quad4/msgpack/v5 v5.8.2
	quad4/pbt v0.0.0
	quad4/reticulum-go v1.0.2
)

require (
	github.com/dunglas/httpsfv v1.1.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/mdlayher/socket v0.6.1 // indirect
	github.com/mdlayher/vsock v1.3.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.60.0 // indirect
	github.com/quic-go/webtransport-go v0.11.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.bug.st/serial v1.8.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.77 // indirect
	quad4/bzip2 v0.0.0 // indirect
	quad4/tagparser v0.0.0 // indirect
)

replace (
	quad4/bzip2 => github.com/Quad4-Software/bzip2 v0.0.0-20260704225916-ca8b2bb66059
	quad4/msgpack/v5 => github.com/Quad4-Software/msgpack/v5 v5.8.2
	quad4/pbt => github.com/Quad4-Software/pbt v0.0.0-20260614183135-abe0cfc4e604
	quad4/reticulum-go => github.com/Quad4-Software/Reticulum-Go v1.0.2
	quad4/tagparser => github.com/Quad4-Software/tagparser v0.1.3-0.20260614183136-daa4d5f437ce
)
