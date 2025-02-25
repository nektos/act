package cmd

import (
	"context"
	"os"
	"testing"
)

// Helper function to test main with different os.Args
func testMain(args []string) (exitCode int) {
	// Save original os.Args and defer restoring it
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Save original os.Exit and defer restoring it
	defer func() { exitFunc = os.Exit }()

	// Mock os.Exit
	fakeExit := func(code int) {
		exitCode = code
	}
	exitFunc = fakeExit

	// Mock os.Args
	os.Args = args

	// Run the main function
	Execute(context.Background(), "")

	return exitCode
}

func TestMainHelp(t *testing.T) {
	exitCode := testMain([]string{"cmd", "--help"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestMainNoArgsError(t *testing.T) {
	exitCode := testMain([]string{"cmd"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
}
