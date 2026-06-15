package main

import (
	"net/http"
	"testing"
)

func TestSaveUploadPreviewAcceptsAndRejectsWithoutPersisting(t *testing.T) {
	h := newContractHarness(t)

	accepted := h.multipart("/api/saves/preview", map[string]string{
		"system":   "gameboy",
		"slotName": "default",
		"rom_sha1": "preview-rom",
	}, "file", "Pokemon Red.sav", buildNonBlankPayload(8192, 0x19))
	assertStatus(t, accepted, http.StatusOK)
	acceptedBody := decodeJSONMap(t, accepted.Body)
	if mustNumber(t, acceptedBody["acceptedCount"], "acceptedCount") != 1 {
		t.Fatalf("expected one accepted preview: %s", prettyJSON(acceptedBody))
	}
	items := mustArray(t, acceptedBody["items"], "items")
	first := mustObject(t, items[0], "items[0]")
	if !mustBool(t, first["accepted"], "items[0].accepted") {
		t.Fatalf("expected accepted preview item: %s", prettyJSON(first))
	}

	rejected := h.multipart("/api/saves/preview", nil, "file", "notes.txt", []byte("not a save file"))
	assertStatus(t, rejected, http.StatusOK)
	rejectedBody := decodeJSONMap(t, rejected.Body)
	if mustNumber(t, rejectedBody["rejectedCount"], "rejectedCount") != 1 {
		t.Fatalf("expected one rejected preview: %s", prettyJSON(rejectedBody))
	}

	list := h.request(http.MethodGet, "/saves?limit=100&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	if len(mustArray(t, listBody["saves"], "saves")) != 1 {
		t.Fatalf("preview should not persist saves: %s", prettyJSON(listBody))
	}
}

func TestRejectedUploadIsQuarantinedAndListedInValidation(t *testing.T) {
	h := newContractHarness(t)

	upload := h.multipart("/saves", nil, "file", "notes.txt", []byte("not a save file"))
	assertStatus(t, upload, http.StatusUnprocessableEntity)

	status := h.request(http.MethodGet, "/api/validation", nil)
	assertStatus(t, status, http.StatusOK)
	body := decodeJSONMap(t, status.Body)
	validation := mustObject(t, body["validation"], "validation")
	if mustNumber(t, validation["quarantineCount"], "validation.quarantineCount") != 1 {
		t.Fatalf("expected one quarantined upload: %s", prettyJSON(validation))
	}
	quarantine := mustArray(t, validation["quarantine"], "validation.quarantine")
	first := mustObject(t, quarantine[0], "quarantine[0]")
	if mustString(t, first["filename"], "filename") != "notes.txt" {
		t.Fatalf("unexpected quarantine file: %s", prettyJSON(first))
	}
	if mustString(t, first["reason"], "reason") == "" {
		t.Fatalf("expected quarantine reason: %s", prettyJSON(first))
	}
}

func TestValidationQuarantineRetryImportsNowValidSave(t *testing.T) {
	h := newContractHarness(t)

	payload := buildNonBlankPayload(8192, 0x29)
	h.app.quarantineRejectedUpload("Pokemon Red.sav", "/media/saves/Pokemon Red.sav", payload, saveUploadPreviewItem{
		Filename:     "Pokemon Red.sav",
		SourcePath:   "/media/saves/Pokemon Red.sav",
		SystemSlug:   "gameboy",
		Format:       "sav",
		DisplayTitle: "Pokemon Red",
		ROMSHA1:      "pokemon-red-rom",
		Reason:       "old parser rejection",
		ParserLevel:  saveParserLevelNone,
		TrustLevel:   "none",
	}, "test-helper")

	status := h.request(http.MethodGet, "/api/validation", nil)
	assertStatus(t, status, http.StatusOK)
	body := decodeJSONMap(t, status.Body)
	validation := mustObject(t, body["validation"], "validation")
	quarantine := mustArray(t, validation["quarantine"], "validation.quarantine")
	first := mustObject(t, quarantine[0], "quarantine[0]")
	id := mustString(t, first["id"], "quarantine[0].id")

	retry := h.request(http.MethodPost, "/api/validation/quarantine/"+id+"/retry", nil)
	assertStatus(t, retry, http.StatusOK)
	retryBody := decodeJSONMap(t, retry.Body)
	if !mustBool(t, retryBody["success"], "retry.success") {
		t.Fatalf("expected retry success: %s", prettyJSON(retryBody))
	}
	if !mustBool(t, retryBody["imported"], "retry.imported") {
		t.Fatalf("expected retry to import: %s", prettyJSON(retryBody))
	}

	after := h.request(http.MethodGet, "/api/validation", nil)
	assertStatus(t, after, http.StatusOK)
	afterBody := decodeJSONMap(t, after.Body)
	afterValidation := mustObject(t, afterBody["validation"], "validation")
	if mustNumber(t, afterValidation["quarantineCount"], "validation.quarantineCount") != 0 {
		t.Fatalf("expected quarantine to be cleared: %s", prettyJSON(afterValidation))
	}
}

