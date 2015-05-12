package core

import (
	"fmt"
	"testing"
)

func TestErrorStringFormat(t *testing.T) {
	refStr := "error string"
	e := Errorf("%s", refStr)

	fileName := "error_test.go"
	lineNum := 10 // line number where error was formed

	expectedStr := fmt.Sprintf("%s [%s %d]", refStr, fileName, lineNum)

	if e.Error() != expectedStr {
		t.Fatalf("error string mismatch. Expected: %q, got %q", expectedStr,
			e.Error())
	}
}
