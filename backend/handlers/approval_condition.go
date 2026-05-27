package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// approvalConditionError is a validation (HTTP 400) failure for an approval
// condition, as opposed to an internal/DB error.
type approvalConditionError struct{ msg string }

func (e *approvalConditionError) Error() string { return e.msg }

var (
	approvalConnectorRe = regexp.MustCompile(`(?i)\s+(?:AND|OR)\s+`)
	approvalFullReqRe   = regexp.MustCompile(`(?i)^\d+\s*x\s*\S+$`)
)

// approvalConditionWellFormed reports whether the whole condition is a sequence
// of complete "N x role" requirements joined by AND/OR, with no dangling token
// (e.g. a trailing "1 x " with no role, or a stray AND). parseApprovalCondition
// alone extracts only the complete atoms and ignores such fragments, so this
// guards against silently accepting a half-typed condition.
func approvalConditionWellFormed(condition string) bool {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return false
	}
	for _, part := range approvalConnectorRe.Split(condition, -1) {
		if !approvalFullReqRe.MatchString(strings.TrimSpace(part)) {
			return false
		}
	}
	return true
}

// existingRoleNames returns the names of all roles. Roles are a single global
// namespace, so a condition is checked against every role.
func existingRoleNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM roles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// validateApprovalCondition checks that a condition is well-formed and only
// references roles that exist (or are listed in builtins — the auto-created admin
// role that exists right after creation). Role names are globally unique, so
// matching is exact. A non-nil *approvalConditionError means the condition is
// invalid (HTTP 400); any other error is internal.
func validateApprovalCondition(ctx context.Context, db *sql.DB, builtins []string, condition string) error {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return &approvalConditionError{"approval condition is required"}
	}
	reqs := parseApprovalCondition(condition)
	if len(reqs) == 0 || !approvalConditionWellFormed(condition) {
		return &approvalConditionError{`approval condition is not parseable; use e.g. "1 x test_project_admin AND 1 x release_manager"`}
	}
	for _, req := range reqs {
		if req.Count < 1 {
			return &approvalConditionError{"approval condition counts must be at least 1"}
		}
	}

	existing, err := existingRoleNames(ctx, db)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(existing)+len(builtins))
	for _, n := range existing {
		known[n] = true
	}
	for _, b := range builtins {
		known[b] = true
	}

	for _, req := range reqs {
		if !known[req.RoleName] {
			return &approvalConditionError{fmt.Sprintf("approval condition references role %q, which does not exist — create it first", req.RoleName)}
		}
	}
	return nil
}
