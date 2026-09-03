package jobs

import (
	"aura/logging"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	c  *cron.Cron
	mu sync.Mutex

	jobSpecs = map[cron.EntryID]string{}

	refreshMediaItemsAndCollectionsJobID cron.EntryID = 0
	refreshMediuxUsersJobID              cron.EntryID = 0
	checkMediuxSiteLinkJobID             cron.EntryID = 0
	checkForMediaItemChangesJobID        cron.EntryID = 0
	handleTempIgnoredItemsJobID          cron.EntryID = 0
	autodownloadJobID                    cron.EntryID = 0
)

var manualPrevRun = map[string]string{}

func init() {
	c = cron.New()
}

func StartJobs() {
	if c != nil {
		c.Start()
		logging.LOGGER.Info().Timestamp().Msg("Cron Jobs Scheduler Started")
	}
}

func StartConfiguredJobs() {
	_ = StartAutoDownloadJob()
	_ = StartRefreshMediaItemsAndCollectionsJob()
	_ = StartRefreshMediuxUsersJob()
	_ = StartCheckMediuxSiteLinkJob()
	_ = StartCheckForMediaItemChangesJob()
	_ = StartHandleTempIgnoredItemsJob()
}

type JobInfo struct {
	ID      cron.EntryID `json:"id"`
	Spec    string       `json:"spec"`
	NextRun string       `json:"next_run"`
	PrevRun string       `json:"prev_run"`
	JobName string       `json:"job_name"`
}

type jobDescriptor struct {
	Name    string
	EntryID *cron.EntryID
	Runner  func()
}

func getJobDescriptors() []jobDescriptor {
	return []jobDescriptor{
		{
			Name:    "AutoDownload Job",
			EntryID: &autodownloadJobID,
			Runner:  RunAutoDownloadJob,
		},
		{
			Name:    "Refresh Media Items and Collections Job",
			EntryID: &refreshMediaItemsAndCollectionsJobID,
			Runner:  RunRefreshMediaItemsAndCollectionsJob,
		},
		{
			Name:    "Refresh Mediux Users Job",
			EntryID: &refreshMediuxUsersJobID,
			Runner:  RunRefreshMediuxUsersJob,
		},
		{
			Name:    "Check Mediux Site Link Availability Job",
			EntryID: &checkMediuxSiteLinkJobID,
			Runner:  RunCheckMediuxSiteLinkJob,
		},
		{
			Name:    "Check for Media Item Changes Job",
			EntryID: &checkForMediaItemChangesJobID,
			Runner:  RunCheckForMediaItemChangesJob,
		},
		{
			Name:    "Handle Temp Ignored Items Job",
			EntryID: &handleTempIgnoredItemsJobID,
			Runner:  RunHandleTempIgnoredItemsJob,
		},
	}
}

func GetListOfJobs() []JobInfo {
	mu.Lock()
	defer mu.Unlock()

	var jobs []JobInfo
	descriptors := getJobDescriptors()

	for idx, desc := range descriptors {
		entryID := *desc.EntryID
		prevRun := manualPrevRun[desc.Name]

		jobInfo := JobInfo{
			ID:      cron.EntryID(idx + 1),
			JobName: desc.Name,
			PrevRun: prevRun,
		}

		if entryID != 0 && c != nil {
			entry := c.Entry(entryID)
			if entry.ID != 0 {
				jobInfo.ID = entry.ID
				if spec, ok := jobSpecs[entry.ID]; ok {
					jobInfo.Spec = spec
				} else {
					jobInfo.Spec = fmt.Sprintf("%T", entry.Schedule)
				}
				jobInfo.NextRun = entry.Next.Format("2006-01-02 15:04:05")
				if jobInfo.PrevRun == "" && !entry.Prev.IsZero() {
					jobInfo.PrevRun = entry.Prev.Format("2006-01-02 15:04:05")
				}
			} else {
				jobInfo.Spec = "Disabled"
				jobInfo.NextRun = "Disabled"
			}
		} else {
			jobInfo.Spec = "Disabled"
			jobInfo.NextRun = "Disabled"
		}

		jobs = append(jobs, jobInfo)
	}

	return jobs
}

func TriggerJob(jobName string, jobID string) error {
	mu.Lock()
	defer mu.Unlock()

	descriptors := getJobDescriptors()
	var matchedDesc *jobDescriptor
	for i := range descriptors {
		if descriptors[i].Name == jobName {
			matchedDesc = &descriptors[i]
			break
		}
	}

	if matchedDesc == nil {
		return fmt.Errorf("unknown job name: %s", jobName)
	}

	go func() {
		mu.Lock()
		manualPrevRun[matchedDesc.Name] = time.Now().Format("2006-01-02 15:04:05")
		entryID := *matchedDesc.EntryID
		mu.Unlock()

		if entryID != 0 && c != nil {
			entry := c.Entry(entryID)
			if entry.ID != 0 && entry.Job != nil {
				entry.Job.Run()
				return
			}
		}

		// If the job is disabled in cron schedule, run its task directly
		if matchedDesc.Runner != nil {
			matchedDesc.Runner()
		}
	}()

	return nil
}
