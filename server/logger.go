// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger is the struct to manage all logging operations in the application.
type Logger struct {
	cfg         *ConfigPushProxy
	cInfoLogger *log.Logger
	fInfoLogger *log.Logger
	cErrLogger  *log.Logger
	fErrLogger  *log.Logger
}

// NewLogger returns a new instance of the logger
func NewLogger(cfg *ConfigPushProxy) *Logger {
	l := &Logger{
		cfg: cfg,
	}
	if cfg.EnableConsoleLog {
		l.cInfoLogger = log.New(os.Stdout, "INFO: ", log.LstdFlags|log.Lshortfile)
		l.cErrLogger = log.New(os.Stdout, "ERR: ", log.LstdFlags|log.Lshortfile)
	}
	if cfg.EnableFileLog {
		lumber := &lumberjack.Logger{
			Filename: cfg.LogFileLocation,
			MaxSize:  10, // megabytes
			Compress: true,
		}
		l.fInfoLogger = log.New(lumber, "INFO: ", log.LstdFlags|log.Lshortfile)
		l.fErrLogger = log.New(lumber, "ERR: ", log.LstdFlags|log.Lshortfile)
	}
	return l
}

// Following are some helper methods that are called from the application.
// They are divided into categories of Info(f), Error(f), and Panic(f).
// They just forward to their underlying loggers depending on the config.

func (l *Logger) Info(v ...interface{}) {
	if l.cfg.EnableConsoleLog {
		l.cInfoLogger.Println(v...)
	}
	if l.cfg.EnableFileLog {
		l.fInfoLogger.Println(v...)
	}
}

func (l *Logger) Infof(format string, v ...interface{}) {
	if l.cfg.EnableConsoleLog {
		l.cInfoLogger.Printf(format, v...)
	}
	if l.cfg.EnableFileLog {
		l.fInfoLogger.Printf(format, v...)
	}
}

func (l *Logger) Error(v ...interface{}) {
	if l.cfg.EnableConsoleLog {
		l.cErrLogger.Println(v...)
	}
	if l.cfg.EnableFileLog {
		l.fErrLogger.Println(v...)
	}
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.cfg.EnableConsoleLog {
		l.cErrLogger.Printf(format, v...)
	}
	if l.cfg.EnableFileLog {
		l.fErrLogger.Printf(format, v...)
	}
}

func (l *Logger) Panic(v ...interface{}) {
	l.Error(v...)
	panic(fmt.Sprintln(v...))
}

func (l *Logger) Panicf(format string, v ...interface{}) {
	l.Errorf(format, v...)
	panic(fmt.Sprintf(format, v...))
}
