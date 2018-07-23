/*
Copyright 2016 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package logger offers simple cross platform logging for Windows and Linux.
// Available logging endpoints are event log (Windows), syslog (Linux), and
// an io.Writer.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type severity int

// Severity levels.
const (
	sInfo severity = iota
	sWarning
	sError
	sFatal
)

// Severity tags.
const (
	tagInfo    = "INFO : "
	tagWarning = "WARN : "
	tagError   = "ERROR: "
	tagFatal   = "FATAL: "
)

const (
	flags    = log.Ldate | log.Lmicroseconds | log.Lshortfile
	initText = "ERROR: Logging before logger.Init.\n"
)

var (
	logLock       sync.Mutex
	defaultLogger *Logger
	sysYear       int
	sysMonth      string
	sysDay        int
	logFileName   string
	sysLogger     *Logger
	fileObj       *os.File
	verbose       bool
)

// initialize resets defaultLogger.  Which allows tests to reset environment.
func initialize() {
	defaultLogger = &Logger{
		infoLog:    log.New(os.Stderr, initText+tagInfo, flags),
		warningLog: log.New(os.Stderr, initText+tagWarning, flags),
		errorLog:   log.New(os.Stderr, initText+tagError, flags),
		fatalLog:   log.New(os.Stderr, initText+tagFatal, flags),
	}
}

func init() {
	initialize()
}

func checkSysTimeOfDay() bool {
	year := time.Now().Year()
	month := time.Now().Month().String()
	day := time.Now().Day()

	if sysYear == year && sysMonth == month && sysDay == day {
		return false
	} else {
		sysYear = year
		sysMonth = month
		sysDay = day
		return true
	}
}

func PathCheck(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else {
		err := os.MkdirAll(path, 0711)
		if err != nil {
			fmt.Println("Error creating directory")
			return err
		}
		return nil
	}
}

func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

func LoggerInit(logPath string, fileName string, verb bool, systemLog bool) *Logger {
	if logPath == "" {
		logPath = path.Join(getCurrentDirectory(), "log")

	}

	verbose = verb

	es := PathCheck(logPath)
	if es != nil {
		//Errorf("logPath Check error:%v", ec)
		return nil
	}

	logFileName = path.Join(logPath, fileName)

	checkSysTimeOfDay()

	file := logFileName + fmt.Sprintf("%d%s%d.log", sysYear, sysMonth, sysDay)

	fmt.Println("file: ", file)

	lf, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		Fatalf("Failed to open log file: %v", err)
		return nil
	}

	logger := Init("Luxshare.Ams", verbose, true, lf)

	if logger != nil {
		defaultLogger = logger
		fileObj = lf
		return logger
	} else {
		lf.Close()
		return nil
	}

}

func loggerReInit() *Logger {
	if defaultLogger != nil {
		defaultLogger.Close()
		fileObj.Close()
	}

	file := logFileName + fmt.Sprintf("%d%s%d.log", sysYear, sysMonth, sysDay)

	lf, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		//logger.Fatalf("Failed to open log file: %v", err)
		return nil
	}

	logger := Init("Luxshare.Ams", verbose, true, lf)

	return logger
}

// Init sets up logging and should be called before log functions, usually in
// the caller's main(). Default log functions can be called before Init(), but log
// output will only go to stderr (along with a warning).
// The first call to Init populates the default logger and returns the
// generated logger, subsequent calls to Init will only return the generated
// logger.
// If the logFile passed in also satisfies io.Closer, logFile.Close will be called
// when closing the logger.
func Init(name string, verbose, systemLog bool, logFile io.Writer) *Logger {
	var il, wl, el io.Writer
	if systemLog {
		var err error
		il, wl, el, err = setup(name)
		if err != nil {
			log.Fatal(err)
		}
	}

	iLogs := []io.Writer{logFile}
	wLogs := []io.Writer{logFile}
	eLogs := []io.Writer{logFile}
	if il != nil {
		iLogs = append(iLogs, il)
	}
	if wl != nil {
		wLogs = append(wLogs, wl)
	}
	if el != nil {
		eLogs = append(eLogs, el)
	}
	// Windows services don't have stdout/stderr. Writes will fail, so try them last.
	eLogs = append(eLogs, os.Stderr)
	if verbose {
		iLogs = append(iLogs, os.Stdout)
		wLogs = append(wLogs, os.Stdout)
	}

	l := Logger{
		infoLog:    log.New(io.MultiWriter(iLogs...), tagInfo, flags),
		warningLog: log.New(io.MultiWriter(wLogs...), tagWarning, flags),
		errorLog:   log.New(io.MultiWriter(eLogs...), tagError, flags),
		fatalLog:   log.New(io.MultiWriter(eLogs...), tagFatal, flags),
	}
	for _, w := range []io.Writer{logFile, il, wl, el} {
		if c, ok := w.(io.Closer); ok && c != nil {
			l.closers = append(l.closers, c)
		}
	}
	l.initialized = true

	logLock.Lock()
	defer logLock.Unlock()
	if !defaultLogger.initialized {
		defaultLogger = &l
	}

	return &l
}

// A Logger represents an active logging object. Multiple loggers can be used
// simultaneously even if they are using the same same writers.
type Logger struct {
	infoLog     *log.Logger
	warningLog  *log.Logger
	errorLog    *log.Logger
	fatalLog    *log.Logger
	closers     []io.Closer
	initialized bool
}

func (l *Logger) output(s severity, depth int, txt string) {
	logLock.Lock()
	defer logLock.Unlock()
	switch s {
	case sInfo:
		l.infoLog.Output(3+depth, txt)
	case sWarning:
		l.warningLog.Output(3+depth, txt)
	case sError:
		l.errorLog.Output(3+depth, txt)
	case sFatal:
		l.fatalLog.Output(3+depth, txt)
	default:
		panic(fmt.Sprintln("unrecognized severity:", s))
	}
}

// Close closes all the underlying log writers, which will flush any cached logs.
// Any errors from closing the underlying log writers will be printed to stderr.
// Once Close is called, all future calls to the logger will panic.
func (l *Logger) Close() {
	logLock.Lock()
	defer logLock.Unlock()
	for _, c := range l.closers {
		if err := c.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close log %v: %v\n", c, err)
		}
	}
}

// Info logs with the Info severity.
// Arguments are handled in the manner of fmt.Print.
func (l *Logger) Info(v ...interface{}) {
	l.output(sInfo, 0, fmt.Sprint(v...))
}

// InfoDepth acts as Info but uses depth to determine which call frame to log.
// InfoDepth(0, "msg") is the same as Info("msg").
func (l *Logger) InfoDepth(depth int, v ...interface{}) {
	l.output(sInfo, depth, fmt.Sprint(v...))
}

// Infoln logs with the Info severity.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Infoln(v ...interface{}) {
	l.output(sInfo, 0, fmt.Sprintln(v...))
}

// Infof logs with the Info severity.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Infof(format string, v ...interface{}) {
	l.output(sInfo, 0, fmt.Sprintf(format, v...))
}

// Warning logs with the Warning severity.
// Arguments are handled in the manner of fmt.Print.
func (l *Logger) Warning(v ...interface{}) {
	l.output(sWarning, 0, fmt.Sprint(v...))
}

// WarningDepth acts as Warning but uses depth to determine which call frame to log.
// WarningDepth(0, "msg") is the same as Warning("msg").
func (l *Logger) WarningDepth(depth int, v ...interface{}) {
	l.output(sWarning, depth, fmt.Sprint(v...))
}

// Warningln logs with the Warning severity.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Warningln(v ...interface{}) {
	l.output(sWarning, 0, fmt.Sprintln(v...))
}

// Warningf logs with the Warning severity.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Warningf(format string, v ...interface{}) {
	l.output(sWarning, 0, fmt.Sprintf(format, v...))
}

// Error logs with the ERROR severity.
// Arguments are handled in the manner of fmt.Print.
func (l *Logger) Error(v ...interface{}) {
	l.output(sError, 0, fmt.Sprint(v...))
}

// ErrorDepth acts as Error but uses depth to determine which call frame to log.
// ErrorDepth(0, "msg") is the same as Error("msg").
func (l *Logger) ErrorDepth(depth int, v ...interface{}) {
	l.output(sError, depth, fmt.Sprint(v...))
}

// Errorln logs with the ERROR severity.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Errorln(v ...interface{}) {
	l.output(sError, 0, fmt.Sprintln(v...))
}

// Errorf logs with the Error severity.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.output(sError, 0, fmt.Sprintf(format, v...))
}

// Fatal logs with the Fatal severity, and ends with os.Exit(1).
// Arguments are handled in the manner of fmt.Print.
func (l *Logger) Fatal(v ...interface{}) {
	l.output(sFatal, 0, fmt.Sprint(v...))
	l.Close()
	os.Exit(1)
}

// FatalDepth acts as Fatal but uses depth to determine which call frame to log.
// FatalDepth(0, "msg") is the same as Fatal("msg").
func (l *Logger) FatalDepth(depth int, v ...interface{}) {
	l.output(sFatal, depth, fmt.Sprint(v...))
	l.Close()
	os.Exit(1)
}

// Fatalln logs with the Fatal severity, and ends with os.Exit(1).
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Fatalln(v ...interface{}) {
	l.output(sFatal, 0, fmt.Sprintln(v...))
	l.Close()
	os.Exit(1)
}

// Fatalf logs with the Fatal severity, and ends with os.Exit(1).
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.output(sFatal, 0, fmt.Sprintf(format, v...))
	l.Close()
	os.Exit(1)
}

// Info uses the default logger and logs with the Info severity.
// Arguments are handled in the manner of fmt.Print.
func Info(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sInfo, 0, fmt.Sprint(v...))
}

// InfoDepth acts as Info but uses depth to determine which call frame to log.
// InfoDepth(0, "msg") is the same as Info("msg").
func InfoDepth(depth int, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sInfo, depth, fmt.Sprint(v...))
}

// Infoln uses the default logger and logs with the Info severity.
// Arguments are handled in the manner of fmt.Println.
func Infoln(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sInfo, 0, fmt.Sprintln(v...))
}

// Infof uses the default logger and logs with the Info severity.
// Arguments are handled in the manner of fmt.Printf.
func Infof(format string, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sInfo, 0, fmt.Sprintf(format, v...))
}

// Warning uses the default logger and logs with the Warning severity.
// Arguments are handled in the manner of fmt.Print.
func Warning(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sWarning, 0, fmt.Sprint(v...))
}

// WarningDepth acts as Warning but uses depth to determine which call frame to log.
// WarningDepth(0, "msg") is the same as Warning("msg").
func WarningDepth(depth int, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sWarning, depth, fmt.Sprint(v...))
}

// Warningln uses the default logger and logs with the Warning severity.
// Arguments are handled in the manner of fmt.Println.
func Warningln(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sWarning, 0, fmt.Sprintln(v...))
}

// Warningf uses the default logger and logs with the Warning severity.
// Arguments are handled in the manner of fmt.Printf.
func Warningf(format string, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sWarning, 0, fmt.Sprintf(format, v...))
}

// Error uses the default logger and logs with the Error severity.
// Arguments are handled in the manner of fmt.Print.
func Error(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sError, 0, fmt.Sprint(v...))
}

// ErrorDepth acts as Error but uses depth to determine which call frame to log.
// ErrorDepth(0, "msg") is the same as Error("msg").
func ErrorDepth(depth int, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sError, depth, fmt.Sprint(v...))
}

// Errorln uses the default logger and logs with the Error severity.
// Arguments are handled in the manner of fmt.Println.
func Errorln(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sError, 0, fmt.Sprintln(v...))
}

// Errorf uses the default logger and logs with the Error severity.
// Arguments are handled in the manner of fmt.Printf.
func Errorf(format string, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sError, 0, fmt.Sprintf(format, v...))
}

// Fatalln uses the default logger, logs with the Fatal severity,
// and ends with os.Exit(1).
// Arguments are handled in the manner of fmt.Print.
func Fatal(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sFatal, 0, fmt.Sprint(v...))
	defaultLogger.Close()
	os.Exit(1)
}

// FatalDepth acts as Fatal but uses depth to determine which call frame to log.
// FatalDepth(0, "msg") is the same as Fatal("msg").
func FatalDepth(depth int, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sFatal, depth, fmt.Sprint(v...))
	defaultLogger.Close()
	os.Exit(1)
}

// Fatalln uses the default logger, logs with the Fatal severity,
// and ends with os.Exit(1).
// Arguments are handled in the manner of fmt.Println.
func Fatalln(v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sFatal, 0, fmt.Sprintln(v...))
	defaultLogger.Close()
	os.Exit(1)
}

// Fatalf uses the default logger, logs with the Fatal severity,
// and ends with os.Exit(1).
// Arguments are handled in the manner of fmt.Printf.
func Fatalf(format string, v ...interface{}) {
	if checkSysTimeOfDay() {
		loggerReInit()
	}
	defaultLogger.output(sFatal, 0, fmt.Sprintf(format, v...))
	defaultLogger.Close()
	os.Exit(1)
}
