package log

import (
	"github.com/sirupsen/logrus"
	"time"
	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"strings"
	"fmt"
	"bytes"
	"os"
	"io/ioutil"
)

type MyLogger struct {
	logger    *logrus.Logger
	skip      int
	logDir    string
	logFile   string
	clockTime rotatelogs.Clock
	// 最长保留时间
	maxAge time.Duration
	// 翻转时间
	rotationTime time.Duration
	// 内存缓存大小  KB
	cacheSize int
	// 强制写入间隔时间，秒,距离上次写入时间到达该值，即使缓存未达到cacheSize
	// 也将缓存写入文件
	forceFireTimeLimit int64
	// log formatter
	logFormatter Formatter
	// fire log channel
	logChan chan CacheChanMsg
	// observer channel
	obsChan chan logrus.Level
	// force fire log check timer
	ticker *time.Ticker

	// 各个级别log上次强制fire时间
	lastFireTime map[logrus.Level]time.Time
	// log cache buffer map
	levelBuffer map[logrus.Level]*bytes.Buffer

	observerGoRoutineId int
	signal              chan int
}

// Fields wraps logrus.Fields, which is a map[string]interface{}
type Fields logrus.Fields

type NoneFormatter struct {
}

func (nf *NoneFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message), nil
}

type rotateClock struct {}

func (r rotateClock) Now() time.Time {
	parse, _ := time.Parse("2006-01-02 15:04:05", time.Now().Format("2006-01-02 15:04:05"))
	return parse
}

var rotateClockI rotateClock

// NewMyLogger Return Default MyLogger struct
func NewMyLogger(level logrus.Level, path string, stdout bool) *MyLogger {
	var ml = &MyLogger{
		lastFireTime: map[logrus.Level]time.Time{
			logrus.ErrorLevel: time.Now(),
			logrus.WarnLevel:  time.Now(),
			logrus.InfoLevel:  time.Now(),
			logrus.DebugLevel: time.Now(),
		},
		levelBuffer: map[logrus.Level]*bytes.Buffer{
			logrus.PanicLevel: new(bytes.Buffer),
			logrus.FatalLevel: new(bytes.Buffer),
			logrus.ErrorLevel: new(bytes.Buffer),
			logrus.WarnLevel:  new(bytes.Buffer),
			logrus.InfoLevel:  new(bytes.Buffer),
			logrus.DebugLevel: new(bytes.Buffer),
		},
		logger:             logrus.New(),
		logChan:            make(chan CacheChanMsg, 10000),
		obsChan:            make(chan logrus.Level, 5),
		forceFireTimeLimit: 60,
		//ticker:             time.NewTicker(time.Second * 10),
		signal: make(chan int, 5),
	}
	if stdout {
		ml.logger.Out = os.Stderr
	} else {
		ml.logger.Out = ioutil.Discard
	}
	ml.logger.Level = level
	ml.logger.Formatter = &NoneFormatter{}
	ml.clockTime = rotateClockI
	ml.maxAge = time.Duration(24*365) * time.Hour        // save at most 365 days
	ml.rotationTime = time.Duration(86400) * time.Second // rotate every day

	if strings.HasSuffix(path, ".log") {
		path = path[:strings.LastIndex(path, ".log")]
	}
	ml.SetHooks(path)

	go readyForRead(ml)
	return ml
}

type nilWriter struct {}

func (nilWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}

func (ml *MyLogger) SetSkip(skip int) {
	ml.skip = skip
}

func (ml *MyLogger) SetLogLevel(level logrus.Level) {
	ml.logger.Level = level
}

func (ml *MyLogger) SetLogFormatter(formatter logrus.Formatter) {
	ml.logger.Formatter = formatter
}

func (ml *MyLogger) SetLogMaxAge(maxAge time.Duration) {
	ml.maxAge = maxAge
}

func (ml *MyLogger) SetRotationTime(rotationTime time.Duration) {
	ml.rotationTime = rotationTime
}

func (ml *MyLogger) SetClockTime(clockTime string) {
	switch clockTime {
	case UTCTime:
		ml.clockTime = rotatelogs.UTC
	case LocalTime:
		ml.clockTime = rotatelogs.Local
	}
}

