package colors

import (
	"io"
	"os"
	"testing"
)

func TestSplogOutput(t *testing.T) {
	out, errOut := captureOutput(t, func() {
		logger := NewSplog()
		logger.SetQuiet(false)
		logger.SetDebug(true)

		logger.Newline()
		logger.Print("plain")
		logger.Println("line")
		logger.Infof("info %s", "msg")
		logger.Successf("ok %s", "msg")
		logger.Warnf("warn %s", "msg")
		logger.Errorf("err %s", "msg")
		logger.Debugf("dbg %s", "msg")
		logger.Tipf("tip %s", "msg")
		logger.Plain("plain2")

		PrintSuccess("%s", "ok")
		PrintWarning("%s", "warn")
		PrintInfo("%s", "info")
		PrintDebug("%s", "dbg")
		PrintError("%s", "err")

		PrintNav("up", "branch")
		PrintNav("down", "branch")
		PrintNav("other", "branch")
		PrintCheckout("branch")
		PrintCreated("branch", "main")
		PrintTracked("branch", "main")
		PrintDeleted("branch")
		PrintRestacked("branch", "main")
		PrintAlreadyUpToDate("branch")
		PrintConflict("branch", "main")
	})

	if out == "" {
		t.Fatalf("expected stdout output")
	}
	if errOut == "" {
		t.Fatalf("expected stderr output")
	}
}

func TestSplogQuietMode(t *testing.T) {
	out, errOut := captureOutput(t, func() {
		logger := NewSplog()
		logger.SetQuiet(true)
		logger.SetDebug(true)

		logger.Newline()
		logger.Print("plain")
		logger.Println("line")
		logger.Infof("info %s", "msg")
		logger.Successf("ok %s", "msg")
		logger.Warnf("warn %s", "msg")
		logger.Debugf("dbg %s", "msg")
		logger.Tipf("tip %s", "msg")
		logger.Plain("plain2")
	})

	if out == "" {
		t.Fatalf("expected some stdout in quiet mode (warn/debug), got empty")
	}
	if errOut != "" {
		t.Fatalf("expected no stderr in quiet mode, got %q", errOut)
	}
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = outW
	os.Stderr = errW

	fn()

	_ = outW.Close()
	_ = errW.Close()

	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)

	return string(outBytes), string(errBytes)
}
