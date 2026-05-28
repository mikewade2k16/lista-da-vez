package roadmap

import "testing"

func TestCountDashboardTasksMapsTaskWorkflowStatuses(t *testing.T) {
	raw := "Raw"
	running := "Running"
	finalizada := "Finalizada"

	counts := countDashboardTasks([]DashboardTask{
		{ID: "task-raw", Status: &raw},
		{ID: "task-running", Status: &running},
		{ID: "task-finalizada", Status: &finalizada},
	}, StatusInProgress)

	if counts.Total != 3 {
		t.Fatalf("Total = %d; want 3", counts.Total)
	}
	if counts.Planning != 1 {
		t.Fatalf("Planning = %d; want 1", counts.Planning)
	}
	if counts.InProgress != 1 {
		t.Fatalf("InProgress = %d; want 1", counts.InProgress)
	}
	if counts.Done != 1 {
		t.Fatalf("Done = %d; want 1", counts.Done)
	}
}

func TestCountDashboardTasksDoesNotInventModuleProgress(t *testing.T) {
	counts := countDashboardTasks(nil, StatusInProgress)

	if counts.Total != 0 || counts.Idea != 0 || counts.Planning != 0 || counts.InProgress != 0 || counts.Done != 0 {
		t.Fatalf("counts = %+v; want all zero without linked tasks", counts)
	}
}
