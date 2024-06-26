// package backend
package main

import (
	"math"
	"testing"
)

//test per handler
// func testuser_handler_getUserByID
// it

func TestBackend(t *testing.T) {
	got := math.Abs(-1)
	if got != 1 {
		t.Errorf("Abs(-1) = %f; want 1", got)
	}
}
