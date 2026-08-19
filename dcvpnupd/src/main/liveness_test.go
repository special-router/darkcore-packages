package main

import (
	"encoding/json"
	"testing"
)

func TestLivenessPayloadJSON(t *testing.T) {
	payload := livenessPayload{
		Observations: []Observation{
			{Tag: "proxy-1", Alive: true, DelayMs: 42, LastSeenTime: 1755600000, LastTryTime: 1755600030},
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	want := `{"observations":[{"tag":"proxy-1","alive":true,"delayMs":42,"lastSeenTime":1755600000,"lastTryTime":1755600030}]}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestTelemetryDisabledWhenUciUnavailable(t *testing.T) {
	// На этой машине нет бинаря uci: exec.Command вернёт ошибку, и
	// telemetryEnabled должна трактовать это как "выключено" — тот же
	// дефолт, что и на роутере с чистой конфигурацией без опции.
	if telemetryEnabled() {
		t.Fatal("expected telemetry disabled when uci is unavailable")
	}
}
