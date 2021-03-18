package logger

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ILogger interface {
	New(name string) ILogger
	Name() string

	Level() Level
	WithLevel(Level) ILogger
	SetLevel(Level)

	Logf(level Level, format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

type logger struct {
	name string

	sync.Mutex
	parent ILogger
	subs   map[string]ILogger

	level  Level
	writer IWriter
}

const namePattern = `[a-zA-Z0-9]([a-zA-Z0-9.-_:][a-zA-Z0-9])*`

var nameRegex = regexp.MustCompile("^" + namePattern + "$")

func (l *logger) New(name string) ILogger {
	l.Lock()
	defer l.Unlock()
	if !nameRegex.MatchString(name) {
		name = "invalid-logger-name"
	}
	if existingLogger, found := l.subs[name]; found {
		return existingLogger
	}
	newLogger := &logger{
		name:   name,
		parent: l,
		subs:   map[string]ILogger{},
		level:  l.level,
		writer: l.writer,
	}
	l.subs[name] = newLogger
	return newLogger
} //logger.New()

func (l *logger) Parent() ILogger { return l.parent }

func (l *logger) Name() string { return l.name }

func (l *logger) Level() Level { return l.level }

func (l *logger) WithLevel(level Level) ILogger {
	l.SetLevel(level)
	return l
}

func (l *logger) SetLevel(newLevel Level) {
	l.Lock()
	defer l.Unlock()
	for _, sub := range l.subs {
		sub.SetLevel(newLevel)
	}
	l.level = newLevel
}

func (l *logger) log(depth int, level Level, msg string) {
	l.writer.Write(
		Record{
			Caller:    GetCaller(depth),
			Timestamp: time.Now(),
			Logger:    l,
			Level:     level,
			Message:   msg,
		},
	)
}

func (l *logger) logf(depth int, level Level, format string, args ...interface{}) {
	if l.level >= level {
		l.log(depth, level, strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", ";"))
	}
}

//public functions
func (l *logger) Logf(level Level, format string, args ...interface{}) {
	l.logf(4, level, format, args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.logf(4, LevelError, format, args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.logf(4, LevelInfo, format, args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	l.logf(4, LevelDebug, format, args...)
}
