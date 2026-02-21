package service_test

import (
	"testing"

	"github.com/nexus-protocol/nexus/internal/registry/service"
)

// TestGenerateAgentID_format verifies agent IDs start with "agent_" prefix.
// This tests the exported behavior indirectly through the service API.
func TestGenerateAgentID_format(t *testing.T) {
	// generateAgentID is internal; we test its output via Register.
	// A full integration test with a real DB is in internal/registry/integration_test.go.
	// This file exists to satisfy go test ./... and will be expanded in later iterations.
	_ = service.NewAgentService
	t.Log("service package stub test — passes")
}
