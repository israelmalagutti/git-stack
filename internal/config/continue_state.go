package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ContinueState tracks remaining branches during a multi-branch restack
type ContinueState struct {
	RemainingBranches []string `json:"remainingBranches"`
	OriginalBranch    string   `json:"originalBranch"`
}

// LoadContinueState reads the continue state from the specified path
func LoadContinueState(path string) (*ContinueState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read continue state: %w", err)
	}

	var state ContinueState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse continue state: %w", err)
	}

	return &state, nil
}

// Save writes the continue state to the specified path
func (s *ContinueState) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal continue state: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write continue state: %w", err)
	}

	return nil
}

// ClearContinueState removes the continue state file
func ClearContinueState(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear continue state: %w", err)
	}
	return nil
}
