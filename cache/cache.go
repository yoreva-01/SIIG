package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yv3000/siig/rules"
)

const staleDuration = 7 * 24 * time.Hour // 7 days

// cacheMetadata tracks fetch timestamps per stack ID
type cacheMetadata struct {
	Fetched map[string]time.Time `json:"fetched"`
}

// cacheDir returns the path to ~/.siig/cache/
func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".siig", "cache"), nil
}

// metadataPath returns the path to cache.json
func metadataPath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cache.json"), nil
}

// loadMetadata reads cache.json or returns empty metadata if missing/corrupt
func loadMetadata() *cacheMetadata {
	meta := &cacheMetadata{Fetched: make(map[string]time.Time)}

	path, err := metadataPath()
	if err != nil {
		return meta
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return meta
	}

	if err := json.Unmarshal(data, meta); err != nil {
		return &cacheMetadata{Fetched: make(map[string]time.Time)}
	}

	if meta.Fetched == nil {
		meta.Fetched = make(map[string]time.Time)
	}

	return meta
}

// saveMetadata writes cache.json
func saveMetadata(meta *cacheMetadata) error {
	path, err := metadataPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ensureCacheDir creates the cache directory if it doesn't exist
func ensureCacheDir() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

// Get returns a cached StackRule if it exists and is not stale (< 7 days old).
// Returns nil if not cached or stale.
func Get(stackID string) (*rules.StackRule, error) {
	if err := rules.ValidateStackID(stackID); err != nil {
		return nil, err
	}
	meta := loadMetadata()

	fetchTime, exists := meta.Fetched[stackID]
	if !exists {
		return nil, nil
	}

	// check staleness
	if time.Since(fetchTime) > staleDuration {
		return nil, nil // stale, re-fetch needed
	}

	// read the cached JSON file
	dir, err := cacheDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(dir, stackID+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil // file missing, treat as cache miss
	}

	var rule rules.StackRule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, nil // corrupt file, treat as cache miss
	}
	if err := rules.Validate(&rule, stackID); err != nil {
		return nil, nil // cached rule describes something else, re-fetch it
	}

	return &rule, nil
}

// Set saves a StackRule to cache and updates the metadata timestamp.
func Set(stackID string, rule *rules.StackRule) error {
	if err := rules.Validate(rule, stackID); err != nil {
		return err
	}
	if err := ensureCacheDir(); err != nil {
		return fmt.Errorf("cannot create cache dir: %w", err)
	}

	// write the stack JSON file
	dir, err := cacheDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(rule, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal rule: %w", err)
	}

	filePath := filepath.Join(dir, stackID+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("cannot write cache file: %w", err)
	}

	// update metadata
	meta := loadMetadata()
	meta.Fetched[stackID] = time.Now()
	if err := saveMetadata(meta); err != nil {
		return fmt.Errorf("cannot save cache metadata: %w", err)
	}

	return nil
}

// IsCached returns true if a stack is cached AND not stale.
func IsCached(stackID string) bool {
	meta := loadMetadata()
	fetchTime, exists := meta.Fetched[stackID]
	if !exists {
		return false
	}
	return time.Since(fetchTime) <= staleDuration
}

// Clear deletes all cached stack JSON files and resets metadata.
func Clear() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}

	// remove the entire cache directory
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("cannot clear cache: %w", err)
	}

	return nil
}
