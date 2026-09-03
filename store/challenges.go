package store

import (
	"fmt"
	"strings"
)

// ChallengesList returns newest first. mine filters to this author as participant.
func (b *Board) ChallengesList(limit int, mine bool) ([]Challenge, error) {
	limit = clampLimit(limit)
	var out []Challenge
	err := b.do(func() error {
		rows, err := readJSONL[Challenge](file(b.dir, fileChallenges))
		if err != nil {
			return err
		}
		filtered := make([]Challenge, 0, len(rows))
		for _, c := range rows {
			if mine && !contains(c.Participants, b.id.Author) {
				continue
			}
			filtered = append(filtered, c)
		}
		out = newestChallenges(filtered, limit)
		return nil
	})
	return out, err
}

// ChallengesGet returns one contest plus check-ins.
func (b *Board) ChallengesGet(id string) (ChallengeView, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ChallengeView{}, teach("challenge_id is required.", `challenges_get(challenge_id="c_…")`)
	}
	var view ChallengeView
	err := b.do(func() error {
		c, err := b.findChallengeLocked(id)
		if err != nil {
			return err
		}
		ins, err := b.checkinsForLocked(id)
		if err != nil {
			return err
		}
		view = ChallengeView{Challenge: c, CheckIns: ins, Scores: scoresFor(c, ins)}
		return nil
	})
	return view, err
}

// ChallengesCreate proposes a contest. All participants must be on the roster.
func (b *Board) ChallengesCreate(in CreateChallenge) (Challenge, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Challenge{}, teach("title is required.", `challenges_create(title="100k steps", kind="steps", mode="sum", target=100000, window_days=14, participants="maya,kit")`)
	}
	kind := strings.TrimSpace(in.Kind)
	if !validKind(kind) {
		return Challenge{}, teach("kind must be sleep_score, steps, run_km, move_minutes, or custom.", `challenges_create(..., kind="steps")`)
	}
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		if kind == kindSleepScore {
			mode = modeAverage
		} else {
			mode = modeSum
		}
	}
	if !validMode(mode) {
		return Challenge{}, teach("mode must be average, sum, or daily.", `challenges_create(..., mode="sum")`)
	}
	parts := in.Participants
	if len(parts) == 0 {
		return Challenge{}, teach("participants are required (author ids from roster_list).", `challenges_create(..., participants="maya,kit")`)
	}
	if !contains(parts, b.id.Author) {
		parts = append([]string{b.id.Author}, parts...)
	}
	var saved Challenge
	err := b.do(func() error {
		if err := b.checkBudgetLocked(); err != nil {
			return err
		}
		start, end, err := windowDates(b.now(), in.WindowDays, strings.TrimSpace(in.WindowStart), strings.TrimSpace(in.WindowEnd))
		if err != nil {
			return err
		}
		roster, err := b.rosterMapLocked()
		if err != nil {
			return err
		}
		for _, p := range parts {
			if !authorRE.MatchString(p) {
				return teach(fmt.Sprintf("participant %q is not a slug.", p), `roster_list() then challenges_create(..., participants="maya,kit")`)
			}
			if _, ok := roster[p]; !ok {
				return teach(
					p+" is not on the board; ask them to register.",
					`roster_list()`,
				)
			}
		}
		rows, err := readJSONL[Challenge](file(b.dir, fileChallenges))
		if err != nil {
			return err
		}
		open := 0
		for _, c := range rows {
			if c.Status == statusOpen {
				open++
			}
		}
		if open >= maxOpenChallenges {
			return teach(formatOpenCap(), `challenges_list() and wait for one to close`)
		}
		now := b.now().UTC()
		saved = Challenge{
			ID:           newID(challengeIDPrefix, now),
			Title:        title,
			Kind:         kind,
			Mode:         mode,
			Target:       in.Target,
			WindowStart:  start,
			WindowEnd:    end,
			Participants: parts,
			Status:       statusOpen,
			CreatedAt:    now,
			CreatedBy:    b.id.Author,
		}
		rows = append(rows, saved)
		if err := writeJSONL(file(b.dir, fileChallenges), rows); err != nil {
			return err
		}
		return b.recordWriteLocked("challenges_create")
	})
	return saved, err
}

// UpdateChallenge is accept / decline / check_in / settle.
type UpdateChallenge struct {
	ID     string
	Action string
	Value  float64
	Proof  string
	HasVal bool
}

