package store

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenRejectsBadIdentity(t *testing.T) {
	t.Parallel()
	if _, err := Open(Config{Path: t.TempDir(), Identity: Identity{Author: "Kit"}}); err == nil {
		t.Fatal("expected slug error")
	}
	if _, err := Open(Config{Path: "", Identity: Identity{Author: "kit"}}); err == nil {
		t.Fatal("expected path error")
	}
}

func TestAuthorAndDefaultWrites(t *testing.T) {
	t.Parallel()
	b, err := Open(Config{
		Path:     t.TempDir(),
		Identity: Identity{Author: "kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Author() != "kit" {
		t.Fatalf("author = %q", b.Author())
	}
}

func TestFromEnvRolesAndWrites(t *testing.T) {
	t.Setenv("BOARDS_AUTHOR", "kit")
	t.Setenv("BOARDS_ROLE", "human")
	t.Setenv("BOARDS_WRITES_PER_DAY", "20")
	t.Setenv("BOARDS_PATH", "/tmp/boards-test")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Identity.Role != RoleHuman || cfg.Identity.WritesPerDay != 20 || cfg.Path != "/tmp/boards-test" {
		t.Fatalf("%+v", cfg)
	}

	t.Setenv("BOARDS_ROLE", "")
	t.Setenv("BOARDS_WRITES_PER_DAY", "")
	t.Setenv("BOARDS_PATH", "")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Identity.Role != RoleAgent || cfg.Identity.WritesPerDay != defaultAgentWrites || cfg.Path != defaultPath {
		t.Fatalf("defaults %+v", cfg)
	}

	t.Setenv("BOARDS_AUTHOR", "KIT")
	if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), "slug") {
		t.Fatalf("bad slug = %v", err)
	}
	t.Setenv("BOARDS_AUTHOR", "kit")
	t.Setenv("BOARDS_ROLE", "god")
	if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), "BOARDS_ROLE") {
		t.Fatalf("bad role = %v", err)
	}
	t.Setenv("BOARDS_ROLE", "agent")
	t.Setenv("BOARDS_WRITES_PER_DAY", "nope")
	if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), "BOARDS_WRITES_PER_DAY") {
		t.Fatalf("bad writes = %v", err)
	}
}

func TestRosterNameRulesAndUpsert(t *testing.T) {
	t.Parallel()
	b := testBoard(t, "kit", 12, nil)
	if _, err := b.RosterCreate("", "Chris"); err == nil {
		t.Fatal("expected agent_name")
	}
	if _, err := b.RosterCreate("Kit", strings.Repeat("x", 41)); err == nil {
		t.Fatal("expected too long")
	}
	first, err := b.RosterCreate("Kit", "Chris")
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.RosterCreate("Kitten", "Christopher")
	if err != nil {
		t.Fatal(err)
	}
	if first.Author != second.Author {
		t.Fatalf("upsert author %q %q", first.Author, second.Author)
	}
	rows, err := b.RosterList()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].AgentName != "Kitten" || rows[0].UserName != "Christopher" {
		t.Fatalf("upsert = %+v", rows)
	}
}

