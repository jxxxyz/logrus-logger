package log

import (
	"time"

	"github.com/sirupsen/logrus"
	"os"
	"io/ioutil"
)

const (
	UTCTime   = "utc"
	LocalTime = "local"
)

var DefaultMyLogger *MyLogger

func InitDefaultMyLogger(level logrus.Level, path string) {
	DefaultMyLogger = NewMyLogger(level, path, false)
}

func StdoutToggle(open bool) {
	if open {
		DefaultMyLogger.logger.Out = os.Stderr
	} else {
		DefaultMyLogger.logger.Out = ioutil.Discard
	}
}

func SetSkip(skip int) {
	DefaultMyLogger.SetSkip(skip)
}

func SetLogLevel(level logrus.Level) {
	DefaultMyLogger.logger.Level = level
}

func SetLogFormatter(formatter logrus.Formatter) {
	DefaultMyLogger.logger.Formatter = formatter
}

func SetHooks(path string) {
	DefaultMyLogger.SetHooks(path)
}

func SetLogMaxAge(maxAge time.Duration) {
	DefaultMyLogger.maxAge = maxAge
}

func SetRotationTime(rotationTime time.Duration) {
	DefaultMyLogger.rotationTime = rotationTime
}

func SetClockTime(clockTime string) {
	DefaultMyLogger.SetClockTime(clockTime)
}

func Debug(args ...interface{}) {
	DefaultMyLogger.Debug(args...)
}

func Info(args ...interface{}) {
	DefaultMyLogger.Info(args...)
}

func Warn(args ...interface{}) {
	DefaultMyLogger.Warn(args...)
}

func Error(args ...interface{}) {
	DefaultMyLogger.Error(args...)
}

func Fatal(args ...interface{}) {
	DefaultMyLogger.Fatal(args...)
}

func Panic(args ...interface{}) {
	DefaultMyLogger.Panic(args...)
}

func Debugf(format string, args ...interface{}) {
	DefaultMyLogger.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	DefaultMyLogger.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	DefaultMyLogger.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	DefaultMyLogger.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	DefaultMyLogger.Fatalf(format, args...)
}

func Panicf(format string, args ...interface{}) {
	DefaultMyLogger.Panicf(format, args...)
}

func SetCacheSize(size int) {
	DefaultMyLogger.SetCacheSize(size)
}

// 强制写入文件间隔，秒，默认1分钟。即当上次写入文件时间与当前时间达到该间隔时，强制将缓存写入文件
func SetForceFireTimeLimit(t int64) {
	DefaultMyLogger.SetForceFireTimeLimit(t)
}

// 监视器监视间隔，默认1分钟。即每1分钟进行一次强制写入文件检查。
func SetForceFireCheckInterval(t time.Duration) {
	DefaultMyLogger.SetForceFireCheckInterval(t)
}

// 将缓存的log刷到文件中
func FlushCache() {
	DefaultMyLogger.FlushCache()
}
