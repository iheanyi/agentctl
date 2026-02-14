package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateServerTestedCancelledSetsUnknown(t *testing.T) {
	m := newTestModel()
	m.allServers = []Server{
		{
			Name:   "demo",
			Status: ServerStatusInstalled,
			Health: HealthStatusChecking,
		},
	}
	m.filteredItems = m.allServers
	m.testCancels["demo"] = func() {}

	updatedModel, _ := m.Update(serverTestedMsg{
		name:    "demo",
		healthy: false,
		err:     context.Canceled,
	})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("Update() returned %T, want Model", updatedModel)
	}

	if _, exists := updated.testCancels["demo"]; exists {
		t.Fatal("expected cancel func for demo to be removed")
	}
	if updated.allServers[0].Health != HealthStatusUnknown {
		t.Fatalf("server health = %q, want %q", updated.allServers[0].Health, HealthStatusUnknown)
	}
}

func TestCancelActiveOperationsClearsState(t *testing.T) {
	m := newTestModel()
	m.toolExecuting = true
	m.importWizardImporting = true

	toolCancelled := false
	importCancelled := false
	testCancelled := false

	m.toolCancel = func() { toolCancelled = true }
	m.importWizardCancel = func() { importCancelled = true }
	m.testCancels["demo"] = func() { testCancelled = true }

	cancelled := m.cancelActiveOperations()
	if len(cancelled) != 3 {
		t.Fatalf("cancelled operations = %d, want 3", len(cancelled))
	}
	if !toolCancelled || !importCancelled || !testCancelled {
		t.Fatalf("expected all cancel functions to run: tool=%v import=%v test=%v", toolCancelled, importCancelled, testCancelled)
	}
	if m.toolExecuting {
		t.Fatal("toolExecuting should be false after cancellation")
	}
	if m.importWizardProgress != "Cancelling import..." {
		t.Fatalf("importWizardProgress = %q, want %q", m.importWizardProgress, "Cancelling import...")
	}
	if len(m.testCancels) != 0 {
		t.Fatalf("testCancels should be empty after cancellation, got %d", len(m.testCancels))
	}
}

func TestHandleImportWizardInputCancelsImport(t *testing.T) {
	m := newTestModel()
	m.importWizardImporting = true

	cancelled := false
	m.importWizardCancel = func() { cancelled = true }

	updatedModel, _ := m.handleImportWizardInput(tea.KeyMsg{Type: tea.KeyEsc})
	updated, ok := updatedModel.(*Model)
	if !ok {
		t.Fatalf("handleImportWizardInput() returned %T, want *Model", updatedModel)
	}

	if !cancelled {
		t.Fatal("expected import cancel function to run")
	}
	if updated.importWizardProgress != "Cancelling import..." {
		t.Fatalf("importWizardProgress = %q, want %q", updated.importWizardProgress, "Cancelling import...")
	}
}