func TestNoticesEdges(t *testing.T) {
	t.Parallel()
	b := testBoard(t, "kit", 12, nil)
	if _, err := b.NoticesCreate("  "); err == nil || !strings.Contains(err.Error(), "body is required") {
		t.Fatalf("empty = %v", err)
	}
	if _, err := b.NoticesCreate(strings.Repeat("a", maxNoticeBody+1)); err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("long = %v", err)
	}
	if _, err := b.NoticesCreate("one"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.NoticesCreate("two"); err != nil {
		t.Fatal(err)
	}
	rows, err := b.NoticesList(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Body != "two" {
		t.Fatalf("newest first = %+v", rows)
	}
	capped, err := b.NoticesList(99)
	if err != nil || len(capped) != 2 {
		t.Fatalf("clamp = %d %v", len(capped), err)
	}
}

func TestChallengeStoreEdges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	open := func(author string) *Board {
		t.Helper()
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

	if _, err := maya.ChallengesCreate(CreateChallenge{}); err == nil {
		t.Fatal("title")
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{Title: "x", Kind: "laps"}); err == nil {
		t.Fatal("kind")
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{Title: "x", Kind: kindSteps, Mode: "max"}); err == nil {
		t.Fatal("mode")
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{Title: "x", Kind: kindSteps, Participants: nil}); err == nil {
		t.Fatal("participants")
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "x", Kind: kindSteps, Participants: []string{"maya", "ghost"},
	}); err == nil || !strings.Contains(err.Error(), "not on the board") {
		t.Fatalf("ghost = %v", err)
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "x", Kind: kindSteps, Participants: []string{"Not A Slug"},
	}); err == nil || !strings.Contains(err.Error(), "not a slug") {
		t.Fatalf("slug = %v", err)
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "x", Kind: kindSteps, WindowDays: 15, Participants: []string{"maya", "kit"},
	}); err == nil || !strings.Contains(err.Error(), "1–14") {
		t.Fatalf("window days = %v", err)
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "x", Kind: kindSteps, WindowStart: "2026-09-03", Participants: []string{"maya", "kit"},
	}); err == nil || !strings.Contains(err.Error(), "pair") {
		t.Fatalf("pair = %v", err)
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "x", Kind: kindSteps, WindowStart: "nope", WindowEnd: "2026-09-10", Participants: []string{"maya", "kit"},
	}); err == nil {
		t.Fatal("bad start")
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "x", Kind: kindSteps, WindowStart: "2026-09-10", WindowEnd: "nope", Participants: []string{"maya", "kit"},
	}); err == nil {
		t.Fatal("bad end")
	}
	if _, err := maya.ChallengesCreate(CreateChallenge{
		Title: "x", Kind: kindSteps, WindowStart: "2026-09-10", WindowEnd: "2026-09-01", Participants: []string{"maya", "kit"},
	}); err == nil || !strings.Contains(err.Error(), "on or after") {
		t.Fatalf("order = %v", err)
	}

	ch, err := maya.ChallengesCreate(CreateChallenge{
		Title: "daily steps", Kind: kindSteps, Mode: modeDaily, Target: 8000,
		WindowStart: "2026-09-01", WindowEnd: "2026-09-14", Participants: []string{"kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(ch.Participants, "maya") {
		t.Fatalf("creator should be added: %+v", ch.Participants)
	}

	mine, err := kit.ChallengesList(10, true)
	if err != nil || len(mine) != 1 {
		t.Fatalf("kit mine = %+v %v", mine, err)
	}
	all, err := kit.ChallengesList(10, false)
	if err != nil || len(all) != 1 {
		t.Fatalf("all = %+v %v", all, err)
	}
	view, err := kit.ChallengesGet(ch.ID)
	if err != nil || view.Challenge.Title != "daily steps" {
		t.Fatalf("get = %+v %v", view, err)
	}
	if _, err := kit.ChallengesGet(""); err == nil {
		t.Fatal("empty get")
	}
	if _, err := kit.ChallengesGet("c_missing"); err == nil {
		t.Fatal("missing get")
	}

	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: "wave"}); err == nil {
		t.Fatal("bad action")
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{Action: actionAccept}); err == nil {
		t.Fatal("empty id")
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionAccept}); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionDecline}); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionAccept}); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn}); err == nil {
		t.Fatal("value required")
	}

	ada := open("ada")
	if _, err := ada.RosterCreate("Ada", "Ada"); err != nil {
		t.Fatal(err)
	}
	if _, err := ada.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 1, HasVal: true}); err == nil || !strings.Contains(err.Error(), "not a participant") {
		t.Fatalf("ada = %v", err)
	}

	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 9000, HasVal: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := maya.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 9000, HasVal: true}); err != nil {
		t.Fatal(err)
	}
	tied, err := maya.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionSettle})
	if err != nil {
		t.Fatal(err)
	}
	if tied.Challenge.Winner != "" || tied.Challenge.Status != statusClosed {
		t.Fatalf("tie = %+v", tied.Challenge)
	}
	if _, err := kit.ChallengesUpdate(UpdateChallenge{ID: ch.ID, Action: actionCheckIn, Value: 1, HasVal: true}); err == nil || !strings.Contains(err.Error(), "not open") {
		t.Fatalf("closed check-in = %v", err)
	}

	future := open("maya")
	future.now = func() time.Time { return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC) }
	openCh, err := maya.ChallengesCreate(CreateChallenge{
		Title: "soon", Kind: kindCustom, WindowDays: 7, Participants: []string{"maya", "kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := future.ChallengesUpdate(UpdateChallenge{ID: openCh.ID, Action: actionCheckIn, Value: 1, HasVal: true}); err == nil || !strings.Contains(err.Error(), "outside the window") {
		t.Fatalf("outside = %v", err)
	}
}

func TestOpenChallengeCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	maya, err := Open(Config{
		Path:     dir,
		Identity: Identity{Author: "maya", Role: RoleAgent, WritesPerDay: 30},
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	kit, err := Open(Config{
		Path:     dir,
		Identity: Identity{Author: "kit", Role: RoleAgent, WritesPerDay: 12},
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maya.RosterCreate("Maya", "Sister"); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.RosterCreate("Kit", "Chris"); err != nil {
		t.Fatal(err)
	}
	for i := range maxOpenChallenges {
		if _, err := maya.ChallengesCreate(CreateChallenge{
			Title: "c" + strings.Repeat("x", i), Kind: kindCustom, WindowDays: 7, Participants: []string{"maya", "kit"},
		}); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	_, err = maya.ChallengesCreate(CreateChallenge{
		Title: "one more", Kind: kindCustom, WindowDays: 7, Participants: []string{"maya", "kit"},
	})
	if err == nil || !strings.Contains(err.Error(), "open challenges") {
		t.Fatalf("cap = %v", err)
	}
}

func TestParseParticipantsAndTeachText(t *testing.T) {
	t.Parallel()
	got := ParseParticipants(" maya, kit, maya, ")
	if len(got) != 2 || got[0] != "maya" || got[1] != "kit" {
		t.Fatalf("%v", got)
	}
	if text, ok := TeachText(errors.New("plain")); ok || text != "" {
		t.Fatalf("plain = %q %v", text, ok)
	}
	plain := (&Error{Msg: "only"}).Error()
	if plain != "only" {
		t.Fatalf("Error = %q", plain)
	}
}

func TestReadJSONLSkipsBlankAndCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/roster.jsonl"
	if err := os.WriteFile(path, []byte("\n{\"author\":\"kit\",\"agent_name\":\"Kit\",\"user_name\":\"Chris\"}\nnot-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readJSONL[RosterEntry](path); err == nil {
		t.Fatal("expected corrupt jsonl")
	}
	okPath := dir + "/ok.jsonl"
	if err := os.WriteFile(okPath, []byte("\n{\"author\":\"kit\",\"agent_name\":\"Kit\",\"user_name\":\"Chris\"}\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := readJSONL[RosterEntry](okPath)
	if err != nil || len(rows) != 1 || rows[0].Author != "kit" {
		t.Fatalf("skip blank = %+v %v", rows, err)
	}
}

func TestItoa36(t *testing.T) {
	t.Parallel()
	if strconv36(0) != "0" {
		t.Fatalf("zero = %q", strconv36(0))
	}
	if strconv36(-15) == "" {
		t.Fatal("neg empty")
	}
}
