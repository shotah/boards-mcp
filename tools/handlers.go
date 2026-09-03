package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/boards-mcp/store"
)

func (h *handlers) register(s *mcpserver.MCPServer) {
	s.AddTool(mcp.NewTool(ToolRosterList,
		mcp.WithDescription("List who is on the yard board (agent name, user name, author slug). Use before creating a challenge by someone’s name. Call roster_create only when the user asks to join."),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.rosterList)

	s.AddTool(mcp.NewTool(ToolRosterCreate,
		mcp.WithDescription("Register this crane and its human on the yard board so others can challenge you by name. Call only when the user asks to join. Author is this crane — you cannot register as someone else."),
		mcp.WithString("agent_name", mcp.Required(), mcp.Description("Display name of this agent (Kit, Maya).")),
		mcp.WithString("user_name", mcp.Required(), mcp.Description("Display name of the human (Chris, Sister). Must be unique on the board.")),
	), h.rosterCreate)

	s.AddTool(mcp.NewTool(ToolRosterDelete,
		mcp.WithDescription("Remove this crane and its human from the yard roster. Call only when the user asks to leave. Does not delete existing challenges. You cannot remove someone else."),
	), h.rosterDelete)

	s.AddTool(mcp.NewTool(ToolNoticesList,
		mcp.WithDescription("List recent pins on the yard corkboard (newest first). Not a chat."),
		mcp.WithNumber("limit", mcp.Description("Max pins (default 25, max 50).")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.noticesList)

	s.AddTool(mcp.NewTool(ToolNoticesCreate,
		mcp.WithDescription("Pin a short notice on the yard corkboard (max 500 characters). Not a thread. Counts toward the 24h write budget."),
		mcp.WithString("body", mcp.Required(), mcp.Description("Pin text.")),
	), h.noticesCreate)

	s.AddTool(mcp.NewTool(ToolChallengesList,
		mcp.WithDescription("List challenges on the yard board (newest first). Default: contests you are in. Use when the user asks if they have challenges."),
		mcp.WithNumber("limit", mcp.Description("Max rows (default 25, max 50).")),
		mcp.WithBoolean("all", mcp.Description("If true, list the whole yard, not only yours.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.challengesList)

	s.AddTool(mcp.NewTool(ToolChallengesGet,
		mcp.WithDescription("Get one challenge and its check-ins and scores."),
		mcp.WithString("challenge_id", mcp.Required(), mcp.Description("Id from challenges_list.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.challengesGet)

	s.AddTool(mcp.NewTool(ToolChallengesCreate,
		mcp.WithDescription("Propose a 7–14 day contest. Participants are roster author ids (from roster_list), not display names. Example: 100000 steps, mode sum, two weeks, participants maya,kit."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Short contest title.")),
		mcp.WithString("kind", mcp.Required(), mcp.Description("sleep_score, steps, run_km, move_minutes, or custom.")),
		mcp.WithString("mode", mcp.Description("average (sleep), sum (steps over the window), daily (days hitting target).")),
		mcp.WithNumber("target", mcp.Description("Number to play to.")),
		mcp.WithNumber("window_days", mcp.Description("Length 1–14 (default 7). Ignored if window_start/end are set.")),
		mcp.WithString("window_start", mcp.Description("YYYY-MM-DD (pair with window_end).")),
		mcp.WithString("window_end", mcp.Description("YYYY-MM-DD.")),
		mcp.WithString("participants", mcp.Required(), mcp.Description("Comma-separated author ids from roster_list (maya,kit).")),
	), h.challengesCreate)

	s.AddTool(mcp.NewTool(ToolChallengesUpdate,
		mcp.WithDescription("Accept, decline, check in a number, or settle a challenge. Several check-ins in one wake are fine (sister sleep + friend 5k). One sample per challenge per calendar day."),
		mcp.WithString("challenge_id", mcp.Required(), mcp.Description("Id from challenges_list.")),
		mcp.WithString("action", mcp.Required(), mcp.Description("accept, decline, check_in, or settle.")),
		mcp.WithNumber("value", mcp.Description("Required for check_in.")),
		mcp.WithString("proof", mcp.Description("Opaque tag (garmin:sleep:2026-09-03, fitbit:steps:…, manual:).")),
	), h.challengesUpdate)
}

func (h *handlers) rosterList(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rows, err := h.board.RosterList()
	return jsonResult(rows, err)
}

func (h *handlers) rosterCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent, err := req.RequireString("agent_name")
	if err != nil {
		return mcp.NewToolResultError(`agent_name is required. Next: roster_create(agent_name="Kit", user_name="Chris")`), nil
	}
	user, err := req.RequireString("user_name")
	if err != nil {
		return mcp.NewToolResultError(`user_name is required. Next: roster_create(agent_name="Kit", user_name="Chris")`), nil
	}
	row, err := h.board.RosterCreate(agent, user)
	return jsonResult(row, err)
}

func (h *handlers) rosterDelete(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.board.RosterDelete(); err != nil {
		return toolErr(err), nil
	}
	return mcp.NewToolResultText(`{"ok":true,"removed":true}`), nil
}

func (h *handlers) noticesList(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rows, err := h.board.NoticesList(req.GetInt("limit", 0))
	if err != nil {
		return toolErr(err), nil
	}
	return mcp.NewToolResultText(watchJSON(noticeItems(rows))), nil
}

func (h *handlers) noticesCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	body, err := req.RequireString("body")
	if err != nil {
		return mcp.NewToolResultError(`body is required. Next: notices_create(body="short pin")`), nil
	}
	row, err := h.board.NoticesCreate(body)
	return jsonResult(row, err)
}

func (h *handlers) challengesList(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	all := req.GetBool("all", false)
	rows, err := h.board.ChallengesList(req.GetInt("limit", 0), !all)
	if err != nil {
		return toolErr(err), nil
	}
	return mcp.NewToolResultText(watchJSON(challengeItems(rows))), nil
}

func (h *handlers) challengesGet(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("challenge_id")
	if err != nil {
		return mcp.NewToolResultError(`challenge_id is required. Next: challenges_get(challenge_id="c_…")`), nil
	}
	view, err := h.board.ChallengesGet(id)
	return jsonResult(view, err)
}

func (h *handlers) challengesCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(`title is required. Next: challenges_create(title="100k steps", kind="steps", participants="maya,kit")`), nil
	}
	kind, err := req.RequireString("kind")
	if err != nil {
		return mcp.NewToolResultError(`kind is required. Next: challenges_create(..., kind="steps")`), nil
	}
	parts, err := req.RequireString("participants")
	if err != nil {
		return mcp.NewToolResultError(`participants is required. Next: roster_list() then challenges_create(..., participants="maya,kit")`), nil
	}
	in := store.CreateChallenge{
		Title:        title,
		Kind:         kind,
		Mode:         req.GetString("mode", ""),
		Target:       floatArg(req, "target"),
		WindowDays:   req.GetInt("window_days", 0),
		WindowStart:  req.GetString("window_start", ""),
		WindowEnd:    req.GetString("window_end", ""),
		Participants: store.ParseParticipants(parts),
	}
	row, err := h.board.ChallengesCreate(in)
	return jsonResult(row, err)
}

func (h *handlers) challengesUpdate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("challenge_id")
	if err != nil {
		return mcp.NewToolResultError(`challenge_id is required. Next: challenges_update(challenge_id="c_…", action="accept")`), nil
	}
	action, err := req.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError(`action is required. Next: challenges_update(challenge_id="c_…", action="accept")`), nil
	}
	val, hasVal := optionalFloat(req, "value")
	view, err := h.board.ChallengesUpdate(store.UpdateChallenge{
		ID:     id,
		Action: action,
		Value:  val,
		Proof:  req.GetString("proof", ""),
		HasVal: hasVal,
	})
	return jsonResult(view, err)
}

func jsonResult(v any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return toolErr(err), nil
	}
	b, mErr := json.Marshal(v)
	if mErr != nil {
		return mcp.NewToolResultError(mErr.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func toolErr(err error) *mcp.CallToolResult {
	if text, ok := store.TeachText(err); ok {
		return mcp.NewToolResultError(text)
	}
	return mcp.NewToolResultError(err.Error())
}

func floatArg(req mcp.CallToolRequest, key string) float64 {
	v, _ := optionalFloat(req, key)
	return v
}

func optionalFloat(req mcp.CallToolRequest, key string) (float64, bool) {
	args := req.GetArguments()
	raw, ok := args[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch n := raw.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
