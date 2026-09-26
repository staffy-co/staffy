package domain

import "testing"

func TestBriefValidationAndPlanBlockers(t *testing.T) {
	brief := Brief{Purpose: "Помогать гостям", Name: "Аня", Languages: []string{"ru"}, TimeZone: "Europe/Moscow", WorkingHours: WorkingHours{Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00"}, DesiredChannels: []string{"phone"}, PrimaryTasks: []string{"Запись"}}
	if !brief.Valid() {
		t.Fatal("valid Brief rejected")
	}
	brief.DesiredChannels = []string{"max", "jivo", "avito", "mobile_app_chat"}
	if !brief.Valid() {
		t.Fatal("project channels rejected")
	}
	brief.DesiredChannels = []string{"mobile_app_chat", "unknown"}
	if brief.Valid() {
		t.Fatal("unknown channel accepted")
	}
	brief.DesiredChannels = []string{"phone"}
	brief.TimeZone = "not-a-time-zone"
	if brief.Valid() {
		t.Fatal("invalid time zone accepted")
	}
	brief.TimeZone = "Local"
	if brief.Valid() {
		t.Fatal("server-local time zone accepted")
	}
	plan := LaunchPlan(Draft{CurrentStep: "launch_plan"})
	if len(plan) != 8 || plan[1].Status != "complete" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	for _, step := range plan[2:] {
		if step.Status != "blocked" || step.Reason == "" {
			t.Fatalf("future dependency marked ready: %+v", step)
		}
	}
}
