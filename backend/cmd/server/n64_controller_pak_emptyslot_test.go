package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression for the N64 Controller Pak HTTP 422 upload bug: real-world mempaks
// (e.g. a MiSTer-written .cpk) can contain a blank/empty-named note slot that
// pakfs surfaces in ReadDirRoot but cannot Open, which previously made both
// countN64ControllerPakEntries and extractLogicalEntries fail with
// `open controller pak entry "": open : invalid argument` — rejecting the whole
// (valid) save with HTTP 422. Fixture is a real Controller Pak exhibiting this.
func TestN64ControllerPakEmptySlotDoesNotError(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "n64_controller_pak_empty_slot.cpk"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	count, err := countN64ControllerPakEntries(buf)
	if err != nil {
		t.Fatalf("countN64ControllerPakEntries errored on real mempak with empty slot: %v", err)
	}

	store, err := newN64ControllerPakStore(t.TempDir())
	if err != nil {
		t.Fatalf("newN64ControllerPakStore: %v", err)
	}
	entries, err := store.extractLogicalEntries("synctest", buf)
	if err != nil {
		if strings.Contains(err.Error(), "open controller pak entry") {
			t.Fatalf("regression: extract still fails on empty slot: %v", err)
		}
		t.Fatalf("extractLogicalEntries errored: %v", err)
	}
	if len(entries) != count {
		t.Fatalf("extract/count disagree: count=%d extracted=%d", count, len(entries))
	}
}
