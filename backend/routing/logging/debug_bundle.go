package routes_logging

import (
	"archive/zip"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Log files are added newest first until they reach this size, the newest is always added
const maxDebugBundleLogSize = 30 << 20 // 30 MiB

// How long to wait for each media server's version
const debugBundleConnectionTimeout = 10 * time.Second

// GetDebugBundle godoc
// @Summary      Download Debug Bundle
// @Description  Download a zip to share when reporting a problem. It has the versions and media servers, the redacted config and the recent logs. Passwords, tokens, API keys, emails and addresses are replaced in the config and the logs.
// @Tags         Logging
// @Produce      application/zip
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {file}    file  "The debug bundle as a zip"
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/logs/debug-bundle [get]
func GetDebugBundle(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Create Debug Bundle", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	exportConfig := config.Current.Clone()
	exportConfig.MediaServer.UserID = ""
	configData, err := yaml.Marshal(exportConfig)
	if err != nil {
		logAction.SetError("Failed to create debug bundle", err.Error(), nil)
		httpx.SendResponse(w, ld, nil)
		return
	}
	// The config is redacted first, so its secrets and addresses are also replaced in the logs
	redactor := config.NewRedactor()
	configData, err = redactor.ConfigYAML(configData)
	if err != nil {
		logAction.SetError("Failed to redact config", err.Error(), nil)
		httpx.SendResponse(w, ld, nil)
		return
	}

	now := time.Now()
	files := []bundleFile{
		{"info.txt", []byte(debugBundleInfo(ctx, &exportConfig, redactor, now))},
		{"config-redacted.yaml", configData},
	}
	logFiles, err := recentLogFiles(logging.LogFolder)
	if err != nil {
		logAction.SetError("Failed to read the log files", err.Error(), nil)
		httpx.SendResponse(w, ld, nil)
		return
	}
	for _, logFile := range logFiles {
		data, err := os.ReadFile(logFile)
		if err != nil {
			continue
		}
		files = append(files, bundleFile{"logs/" + filepath.Base(logFile), []byte(redactor.Text(string(data)))})
	}

	logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("log_files", len(logFiles)).Msg("Debug bundle created")

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="aura-debug-%s.zip"`, now.Format("2006-01-02")))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	zw := zip.NewWriter(w)
	for _, file := range files {
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: file.name, Method: zip.Deflate, Modified: now})
		if err != nil {
			return
		}
		if _, err := fw.Write(file.data); err != nil {
			return
		}
	}
	_ = zw.Close()
}

type bundleFile struct {
	name string
	data []byte
}

// recentLogFiles returns the log files newest first, up to maxDebugBundleLogSize
func recentLogFiles(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	type logFile struct {
		path     string
		size     int64
		modified time.Time
	}
	var all []logFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "aura") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		all = append(all, logFile{filepath.Join(folder, entry.Name()), info.Size(), info.ModTime()})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].modified.After(all[j].modified) })

	var paths []string
	var total int64
	for _, file := range all {
		if len(paths) > 0 && total+file.size > maxDebugBundleLogSize {
			break
		}
		paths = append(paths, file.path)
		total += file.size
	}
	return paths, nil
}

// debugBundleInfo describes the versions, platform and media servers
func debugBundleInfo(ctx context.Context, cfg *config.Config, redactor *config.Redactor, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "aura debug bundle, created %s\n\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "aura version: %s\n", config.AppVersion)
	switch {
	case config.AppCommit != "":
		fmt.Fprintf(&b, "Build: %s, commit %s, built %s\n", config.AppBuildNumber, config.AppCommit, config.AppBuildDate)
	case config.AppBuildNumber != "":
		fmt.Fprintf(&b, "Build: %s\n", config.AppBuildNumber)
	default:
		b.WriteString("Build: none, built without the build details\n")
	}
	if version, Err := database.GetCurrentVersion(ctx); Err.Message != "" {
		fmt.Fprintf(&b, "Database version: unknown (%s), latest %d\n", Err.Message, database.LATEST_DB_VERSION)
	} else {
		fmt.Fprintf(&b, "Database version: %d, latest %d\n", version, database.LATEST_DB_VERSION)
	}
	fmt.Fprintf(&b, "Platform: %s/%s, %s\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
	_, err := os.Stat("/.dockerenv")
	fmt.Fprintf(&b, "Docker: %t\n", err == nil)
	fmt.Fprintf(&b, "Config loaded: %t, valid: %t\n", config.Loaded, config.Valid)
	fmt.Fprintf(&b, "Log level: %s\n", logging.LogLevel.String())
	if count, Err := database.CountActiveDownloadQueueJobs(ctx); Err.Message == "" {
		fmt.Fprintf(&b, "Active download queue jobs: %d\n", count)
	}

	// The servers are checked at the same time, so one that's down doesn't hold up the rest
	connections := make([]string, len(cfg.MediaServers))
	var wg sync.WaitGroup
	for i, server := range cfg.MediaServers {
		if !server.Enabled && server.ID != cfg.MainMediaServer {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			connections[i] = checkConnection(ctx, server)
		}()
	}
	wg.Wait()

	b.WriteString("\nMedia servers:\n")
	if len(cfg.MediaServers) == 0 {
		b.WriteString("- none\n")
	}
	for i, server := range cfg.MediaServers {
		isMain := server.ID == cfg.MainMediaServer
		role := "other"
		if isMain {
			role = "main"
		}
		// The name isn't added, it's chosen by the user
		fmt.Fprintf(&b, "- Server %d (%s): %s, enabled: %t\n", i+1, role, server.Type, server.Enabled || isMain)
		if connections[i] != "" {
			// Errors can include the server's address
			fmt.Fprintf(&b, "  Connection: %s\n", redactor.Text(connections[i]))
		}
		for _, library := range server.Libraries {
			if library.Type == "" {
				fmt.Fprintf(&b, "  Library: %s\n", library.Title)
			} else {
				fmt.Fprintf(&b, "  Library: %s (%s)\n", library.Title, library.Type)
			}
		}
		if len(server.LibraryMap) > 0 {
			mapped := make([]string, 0, len(server.LibraryMap))
			for library, main := range server.LibraryMap {
				mapped = append(mapped, fmt.Sprintf("%s -> %s", library, main))
			}
			sort.Strings(mapped)
			fmt.Fprintf(&b, "  Library map: %s\n", strings.Join(mapped, ", "))
		}
	}
	return b.String()
}

// checkConnection returns the server's version, or why it couldn't connect
func checkConnection(ctx context.Context, server config.Config_MediaServer) string {
	ctx, cancel := context.WithTimeout(ctx, debugBundleConnectionTimeout)
	defer cancel()
	ok, _, version, Err := mediaserver.TestConnection(ctx, &server)
	switch {
	case Err.Message != "":
		return fmt.Sprintf("failed (%s)", Err.Message)
	case !ok:
		return "failed"
	default:
		return "ok, version " + version
	}
}
