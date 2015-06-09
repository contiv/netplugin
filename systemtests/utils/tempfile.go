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

package utils

import (
	"io/ioutil"
	"os"

	"github.com/contiv/netplugin/core"
)

// TempFileCtx allows managing temporary file contexts
type TempFileCtx struct {
	dir   string
	files []*os.File
}

// Create creates a context with specified file-contents
func (ctx *TempFileCtx) Create(fileContents string) (*os.File, error) {
	if ctx.dir != "" && len(ctx.files) != 0 {
		return nil, core.Errorf("Create context called for an already created context!")
	}

	dir, err := ioutil.TempDir("", "netp_tests")
	if err != nil {
		return nil, err
	}
	ctx.dir = dir
	defer func() {
		if err != nil {
			ctx.Destroy()
		}
	}()

	var file *os.File
	file, err = ioutil.TempFile(dir, "netp_tests")
	if err != nil {
		return nil, err
	}
	ctx.files = append(ctx.files, file)

	_, err = file.Write([]byte(fileContents))
	if err != nil {
		return nil, err
	}

	return file, nil
}

// AddFile adds a file to the context with specified file-contents
func (ctx *TempFileCtx) AddFile(fileContents string) (*os.File, error) {
	file, err := ioutil.TempFile(ctx.dir, "netp_tests")
	if err != nil {
		return nil, err
	}
	ctx.files = append(ctx.files, file)
	defer func() {
		if err != nil {
			ctx.files = ctx.files[:len(ctx.files)-1]
			file.Close()
			os.Remove(file.Name())
		}
	}()

	_, err = file.Write([]byte(fileContents))
	if err != nil {
		return nil, err
	}

	return file, nil
}

// Destroy cleans up the context
func (ctx *TempFileCtx) Destroy() {
	if ctx.dir == "" {
		return
	}

	for _, file := range ctx.files {
		file.Close()
	}
	os.RemoveAll(ctx.dir)
	ctx.files = []*os.File{}
	ctx.dir = ""
}
