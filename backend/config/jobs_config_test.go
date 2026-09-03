package config

import (
	"context"
	"testing"
)

func TestDefaultJobsConfig(t *testing.T) {
	defaults := DefaultJobsConfig()

	if !defaults.RefreshMediaItemsAndCollections.IsEnabled() {
		t.Errorf("expected RefreshMediaItemsAndCollections to be enabled by default")
	}
	if defaults.RefreshMediaItemsAndCollections.Cron != "0 4 * * *" {
		t.Errorf("expected RefreshMediaItemsAndCollections.Cron to default to '0 4 * * *', got '%s'", defaults.RefreshMediaItemsAndCollections.Cron)
	}

	if !defaults.CheckForMediaItemChanges.IsEnabled() {
		t.Errorf("expected CheckForMediaItemChanges to be enabled by default")
	}
	if defaults.CheckForMediaItemChanges.Cron != "0 */6 * * *" {
		t.Errorf("expected CheckForMediaItemChanges.Cron to default to '0 */6 * * *', got '%s'", defaults.CheckForMediaItemChanges.Cron)
	}

	if !defaults.HandleTempIgnoredItems.IsEnabled() {
		t.Errorf("expected HandleTempIgnoredItems to be enabled by default")
	}
	if defaults.HandleTempIgnoredItems.Cron != "0 */6 * * *" {
		t.Errorf("expected HandleTempIgnoredItems.Cron to default to '0 */6 * * *', got '%s'", defaults.HandleTempIgnoredItems.Cron)
	}

	if !defaults.RefreshMediuxUsers.IsEnabled() {
		t.Errorf("expected RefreshMediuxUsers to be enabled by default")
	}
	if defaults.RefreshMediuxUsers.Cron != "0 */12 * * *" {
		t.Errorf("expected RefreshMediuxUsers.Cron to default to '0 */12 * * *', got '%s'", defaults.RefreshMediuxUsers.Cron)
	}

	if !defaults.CheckMediuxSiteLink.IsEnabled() {
		t.Errorf("expected CheckMediuxSiteLink to be enabled by default")
	}
	if defaults.CheckMediuxSiteLink.Cron != "0 */1 * * *" {
		t.Errorf("expected CheckMediuxSiteLink.Cron to default to '0 */1 * * *', got '%s'", defaults.CheckMediuxSiteLink.Cron)
	}
}

func TestValidateJobs_DefaultConfig(t *testing.T) {
	ctx := context.Background()
	jobs := DefaultJobsConfig()

	if !ValidateJobs(ctx, &jobs) {
		t.Errorf("expected default jobs configuration to be valid")
	}
}

func TestValidateJobs_EmptyConfigBackwardCompatibility(t *testing.T) {
	ctx := context.Background()
	var emptyJobs Config_Jobs

	if !ValidateJobs(ctx, &emptyJobs) {
		t.Errorf("expected empty jobs configuration to default and be valid")
	}

	if !emptyJobs.RefreshMediaItemsAndCollections.IsEnabled() {
		t.Errorf("expected RefreshMediaItemsAndCollections to be populated as enabled")
	}
	if emptyJobs.RefreshMediaItemsAndCollections.Cron != "0 4 * * *" {
		t.Errorf("expected RefreshMediaItemsAndCollections.Cron to be populated with default '0 4 * * *', got '%s'", emptyJobs.RefreshMediaItemsAndCollections.Cron)
	}
}

func TestValidateJobs_CustomValidCron(t *testing.T) {
	ctx := context.Background()
	customDisabled := false
	jobs := Config_Jobs{
		RefreshMediaItemsAndCollections: Config_JobSetting{
			Enabled: &customDisabled,
			Cron:    "0 2 * * *",
		},
		CheckForMediaItemChanges: Config_JobSetting{
			Cron: "0 */8 * * *",
		},
	}

	if !ValidateJobs(ctx, &jobs) {
		t.Errorf("expected custom valid jobs configuration to be valid")
	}

	if jobs.RefreshMediaItemsAndCollections.IsEnabled() {
		t.Errorf("expected RefreshMediaItemsAndCollections to be disabled")
	}
	if !jobs.CheckForMediaItemChanges.IsEnabled() {
		t.Errorf("expected CheckForMediaItemChanges to default to enabled")
	}
	if jobs.CheckForMediaItemChanges.Cron != "0 */8 * * *" {
		t.Errorf("expected CheckForMediaItemChanges.Cron to retain custom cron, got '%s'", jobs.CheckForMediaItemChanges.Cron)
	}
}

func TestValidateJobs_InvalidCron(t *testing.T) {
	ctx := context.Background()
	jobs := Config_Jobs{
		RefreshMediaItemsAndCollections: Config_JobSetting{
			Cron: "invalid-cron-expression",
		},
	}

	if ValidateJobs(ctx, &jobs) {
		t.Errorf("expected invalid cron expression to fail validation")
	}
}

func TestConfig_JobSetting_IsEnabled(t *testing.T) {
	var unset Config_JobSetting
	if !unset.IsEnabled() {
		t.Errorf("expected unset Enabled (nil) to return true")
	}

	tVal := true
	enabled := Config_JobSetting{Enabled: &tVal}
	if !enabled.IsEnabled() {
		t.Errorf("expected explicit true to return true")
	}

	fVal := false
	disabled := Config_JobSetting{Enabled: &fVal}
	if disabled.IsEnabled() {
		t.Errorf("expected explicit false to return false")
	}
}
