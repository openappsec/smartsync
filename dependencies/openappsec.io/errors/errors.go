// Copyright (C) 2022 Check Point Software Technologies Ltd. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

import (
	"fmt"

	"golang.org/x/xerrors"
)

// Severity level
type Severity string

// Severity level consts
const (
	Critical Severity = "Critical"
	High     Severity = "High"
	Medium   Severity = "Medium"
	Low      Severity = "Low"
	Info     Severity = "Info"
)

// Class defines an errors behavior
type Class int

const (
	// ClassUnknown default Class
	ClassUnknown Class = iota
	// ClassBadInput for bad input errors
	ClassBadInput
	// ClassForbidden for forbidden actions
	ClassForbidden
	// ClassNotFound for resources that are not existent
	ClassNotFound
	// ClassInternal for internal errors
	ClassInternal
	// ClassUnauthorized for unauthorized actions
	ClassUnauthorized
	// ClassBadGateway for server bad gateway response
	ClassBadGateway
)

// Err represents a single error.
type Err struct {
	err   error
	label string
	class Class
	code  code
}

// Error is our version of the error interface, exposing the regular error capabilities alongside the ability to set class
type Error interface {
	error
	SetLabel(label string) Error
	SetClass(c Class) Error
}

// New creates a new error
func New(message string) Error {
	return &Err{
		err: xerrors.New(message),
	}
}

// Wrap wraps an existing error in more contextual information
func Wrap(err error, message string) Error {
	if err == nil {
		return &Err{
			err: xerrors.New(message),
		}
	}

	return &Err{
		err: xerrors.Errorf("%s: %w", message, err),
	}
}

// Wrapf wraps an existing error in more contextual information and allow for format operations
func Wrapf(err error, message string, a ...interface{}) Error {
	if err == nil {
		return &Err{
			err: xerrors.Errorf("%s", fmt.Sprintf(message, a...)),
		}
	}

	return &Err{
		err: xerrors.Errorf("%s: %w", fmt.Sprintf(message, a...), err),
	}
}

// Errorf creates a new error using a formatted string
func Errorf(format string, a ...interface{}) Error {
	return &Err{
		err: xerrors.Errorf(format, a...),
	}
}

// SetClass set the error's Class, the Class defines the errors behavior
func (e *Err) SetClass(class Class) Error {
	e.class = class
	return e
}

// SetLabel set the error's Label, with the Label it's possible to distinguish between errors of the same class
func (e *Err) SetLabel(label string) Error {
	e.label = label
	return e
}

// Error returns the message from within an error
func (e *Err) Error() string {
	return e.err.Error()
}

// IsClass checks whether an error, or any of the errors wrapped within it, is of a given Class
func IsClass(err error, class Class) bool {
	if err == nil {
		return false
	}

	e, ok := err.(*Err)
	if !ok {
		return false
	}
	if e.class == class {
		return true
	}
	return IsClass(xerrors.Unwrap(e.err), class)
}

// IsClassTopLevel checks whether an error, or any of the errors wrapped within it, which has a defined
// class, is of a given Class.
func IsClassTopLevel(err error, class Class) bool {
	if err == nil {
		return false
	}

	e, ok := err.(*Err)
	if !ok {
		return false
	}
	if e.class == ClassUnknown {
		return IsClassTopLevel(xerrors.Unwrap(e.err), class)
	}
	return e.class == class
}

// IsLabel checks whether a label, or any of the errors wrapped within it, is of a given Label
func IsLabel(err error, label string) bool {
	if err == nil {
		return false
	}

	e, ok := err.(*Err)
	if !ok {
		return false
	}
	if e.label == label {
		return true
	}
	return IsLabel(xerrors.Unwrap(e.err), label)
}
