package store

import (
	"fmt"
	"strings"
	"time"
)

// RosterList returns every registered crane.
func (b *Board) RosterList() ([]RosterEntry, error) {
	var out []RosterEntry
	err := b.do(func() error {
		rows, err := readJSONL[RosterEntry](file(b.dir, fileRoster))
		out = rows
		return err
	})
	return out, err
}

// RosterCreate upserts this author. user_name must be unique across others.
func (b *Board) RosterCreate(agentName, userName string) (RosterEntry, error) {
	agentName, err := requireName("agent_name", agentName)
	if err != nil {
		return RosterEntry{}, err
	}
	userName, err = requireName("user_name", userName)
	if err != nil {
		return RosterEntry{}, err
	}
	var saved RosterEntry
	err = b.do(func() error {
		if err := b.checkBudgetLocked(); err != nil {
			return err
		}
		rows, err := readJSONL[RosterEntry](file(b.dir, fileRoster))
		if err != nil {
			return err
		}
		want := uniqueLower(userName)
		kept := rows[:0]
		for _, r := range rows {
			if r.Author == b.id.Author {
				continue
			}
			if uniqueLower(r.UserName) == want {
				return teach(
					fmt.Sprintf("user_name %q is taken by %s.", userName, r.Author),
					`roster_list() then pick another user_name`,
				)
			}
			kept = append(kept, r)
		}
		saved = RosterEntry{
			Author:    b.id.Author,
			AgentName: agentName,
			UserName:  userName,
			UpdatedAt: b.now().UTC(),
		}
		kept = append(kept, saved)
		if err := writeJSONL(file(b.dir, fileRoster), kept); err != nil {
			return err
		}
		return b.recordWriteLocked("roster_create")
	})
	return saved, err
}

// RosterDelete removes this author's row. Missing is a teach-in.
func (b *Board) RosterDelete() error {
	return b.do(func() error {
		if err := b.checkBudgetLocked(); err != nil {
			return err
		}
		rows, err := readJSONL[RosterEntry](file(b.dir, fileRoster))
		if err != nil {
			return err
		}
		kept := make([]RosterEntry, 0, len(rows))
		found := false
		for _, r := range rows {
			if r.Author == b.id.Author {
				found = true
				continue
			}
			kept = append(kept, r)
		}
		if !found {
			return teach("you are not on the roster.", `roster_create(agent_name="Kit", user_name="Chris")`)
		}
		if err := writeJSONL(file(b.dir, fileRoster), kept); err != nil {
			return err
		}
		return b.recordWriteLocked("roster_delete")
	})
}

func (b *Board) rosterMapLocked() (map[string]RosterEntry, error) {
	rows, err := readJSONL[RosterEntry](file(b.dir, fileRoster))
	if err != nil {
		return nil, err
	}
	m := make(map[string]RosterEntry, len(rows))
	for _, r := range rows {
		m[r.Author] = r
	}
	return m, nil
}

func (b *Board) checkBudgetLocked() error {
	events, err := readJSONL[writeEvent](file(b.dir, fileWrites))
	if err != nil {
		return err
	}
	cutoff := b.now().UTC().Add(-24 * time.Hour)
	n := 0
	var oldest time.Time
	for _, e := range events {
		if e.Author == b.id.Author && e.At.After(cutoff) {
			n++
			if oldest.IsZero() || e.At.Before(oldest) {
				oldest = e.At
			}
		}
	}
	if n >= b.id.WritesPerDay {
		next := b.now().UTC().Add(24 * time.Hour)
		if !oldest.IsZero() {
			next = oldest.Add(24 * time.Hour)
		}
		return teach(
			fmt.Sprintf("%d writes used in 24h.", b.id.WritesPerDay),
			fmt.Sprintf("wait until %s (UTC)", next.Format(time.RFC3339)),
		)
	}
	return nil
}

func (b *Board) recordWriteLocked(kind string) error {
	return appendJSONL(file(b.dir, fileWrites), writeEvent{
		At:     b.now().UTC(),
		Author: b.id.Author,
		Kind:   kind,
	})
}

func (b *Board) NoticesList(limit int) ([]Notice, error) {
	limit = clampLimit(limit)
	var out []Notice
	err := b.do(func() error {
		rows, err := readJSONL[Notice](file(b.dir, fileNotices))
		if err != nil {
			return err
		}
		out = newestNotices(rows, limit)
		return nil
	})
	return out, err
}

// NoticesCreate pins a body (max 500).
func (b *Board) NoticesCreate(body string) (Notice, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Notice{}, teach("body is required.", `notices_create(body="Ada crushed 8k")`)
	}
	if len(body) > maxNoticeBody {
		return Notice{}, teach("body is too long (max 500).", `notices_create(body="short pin")`)
	}
	var saved Notice
	err := b.do(func() error {
		if err := b.checkBudgetLocked(); err != nil {
			return err
		}
		now := b.now().UTC()
		saved = Notice{
			ID:        newID(noticeIDPrefix, now),
			Author:    b.id.Author,
			Body:      body,
			CreatedAt: now,
		}
		rows, err := readJSONL[Notice](file(b.dir, fileNotices))
		if err != nil {
			return err
		}
		rows = append(rows, saved)
		if len(rows) > maxNoticesKept {
			rows = rows[len(rows)-maxNoticesKept:]
		}
		if err := writeJSONL(file(b.dir, fileNotices), rows); err != nil {
			return err
		}
		return b.recordWriteLocked("notices_create")
	})
	return saved, err
}

func newestNotices(rows []Notice, limit int) []Notice {
	n := len(rows)
	if n == 0 {
		return nil
	}
	out := make([]Notice, 0, limit)
	for i := n - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, rows[i])
	}
	return out
}
