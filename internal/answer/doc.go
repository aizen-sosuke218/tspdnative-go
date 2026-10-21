// Package answer builds the F5 TSPD answer cookie without executing the
// sensor.
//
// The builder consumes the challenge page, the fetched sensor script bodies
// and the fingerprint inputs (language and user agent). It returns the cookie
// name and value that clear the challenge.
package answer

import "errors"

// Envelope scopes used by the answer protocol.
const (
	// ScopeChallenge is the outer challenge message scope.
	ScopeChallenge byte = 0x01

	// ScopeFingerprint is the fingerprint seal scope.
	ScopeFingerprint byte = 0x04

	// ScopeInner is the embedded context message scope.
	ScopeInner byte = 0xAB
)

// Cookie is the assembled answer cookie.
type Cookie struct {
	Name  string
	Value string
}

var (
	// ErrBobcmnNotFound reports a page without a bobcmn configuration.
	ErrBobcmnNotFound = errors.New("answer: bobcmn not found")

	// ErrChallengeNotFound reports a page without a hex challenge.
	ErrChallengeNotFound = errors.New("answer: challenge not found")

	// ErrKeyNotFound reports sensor scripts without a usable key.
	ErrKeyNotFound = errors.New("answer: key0 not found in external scripts")

	// ErrConfigTooShort reports a bobcmn layout older than the supported build.
	ErrConfigTooShort = errors.New("answer: bobcmn has fewer than 25 entries")

	// ErrFieldTooLong reports a fingerprint or answer field above its wire limit.
	ErrFieldTooLong = errors.New("answer: field too long")
)

const (
	requiredEntries = 25
	nameSuffixBase  = 75
	field5Compare   = 3
)
