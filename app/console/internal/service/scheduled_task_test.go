package service

import (
	"context"
	"testing"
	"time"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
)

// Console and the worker's cron must agree on what a schedule is, or an edit
// saves fine and then silently fails to register at the next reconcile.
func TestScheduledTaskUpdateRejectsAScheduleTheWorkerCannotRun(t *testing.T) {
	service, task := newScheduledTaskFixture(t)
	ctx := context.Background()

	for _, invalid := range []string{
		"not-a-cron",
		"* * * * *",    // five fields: valid elsewhere, but this cron wants seconds
		"99 * * * * *", // out of range
	} {
		if _, err := service.Update(ctx, UpdateScheduledTaskInput{TaskID: task.ID, Schedule: invalid}); err == nil {
			t.Fatalf("schedule %q was accepted", invalid)
		}
	}

	if got := reloadScheduledTask(t, service, task.ID).Schedule; got != task.Schedule {
		t.Fatalf("schedule = %q, want it unchanged after rejected edits", got)
	}
}

func TestScheduledTaskUpdateAppliesScheduleMutexAndTimeout(t *testing.T) {
	service, task := newScheduledTaskFixture(t)
	mutex := false
	timeout := 90

	updated, err := service.Update(context.Background(), UpdateScheduledTaskInput{
		TaskID:         task.ID,
		Schedule:       "0 30 2 * * *",
		Mutex:          &mutex,
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Schedule != "0 30 2 * * *" || updated.Mutex || updated.TimeoutSeconds != 90 {
		t.Fatalf("updated task = %+v", updated)
	}
}

func TestScheduledTaskUpdateRejectsANegativeTimeout(t *testing.T) {
	service, task := newScheduledTaskFixture(t)
	timeout := -1

	if _, err := service.Update(context.Background(), UpdateScheduledTaskInput{
		TaskID: task.ID, TimeoutSeconds: &timeout,
	}); err == nil {
		t.Fatal("negative timeout was accepted")
	}
}

func TestScheduledTaskRequestRunMarksThePendingRequest(t *testing.T) {
	service, task := newScheduledTaskFixture(t)
	requestedAt := time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return requestedAt }

	updated, err := service.RequestRun(context.Background(), RequestScheduledTaskRunInput{TaskID: task.ID})
	if err != nil {
		t.Fatalf("request run: %v", err)
	}
	if !updated.HasPendingRunRequest() {
		t.Fatal("request run must leave a pending flag for the worker to claim")
	}
}

// Running a disabled task would need it registered in the scheduler, which it
// is not — so refuse here rather than let the worker record a skip.
func TestScheduledTaskRequestRunRefusesADisabledTask(t *testing.T) {
	service, task := newScheduledTaskFixture(t)
	if _, err := service.SetStatus(context.Background(), SetScheduledTaskStatusInput{TaskID: task.ID, Enabled: false}); err != nil {
		t.Fatalf("disable: %v", err)
	}

	_, err := service.RequestRun(context.Background(), RequestScheduledTaskRunInput{TaskID: task.ID})
	if err == nil {
		t.Fatal("a disabled task must not accept a manual run")
	}
	if errx.Normalize(err).HTTPStatus != 400 {
		t.Fatalf("error = %v, want a 400", err)
	}
}

func TestScheduledTaskRequestRunRefusesASecondPendingRequest(t *testing.T) {
	service, task := newScheduledTaskFixture(t)
	ctx := context.Background()
	if _, err := service.RequestRun(ctx, RequestScheduledTaskRunInput{TaskID: task.ID}); err != nil {
		t.Fatalf("first request: %v", err)
	}

	if _, err := service.RequestRun(ctx, RequestScheduledTaskRunInput{TaskID: task.ID}); err == nil {
		t.Fatal("a second request must be refused while one is still pending")
	}
}

func TestScheduledTaskOperationsRejectAnUnknownID(t *testing.T) {
	service, _ := newScheduledTaskFixture(t)
	ctx := context.Background()

	if _, err := service.Update(ctx, UpdateScheduledTaskInput{TaskID: "missing", Schedule: "0 0 * * * *"}); err == nil {
		t.Fatal("update accepted an unknown task")
	}
	if _, err := service.SetStatus(ctx, SetScheduledTaskStatusInput{TaskID: "missing", Enabled: true}); err == nil {
		t.Fatal("status accepted an unknown task")
	}
	if _, err := service.RequestRun(ctx, RequestScheduledTaskRunInput{TaskID: "missing"}); err == nil {
		t.Fatal("run accepted an unknown task")
	}
}

func TestScheduledTaskListFiltersByEnabledAndKeyword(t *testing.T) {
	service, _ := newScheduledTaskFixture(t)
	ctx := context.Background()
	disabled := model.ConsoleScheduledTask{
		Name: "console.digest", DisplayName: "汇总", Schedule: "0 0 * * * *", Enabled: false,
	}
	if err := service.dbs.Default().Create(&disabled).Error; err != nil {
		t.Fatalf("seed second task: %v", err)
	}

	enabled := true
	onlyEnabled, err := service.List(ctx, ListScheduledTasksInput{Enabled: &enabled})
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(onlyEnabled.List) != 1 || onlyEnabled.List[0].Name != "console.purge" {
		t.Fatalf("enabled filter returned %+v", onlyEnabled.List)
	}

	byKeyword, err := service.List(ctx, ListScheduledTasksInput{Keyword: "汇总"})
	if err != nil {
		t.Fatalf("list by keyword: %v", err)
	}
	if len(byKeyword.List) != 1 || byKeyword.List[0].Name != "console.digest" {
		t.Fatalf("keyword filter returned %+v", byKeyword.List)
	}
}

// list_all used to reach the query as LIMIT 0 and return no rows.
func TestScheduledTaskListAllReturnsEveryTask(t *testing.T) {
	service, _ := newScheduledTaskFixture(t)

	result, err := service.List(context.Background(), ListScheduledTasksInput{
		Request: pagination.Request{ListAll: true},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result.List) != 1 || result.Meta.Total != 1 {
		t.Fatalf("list_all returned %d of %d tasks, want 1 of 1", len(result.List), result.Meta.Total)
	}
}

func newScheduledTaskFixture(t *testing.T) (*ScheduledTaskService, model.ConsoleScheduledTask) {
	t.Helper()
	db := testkit.OpenDB(t, &model.ConsoleScheduledTask{})

	task := model.ConsoleScheduledTask{
		Name:        "console.purge",
		DisplayName: "清理过期会话",
		Schedule:    "0 17 3 * * *",
		Enabled:     true,
		Mutex:       true,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return NewScheduledTaskService(database.NewConnectionsFromDBs(db, nil), pagination.Policy{}), task
}

func reloadScheduledTask(t *testing.T, service *ScheduledTaskService, taskID string) model.ConsoleScheduledTask {
	t.Helper()
	var task model.ConsoleScheduledTask
	if err := service.dbs.Default().Where("id = ?", taskID).First(&task).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	return task
}
