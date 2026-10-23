package logging

import (
	"context"
	"maps"
	"runtime"
	"time"
)

// --- Context Keys ---
type logDataKeyType struct{}
type logActionKeyType struct{}

var logDataKey = logDataKeyType{}
var logActionKey = logActionKeyType{}

// --- Context Helpers ---

// WithLogData stores LogData in context.
func WithLogData(ctx context.Context, ld *LogData) context.Context {
	return context.WithValue(ctx, logDataKey, ld)
}

// LogDataFromContext retrieves LogData from context.
func LogDataFromContext(ctx context.Context) *LogData {
	ld, _ := ctx.Value(logDataKey).(*LogData)
	return ld
}

// WithCurrentAction stores the current LogAction in context.
func WithCurrentAction(ctx context.Context, action *LogAction) context.Context {
	return context.WithValue(ctx, logActionKey, action)
}

// CurrentActionFromContext retrieves the current LogAction from context.
func CurrentActionFromContext(ctx context.Context) *LogAction {
	act, _ := ctx.Value(logActionKey).(*LogAction)
	return act
}

// CreateLoggingContext returns a context with LogData and the LogData instance.
func CreateLoggingContext(ctx context.Context, name string) (context.Context, *LogData) {
	if ctx == nil {
		ctx = context.Background()
	}
	ld := LogDataFromContext(ctx)
	if ld == nil {
		ld = NewLogData(name)
		ctx = WithLogData(ctx, ld)
	}
	return ctx, ld
}

// AddSubActionToContext adds a sub-action to the current action in context.
func AddSubActionToContext(ctx context.Context, name, level string) (context.Context, *LogAction) {
	parent := CurrentActionFromContext(ctx)
	if parent == nil {
		// Without a current action the sub-action isn't logged, but can still be used
		parent = &LogAction{Level: LevelDebug}
	}
	sub := parent.AddSubAction(name, level)
	ctx = WithCurrentAction(ctx, sub)
	return ctx, sub
}

// --- LogData and LogAction Constructors ---

// NewLogData creates a new LogData instance.
func NewLogData(name string) *LogData {
	return &LogData{
		Status:    "",
		Message:   name,
		Timestamp: time.Now(),
		Route:     nil,
		Actions:   []*LogAction{},
	}
}

// AddAction adds a top-level action to LogData.
func (ld *LogData) AddAction(name, level string) *LogAction {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	action := &LogAction{
		Name:      name,
		Timestamp: time.Now(),
		Level:     ifEmpty(level, LevelDebug),
	}
	ld.Actions = append(ld.Actions, action)
	return action
}

// AddSubAction adds a sub-action to a LogAction.
func (a *LogAction) AddSubAction(name, level string) *LogAction {
	a.mu.Lock()
	defer a.mu.Unlock()
	sub := &LogAction{
		Name:      name,
		Timestamp: time.Now(),
		Level:     ifEmpty(level, a.Level),
		Result:    make(map[string]any),
		Warnings:  make(map[string]any),
	}
	a.SubActions = append(a.SubActions, sub)
	return sub
}

// --- Action Completion and Error Helpers ---

// Complete marks the action as completed and calculates elapsed time.
func (a *LogAction) Complete() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Make Complete idempotent
	if a.Completed {
		return
	}
	a.Completed = true

	a.ElapsedMicroseconds = time.Since(a.Timestamp).Microseconds()
	if a.Status == "" {
		a.Status = StatusSuccess
	}
	for _, sub := range a.SubActions {
		sub.Complete()
		if sub.Status == StatusError {
			a.Status = StatusError
			a.Error = sub.Error
		}
	}
}

// SetError marks the action as error and attaches error details.
func (a *LogAction) SetError(message, help string, detail map[string]any) {
	a.Status = StatusError
	a.Level = LevelError
	if a.Error == nil {
		a.Error = &LogErrorInfo{}
	}
	a.Error.Message = message
	a.Error.Help = help
	a.Error.Detail = detail
	a.Error.Function = getFunctionName()
	a.Error.LineNumber = getLineNumber()
	a.Complete()
}

func (a *LogAction) SetErrorFromInfo(err LogErrorInfo) {
	a.SetError(err.Message, err.Help, err.Detail)
}

// --- Utility Functions ---

// AppendResult appends or merges a value to the Result map for a given key.
// If the key does not exist, it sets the value.
// If the key exists and is a slice, it appends to the slice.
// If the key exists and is a map, it merges the maps.
func (a *LogAction) AppendResult(key string, value any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Result == nil {
		a.Result = make(map[string]any)
	}

	existing, exists := a.Result[key]
	switch v := value.(type) {
	case []any:
		if exists {
			if slice, ok := existing.([]any); ok {
				a.Result[key] = append(slice, v...)
				return
			}
		}
		a.Result[key] = v
	case map[string]any:
		if exists {
			if m, ok := existing.(map[string]any); ok {
				maps.Copy(m, v)
				a.Result[key] = m
				return
			}
		}
		a.Result[key] = v
	default:
		if exists {
			// If already a slice, append
			if slice, ok := existing.([]any); ok {
				a.Result[key] = append(slice, v)
				return
			}
			// Otherwise, make a slice
			a.Result[key] = []any{existing, v}
		} else {
			a.Result[key] = v
		}
	}
}

// AppendWarning appends or merges a value to the Warnings map for a given key.
// If the key does not exist, it sets the value.
// If the key exists and is a slice, it appends to the slice.
// If the key exists and is a map, it merges the maps.
func (a *LogAction) AppendWarning(key string, value any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Warnings == nil {
		a.Warnings = make(map[string]any)
	}

	if a.Status == "" {
		a.Status = StatusWarn
	}
	if a.Level != LevelError {
		a.Level = LevelWarn
	}

	existing, exists := a.Warnings[key]
	switch v := value.(type) {
	case []any:
		if exists {
			if slice, ok := existing.([]any); ok {
				a.Warnings[key] = append(slice, v...)
				return
			}
		}
		a.Warnings[key] = v
	case map[string]any:
		if exists {
			if m, ok := existing.(map[string]any); ok {
				maps.Copy(m, v)
				a.Warnings[key] = m
				return
			}
		}
		a.Warnings[key] = v
	default:
		if exists {
			// If already a slice, append
			if slice, ok := existing.([]any); ok {
				a.Warnings[key] = append(slice, v)
				return
			}
			// Otherwise, make a slice
			a.Warnings[key] = []any{existing, v}
		} else {
			a.Warnings[key] = v
		}
	}
}

func getLineNumber() int {
	if _, _, line, ok := runtime.Caller(2); ok {
		return line
	}
	return 0
}

func getFunctionName() string {
	if pc, _, _, ok := runtime.Caller(2); ok {
		return runtime.FuncForPC(pc).Name()
	}
	return ""
}

func ifEmpty(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

// LogLevelToInt converts a log level string to its integer value.
func LogLevelToInt(level string) int {
	switch level {
	case LevelTrace:
		return -1
	case LevelDebug:
		return 0
	case LevelInfo:
		return 1
	case LevelWarn:
		return 2
	case LevelError:
		return 3
	default:
		return 1 // Default to Info
	}
}
