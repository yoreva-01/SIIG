package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yv3000/siig/internal/version"
	"github.com/yv3000/siig/rules"
)

// maxRuleBytes bounds how much of a response body we are willing to parse.
const maxRuleBytes = 1 << 20 // 1 MiB

// Package-level so tests can point the fetcher at a local server; production
// uses the defaults.
var (
	baseURL   = "https://raw.githubusercontent.com/yv3000/siig-rules/main/db/"
	timeout   = 5 * time.Second
	client    = &http.Client{}
	userAgent = "siig/" + version.Version
)

// FetchStack fetches a single stack JSON by ID from siig-rules.
// Returns nil, nil if the stack doesn't exist (404).
// Returns nil, err on network, size, parse or validation failures.
func FetchStack(stackID string) (*rules.StackRule, error) {
	if err := rules.ValidateStackID(stackID); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+stackID+".json", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed for %s: %w", stackID, err)
	}
	defer resp.Body.Close()

	// 404 means the stack doesn't exist in siig-rules — not an error
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, stackID)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRuleBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read response for %s: %w", stackID, err)
	}
	if len(body) > maxRuleBytes {
		return nil, fmt.Errorf("rule for %s exceeds %d bytes", stackID, maxRuleBytes)
	}

	var rule rules.StackRule
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rule); err != nil {
		return nil, fmt.Errorf("cannot parse JSON for %s: %w", stackID, err)
	}
	if err := rules.Validate(&rule, stackID); err != nil {
		return nil, fmt.Errorf("rejected rule for %s: %w", stackID, err)
	}

	return &rule, nil
}
