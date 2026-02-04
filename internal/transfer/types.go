// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-04
// Last Modified: 2026-02-04

// Package transfer provides the transfer rules engine for cross-repository issue routing.
package transfer

import "github.com/similigh/simili-bot/internal/core/config"

// IssueInput represents the issue data used for rule matching.
type IssueInput struct {
	Title  string
	Body   string
	Labels []string
	Author string
}

// MatchResult contains the result of rule matching.
type MatchResult struct {
	Matched bool
	Rule    *config.TransferRule
	Target  string
	Reason  string
}
