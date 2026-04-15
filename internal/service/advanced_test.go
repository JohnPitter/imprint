package service

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ActionService
// ---------------------------------------------------------------------------

func TestActionService_CreateAndList(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	action, err := svc.CreateAction("Build API", "REST endpoints", "pending", 5, nil, nil)
	if err != nil {
		t.Fatalf("CreateAction: %v", err)
	}
	if action.Title != "Build API" {
		t.Errorf("Title = %q, want %q", action.Title, "Build API")
	}

	list, err := svc.ListActions("", "", 10, 0)
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
}

func TestActionService_Frontier(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	a1, err := svc.CreateAction("Step 1", "", "pending", 5, nil, nil)
	if err != nil {
		t.Fatalf("CreateAction 1: %v", err)
	}
	a2, err := svc.CreateAction("Step 2", "", "pending", 5, nil, nil)
	if err != nil {
		t.Fatalf("CreateAction 2: %v", err)
	}

	// a1 blocks a2.
	if _, err := svc.CreateEdge(a1.ID, a2.ID, "blocks"); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	frontier, err := svc.GetFrontier()
	if err != nil {
		t.Fatalf("GetFrontier: %v", err)
	}
	if len(frontier) != 1 {
		t.Fatalf("frontier len = %d, want 1", len(frontier))
	}
	if frontier[0].ID != a1.ID {
		t.Errorf("frontier[0].ID = %q, want %q", frontier[0].ID, a1.ID)
	}
}

// ---------------------------------------------------------------------------
// AdvancedService
// ---------------------------------------------------------------------------

func TestAdvancedService_SignalRoundtrip(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)

	sig, err := svc.SendSignal("agent-a", "agent-b", "ping", "info")
	if err != nil {
		t.Fatalf("SendSignal: %v", err)
	}
	if sig.Content != "ping" {
		t.Errorf("Content = %q, want ping", sig.Content)
	}

	list, err := svc.ListSignals("agent-b", 10)
	if err != nil {
		t.Fatalf("ListSignals: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
}

func TestAdvancedService_LessonLifecycle(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)

	lesson, err := svc.CreateLesson("Never use eval in production", "security", "manual", nil, nil)
	if err != nil {
		t.Fatalf("CreateLesson: %v", err)
	}

	results, err := svc.SearchLessons("eval", 10)
	if err != nil {
		t.Fatalf("SearchLessons: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("search results = %d, want 1", len(results))
	}

	if err := svc.StrengthenLesson(lesson.ID); err != nil {
		t.Fatalf("StrengthenLesson: %v", err)
	}

	got, err := c.Lessons.GetByID(lesson.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Reinforcements != 1 {
		t.Errorf("Reinforcements = %d, want 1", got.Reinforcements)
	}
}
