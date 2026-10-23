package jobs

import (
	"aura/config"
	"aura/logging"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// JobKey is a stable identifier for a background job
type JobKey string

const (
	JobKeyAutoDownload                    JobKey = "auto_download"
	JobKeyRefreshMediaItemsAndCollections JobKey = "refresh_media_items_and_collections"
	JobKeyCheckForMediaItemChanges        JobKey = "check_for_media_item_changes"
	JobKeyCheckForNewMediaItems           JobKey = "check_for_new_media_items"
	JobKeyHandleTempIgnoredItems          JobKey = "handle_temp_ignored_items"
	JobKeyRefreshMediuxUsers              JobKey = "refresh_mediux_users"
	JobKeyCheckMediuxSiteLink             JobKey = "check_mediux_site_link"
	JobKeyBackupDatabase                  JobKey = "backup_database"
	JobKeyUpgradeSavedSets                JobKey = "upgrade_saved_sets"
)

type jobDefinition struct {
	Key         JobKey
	Name        string
	Description string
	Setting     func() config.Config_JobSetting
	Default     config.JobSettingDefault
	Start       func() error
	RunNow      func()
}

var jobDefinitions = []jobDefinition{
	{
		Key:         JobKeyRefreshMediaItemsAndCollections,
		Name:        "Refresh Media Items and Collections Job",
		Description: "Re-scans your Media Server libraries, pulling in new and updated movies, shows, and collections to update the backend cache.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.RefreshMediaItemsAndCollections },
		Default:     config.JobDefaults.RefreshMediaItemsAndCollections,
		Start:       StartRefreshMediaItemsAndCollectionsJob,
		RunNow:      RunRefreshMediaItemsAndCollectionsJobNow,
	},
	{
		Key:         JobKeyCheckForNewMediaItems,
		Name:        "Check for New Media Items Job",
		Description: "Asks your Media Server libraries for their newest movies and shows, and adds the ones aura doesn't have yet. It only looks at the newest items, so it can run often.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.CheckForNewMediaItems },
		Default:     config.JobDefaults.CheckForNewMediaItems,
		Start:       StartCheckForNewMediaItemsJob,
		RunNow:      RunCheckForNewMediaItemsJobNow,
	},
	{
		Key:         JobKeyCheckForMediaItemChanges,
		Name:        "Check for Media Item Changes Job",
		Description: "Compare Media Items from the database with the Media Server cache to see if any items have been removed or changed. If a Media Item is no longer in the cache, it will be removed from the database unless it has Saved Sets or is Temp Ignored.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.CheckForMediaItemChanges },
		Default:     config.JobDefaults.CheckForMediaItemChanges,
		Start:       StartCheckForMediaItemChangesJob,
		RunNow:      RunCheckForMediaItemChangesJobNow,
	},
	{
		Key:         JobKeyHandleTempIgnoredItems,
		Name:        "Handle Temp Ignored Items Job",
		Description: "Rechecks temporarily ignored media items to see if a matching set has since become available on MediUX.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.HandleTempIgnoredItems },
		Default:     config.JobDefaults.HandleTempIgnoredItems,
		Start:       StartHandleTempIgnoredItemsJob,
		RunNow:      RunHandleTempIgnoredItemsJobNow,
	},
	{
		Key:         JobKeyRefreshMediuxUsers,
		Name:        "Refresh Mediux Users Job",
		Description: "Update the list of MediUX users in the backend cache, so that search results reflect the current set of users within MediUX.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.RefreshMediuxUsers },
		Default:     config.JobDefaults.RefreshMediuxUsers,
		Start:       StartRefreshMediuxUsersJob,
		RunNow:      RunRefreshMediuxUsersJobNow,
	},
	{
		Key:         JobKeyCheckMediuxSiteLink,
		Name:        "Check Mediux Site Link Availability Job",
		Description: "Currently, MediUX has 2 sites. 1 for live and 1 for testing. This job checks the availability of the testing site, and falls back to the live site if the testing site is unavailable.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.CheckMediuxSiteLink },
		Default:     config.JobDefaults.CheckMediuxSiteLink,
		Start:       StartCheckMediuxSiteLinkJob,
		RunNow:      RunCheckMediuxSiteLinkJobNow,
	},
	{
		Key:         JobKeyAutoDownload,
		Name:        "AutoDownload Job",
		Description: "Automatically checks your saved sets for new or updated images and downloads them.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.AutoDownload },
		Default:     config.JobDefaults.AutoDownload,
		Start:       StartAutoDownloadJob,
		RunNow:      RunAutoDownloadJobNow,
	},
	{
		Key:         JobKeyBackupDatabase,
		Name:        "Backup Database Job",
		Description: "Saves a copy of the database, with your settings and saved sets, to the backups folder next to it. The oldest copies are deleted so only the number set in Settings > Backups are kept.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.BackupDatabase },
		Default:     config.JobDefaults.BackupDatabase,
		Start:       StartBackupDatabaseJob,
		RunNow:      RunBackupDatabaseJobNow,
	},
	{
		Key:         JobKeyUpgradeSavedSets,
		Name:        "Upgrade Saved Sets Job",
		Description: "When Automatically Upgrade Sets is on in User Preferences, replaces saved sets with a set from a higher ranked favorite creator. Sets with Auto Upgrade off are kept.",
		Setting:     func() config.Config_JobSetting { return config.Current.Jobs.UpgradeSavedSets },
		Default:     config.JobDefaults.UpgradeSavedSets,
		Start:       StartUpgradeSavedSetsJob,
		RunNow:      RunUpgradeSavedSetsJobNow,
	},
}

func findJobDefinition(key JobKey) (jobDefinition, bool) {
	for _, def := range jobDefinitions {
		if def.Key == key {
			return def, true
		}
	}
	return jobDefinition{}, false
}

var (
	c  *cron.Cron
	mu sync.Mutex

	// jobEntryIDs holds the cron.EntryID currently backing a job, if it is enabled
	// and scheduled. A key absent from this map (or mapped to 0) means the job is
	// not currently scheduled
	jobEntryIDs   = map[JobKey]cron.EntryID{}
	manualPrevRun = map[JobKey]string{}
)

func init() {
	// A panic in a job is logged instead of stopping aura
	c = cron.New(cron.WithChain(cron.Recover(cronLogger{})))
}

// cronLogger logs the cron library's errors, like a panic in a job
type cronLogger struct{}

func (cronLogger) Info(msg string, keysAndValues ...any) {}

func (cronLogger) Error(err error, msg string, keysAndValues ...any) {
	event := logging.LOGGER.Error().Timestamp().Err(err)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		event = event.Interface(fmt.Sprint(keysAndValues[i]), keysAndValues[i+1])
	}
	event.Msg("PANIC: in Cron Job")
}

