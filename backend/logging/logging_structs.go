package logging

import (
	"sync"
	"time"
)

const (
	StatusSuccess = "success"
	StatusError   = "error"
	StatusWarn    = "warn"

	LevelTrace = "trace"
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// LogData is the base element used for both HTTP routes and background jobs.
type LogData struct {
	Level               string        `json:"level,omitempty"` // Optional level for UI filtering (e.g. "trace", "debug","info","warn","error")
	Status              string        `json:"status"`          // Status: "success", "error", or "warn"
	Message             string        `json:"message"`         // Human name for the overall operation: "Add to DB", "Fetch All Media Items" or Route.Path
	Timestamp           time.Time     `json:"timestamp"`       // When the operation started
	Time                time.Time     `json:"time,omitzero"`   // Time of the log entry (same as Timestamp, kept for compatibility)
	ElapsedMicroseconds int64         `json:"elapsed_us"`      // Elapsed time in microseconds
	Route               *LogRouteInfo `json:"route,omitempty"` // Optional route metadata (nil for background jobs)
	Actions             []*LogAction  `json:"actions"`         // Slice of actions within this operation
	mu                  sync.Mutex    // Mutex to protect concurrent access to Actions
}

// RouteInfo holds request-specific metadata populated by the Chi middleware.
type LogRouteInfo struct {
	Method        string              `json:"method"`           // HTTP method (e.g. GET, POST)
	Path          string              `json:"path"`             // Registered route path (e.g. /api/v1/media/{id})
	Params        map[string][]string `json:"params,omitempty"` // Query/path/route params: map[string][]string to preserve multiple values
	IP            string              `json:"ip,omitempty"`     // Client IP address (middleware can capture)
	ResponseBytes int64               `json:"response_bytes"`   // Number of bytes written in the response (middleware can capture)
	Action        string              `json:"action,omitempty"` // The route's key in the Logs page actions, set when the logs are read
}

// LogAction is a single step inside an operation.
type LogAction struct {
	Name                string         `json:"name"`                  // Human name for the action: "Query DB", "Call External API"
	Status              string         `json:"status"`                // "success", "error", or "warn"
	Level               string         `json:"level,omitempty"`       // Optional level for UI filtering (e.g. "trace", "debug","info","warn","error")
	Warnings            map[string]any `json:"warnings,omitempty"`    // Warning message (if any)
	Error               *LogErrorInfo  `json:"error,omitempty"`       // Optional error details (omitted when nil)
	Timestamp           time.Time      `json:"timestamp"`             // When the action started (useful for ordering)
	ElapsedMicroseconds int64          `json:"elapsed_us"`            // Elapsed time for the action in microseconds
	Result              map[string]any `json:"result,omitempty"`      // Optional result data
	SubActions          []*LogAction   `json:"sub_actions,omitempty"` // Optional nested sub-actions
	Completed           bool           `json:"-"`
	mu                  sync.Mutex     // Mutex to protect concurrent access to SubActions
}

// LogErrorInfo contains structured error information.
type LogErrorInfo struct {
	Function   string         `json:"function,omitempty" example:"Reload"`
	LineNumber int            `json:"line_number,omitempty" example:"42"`
	Message    string         `json:"message" example:"Invalid or expired token"`
	Detail     map[string]any `json:"detail,omitempty" swaggertype:"object"`
	Help       string         `json:"help,omitempty" example:"Check server logs for more details"`
}
