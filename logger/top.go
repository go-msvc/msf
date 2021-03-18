package logger

func New(name string) ILogger {
	return top.New("")
}

var (
	top *logger
)

func init() {
	top = &logger{
		name:   "",
		parent: nil,
		subs:   map[string]ILogger{},
		level:  LevelDebug,
		writer: defaultWriter{},
	}
}

func Logf(level Level, format string, args ...interface{}) {
	top.logf(4, level, format, args...)
}

func Errorf(format string, args ...interface{}) {
	top.logf(4, LevelError, format, args...)
}

func Infof(format string, args ...interface{}) {
	top.logf(4, LevelInfo, format, args...)
}

func Debugf(format string, args ...interface{}) {
	top.logf(4, LevelDebug, format, args...)
}
