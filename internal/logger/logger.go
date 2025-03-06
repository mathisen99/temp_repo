package logger

import (
	"fmt"
	"github.com/fatih/color"
	stdlog "log"
	"os"
	"path/filepath"
	"time"
)

type LogLevel string

const (
	LevelInfo    LogLevel = "INFO"
	LevelSuccess LogLevel = "SUCCESS"
	LevelWarning LogLevel = "WARNING"
	LevelError   LogLevel = "ERROR"
	LevelDebug   LogLevel = "DEBUG"
	LevelNotice  LogLevel = "NOTICE"
)

var (
	errorLogger *stdlog.Logger
	errorLogFile *os.File
	
	// Separate AI logger that doesn't write to the error log
	aiLogger *stdlog.Logger
	aiLogFile *os.File
)

func init() {
	// Set up logs directory
	dataDir := filepath.Join("data")
	
	// Ensure the directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Printf("Error creating data directory: %v\n", err)
		return
	}
	
	// Set up error log file
	logPath := filepath.Join(dataDir, "error.log")

	// Create or open error log file
	var err error
	errorLogFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening error log file: %v\n", err)
	} else {
		// Initialize error logger
		errorLogger = stdlog.New(errorLogFile, "", 0)
	}
	
	// Set up AI log file (separate from error log)
	aiLogPath := filepath.Join(dataDir, "ai.log")
	
	// Create or open AI log file
	aiLogFile, err = os.OpenFile(aiLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening AI log file: %v\n", err)
	} else {
		// Initialize AI logger
		aiLogger = stdlog.New(aiLogFile, "", 0)
	}
}

// CloseLogFile should be called during shutdown to properly close all log files
func CloseLogFile() {
	if errorLogFile != nil {
		errorLogFile.Close()
	}
	
	if aiLogFile != nil {
		aiLogFile.Close()
	}
	
	fmt.Println("All log files closed")
}

var colorMap = map[string]func(a ...interface{}) string{
	string(LevelInfo):    color.New(color.FgBlue).SprintFunc(),
	string(LevelSuccess): color.New(color.FgGreen).SprintFunc(),
	string(LevelWarning): color.New(color.FgYellow).SprintFunc(),
	string(LevelError):   color.New(color.FgRed).SprintFunc(),
	string(LevelDebug):   color.New(color.FgCyan).SprintFunc(),
	string(LevelNotice):  color.New(color.FgMagenta).SprintFunc(),

	"blue":    color.New(color.FgBlue).SprintFunc(),
	"green":   color.New(color.FgGreen).SprintFunc(),
	"yellow":  color.New(color.FgYellow).SprintFunc(),
	"red":     color.New(color.FgRed).SprintFunc(),
	"cyan":    color.New(color.FgCyan).SprintFunc(),
	"magenta": color.New(color.FgMagenta).SprintFunc(),
	"white":   color.New(color.FgWhite).SprintFunc(),
	"black":   color.New(color.FgBlack).SprintFunc(),
	"purple":  color.New(color.FgHiMagenta).SprintFunc(),

	"bright_blue":    color.New(color.FgHiBlue).SprintFunc(),
	"bright_green":   color.New(color.FgHiGreen).SprintFunc(),
	"bright_yellow":  color.New(color.FgHiYellow).SprintFunc(),
	"bright_red":     color.New(color.FgHiRed).SprintFunc(),
	"bright_cyan":    color.New(color.FgHiCyan).SprintFunc(),
	"bright_magenta": color.New(color.FgHiMagenta).SprintFunc(),
	"bright_white":   color.New(color.FgHiWhite).SprintFunc(),
	"bright_black":   color.New(color.FgHiBlack).SprintFunc(),
}

func GetColorFunc(colorName string) func(a ...interface{}) string {
	if fn, ok := colorMap[colorName]; ok {
		return fn
	}
	return colorMap["white"]
}

func logMessage(level LogLevel, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	
	// Print to console with color
	colorFunc := GetColorFunc(string(level))
	fmt.Println(colorFunc(fmt.Sprintf("[%s] ", level)) + message)
	
	// Log only error and warning messages to error.log
	// Debug messages should not go to error.log to avoid cluttering it with AI processing
	if level == LevelError || level == LevelWarning {
		if errorLogger != nil {
			errorLogger.Printf("[%s] %s: %s", level, timestamp, message)
		}
	}
}

func Infof(format string, args ...interface{}) {
	logMessage(LevelInfo, format, args...)
}

func Successf(format string, args ...interface{}) {
	logMessage(LevelSuccess, format, args...)
}

func Warnf(format string, args ...interface{}) {
	logMessage(LevelWarning, format, args...)
}

func Errorf(format string, args ...interface{}) {
	logMessage(LevelError, format, args...)
}

func Debugf(format string, args ...interface{}) {
	logMessage(LevelDebug, format, args...)
}

// AIDebugf logs AI-related debug messages to the AI log file instead of error.log
// This helps keep the error log clean while still having detailed AI logs
func AIDebugf(format string, args ...interface{}) {
	// Format the message
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	
	// Print to console with color (same as regular debug)
	colorFunc := GetColorFunc(string(LevelDebug))
	fmt.Println(colorFunc(fmt.Sprintf("[AI-DEBUG] ")) + message)
	
	// Log to AI log file instead of error.log
	if aiLogger != nil {
		aiLogger.Printf("[DEBUG] %s: %s", timestamp, message)
	}
}

func Noticef(format string, args ...interface{}) {
	logMessage(LevelNotice, format, args...)
}

func Purplef(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	colorFunc := GetColorFunc("purple")
	fmt.Println(colorFunc("[PURPLE] ") + message)
}

func Whitef(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	colorFunc := GetColorFunc("white")
	fmt.Println(colorFunc("[WHITE] ") + message)
}

func Bluef(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	colorFunc := GetColorFunc("blue")
	fmt.Println(colorFunc("[MOTD] ") + message)
}

func ChanMsgf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	colorFunc := GetColorFunc("green")
	fmt.Println(colorFunc("[CHAN] ") + message)
}
