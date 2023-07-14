package log

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type FilteredLogSink struct {
	logger        logr.LogSink
	info          logr.RuntimeInfo
	excludedNames []string
}

func (l *FilteredLogSink) Init(info logr.RuntimeInfo) {
	l.info = info
}

func (l *FilteredLogSink) Enabled(level int) bool {
	return l.logger.Enabled(level)
}

func (l *FilteredLogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	l.logger.Info(level, msg, keysAndValues...)
}

func (l *FilteredLogSink) Error(err error, msg string, keysAndValues ...interface{}) {
	l.logger.Error(err, msg, keysAndValues...)
}

func (l *FilteredLogSink) WithName(name string) logr.LogSink {
	for _, excludedName := range l.excludedNames {
		if name == excludedName {
			return log.NullLogSink{}
		}
	}

	return l.logger.WithName(name)
}

func (l *FilteredLogSink) WithValues(tags ...interface{}) logr.LogSink {
	return l.logger.WithValues(tags...)
}

func NewFilteredLogSink(logger logr.LogSink, excludedNames []string) *FilteredLogSink {
	l := &FilteredLogSink{
		logger:        logger,
		excludedNames: excludedNames,
	}
	return l
}
