// Package identity exposes the Identity
// interface
package identity

// Identity has to be implemented
// by authenticators
type Identity interface {
	Username() string
	SetUsername(username string)
	Authenticated() bool
	SetAuthenticated(auth bool)
}