func TestValidationQuarantineDeleteRemovesOnlyQuarantineItem(t *testing.T) {
	h := newContractHarness(t)

	upload := h.multipart("/saves", nil, "file", "notes.txt", []byte("not a save file"))
	assertStatus(t, upload, http.StatusUnprocessableEntity)
	status := h.request(http.MethodGet, "/api/validation", nil)
	body := decodeJSONMap(t, status.Body)
	validation := mustObject(t, body["validation"], "validation")
	id := mustString(t, mustObject(t, mustArray(t, validation["quarantine"], "validation.quarantine")[0], "quarantine[0]")["id"], "quarantine[0].id")

	deleted := h.request(http.MethodDelete, "/api/validation/quarantine/"+id, nil)
	assertStatus(t, deleted, http.StatusOK)

	after := h.request(http.MethodGet, "/api/validation", nil)
	assertStatus(t, after, http.StatusOK)
	afterBody := decodeJSONMap(t, after.Body)
	afterValidation := mustObject(t, afterBody["validation"], "validation")
	if mustNumber(t, afterValidation["quarantineCount"], "validation.quarantineCount") != 0 {
		t.Fatalf("expected quarantine delete to remove item: %s", prettyJSON(afterValidation))
	}
}

// Re-uploading the same payload (same SHA256) must not create a new quarantine
// entry — instead the existing record's RetryCount + LastSeenAt are bumped.
// Without this, a misbehaving sync client (we observed sgm-steamdeck-helper
// retrying a rejected save every 30s cycle) makes quarantine grow unbounded
// on disk.
func TestQuarantineDedupesByPayloadSHA256(t *testing.T) {
	h := newContractHarness(t)

	body := []byte("not a save file")
	for i := 0; i < 5; i++ {
		upload := h.multipart("/saves", nil, "file", "notes.txt", body)
		assertStatus(t, upload, http.StatusUnprocessableEntity)
	}

	status := h.request(http.MethodGet, "/api/validation", nil)
	assertStatus(t, status, http.StatusOK)
	statusBody := decodeJSONMap(t, status.Body)
	validation := mustObject(t, statusBody["validation"], "validation")
	if got := mustNumber(t, validation["quarantineCount"], "validation.quarantineCount"); got != 1 {
		t.Fatalf("expected exactly 1 quarantine entry after 5 dup uploads, got %v: %s", got, prettyJSON(validation))
	}
	quarantine := mustArray(t, validation["quarantine"], "validation.quarantine")
	first := mustObject(t, quarantine[0], "quarantine[0]")
	retryCount := mustNumber(t, first["retryCount"], "quarantine[0].retryCount")
	if retryCount != 5 {
		t.Fatalf("expected retryCount=5 after 5 uploads, got %v: %s", retryCount, prettyJSON(first))
	}
	if first["lastSeenAt"] == nil {
		t.Fatalf("expected lastSeenAt to be set after a retry: %s", prettyJSON(first))
	}
}
