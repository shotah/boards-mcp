package tools

import (
	"encoding/json"

	"github.com/shotah/boards-mcp/store"
)

// WatchItem is one kernel watch row.
type WatchItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
}

// WatchResult is {"items":[...]} the gantry watch poller parses.
type WatchResult struct {
	Items []WatchItem `json:"items"`
}

func watchJSON(items []WatchItem) string {
	if items == nil {
		items = []WatchItem{}
	}
	b, err := json.Marshal(WatchResult{Items: items})
	if err != nil {
		return `{"items":[]}`
	}
	return string(b)
}

func noticeItems(rows []store.Notice) []WatchItem {
	out := make([]WatchItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, WatchItem{
			ID:      r.ID,
			Title:   r.Body,
			URL:     "boards:notice/" + r.ID,
			Summary: r.Author + ": " + r.Body,
		})
	}
	return out
}

func challengeItems(rows []store.Challenge) []WatchItem {
	out := make([]WatchItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, WatchItem{
			ID:      r.ID,
			Title:   r.Title,
			URL:     "boards:challenge/" + r.ID,
			Summary: r.Kind + " " + r.Status + " " + r.WindowStart + ".." + r.WindowEnd,
		})
	}
	return out
}
