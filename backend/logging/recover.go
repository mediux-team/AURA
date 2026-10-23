package logging

import "runtime/debug"

// Go runs fn in the background, and logs a panic instead of stopping aura
func Go(name string, fn func()) {
	go func() {
		defer RecoverPanic(name)
		fn()
	}()
}

// RecoverPanic logs a panic with its stack. Call it with defer.
func RecoverPanic(name string) {
	if r := recover(); r != nil {
		LOGGER.Error().Timestamp().Interface("recover", r).Str("stack", string(debug.Stack())).Msgf("PANIC: in %s", name)
	}
}
