package main

import "testing"

func TestNumberedSaveSlotTitlePattern(t *testing.T) {
	// True positives — real numbered slot/mission artifacts that we DO want to drop.
	wantNoise := []string{
		"1 - Save",
		"01 - Save Slot",
		"02 - Backup",
		"3 - Mission",
		"4 - Auto",
		"05 - Autosave",
		"7 - Player Slot",
		"008 - File 8",
		"01 - State",
		"02 - Memory Card",
		"01 - SP0",
		"02 - SP12",
	}
	for _, title := range wantNoise {
		t.Run("noise/"+title, func(t *testing.T) {
			if !numberedSaveSlotTitlePattern.MatchString(title) {
				t.Errorf("expected %q to match numberedSaveSlotTitlePattern (real slot artifact)", title)
			}
		})
	}

	// False positives — real game titles that previously got rejected and shouldn't.
	// See: 2026-06-07 burn where Deck saves for 007 - Everything or Nothing (GBA)
	// were rejected with status=422 because the title prefix "007 - " matched the
	// over-broad original pattern `^\s*[0-9]{1,3}\s*-\s*[A-Za-z]`.
	wantGame := []string{
		"007 - Everything or Nothing",
		"007 - The World Is Not Enough",
		"007 - GoldenEye",
		"007 - Agent Under Fire",
		"3 - Count Bout",                      // real arcade game, no dash
		"19 - Centerfold",                     // real game
		"99 - The Last Homeland",              // hypothetical real game
		"4 - Wheel Thunder",                   // SNK Dreamcast game
		"007 - James Bond: Quantum of Solace", // real game
		"42 - The Answer",                     // hypothetical, real-game-looking
	}
	for _, title := range wantGame {
		t.Run("game/"+title, func(t *testing.T) {
			if numberedSaveSlotTitlePattern.MatchString(title) {
				t.Errorf("expected %q to NOT match numberedSaveSlotTitlePattern (real game title)", title)
			}
		})
	}
}
