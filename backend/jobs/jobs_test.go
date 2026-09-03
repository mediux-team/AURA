package jobs

import (
	"aura/config"
	"testing"
)

func TestGetListOfJobs(t *testing.T) {
	// Initialize default config
	config.Current = config.DefaultConfig()

	jobs := GetListOfJobs()
	if len(jobs) != 6 {
		t.Fatalf("expected 6 jobs in list, got %d", len(jobs))
	}

	expectedNames := map[string]bool{
		"AutoDownload Job":                        false,
		"Refresh Media Items and Collections Job": false,
		"Refresh Mediux Users Job":                false,
		"Check Mediux Site Link Availability Job": false,
		"Check for Media Item Changes Job":        false,
		"Handle Temp Ignored Items Job":           false,
	}

	for _, j := range jobs {
		if _, ok := expectedNames[j.JobName]; ok {
			expectedNames[j.JobName] = true
		} else {
			t.Errorf("unexpected job name: %s", j.JobName)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected job '%s' was not found in list", name)
		}
	}
}

func TestJobsSchedulingAndDisabling(t *testing.T) {
	cfg := config.DefaultConfig()

	// Disable RefreshMediaItemsAndCollections
	disabled := false
	cfg.Jobs.RefreshMediaItemsAndCollections.Enabled = &disabled

	// Set custom cron for CheckForMediaItemChanges
	cfg.Jobs.CheckForMediaItemChanges.Cron = "0 12 * * *"
	config.Current = cfg

	err := StartRefreshMediaItemsAndCollectionsJob()
	if err != nil {
		t.Errorf("expected no error when starting disabled RefreshMediaItemsAndCollectionsJob, got %v", err)
	}
	if refreshMediaItemsAndCollectionsJobID != 0 {
		t.Errorf("expected refreshMediaItemsAndCollectionsJobID to remain 0 when disabled")
	}

	err = StartCheckForMediaItemChangesJob()
	if err != nil {
		t.Errorf("expected no error when starting CheckForMediaItemChangesJob, got %v", err)
	}
	if checkForMediaItemChangesJobID == 0 {
		t.Errorf("expected checkForMediaItemChangesJobID to be set when enabled")
	}

	// Verify in GetListOfJobs
	jobs := GetListOfJobs()
	for _, j := range jobs {
		if j.JobName == "Refresh Media Items and Collections Job" {
			if j.Spec != "Disabled" || j.NextRun != "Disabled" {
				t.Errorf("expected disabled job to have 'Disabled' spec and next_run, got spec='%s', next_run='%s'", j.Spec, j.NextRun)
			}
		}
		if j.JobName == "Check for Media Item Changes Job" {
			if j.Spec != "0 12 * * *" {
				t.Errorf("expected custom spec '0 12 * * *', got '%s'", j.Spec)
			}
		}
	}
}

func TestTriggerUnknownJob(t *testing.T) {
	err := TriggerJob("NonExistent Job", "99")
	if err == nil {
		t.Errorf("expected error when triggering unknown job, got nil")
	}
}
