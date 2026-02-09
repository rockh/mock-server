package main

import (
	"fmt"
	"strings"
)

// ANSI color codes for terminal output
const (
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorReset  = "\033[0m"
)

// Log levels with symbols — widths tuned so columns align.
const (
	LogInfo    = "ℹ  info   "
	LogWarning = "⚠  warning"
	LogError   = "✖  error  "
	LogSuccess = "✔  success"
)

// Logger components
const (
	ComponentHTTPServer = "HTTP SERVER"
	ComponentValidator  = "VALIDATOR"
	ComponentNegotiator = "NEGOTIATOR"
)

// Logger provides structured logging similar to Prism CLI.
type Logger struct {
	indent string
}

// NewLogger creates a new Logger.
func NewLogger() *Logger {
	return &Logger{indent: "    "}
}

// RequestReceived prints the first line like Prism.
func (l *Logger) RequestReceived(method, path string) {
	fmt.Printf("[%s] %s %s %s   %s\n",
		ComponentHTTPServer,
		strings.ToLower(method),
		path,
		LogInfo,
		"Request received",
	)
}

func (l *Logger) log(component, level, message string) {
	var colorCode string
	switch level {
	case LogWarning:
		colorCode = colorYellow
	case LogError:
		colorCode = colorRed
	default:
		colorCode = ""
	}

	if colorCode != "" {
		fmt.Printf("%s[%s] %s%s%s   %s\n", l.indent, component, colorCode, level, colorReset, message)
	} else {
		fmt.Printf("%s[%s] %s   %s\n", l.indent, component, level, message)
	}
}

// Info logs an info message.
func (l *Logger) Info(component, message string) {
	l.log(component, LogInfo, message)
}

// Warning logs a warning message.
func (l *Logger) Warning(component, message string) {
	l.log(component, LogWarning, message)
}

// Error logs an error message.
func (l *Logger) Error(component, message string) {
	l.log(component, LogError, message)
}

// Success logs a success message.
func (l *Logger) Success(component, message string) {
	l.log(component, LogSuccess, message)
}

// RespondWith emits the standard Prism negotiation block.
func (l *Logger) RespondWith(statusCode int) {
	l.Success(ComponentNegotiator, fmt.Sprintf("Found response %d. I'll try with it.", statusCode))
	l.Success(ComponentNegotiator, fmt.Sprintf("The response %d has a schema. I'll keep going with this one", statusCode))
	l.Success(ComponentNegotiator, fmt.Sprintf("Responding with the requested status code %d", statusCode))
	l.Info(ComponentNegotiator, fmt.Sprintf("> Responding with \"%d\"", statusCode))
}

// Violation emits a final Violation line.
func (l *Logger) Violation(message string) {
	l.Error(ComponentValidator, "Violation: request "+message)
}
