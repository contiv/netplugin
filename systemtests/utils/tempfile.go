package utils

import (
	"io/ioutil"
	"os"

	"github.com/contiv/netplugin/core"
)

type TempFileCtx struct {
	dir   string
	files []*os.File
}

func (ctx *TempFileCtx) Create(fileContents string) (*os.File, error) {
	if ctx.dir != "" && len(ctx.files) != 0 {
		return nil, &core.Error{Desc: "Create context called for an already created context!"}
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

	var file *os.File = nil
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
