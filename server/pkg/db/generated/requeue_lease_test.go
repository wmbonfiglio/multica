package db

import (
	"strings"
	"testing"
)

// TestRequeueExpiredClaimLeases_FiltersOfflineRuntimes is a regression test
// ensuring that the RequeueExpiredClaimLeases SQL only requeues tasks whose
// runtime is still online. Tasks on offline/dead runtimes must stay
// dispatched so FailTasksForOfflineRuntimes can properly fail+retry them.
func TestRequeueExpiredClaimLeases_FiltersOfflineRuntimes(t *testing.T) {
	sql := requeueExpiredClaimLeases

	if !strings.Contains(sql, "INNER JOIN agent_runtime") {
		t.Fatal("RequeueExpiredClaimLeases must JOIN agent_runtime to check liveness")
	}
	if !strings.Contains(sql, "ar.status = 'online'") {
		t.Fatal("RequeueExpiredClaimLeases must filter by ar.status = 'online'")
	}
}

// TestFailAgentTask_TokenlessCannotBypassTokenedRow is a regression test
// ensuring that the FailAgentTask SQL uses strict token matching:
// tokenless requests can only fail rows where claim_token IS NULL.
func TestFailAgentTask_TokenlessCannotBypassTokenedRow(t *testing.T) {
	sql := failAgentTask

	// The old vulnerable pattern: ($6::uuid IS NULL OR claim_token = $6)
	// allows tokenless requests to match ANY row.
	if strings.Contains(sql, "IS NULL OR claim_token =") {
		t.Fatal("FailAgentTask must not use 'IS NULL OR claim_token =' pattern (tokenless bypass)")
	}

	// The correct pattern requires both conditions:
	// (param IS NULL AND claim_token IS NULL) OR claim_token = param
	if !strings.Contains(sql, "IS NULL AND claim_token IS NULL") {
		t.Fatal("FailAgentTask must require claim_token IS NULL for tokenless requests")
	}
}
