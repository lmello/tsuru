// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errors provides facilities with error handling.
package errors

// HTTP represents an HTTP error. It implements the error interface.
//
// Each HTTP error has a Code and a message explaining what went wrong.
type HTTP struct {
	// Status code.
	Code int

	// Message explaining what went wrong.
	Message string
}

func (e *HTTP) Error() string {
	return e.Message
}

// ValidationError is an error implementation used whenever a validation
// failure occurs.
type ValidationError struct {
	Message string
}

func (err *ValidationError) Error() string {
	return err.Message
}