func (ml *MyLogger) SetHooks(path string) {
	errorWriter, _ := rotatelogs.New(
		path+".error.%Y-%m-%d.log",
		rotatelogs.WithLinkName(path+".error.log"),
		rotatelogs.WithClock(ml.clockTime),
		rotatelogs.WithMaxAge(ml.maxAge),
		rotatelogs.WithRotationTime(ml.rotationTime),
	)
	infoWriter, _ := rotatelogs.New(
		path+".info.%Y-%m-%d.log",
		rotatelogs.WithLinkName(path+".info.log"),
		rotatelogs.WithClock(ml.clockTime),
		rotatelogs.WithMaxAge(ml.maxAge),
		rotatelogs.WithRotationTime(ml.rotationTime),
	)

	debugWriter, _ := rotatelogs.New(
		path+".debug.%Y-%m-%d.log",
		rotatelogs.WithLinkName(path+".debug.log"),
		rotatelogs.WithClock(ml.clockTime),
		rotatelogs.WithMaxAge(ml.maxAge),
		rotatelogs.WithRotationTime(ml.rotationTime),
	)

	ml.logger.Hooks.Add(lfshook.NewHook(
		lfshook.WriterMap{
			logrus.PanicLevel: errorWriter,
			logrus.FatalLevel: errorWriter,
			logrus.ErrorLevel: errorWriter,
			logrus.WarnLevel:  infoWriter,
			logrus.InfoLevel:  infoWriter,
			logrus.DebugLevel: debugWriter,
		},
		ml.logger.Formatter,
	))
}

func (ml *MyLogger) SetCacheSize(size int) {
	initForceFireChecker(ml, size, 0, 0)
}

func (ml *MyLogger) SetForceFireTimeLimit(t int64) {
	initForceFireChecker(ml, 0, t, 0)
}

func (ml *MyLogger) SetForceFireCheckInterval(t time.Duration) {
	initForceFireChecker(ml, 0, 0, t)
}

func (ml *MyLogger) FlushCache() {
	for level, _ := range ml.lastFireTime {
		buffer := ml.levelBuffer[level]
		logFunc(ml.logger.WithFields(logrus.Fields{}), level)(buffer.String())
		buffer.Reset()
		ml.lastFireTime[level] = time.Now()
		//ml.obsChan <- level
	}
}

func initForceFireChecker(ml *MyLogger, cacheSize int, timeLimit int64, interval time.Duration) {
	if cacheSize > 0 {
		ml.cacheSize = cacheSize
	}
	if timeLimit > 0 {
		ml.forceFireTimeLimit = timeLimit
	}
	if ml.ticker == nil {
		// 此时没有监视线程在运行
		if interval <= 0 {
			interval = time.Second * 30
		}
		if ml.cacheSize > 0 {
			ml.ticker = time.NewTicker(interval)
			go observerLog(ml, ml.observerGoRoutineId)
		}
	} else {
		// 此时有监视线程在跑
		if interval > 0 {
			ml.ticker.Stop()
			ml.signal <- ml.observerGoRoutineId
			ml.ticker = time.NewTicker(interval)
			ml.observerGoRoutineId ++
			go observerLog(ml, ml.observerGoRoutineId)
		}
	}
}

// Debug logs a message at level Debug on the standard ml.logger.
func (ml *MyLogger) Debug(args ...interface{}) {
	logCache(logrus.DebugLevel, ml, nil, args...)
}

// Info logs a message at level Info on the standard ml.logger.
func (ml *MyLogger) Info(args ...interface{}) {
	logCache(logrus.InfoLevel, ml, nil, args...)
}

// Warn logs a message at level Warn on the standard ml.logger.
func (ml *MyLogger) Warn(args ...interface{}) {
	logCache(logrus.WarnLevel, ml, nil, args...)
}

// Error logs a message at level Error on the standard ml.logger.
func (ml *MyLogger) Error(args ...interface{}) {
	logCache(logrus.ErrorLevel, ml, nil, args...)
}

// Fatal logs a message at level Fatal on the standard ml.logger.
func (ml *MyLogger) Fatal(args ...interface{}) {
	logCache(logrus.FatalLevel, ml, nil, args...)
}

// Panic logs a message at level Panic on the standard ml.logger.
func (ml *MyLogger) Panic(args ...interface{}) {
	logCache(logrus.PanicLevel, ml, nil, args...)
}

// Debug logs a message at level Debug on the standard ml.logger.
func (ml *MyLogger) Debugf(format string, args ...interface{}) {
	logCachef(logrus.DebugLevel, ml, format, args...)
}

// Info logs a message at level Info on the standard ml.logger.
func (ml *MyLogger) Infof(format string, args ...interface{}) {
	logCachef(logrus.InfoLevel, ml, format, args...)
}

// Warn logs a message at level Warn on the standard ml.logger.
func (ml *MyLogger) Warnf(format string, args ...interface{}) {
	logCachef(logrus.WarnLevel, ml, format, args...)
}

// Error logs a message at level Error on the standard ml.logger.
func (ml *MyLogger) Errorf(format string, args ...interface{}) {
	logCachef(logrus.ErrorLevel, ml, format, args...)
}

// Fatal logs a message at level Fatal on the standard ml.logger.
func (ml *MyLogger) Fatalf(format string, args ...interface{}) {
	logCachef(logrus.FatalLevel, ml, format, args...)
}

// Panic logs a message at level Panic on the standard ml.logger.
func (ml *MyLogger) Panicf(format string, args ...interface{}) {
	logCachef(logrus.PanicLevel, ml, format, args...)
}

