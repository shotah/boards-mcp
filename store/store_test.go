package store

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func testBoard(t *testing.T, author string, writes int, now *time.Time) *Board {
	t.Helper()
	clock := func() time.Time {
		if now != nil {
			return now.UTC()
		}
		return time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	}
	b, err := Open(Config{
		Path: t.TempDir(),
		Identity: Identity{
			Author:       author,
			Role:         RoleAgent,
			WritesPerDay: writes,
		},
		Now: clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRosterCreateListDelete(t *testing.T) {
	t.Parallel()
	b := testBoard(t, "maya", 12, nil)
	if _, err := b.RosterCreate("Maya", "Sister"); err != nil {
		t.Fatal(err)
	}
	rows, err := b.RosterList()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Author != "maya" || rows[0].UserName != "Sister" {
		t.Fatalf("roster = %+v", rows)
	}
	if err := b.RosterDelete(); err != nil {
		t.Fatal(err)
	}
	rows, err = b.RosterList()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("after delete = %+v", rows)
	}
	if err := b.RosterDelete(); err == nil || !strings.Contains(err.Error(), "not on the roster") {
		t.Fatalf("second delete = %v", err)
	}
}

func TestRosterUserNameCollision(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	open := func(author string) *Board {
		b, err := Open(Config{
			Path:     dir,
			Identity: Identity{Author: author, Role: RoleAgent, WritesPerDay: 12},
			Now:      func() time.Time { return time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC) },
		})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if _, err := open("maya").RosterCreate("Maya", "Chris"); err != nil {
		t.Fatal(err)
	}
	err := error(nil)
	_, err = open("kit").RosterCreate("Kit", "chris")
	if err == nil || !strings.Contains(err.Error(), "taken") {
		t.Fatalf("collision = %v", err)
	}
}

func TestRosterIgnoresSpoofedAuthor(t *testing.T) {
	t.Parallel()
	b := testBoard(t, "kit", 12, nil)
	got, err := b.RosterCreate("Kit", "Chris")
	if err != nil {
		t.Fatal(err)
	}
	if got.Author != "kit" {
		t.Fatalf("author = %q", got.Author)
	}
}

func TestTwoCheckInsSameWake(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	open := func(author string) *Board {
		b, err := Open(Config{
			Path:     dir,
			Identity: Identity{Author: author, Role: RoleAgent, WritesPerDay: 12},
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
	sleep, err := maya.ChallengesCreate(CreateChallenge{
		Title: "sleep week", Kind: kindSleepScore, Mode: modeAverage, Target: 80,
		WindowDays: 14, Participants: []string{"maya", "kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := maya.ChallengesCreate(CreateChallenge{
		Title: "100k", Kind: kindSteps, Mode: modeSum, Target: 100000,
		WindowDays: 14, Participants: []string{"maya", "kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: sleep.ID, Action: actionCheckIn, Value: 82, HasVal: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: steps.ID, Action: actionCheckIn, Value: 9000, HasVal: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateCheckInSameDay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	open := func(author string) *Board {
		b, err := Open(Config{
			Path:     dir,
			Identity: Identity{Author: author, Role: RoleAgent, WritesPerDay: 12},
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
	ch, err := maya.ChallengesCreate(CreateChallenge{
		Title: "sleep", Kind: kindSleepScore, WindowDays: 7, Participants: []string{"maya", "kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 80, HasVal: true}); err != nil {
		t.Fatal(err)
	}
	_, err = kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 81, HasVal: true})
	if err == nil || !strings.Contains(err.Error(), "already checked in") {
		t.Fatalf("dup = %v", err)
	}
}

func TestWriteBudget(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	b := testBoard(t, "kit", 2, &now)
	if _, err := b.RosterCreate("Kit", "Chris"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.NoticesCreate("one"); err != nil {
		t.Fatal(err)
	}
	_, err := b.NoticesCreate("two")
	if err == nil || !strings.Contains(err.Error(), "writes used") {
		t.Fatalf("budget = %v", err)
	}
	now = now.Add(25 * time.Hour)
	if _, err := b.NoticesCreate("later"); err != nil {
		t.Fatal(err)
	}
}

func TestFlockTwoGoroutines(t *testing.T) {
	t.Parallel()
	b := testBoard(t, "kit", 12, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			_, err := b.NoticesCreate("pin")
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := b.NoticesList(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("notices = %d", len(rows))
	}
}

func TestFourteenDayAverageSettle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	open := func(author string) *Board {
		b, err := Open(Config{
			Path:     dir,
			Identity: Identity{Author: author, Role: RoleAgent, WritesPerDay: 12},
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
	ch, err := maya.ChallengesCreate(CreateChallenge{
		Title: "sleep fortnight", Kind: kindSleepScore, Mode: modeAverage, Target: 80,
		WindowDays: 14, Participants: []string{"maya", "kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maya.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 70, HasVal: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 90, HasVal: true}); err != nil {
		t.Fatal(err)
	}
	view, err := maya.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionSettle})
	if err != nil {
		t.Fatal(err)
	}
	if view.Challenge.Winner != "kit" || view.Challenge.Status != statusClosed {
		t.Fatalf("settle = %+v scores=%+v", view.Challenge, view.Scores)
	}
}

func TestFromEnvRequiresAuthor(t *testing.T) {
	t.Setenv("BOARDS_AUTHOR", "")
	t.Setenv("BOARDS_PATH", t.TempDir())
	_, err := FromEnv()
	if err == nil || !strings.Contains(err.Error(), "BOARDS_AUTHOR") {
		t.Fatalf("fromenv = %v", err)
	}
}
