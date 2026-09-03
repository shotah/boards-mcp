package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	RoleAgent = "agent"
	RoleHuman = "human"

	defaultAgentWrites = 12
	defaultHumanWrites = 30
	defaultPath        = "/boards"

	maxNoticeBody     = 500
	defaultListLimit  = 25
	maxListLimit      = 50
	maxOpenChallenges = 8
	maxNoticesKept    = 100
	defaultWindowDays = 7
	maxWindowDays     = 14
	noticeIDPrefix    = "n_"
	challengeIDPrefix = "c_"
	statusOpen        = "open"
	statusClosed      = "closed"
	kindSleepScore    = "sleep_score"
	kindSteps         = "steps"
	kindRunKM         = "run_km"
	kindMoveMinutes   = "move_minutes"
	kindCustom        = "custom"
	modeAverage       = "average"
	modeSum           = "sum"
	modeDaily         = "daily"
	actionAccept      = "accept"
	actionDecline     = "decline"
	actionCheckIn     = "check_in"
	actionSettle      = "settle"
)

var authorRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// Identity is who this process writes as. Author is never taken from tool args.
type Identity struct {
	Author       string
	Role         string
	WritesPerDay int
}

// Config opens a board directory.
type Config struct {
	Path     string
	Identity Identity
	Now      func() time.Time
}

// FromEnv reads BOARDS_* (author required).
func FromEnv() (Config, error) {
	author := strings.TrimSpace(os.Getenv("BOARDS_AUTHOR"))
	if author == "" {
		return Config{}, teach("BOARDS_AUTHOR is required.", `set BOARDS_AUTHOR to this crane slug (kit, maya)`)
	}
	if !authorRE.MatchString(author) {
		return Config{}, teach("BOARDS_AUTHOR must be a short slug (kit, maya).", `set BOARDS_AUTHOR=kit`)
	}
	role := strings.ToLower(strings.TrimSpace(os.Getenv("BOARDS_ROLE")))
	if role == "" {
		role = RoleAgent
	}
	if role != RoleAgent && role != RoleHuman {
		return Config{}, teach("BOARDS_ROLE must be agent or human.", `set BOARDS_ROLE=agent`)
	}
	writes := defaultAgentWrites
	if role == RoleHuman {
		writes = defaultHumanWrites
	}
	if raw := strings.TrimSpace(os.Getenv("BOARDS_WRITES_PER_DAY")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			return Config{}, teach("BOARDS_WRITES_PER_DAY must be 1–100.", `set BOARDS_WRITES_PER_DAY=12`)
		}
		writes = n
	}
	path := strings.TrimSpace(os.Getenv("BOARDS_PATH"))
	if path == "" {
		path = defaultPath
	}
	return Config{
		Path: path,
		Identity: Identity{
			Author:       author,
			Role:         role,
			WritesPerDay: writes,
		},
	}, nil
}

// RosterEntry is one crane + human on the board.
type RosterEntry struct {
	Author    string    `json:"author"`
	AgentName string    `json:"agent_name"`
	UserName  string    `json:"user_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Notice is a short pin.
type Notice struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Challenge is a dated contest.
type Challenge struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Kind         string    `json:"kind"`
	Mode         string    `json:"mode"`
	Target       float64   `json:"target"`
	WindowStart  string    `json:"window_start"`
	WindowEnd    string    `json:"window_end"`
	Participants []string  `json:"participants"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
	Accepted     []string  `json:"accepted,omitempty"`
	Declined     []string  `json:"declined,omitempty"`
	Winner       string    `json:"winner,omitempty"`
}

// CheckIn is one daily sample.
type CheckIn struct {
	ChallengeID string    `json:"challenge_id"`
	Author      string    `json:"author"`
	At          time.Time `json:"at"`
	Day         string    `json:"day"`
	Value       float64   `json:"value"`
	Proof       string    `json:"proof,omitempty"`
}

// ChallengeView is get payload.
type ChallengeView struct {
	Challenge Challenge `json:"challenge"`
	CheckIns  []CheckIn `json:"check_ins"`
	Scores    []Score   `json:"scores,omitempty"`
}

// Score is a settle aggregate.
type Score struct {
	Author string  `json:"author"`
	Value  float64 `json:"value"`
}

type writeEvent struct {
	At     time.Time `json:"at"`
	Author string    `json:"author"`
	Kind   string    `json:"kind"`
}

// CreateChallenge holds create args.
type CreateChallenge struct {
	Title        string
	Kind         string
	Mode         string
	Target       float64
	WindowDays   int
	WindowStart  string
	WindowEnd    string
	Participants []string
}

func clampLimit(limit int) int {
	if limit < 1 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func contains(xs []string, v string) bool {
	return slices.Contains(xs, v)
}

func uniqueLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func ParseParticipants(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		id := strings.TrimSpace(p)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func validKind(k string) bool {
	switch k {
	case kindSleepScore, kindSteps, kindRunKM, kindMoveMinutes, kindCustom:
		return true
	default:
		return false
	}
}

func validMode(m string) bool {
	switch m {
	case modeAverage, modeSum, modeDaily:
		return true
	default:
		return false
	}
}

func windowDates(now time.Time, days int, start, end string) (string, string, error) {
	if start != "" || end != "" {
		if start == "" || end == "" {
			return "", "", teach("window_start and window_end are a pair.", `challenges_create(..., window_days=14)`)
		}
		if _, err := time.Parse("2006-01-02", start); err != nil {
			return "", "", teach("window_start must be YYYY-MM-DD.", `challenges_create(..., window_days=14)`)
		}
		if _, err := time.Parse("2006-01-02", end); err != nil {
			return "", "", teach("window_end must be YYYY-MM-DD.", `challenges_create(..., window_days=14)`)
		}
		if end < start {
			return "", "", teach("window_end must be on or after window_start.", `challenges_create(..., window_days=14)`)
		}
		return start, end, nil
	}
	if days < 1 {
		days = defaultWindowDays
	}
	if days > maxWindowDays {
		return "", "", teach(fmt.Sprintf("window is 1–%d days.", maxWindowDays), `challenges_create(..., window_days=14)`)
	}
	s := now.UTC().Format("2006-01-02")
	e := now.UTC().AddDate(0, 0, days).Format("2006-01-02")
	return s, e, nil
}

func file(dir, name string) string {
	return filepath.Join(dir, name)
}
