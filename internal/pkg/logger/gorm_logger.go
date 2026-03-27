// Package logger provides logging utilities for Ascenda, ported from GPWA.
package logger

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
	"github.com/sirupsen/logrus"
)

// GormLogger routes GORM's SQL output through a logrus.Entry.
// Use NewGormLogger to create one and pass it via gorm.Config{Logger: ...}.
//
// Recommended settings for Ascenda:
//
//	dev:        level=Info,  slowThreshold=200ms, ignoreNotFound=true
//	production: level=Warn,  slowThreshold=500ms, ignoreNotFound=true
type GormLogger struct {
	entry          *logrus.Entry
	level          glogger.LogLevel
	slowThreshold  time.Duration
	ignoreNotFound bool
}

// NewGormLogger returns a glogger.Interface backed by the given logrus Entry.
func NewGormLogger(entry *logrus.Entry, level glogger.LogLevel, slowThreshold time.Duration, ignoreNotFound bool) glogger.Interface {
	return &GormLogger{
		entry:          entry,
		level:          level,
		slowThreshold:  slowThreshold,
		ignoreNotFound: ignoreNotFound,
	}
}

func (l *GormLogger) LogMode(level glogger.LogLevel) glogger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *GormLogger) Info(ctx context.Context, s string, args ...interface{}) {
	if l.level >= glogger.Info {
		l.entry.WithContext(ctx).Infof(s, args...)
	}
}

func (l *GormLogger) Warn(ctx context.Context, s string, args ...interface{}) {
	if l.level >= glogger.Warn {
		l.entry.WithContext(ctx).Warnf(s, args...)
	}
}

func (l *GormLogger) Error(ctx context.Context, s string, args ...interface{}) {
	if l.level >= glogger.Error {
		l.entry.WithContext(ctx).Errorf(s, args...)
	}
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level == glogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := logrus.Fields{
		"elapsed_ms": elapsed.Milliseconds(),
		"rows":       rows,
		"sql":        sql,
		"caller":     utils.FileWithLineNum(),
	}

	entry := l.entry.WithContext(ctx).WithFields(fields)

	switch {
	case err != nil:
		if l.ignoreNotFound && errors.Is(err, gorm.ErrRecordNotFound) {
			if l.level >= glogger.Info {
				entry.Debug("sql not found")
			}
			return
		}
		if l.level >= glogger.Error {
			entry.WithError(err).Error("sql error")
		}

	case l.slowThreshold > 0 && elapsed > l.slowThreshold:
		if l.level >= glogger.Warn {
			entry.WithField("slow_threshold_ms", l.slowThreshold.Milliseconds()).
				Warn("slow sql query")
		}

	default:
		if l.level >= glogger.Info {
			entry.Debug("sql query")
		}
	}
}
