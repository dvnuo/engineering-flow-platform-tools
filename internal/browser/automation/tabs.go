package automation

import (
	"context"
	"strings"
)

type TabListResult struct {
	Session string   `json:"session"`
	Tabs    []Target `json:"tabs"`
}

type TabResult struct {
	Session string `json:"session"`
	Tab     Target `json:"tab"`
}

func (m *Manager) TabList(ctx context.Context, sessionName string) (TabListResult, error) {
	session, err := m.RunningSession(ctx, sessionName)
	if err != nil {
		return TabListResult{}, err
	}
	targets, err := m.clientFor(session).ListTargets(ctx)
	if err != nil {
		return TabListResult{}, err
	}
	return TabListResult{Session: session.Name, Tabs: publicTargets(PageTargets(targets), session.ActiveTargetID)}, nil
}

func (m *Manager) CurrentTab(ctx context.Context, sessionName string) (TabResult, error) {
	session, target, err := m.currentTarget(ctx, sessionName)
	if err != nil {
		return TabResult{}, err
	}
	return TabResult{Session: session.Name, Tab: publicTarget(target, true)}, nil
}

func (m *Manager) ActivateTab(ctx context.Context, sessionName, targetID string) (TabResult, error) {
	if strings.TrimSpace(targetID) == "" {
		return TabResult{}, invalidArgs("--target-id is required", "Run browser tab list --json and pass the page target id.")
	}
	session, err := m.RunningSession(ctx, sessionName)
	if err != nil {
		return TabResult{}, err
	}
	target, err := m.findTarget(ctx, session, targetID)
	if err != nil {
		return TabResult{}, err
	}
	if err := m.clientFor(session).Activate(ctx, targetID); err != nil {
		return TabResult{}, err
	}
	session.ActiveTargetID = targetID
	if err := m.Store.Save(session); err != nil {
		return TabResult{}, err
	}
	return TabResult{Session: session.Name, Tab: publicTarget(target, true)}, nil
}

func (m *Manager) OpenTab(ctx context.Context, sessionName, rawURL string) (TabResult, error) {
	if strings.TrimSpace(rawURL) == "" {
		return TabResult{}, invalidArgs("--url is required", "Run browser schema tab.open --json.")
	}
	if err := validateHTTPURL(rawURL, "--url"); err != nil {
		return TabResult{}, err
	}
	session, err := m.RunningSession(ctx, sessionName)
	if err != nil {
		return TabResult{}, err
	}
	target, err := m.clientFor(session).Open(ctx, rawURL)
	if err != nil {
		return TabResult{}, err
	}
	session.ActiveTargetID = target.ID
	if err := m.Store.Save(session); err != nil {
		return TabResult{}, err
	}
	return TabResult{Session: session.Name, Tab: publicTarget(target, true)}, nil
}

func (m *Manager) ResolveTarget(ctx context.Context, sessionName, targetID string) (Session, Target, error) {
	if strings.TrimSpace(targetID) == "" {
		return m.currentTarget(ctx, sessionName)
	}
	session, err := m.RunningSession(ctx, sessionName)
	if err != nil {
		return Session{}, Target{}, err
	}
	target, err := m.findTarget(ctx, session, targetID)
	if err != nil {
		return Session{}, Target{}, err
	}
	return session, target, nil
}

func (m *Manager) currentTarget(ctx context.Context, sessionName string) (Session, Target, error) {
	session, err := m.RunningSession(ctx, sessionName)
	if err != nil {
		return Session{}, Target{}, err
	}
	targets, err := m.clientFor(session).ListTargets(ctx)
	if err != nil {
		return Session{}, Target{}, err
	}
	pages := PageTargets(targets)
	if session.ActiveTargetID != "" {
		for _, target := range pages {
			if target.ID == session.ActiveTargetID {
				return session, target, nil
			}
		}
	}
	if len(pages) == 0 {
		return Session{}, Target{}, NewError("target_not_found", "No page targets were found in the browser session.", "Open a tab in the browser session or run browser tab open --url <url> --json.", 404)
	}
	session.ActiveTargetID = pages[0].ID
	if err := m.Store.Save(session); err != nil {
		return Session{}, Target{}, err
	}
	return session, pages[0], nil
}

func (m *Manager) findTarget(ctx context.Context, session Session, targetID string) (Target, error) {
	targets, err := m.clientFor(session).ListTargets(ctx)
	if err != nil {
		return Target{}, err
	}
	for _, target := range PageTargets(targets) {
		if target.ID == targetID {
			return target, nil
		}
	}
	return Target{}, NewError("target_not_found", "Target id was not found in the browser session.", "Run browser tab list --json and choose a page target id.", 404)
}

func (m *Manager) clientFor(session Session) *DevToolsClient {
	if m.Client != nil {
		return m.Client
	}
	return NewDevToolsClient(session.DebugAddr, session.DebugPort)
}

func publicTargets(targets []Target, activeTargetID string) []Target {
	out := make([]Target, 0, len(targets))
	for _, target := range targets {
		out = append(out, publicTarget(target, target.ID == activeTargetID))
	}
	return out
}

func publicTarget(target Target, active bool) Target {
	target = RedactedTarget(target)
	target.Active = active
	return target
}
