package main

import (
	"testing"
)

func TestRunRejectsUnknownCommand(t *testing.T) {
	if err := run([]string{"retry"}); err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestRunValidatesReplayArgumentsBeforeConnecting(t *testing.T) {
	err := run([]string{
		"retry-failed",
		"--tenant-id", "not-a-uuid",
		"--event-id", "also-not-a-uuid",
		"--consumer", "pdf-service",
		"--reason", "operator replay",
	})
	if err == nil {
		t.Fatal("expected invalid tenant UUID error")
	}
}
