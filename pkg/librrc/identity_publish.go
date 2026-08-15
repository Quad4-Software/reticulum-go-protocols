// SPDX-License-Identifier: 0BSD
package librrc

import (
	"fmt"

	"quad4/reticulum-go-protocols/pkg/rrc"
	"quad4/reticulum-go/pkg/identity"
)

func IdentitySeedDestination(identityHandle uint64, destHash []byte) int {
	if len(destHash) != rrc.IdentityLength {
		return setLastError(fmt.Errorf("%w: destination hash", errInvalidArg))
	}
	idRec, err := identityByHandle(identityHandle)
	if err != nil {
		return setLastError(err)
	}
	identity.Remember(nil, destHash, idRec.identity.GetPublicKey(), nil)
	return OK
}
