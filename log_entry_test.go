// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"testing"
)

// parseJSON must never panic when a plugin emits a JSON log line with
// unexpected types for the hclog-reserved keys. A panic in the log pump
// would crash the host process.
func TestParseJSON_NonStringMessageDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("parseJSON panicked on non-string @message: %v", r)
		}
	}()
	// @message is a number rather than a string
	input := []byte(`{"@message": 42, "@level": "info"}`)
	entry, err := parseJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Message != "" {
		t.Fatalf("expected empty Message when @message is not a string, got %q", entry.Message)
	}
	if entry.Level != "info" {
		t.Fatalf("expected Level=info, got %q", entry.Level)
	}
}

func TestParseJSON_NonStringLevelDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("parseJSON panicked on non-string @level: %v", r)
		}
	}()
	input := []byte(`{"@message": "hi", "@level": 7}`)
	entry, err := parseJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Level != "" {
		t.Fatalf("expected empty Level when @level is not a string, got %q", entry.Level)
	}
}

func TestParseJSON_NonStringTimestampDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("parseJSON panicked on non-string @timestamp: %v", r)
		}
	}()
	input := []byte(`{"@message": "hi", "@timestamp": 12345}`)
	if _, err := parseJSON(input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseJSON_Valid(t *testing.T) {
	input := []byte(`{"@message": "hi", "@level": "debug", "key": "value"}`)
	entry, err := parseJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Message != "hi" || entry.Level != "debug" {
		t.Fatalf("bad parse: %+v", entry)
	}
}
