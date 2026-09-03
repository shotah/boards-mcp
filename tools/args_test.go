package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMissingToolArgs(t *testing.T) {
	t.Parallel()
	s := serverWith(t, boardAt(t, t.TempDir(), "kit", time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)))
	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{ToolRosterCreate, map[string]any{"user_name": "Chris"}, "agent_name is required"},
		{ToolRosterCreate, map[string]any{"agent_name": "Kit"}, "user_name is required"},
		{ToolNoticesCreate, map[string]any{}, "body is required"},
		{ToolChallengesCreate, map[string]any{"kind": "steps", "participants": "kit"}, "title is required"},
		{ToolChallengesCreate, map[string]any{"title": "x", "participants": "kit"}, "kind is required"},
		{ToolChallengesCreate, map[string]any{"title": "x", "kind": "steps"}, "participants is required"},
		{ToolChallengesGet, map[string]any{}, "challenge_id is required"},
		{ToolChallengesUpdate, map[string]any{"action": "accept"}, "challenge_id is required"},
		{ToolChallengesUpdate, map[string]any{"challenge_id": "c_x"}, "action is required"},
	}
	for _, tc := range cases {
		text, isErr := callTool(t, s, tc.tool, tc.args)
		if !isErr || !strings.Contains(text, tc.want) {
			t.Errorf("%s %+v = %q err=%v", tc.tool, tc.args, text, isErr)
		}
	}
}

func TestListLimitAndAll(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	maya := boardAt(t, dir, "maya", now)
	kit := boardAt(t, dir, "kit", now)
	if _, err := maya.RosterCreate("Maya", "Sister"); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.RosterCreate("Kit", "Chris"); err != nil {
		t.Fatal(err)
	}
	sMaya := serverWith(t, maya)
	sKit := serverWith(t, kit)
	text, isErr := callTool(t, sMaya, ToolChallengesCreate, map[string]any{
		"title": "solo", "kind": "custom", "participants": "maya",
	})
	if isErr {
		t.Fatal(text)
	}
	text, isErr = callTool(t, sKit, ToolChallengesList, map[string]any{"all": true, "limit": 10.0})
	if isErr || !strings.Contains(text, "solo") {
		t.Fatalf("all = %s err=%v", text, isErr)
	}
	text, isErr = callTool(t, sKit, ToolChallengesList, map[string]any{"all": false})
	if isErr {
		t.Fatal(text)
	}
	var listed WatchResult
	if err := json.Unmarshal([]byte(text), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("kit mine = %+v", listed)
	}
	text, isErr = callTool(t, sMaya, ToolNoticesList, map[string]any{"limit": 5.0})
	if isErr {
		t.Fatal(text)
	}
}

func TestWatchJSONEmptyAndHelpers(t *testing.T) {
	t.Parallel()
	if got := watchJSON(nil); got != `{"items":[]}` {
		t.Fatalf("nil = %s", got)
	}
	res := toolErr(errors.New("plain boom"))
	if !res.IsError || len(res.Content) == 0 {
		t.Fatalf("%+v", res)
	}
	out, err := jsonResult(make(chan int), nil)
	if err != nil || out == nil || !out.IsError {
		t.Fatalf("chan marshal = %+v %v", out, err)
	}
}

func TestOptionalFloatKinds(t *testing.T) {
	t.Parallel()
	args := map[string]any{}
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
	if _, ok := optionalFloat(req, "value"); ok {
		t.Fatal("missing")
	}
	args["value"] = nil
	if _, ok := optionalFloat(req, "value"); ok {
		t.Fatal("nil")
	}
	args["value"] = 3
	if v, ok := optionalFloat(req, "value"); !ok || v != 3 {
		t.Fatalf("int %v %v", v, ok)
	}
	args["value"] = int64(4)
	if v, ok := optionalFloat(req, "value"); !ok || v != 4 {
		t.Fatalf("int64 %v %v", v, ok)
	}
	args["value"] = json.Number("5.5")
	if v, ok := optionalFloat(req, "value"); !ok || v != 5.5 {
		t.Fatalf("number %v %v", v, ok)
	}
	args["value"] = json.Number("nope")
	if _, ok := optionalFloat(req, "value"); ok {
		t.Fatal("bad number")
	}
	args["value"] = "x"
	if _, ok := optionalFloat(req, "value"); ok {
		t.Fatal("string")
	}
}
