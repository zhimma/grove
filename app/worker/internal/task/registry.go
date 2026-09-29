// Package task is the Worker's registry of scheduled work. It is the only
// place a scheduled task can come from: console_scheduled_tasks stores when a
// task runs, never what it does, so a row whose Name is absent here has no
// handler and nothing happens.
package task

import (
	"fmt"
	"time"

	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/scheduler"
)

// Definition is one task as the code declares it. Schedule, Mutex and Timeout
// are defaults used to seed a missing row; once the row exists the database is
// authoritative so an operator can retune without a redeploy.
type Definition struct {
	Name        string
	DisplayName string
	Schedule    string
	Mutex       bool
	Timeout     time.Duration
	Job         scheduler.Job
}

// Definitions returns every task this binary can run, keyed by name.
func Definitions(dbs *database.Connections) (map[string]Definition, error) {
	if dbs == nil {
		return nil, fmt.Errorf("task registry requires database connections")
	}

	defined := []Definition{
		{
			Name:        "console.purge-expired-sessions",
			DisplayName: "清理过期后台会话",
			Schedule:    "0 17 3 * * *",
			Mutex:       true,
			Timeout:     5 * time.Minute,
			Job:         scheduler.JobFunc(newPurgeExpiredSessions(dbs).Run),
		},
	}

	registry := make(map[string]Definition, len(defined))
	for _, definition := range defined {
		if definition.Name == "" {
			return nil, fmt.Errorf("task definition requires a name")
		}
		if definition.Job == nil {
			return nil, fmt.Errorf("task %q requires a job", definition.Name)
		}
		if err := scheduler.ValidateSchedule(definition.Schedule); err != nil {
			return nil, fmt.Errorf("task %q: %w", definition.Name, err)
		}
		if _, exists := registry[definition.Name]; exists {
			return nil, fmt.Errorf("duplicate task %q", definition.Name)
		}
		registry[definition.Name] = definition
	}
	return registry, nil
}