func StartJobs() {
	if c != nil {
		c.Start()
		logging.LOGGER.Info().Timestamp().Msg("Cron Jobs Scheduler Started")
	}
}

// StopJobs stops the scheduler and waits for running jobs to finish, or for ctx to end
func StopJobs(ctx context.Context) {
	if c == nil {
		return
	}
	select {
	case <-c.Stop().Done():
	case <-ctx.Done():
		logging.LOGGER.Warn().Timestamp().Msg("Stopped waiting for running cron jobs to finish")
	}
}

// StartJob (re)starts a single job by key, applying its current config
// Used both at startup and when a config update changes
func StartJob(key JobKey) error {
	def, ok := findJobDefinition(key)
	if !ok {
		return fmt.Errorf("unknown job: %s", key)
	}
	return def.Start()
}

// TriggerJob runs a job immediately, regardless of whether it is currently enabled/scheduled for automatic runs
func TriggerJob(key JobKey) error {
	def, ok := findJobDefinition(key)
	if !ok {
		return fmt.Errorf("unknown job: %s", key)
	}
	def.RunNow()
	return nil
}

// removeScheduledJob removes a job's cron entry and clears its tracked entry ID
func removeScheduledJob(key JobKey) {
	if entryID, ok := jobEntryIDs[key]; ok && entryID != 0 {
		c.Remove(entryID)
	}
	delete(jobEntryIDs, key)
}

// addScheduledJob registers spec as the cron schedule for key
func addScheduledJob(key JobKey, spec string, job func()) error {
	entryID, err := c.AddFunc(spec, job)
	if err != nil {
		return err
	}
	jobEntryIDs[key] = entryID
	return nil
}

// recordManualRun stamps a job as having just been run manually
func recordManualRun(key JobKey) {
	mu.Lock()
	defer mu.Unlock()
	manualPrevRun[key] = time.Now().Format("2006-01-02 15:04:05")
}

// nextRunString returns the human-readable next run time for a scheduled job
func nextRunString(key JobKey) string {
	mu.Lock()
	defer mu.Unlock()
	if c == nil {
		return ""
	}
	entryID, ok := jobEntryIDs[key]
	if !ok || entryID == 0 {
		return ""
	}
	return c.Entry(entryID).Next.String()
}

type JobInfo struct {
	ID          string `json:"id"`
	Spec        string `json:"spec"`
	Enabled     bool   `json:"enabled"`
	NextRun     string `json:"next_run"`
	PrevRun     string `json:"prev_run"`
	JobName     string `json:"job_name"`
	Description string `json:"description"`
}

// GetListOfJobs returns one JobInfo per known job, including jobs that are
// currently disabled. Each job's ID is its stable JobKey, so it never collides
// with another job's ID regardless of scheduling state.
func GetListOfJobs() []JobInfo {
	mu.Lock()
	defer mu.Unlock()

	jobsList := make([]JobInfo, 0, len(jobDefinitions))
	for _, def := range jobDefinitions {
		enabled, cron := def.Setting().Resolve(def.Default)

		info := JobInfo{
			ID:          string(def.Key),
			JobName:     def.Name,
			Description: def.Description,
			Enabled:     enabled,
			Spec:        cron,
		}

		if entryID, ok := jobEntryIDs[def.Key]; ok && entryID != 0 && c != nil {
			entry := c.Entry(entryID)
			if entry.ID != 0 {
				info.NextRun = entry.Next.Format("2006-01-02 15:04:05")
				if manual, ok := manualPrevRun[def.Key]; ok {
					info.PrevRun = manual
				} else if !entry.Prev.IsZero() {
					info.PrevRun = entry.Prev.Format("2006-01-02 15:04:05")
				}
			}
		}

		jobsList = append(jobsList, info)
	}
	return jobsList
}
