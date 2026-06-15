package main

import (
	"strings"
	"testing"
)

// buildGBAPayloadWithSignature returns a 32K FLASH-erased buffer with mGBA's
// SRAM_V signature embedded at a low offset, mimicking what standalone mGBA
// would write.
func buildGBAPayloadWithSignature(size int, signature string) []byte {
	payload := make([]byte, size)
	for idx := range payload {
		payload[idx] = 0xFF
	}
	// Embed signature at byte 64 — mGBA writes its version string at a
	// well-defined offset, but for validator purposes any location within
	// the first 512KB scan window works.
	copy(payload[64:], []byte(signature))
	// Add some non-zero bytes so the blank check passes.
	for i := 0; i < 32; i++ {
		payload[i] = 0x5a
	}
	return payload
}

// buildGBAPayloadNoSignature returns a same-size buffer with NO library
// signature and NO AGB cartridge header — simulating what RetroArch's
// libretro-mGBA core writes for Pokemon Emerald, EA-Sports 007 games, etc.
func buildGBAPayloadNoSignature(size int) []byte {
	payload := make([]byte, size)
	for idx := range payload {
		payload[idx] = 0xFF
	}
	// Real-world EA Sports 007 save header (BOND0053 backwards) at offset 0 —
	// guarantees no signature/AGB header collisions.
	copy(payload[0:8], []byte("3500DNOB"))
	// Some non-zero bytes scattered through so the blank check passes.
	for i := 0; i < 32; i++ {
		payload[i] = 0x5a
	}
	return payload
}

// Regression guard: GBA saves WITH the standard library signature must
// continue to pass without any advisory downgrade (no warning emitted).
func TestGBARawSaveAcceptedWithLibrarySignature(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Some Standalone mGBA Save.srm",
		Payload:             buildGBAPayloadWithSignature(32768, "SRAM_V113"),
		Game:                game{Name: "Some Standalone mGBA Save"},
		Format:              "sram",
		ROMSHA1:             "abc123",
		SlotName:            "default",
		SystemSlug:          "gba",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected GBA save with SRAM_V signature to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected GBA inspection metadata")
	}
	for _, w := range result.Input.Inspection.Warnings {
		if strings.Contains(w, "without the standard library signature footer") {
			t.Errorf("did NOT expect advisory-downgrade warning when signature is present: %q", w)
		}
	}
}

// THE FIX: a GBA save from a trusted helper (e.g. RetroArch via SGM-Helper),
// with rom_sha1 present and non-blank payload, but NO library signature,
// must be accepted — with a warning explaining the advisory downgrade.
func TestGBARawSaveAcceptedWithoutSignatureUnderHelperTrust(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Pokemon - Emerald Version (USA, Europe).srm",
		Payload:             buildGBAPayloadNoSignature(131072),
		Game:                game{Name: "Pokemon - Emerald Version"},
		Format:              "flash",
		ROMSHA1:             "pokemon-emerald-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "gba",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected GBA save from trusted helper without signature to be ACCEPTED, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected GBA inspection metadata")
	}
	foundWarning := false
	for _, w := range result.Input.Inspection.Warnings {
		if strings.Contains(w, "without the standard library signature footer") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected advisory-downgrade warning, got warnings: %+v", result.Input.Inspection.Warnings)
	}
}

// Security guard: same scenario as above BUT without rom_sha1 — must reject.
// The advisory downgrade requires all three trust conditions; missing any
// one of them keeps the original reject behavior.
func TestGBARawSaveRejectedWithoutROMSHA1EvenWithHelperTrust(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Pokemon - Emerald Version (USA, Europe).srm",
		Payload:             buildGBAPayloadNoSignature(131072),
		Game:                game{Name: "Pokemon - Emerald Version"},
		Format:              "flash",
		SlotName:            "default",
		SystemSlug:          "gba",
		TrustedHelperSystem: true,
		// rom_sha1 deliberately absent
	})
	if !result.Rejected {
		t.Fatal("expected GBA save without rom_sha1 to be rejected even with helper trust")
	}
	// Pre-signature check (RequireROMSHA1) fires first.
	if !strings.Contains(result.RejectReason, "rom_sha1") {
		t.Errorf("expected rom_sha1 rejection reason, got %q", result.RejectReason)
	}
}

// Security guard: helper-trust signal absent — even with rom_sha1 and
// non-blank payload, must reject. The advisory downgrade is opt-in via
// trusted authenticated channel.
func TestGBARawSaveRejectedWithoutHelperTrustEvenWithROMSHA1(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:   "Pokemon - Emerald Version (USA, Europe).srm",
		Payload:    buildGBAPayloadNoSignature(131072),
		Game:       game{Name: "Pokemon - Emerald Version"},
		Format:     "flash",
		ROMSHA1:    "pokemon-emerald-rom-sha1",
		SlotName:   "default",
		SystemSlug: "gba",
		// TrustedHelperSystem deliberately false
	})
	if !result.Rejected {
		t.Fatal("expected GBA save without helper trust to be rejected even with rom_sha1")
	}
}

// Security guard: blank payload must still be rejected — the advisory
// downgrade explicitly requires non-blank.
func TestGBARawSaveRejectedWhenBlankEvenUnderHelperTrust(t *testing.T) {
	blankFF := make([]byte, 131072)
	for idx := range blankFF {
		blankFF[idx] = 0xFF
	}
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "freshly-launched-game.srm",
		Payload:             blankFF,
		Game:                game{Name: "freshly launched game"},
		Format:              "flash",
		ROMSHA1:             "some-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "gba",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected blank GBA save to be rejected even under helper trust")
	}
}
