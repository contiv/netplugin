package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestErrorStringFormat(t *testing.T) {
	refStr := "error string"
	e := Errorf("%s", refStr)

	fileName := "error_test.go"
	lineNum := 11 // line number where error was formed
	funcName := "github.com/contiv/netplugin/core.TestErrorStringFormat"

	expectedStr := fmt.Sprintf("%s [%s %d]", funcName, fileName, lineNum)
	if errMsg := strings.Split(e.Error(), "\n"); errMsg[0] != refStr || errMsg[1] != expectedStr {
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

	if e.Error() == "an error\n" {
		t.Fatal("Error message did not yield stack trace")
	}

	lines := strings.Split(e.Error(), "\n")

	if len(lines) != 6 {
		t.Fatalf("Stack trace yielded incorrect count: %d", len(lines))
	}
}
