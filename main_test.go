package main

import "testing"

func TestVersionConstIsSet(t *testing.T) {
	if version == "" {
		t.Fatal("version should not be empty")
	}
}
