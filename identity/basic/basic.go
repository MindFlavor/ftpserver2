// Package basicidentity implements
// the identity.Identity interface for
// basic (ie delegated) identity
// authentication.
package basicidentity

import (
	"fmt"

	"github.com/mindflavor/ftpserver2/identity"
)

type basicIdentity struct {
	username        string
	isAuthenticated bool
}

// New creates a new identity.Identity
// in the basicidentity package
func New(username string, isAuthenticated bool) identity.Identity {
	return &basicIdentity{
		username:        username,
		isAuthenticated: isAuthenticated,
	}
}

func (bid *basicIdentity) Username() string {
	return bid.username
}

func (bid *basicIdentity) SetUsername(username string) {
	bid.username = username
}

func (bid *basicIdentity) Authenticated() bool {
	return bid.isAuthenticated
}

func (bid *basicIdentity) SetAuthenticated(auth bool) {
	bid.isAuthenticated = auth
}

func (bid *basicIdentity) String() string {
	if bid.isAuthenticated {
		return fmt.Sprintf("{%s}", bid.username)
	}
	return fmt.Sprintf("{**NOTAUTH** %s}", bid.username)
}