// ChallengesUpdate mutates one contest.
func (b *Board) ChallengesUpdate(in UpdateChallenge) (ChallengeView, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return ChallengeView{}, teach("challenge_id is required.", `challenges_update(challenge_id="c_…", action="accept")`)
	}
	action := strings.TrimSpace(in.Action)
	var view ChallengeView
	err := b.do(func() error {
		switch action {
		case actionAccept, actionDecline, actionCheckIn, actionSettle:
		default:
			return teach("action must be accept, decline, check_in, or settle.", `challenges_update(challenge_id="c_…", action="accept")`)
		}
		if err := b.checkBudgetLocked(); err != nil {
			return err
		}
		c, err := b.findChallengeLocked(id)
		if err != nil {
			return err
		}
		if !contains(c.Participants, b.id.Author) && action != actionSettle {
			return teach("you are not a participant.", `challenges_list()`)
		}
		switch action {
		case actionAccept:
			c.Accepted = addUnique(c.Accepted, b.id.Author)
			c.Declined = removeStr(c.Declined, b.id.Author)
		case actionDecline:
			c.Declined = addUnique(c.Declined, b.id.Author)
			c.Accepted = removeStr(c.Accepted, b.id.Author)
		case actionCheckIn:
			if c.Status != statusOpen {
				return teach("challenge is not open.", `challenges_list()`)
			}
			if !in.HasVal {
				return teach("value is required for check_in.", `challenges_update(challenge_id="c_…", action="check_in", value=8231)`)
			}
			day := b.now().UTC().Format("2006-01-02")
			if day < c.WindowStart || day > c.WindowEnd {
				return teach("check-in is outside the window.", `challenges_get(challenge_id="`+id+`")`)
			}
			ins, err := readJSONL[CheckIn](file(b.dir, fileCheckins))
			if err != nil {
				return err
			}
			for _, row := range ins {
				if row.ChallengeID == id && row.Author == b.id.Author && row.Day == day {
					return teach("already checked in today for this challenge.", `challenges_get(challenge_id="`+id+`")`)
				}
			}
			now := b.now().UTC()
			ins = append(ins, CheckIn{
				ChallengeID: id,
				Author:      b.id.Author,
				At:          now,
				Day:         day,
				Value:       in.Value,
				Proof:       strings.TrimSpace(in.Proof),
			})
			if err := writeJSONL(file(b.dir, fileCheckins), ins); err != nil {
				return err
			}
		case actionSettle:
			ins, err := b.checkinsForLocked(id)
			if err != nil {
				return err
			}
			sc := scoresFor(c, ins)
			c.Winner = pickWinner(sc)
			c.Status = statusClosed
			view.Scores = sc
		}
		if err := b.replaceChallengeLocked(c); err != nil {
			return err
		}
		ins, err := b.checkinsForLocked(id)
		if err != nil {
			return err
		}
		view.Challenge = c
		view.CheckIns = ins
		if view.Scores == nil {
			view.Scores = scoresFor(c, ins)
		}
		return b.recordWriteLocked("challenges_update_" + action)
	})
	return view, err
}

func (b *Board) findChallengeLocked(id string) (Challenge, error) {
	rows, err := readJSONL[Challenge](file(b.dir, fileChallenges))
	if err != nil {
		return Challenge{}, err
	}
	for _, c := range rows {
		if c.ID == id {
			return c, nil
		}
	}
	return Challenge{}, teach("challenge not found.", `challenges_list()`)
}

func (b *Board) replaceChallengeLocked(upd Challenge) error {
	rows, err := readJSONL[Challenge](file(b.dir, fileChallenges))
	if err != nil {
		return err
	}
	for i, c := range rows {
		if c.ID == upd.ID {
			rows[i] = upd
			return writeJSONL(file(b.dir, fileChallenges), rows)
		}
	}
	return teach("challenge not found.", `challenges_list()`)
}

func (b *Board) checkinsForLocked(id string) ([]CheckIn, error) {
	rows, err := readJSONL[CheckIn](file(b.dir, fileCheckins))
	if err != nil {
		return nil, err
	}
	out := make([]CheckIn, 0)
	for _, r := range rows {
		if r.ChallengeID == id {
			out = append(out, r)
		}
	}
	return out, nil
}

func newestChallenges(rows []Challenge, limit int) []Challenge {
	n := len(rows)
	if n == 0 {
		return nil
	}
	out := make([]Challenge, 0, limit)
	for i := n - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, rows[i])
	}
	return out
}

func addUnique(xs []string, v string) []string {
	if contains(xs, v) {
		return xs
	}
	return append(xs, v)
}

func removeStr(xs []string, v string) []string {
	out := xs[:0]
	for _, x := range xs {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func scoresFor(c Challenge, ins []CheckIn) []Score {
	by := map[string][]CheckIn{}
	for _, row := range ins {
		by[row.Author] = append(by[row.Author], row)
	}
	out := make([]Score, 0, len(c.Participants))
	for _, p := range c.Participants {
		rows := by[p]
		var v float64
		switch c.Mode {
		case modeSum:
			for _, r := range rows {
				v += r.Value
			}
		case modeDaily:
			for _, r := range rows {
				if r.Value >= c.Target {
					v++
				}
			}
		default: // average
			if len(rows) == 0 {
				break
			}
			sum := 0.0
			for _, r := range rows {
				sum += r.Value
			}
			v = sum / float64(len(rows))
		}
		out = append(out, Score{Author: p, Value: v})
	}
	return out
}

func pickWinner(sc []Score) string {
	if len(sc) == 0 {
		return ""
	}
	best := sc[0]
	tie := false
	for i := 1; i < len(sc); i++ {
		if sc[i].Value > best.Value {
			best = sc[i]
			tie = false
		} else if sc[i].Value == best.Value {
			tie = true
		}
	}
	if tie {
		return ""
	}
	return best.Author
}
