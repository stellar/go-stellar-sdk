package log

import (
	"context"
	"fmt"
	"io"

	gerr "github.com/go-errors/errors"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

const timeStampFormat = "2006-01-02T15:04:05.000Z07:00"

// Ctx appends all fields from `e` to the new logger created from `ctx`
// logger and returns it.
func (e *Entry) Ctx(ctx context.Context) *Entry {
	if ctx == nil {
		return e
	}

	found := ctx.Value(&loggerContextKey)
	if found == nil {
		return e
	}

	entry := found.(*Entry)

	// Copy all fields from e to entry
	for key, value := range e.entry.Data {
		entry = entry.WithField(key, value)
	}

	return entry
}

func (e *Entry) SetLevel(level logrus.Level) {
	e.entry.Logger.SetLevel(level)
}

func (e *Entry) SetExitFunc(exitFun func(int)) {
	e.entry.Logger.ExitFunc = exitFun
}

func (e *Entry) UseJSONFormatter() {
	formatter := new(logrus.JSONFormatter)
	formatter.TimestampFormat = timeStampFormat
	e.entry.Logger.Formatter = formatter
}

func (e *Entry) DisableColors() {
	if f, ok := e.entry.Logger.Formatter.(*logrus.TextFormatter); ok {
		f.DisableColors = true
	}
}

func (e *Entry) DisableTimestamp() {
	if f, ok := e.entry.Logger.Formatter.(*logrus.TextFormatter); ok {
		f.DisableTimestamp = true
	} else if f, ok := e.entry.Logger.Formatter.(*logrus.JSONFormatter); ok {
		f.DisableTimestamp = true
	}
}

// WithField creates a child logger annotated with the provided key value pair.
// A subsequent call to one of the logging methods (Debug(), Error(), etc.) to
// the return value from this function will cause the emitted log line to
// include the provided value.
func (e *Entry) WithField(key string, value interface{}) *Entry {
	return &Entry{
		entry: *e.entry.WithField(key, value),
	}
}

// WithFields creates a child logger annotated with the provided key value
// pairs.
func (e *Entry) WithFields(fields F) *Entry {
	return &Entry{
		entry: *e.entry.WithFields(logrus.Fields(fields)),
	}
}

// AddHook adds a hook to the logger hooks.
func (e *Entry) AddHook(hook logrus.Hook) {
	e.entry.Logger.AddHook(hook)
}

// SetOutput sets the logger output.
func (e *Entry) SetOutput(output io.Writer) {
	e.entry.Logger.SetOutput(output)
}

// WithStack annotates this error with a stack trace from `stackProvider`, if
// available.  normally `stackProvider` would be an error that implements
// `errors.StackTracer`.
func (e *Entry) WithStack(stackProvider interface{}) *Entry {
	stack := "unknown"

	if sp1, ok := stackProvider.(errors.StackTracer); ok {
		stack = fmt.Sprint(sp1.StackTrace())
	} else if sp2, ok := stackProvider.(*gerr.Error); ok {
		stack = fmt.Sprint(sp2.ErrorStack())
	}

	return e.WithField("stack", stack)
}

// Add an error as single field to the log entry.  All it does is call
// `WithError` for the given `error`.
func (e *Entry) WithError(err error) *Entry {
	return &Entry{
		entry: *e.entry.WithError(err),
	}
}

// Add a context to the log entry.
func (e *Entry) WithContext(ctx context.Context) *Entry {
	return &Entry{
		entry: *e.entry.WithContext(ctx),
	}
}

// Debugf logs a message at the debug severity.
func (e *Entry) Debugf(format string, args ...interface{}) {
	e.entry.Debugf(format, args...)
}

// Debug logs a message at the debug severity.
func (e *Entry) Debug(args ...interface{}) {
	e.entry.Debug(args...)
}

// Infof logs a message at the Info severity.
func (e *Entry) Infof(format string, args ...interface{}) {
	e.entry.Infof(format, args...)
}

// Info logs a message at the Info severity.
func (e *Entry) Info(args ...interface{}) {
	e.entry.Info(args...)
}

// Warnf logs a message at the Warn severity.
func (e *Entry) Warnf(format string, args ...interface{}) {
	e.entry.Warnf(format, args...)
}

// Warn logs a message at the Warn severity.
func (e *Entry) Warn(args ...interface{}) {
	e.entry.Warn(args...)
}

// Errorf logs a message at the Error severity.
func (e *Entry) Errorf(format string, args ...interface{}) {
	e.entry.Errorf(format, args...)
}

// Error logs a message at the Error severity.
func (e *Entry) Error(args ...interface{}) {
	e.entry.Error(args...)
}

// Fatalf logs a message at the Fatal severity.
func (e *Entry) Fatalf(format string, args ...interface{}) {
	e.entry.Fatalf(format, args...)
}

// Fatal logs a message at the Fatal severity.
func (e *Entry) Fatal(args ...interface{}) {
	e.entry.Fatal(args...)
}

// Panicf logs a message at the Panic severity.
func (e *Entry) Panicf(format string, args ...interface{}) {
	e.entry.Panicf(format, args...)
}

// Panic logs a message at the Panic severity.
func (e *Entry) Panic(args ...interface{}) {
	e.entry.Panic(args...)
}

func (e *Entry) Print(args ...interface{}) {
	e.entry.Print(args...)
}

// StartTest shifts this logger into "test" mode, ensuring that log lines will
// be recorded (rather than outputted).  The returned function concludes the
// test, switches the logger back into normal mode and returns a slice of all
// raw logrus entries that were created during the test.
func (e *Entry) StartTest(level logrus.Level) func() []logrus.Entry {
	if e.isTesting {
		panic("cannot start logger test: already testing")
	}

	e.isTesting = true

	hook := &test.Hook{}
	e.entry.Logger.AddHook(hook)

	old := e.entry.Logger.Out
	e.entry.Logger.Out = io.Discard

	oldLevel := e.entry.Logger.GetLevel()
	e.entry.Logger.SetLevel(level)

	return func() []logrus.Entry {
		e.entry.Logger.SetLevel(oldLevel)
		e.entry.Logger.SetOutput(old)
		e.removeHook(hook)
		e.isTesting = false
		return hook.Entries
	}
}

// removeHook removes a hook, in the most complicated way possible.
func (e *Entry) removeHook(target logrus.Hook) {
	for lvl, hooks := range e.entry.Logger.Hooks {
		kept := []logrus.Hook{}

		for _, hook := range hooks {
			if hook != target {
				kept = append(kept, hook)
			}
		}

		e.entry.Logger.Hooks[lvl] = kept
	}
}
