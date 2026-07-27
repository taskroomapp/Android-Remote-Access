package cryptokit

import "errors"

// ErrInvalidEnvelope indicates a malformed or unsupported ciphertext envelope.
var ErrInvalidEnvelope = errors.New("invalid cryptokit envelope")
