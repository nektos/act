package main

import (
	"os"
	"testing"
)

func TestMain(_ *testing.T) {
	os.Args = []string{"act", "--help"}
	main()
}