// Debug logs a message with fields at level Debug on the standard ml.logger.
func (ml *MyLogger) InfoWithFields(l interface{}, f Fields) {
	logCache(logrus.InfoLevel, ml, f, l)
}

// Debug logs a message with fields at level Debug on the standard ml.logger.
func (ml *MyLogger) DebugWithFields(l interface{}, f Fields) {
	logCache(logrus.DebugLevel, ml, f, l)
}

// Debug logs a message with fields at level Debug on the standard ml.logger.
func (ml *MyLogger) WarnWithFields(l interface{}, f Fields) {
	logCache(logrus.WarnLevel, ml, f, l)
}

// Debug logs a message with fields at level Debug on the standard ml.logger.
func (ml *MyLogger) ErrorWithFields(l interface{}, f Fields) {
	logCache(logrus.ErrorLevel, ml, f, l)
}

// Debug logs a message with fields at level Debug on the standard ml.logger.
func (ml *MyLogger) FatalWithFields(l interface{}, f Fields) {
	logCache(logrus.FatalLevel, ml, f, l)
}

// Debug logs a message with fields at level Debug on the standard ml.logger.
func (ml *MyLogger) PanicWithFields(l interface{}, f Fields) {
	logCache(logrus.PanicLevel, ml, f, l)
}

func observerLog(ml *MyLogger, id int) {
	//log.Printf("------启动 go observer id=%v", id)
	if ml.ticker == nil {
		return
	}
loop:
	for {
		select {
		case <-ml.ticker.C:
			for level, timeC := range ml.lastFireTime {
				diff := time.Now().Unix() - timeC.Unix()
				//log.Printf("定时器到时，level=%v, diff=%v, logForce=%v", level, diff, ml.forceFireTimeLimit)
				if diff >= ml.forceFireTimeLimit {
					ml.obsChan <- level
				}
			}
		case goId := <-ml.signal:
			//log.Printf("接受停止信号：%v, %v", goId, id)
			if goId == id {
				break loop
			}
		}
	}
}

func readyForRead(ml *MyLogger) {
	for {
		select {
		case msg := <-ml.logChan:
			buffer := ml.levelBuffer[msg.Level]
			buffer.Write(msg.FormattedMsg)

			//log.Printf("存入缓存，buffer len=%v, cache size=%v", buffer.Len(), ml.cacheSize)
			if buffer.Len() > ml.cacheSize {
				//log.Printf("写入文件，level=%v, size=%v", msg.Level, buffer.Len())
				logFunc(msg.Entry, msg.Level)(buffer.String())
				buffer.Reset()
				ml.lastFireTime[msg.Level] = time.Now()
			}
		case level := <-ml.obsChan:
			//log.Printf("强制写入文件，level=%v", level)
			buffer := ml.levelBuffer[level]
			logFunc(ml.logger.WithFields(logrus.Fields{}), level)(buffer.String())
			buffer.Reset()
			ml.lastFireTime[level] = time.Now()
			time.Sleep(time.Millisecond * 200)

		}
	}
}

type CacheChanMsg struct {
	Level        logrus.Level
	FormattedMsg []byte
	Entry        *logrus.Entry
}

// log cache for printf
func logCachef(level logrus.Level, ml *MyLogger, format string, args ...interface{}) {
	logMsg := fmt.Sprintf(format, args...)
	doLog(4, level, ml, nil, logMsg)
}

// log cache for print
func logCache(level logrus.Level, ml *MyLogger, f Fields, args ...interface{}) {
	doLog(4, level, ml, f, args...)
}

func doLog(skip int, level logrus.Level, ml *MyLogger, f Fields, args ...interface{}) {
	if ml.logger.Level >= level {
		fs := logrus.Fields{}
		if f != nil {
			fs = logrus.Fields(f)
		}
		msg := fmt.Sprint(args...)

		entry := ml.logger.WithFields(fs)

		if caller, err := extractCallerInfo(ml.skip + skip + 1); err != nil {
			entry.Data["file"] = ""
		} else {
			entry.Data["file"] = fmt.Sprintf("%s:%d", caller.fileName, caller.line)
		}
		entry.Time = time.Now()
		entry.Level = level
		entry.Message = msg

		formatMsg, _ := ml.logFormatter.Format(entry)

		ml.logChan <- CacheChanMsg{Level: level, FormattedMsg: formatMsg, Entry: entry}
	}
}

func logFunc(entry *logrus.Entry, level logrus.Level) func(args ...interface{}) {
	switch level {
	case logrus.PanicLevel:
		return entry.Panic
	case logrus.FatalLevel:
		return entry.Fatal
	case logrus.ErrorLevel:
		return entry.Error
	case logrus.WarnLevel:
		return entry.Warn
	case logrus.InfoLevel:
		return entry.Info
	case logrus.DebugLevel:
		return entry.Debug
	}
	return func(args ...interface{}) {
		entry.Errorf("Error while get entry level log")
	}
}
