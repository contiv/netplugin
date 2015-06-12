package core

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestErrorStringFormat(t *testing.T) {
	refStr := "error string"
	e := Errorf("%s", refStr)

	fileName := "error_test.go"
	lineNum := 12 // line number where error was formed
	funcName := "github.com/contiv/netplugin/core.TestErrorStringFormat"

	expectedStr := fmt.Sprintf("%s [%s %s %d]", refStr, funcName, fileName, lineNum)

	if e.Error() != expectedStr {
		t.Fatalf("error string mismatch. Expected: %q, got %q", expectedStr,
			e.Error())
	}
}

func getError(msg string) *Error {
	return Errorf(msg)
}

func TestErrorStackTrace(t *testing.T) {
	msg := "an error"
	e := getError(msg)

	if e.desc != msg {
		t.Fatal("Description did not match provided")
	}

	fileName := "error_test.go"
	lineNum := 27 // line number where error was formed
	funcName := "github.com/contiv/netplugin/core.getError"

	expectedStr := fmt.Sprintf("%s [%s %s %d]", msg, funcName, fileName, lineNum)

	if e.Error() != expectedStr {
		t.Fatalf("Error message yielded an incorrect result with CONTIV_TRACE unset: %s", e.Error())
	}

	os.Setenv("CONTIV_TRACE", "1")
	if e.Error() == "an error\n" {
		t.Fatal("Error message did not yield stack trace with CONTIV_TRACE set")
	}

	lines := strings.Split(e.Error(), "\n")

	if len(lines) != 6 {
		t.Fatalf("Stack trace yielded incorrect count: %d", len(lines))
	}
}
