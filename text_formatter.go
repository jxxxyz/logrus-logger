package log

import (
	"bytes"
	"fmt"

	"github.com/sirupsen/logrus"
)

type Formatter struct {
	QuoteEmptyFields bool
	File             string
	Line             int
	TimestampFormat  string
	LogFormat        string
}

const (
	defaultTimestampFormat = "102-150405"
	defaultLogFormat       = "[%lvl%]: %time% - %msg%"
)

func (f *Formatter) levelString(level logrus.Level) string {
	switch level {
	case logrus.DebugLevel:
		return "D"
	case logrus.InfoLevel:
		return "I"
	case logrus.WarnLevel:
		return "W"
	case logrus.ErrorLevel:
		return "E"
	case logrus.FatalLevel:
		return "F"
	case logrus.PanicLevel:
		return "P"
	}

	return "U"
}

func (f *Formatter) appendKeyValue(b *bytes.Buffer, value interface{}) {
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}
	b.WriteString(stringVal)
}

// TODO log format
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}

	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	output := f.LogFormat
	if output == "" {
		output = defaultLogFormat
	}

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}

	// 2018-08-17_14:47:33.118262 D main.go:123 当前时间:2018-08-17 14:47:33.118092779 +0800 CST m=+0.002835666, diff=14h47m33s
	// 80817-144733D main.go:123 当前时间:2018-08-17 14:47:33.118092779 +0800 CST m=+0.002835666, diff=14h47m33s
	f.appendKeyValue(b, entry.Time.Format(timestampFormat) + f.levelString(entry.Level))

	for _, key := range keys {
		f.appendKeyValue(b, entry.Data[key])
	}

	if entry.Message != "" {
		f.appendKeyValue(b, entry.Message)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}
