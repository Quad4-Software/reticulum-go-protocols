// SPDX-License-Identifier: 0BSD
package liblxmf

import (
	"fmt"

	"quad4/reticulum-go-protocols/pkg/lxmf"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
)

type identityRecord struct {
	identity *identity.Identity
}

func IdentityGenerate() (uint64, int) {
	id, err := identity.New()
	if err != nil {
		return 0, setLastError(err)
	}
	return handles.insert(kindIdentity, &identityRecord{identity: id}), OK
}

func IdentityDestroy(handle uint64) int {
	if !handles.delete(handle) {
		return setLastError(errInvalidHandle)
	}
	return OK
}

func IdentityHash(handle uint64) ([]byte, int) {
	rec, err := identityByHandle(handle)
	if err != nil {
		return nil, setLastError(err)
	}
	return append([]byte(nil), rec.identity.Hash()...), OK
}

func IdentityPublicKey(handle uint64) ([]byte, int) {
	rec, err := identityByHandle(handle)
	if err != nil {
		return nil, setLastError(err)
	}
	return append([]byte(nil), rec.identity.GetPublicKey()...), OK
}

func IdentityDeliveryHash(handle uint64) ([]byte, int) {
	rec, err := identityByHandle(handle)
	if err != nil {
		return nil, setLastError(err)
	}
	dest, err := destination.New(rec.identity, destination.Out, destination.Single, lxmf.AppName, nil, "delivery")
	if err != nil {
		return nil, setLastError(err)
	}
	return append([]byte(nil), dest.GetHash()...), OK
}

func IdentityRegisterRecall(handle uint64) int {
	rec, err := identityByHandle(handle)
	if err != nil {
		return setLastError(err)
	}
	return IdentityRegisterRecallBytes(rec.identity)
}

func IdentityRegisterRecallBytes(id *identity.Identity) int {
	if id == nil {
		return setLastError(fmt.Errorf("%w: identity nil", errInvalidArg))
	}
	dest, err := destination.New(id, destination.Out, destination.Single, lxmf.AppName, nil, "delivery")
	if err != nil {
		return setLastError(err)
	}
	identity.Remember(nil, dest.GetHash(), id.GetPublicKey(), nil)
	return OK
}

func IdentityRegisterRecallSource(sourceHash, publicKey []byte) int {
	if len(sourceHash) != lxmf.DestinationLength || len(publicKey) == 0 {
		return setLastError(fmt.Errorf("%w: recall source", errInvalidArg))
	}
	identity.Remember(nil, append([]byte(nil), sourceHash...), append([]byte(nil), publicKey...), nil)
	return OK
}

func identityByHandle(id uint64) (*identityRecord, error) {
	ref, err := handles.get(id, kindIdentity)
	if err != nil {
		return nil, err
	}
	return ref.(*identityRecord), nil
}

func identityFromHandle(id uint64) (*identity.Identity, int) {
	rec, err := identityByHandle(id)
	if err != nil {
		return nil, setLastError(err)
	}
	return rec.identity, OK
}
