package filter

import (
	"testing"
)

func TestCIn(t *testing.T) {
	if !cIn("xyz", 'y') {
		t.Fatal()
	}
	if cIn("xyz", 'a') {
		t.Fatal()
	}
}

func TestCIndex(t *testing.T) {
	if cIndex("xyz", 'y') != 1 {
		t.Fatal()
	}
	if cIndex("xyz", 'a') != -1 {
		t.Fatal()
	}
}
