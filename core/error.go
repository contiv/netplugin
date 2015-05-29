/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"fmt"
	"runtime"
	"strings"
)

// Error is our custom error with description, file, and line.
type Error struct {
	desc string
	file string
	line int
}

// Error() allows *core.Error to present the `error` interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s [%s %d]", e.desc, e.file, e.line)
}

// Errorf returns an *Error based on the format specification provided.
func Errorf(f string, args ...interface{}) *Error {
	e := &Error{}
	e.desc = fmt.Sprintf(f, args...)
	_, e.file, e.line, _ = runtime.Caller(1)
	e.file = e.file[strings.LastIndex(e.file, "/")+1:]
	return e
}

// ErrIfKeyExists checks if the error message contains "Key not found".
func ErrIfKeyExists(err error) error {
	if err == nil || strings.Contains(err.Error(), "Key not found") {
		return nil
	}

	return err
}
