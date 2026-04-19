// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

// BasicError wraps error types so that they can be messaged across RPC
// channels. Since "error" is an interface, the underlying structure cannot
// always be gob-encoded; BasicError is a concrete error implementation that
// can be sent over the wire.
type BasicError struct {
	Message string
}

// NewBasicError is used to create a BasicError.
//
// err is allowed to be nil.
func NewBasicError(err error) *BasicError {
	if err == nil {
		return nil
	}

	return &BasicError{err.Error()}
}

func (e *BasicError) Error() string {
	return e.Message
}
