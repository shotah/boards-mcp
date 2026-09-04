package store

import (
	"strings"
	"testing"
	"time"
)

func TestKindCatalog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	open := func(author string) *Board {
		t.Helper()
		b, err := Open(Config{
			Path:     dir,
			Identity: Identity{Author: author, Role: RoleAgent, WritesPerDay: 30},
			Now:      func() time.Time { return now },
		})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	maya := open("maya")
	kit := open("kit")
	if _, err := maya.RosterCreate("Maya", "Sister"); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.RosterCreate("Kit", "Chris"); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{kindSteps, kindDistance, kindElevation, kindMove, kindSleep, kindCount, kindCustom} {
		got, err := maya.ChallengesCreate(CreateChallenge{
			Title: kind + " week", Kind: kind, WindowDays: 7, Participants: []string{"maya", "kit"},
		})
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if got.Kind != kind {
			t.Fatalf("stored %q want %q", got.Kind, kind)
		}
		wantMode := modeSum
		if kind == kindSleep {
			wantMode = modeAverage
		}
		if got.Mode != wantMode {
			t.Fatalf("%s mode %q want %q", kind, got.Mode, wantMode)
		}
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "laps", Kind: "laps", WindowDays: 7, Participants: []string{"maya", "kit"},
	}); err == nil || !strings.Contains(err.Error(), "steps, distance, elevation, move, sleep, count, or custom") {
		t.Fatalf("laps = %v", err)
	}
}
