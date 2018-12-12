package log

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	//staticFuncCallDepth = 3 // See 'commonLogger.log' method comments
	loggerFuncCallDepth = 2
)

var (
	workingDir     = "/"
	stackCache     map[uintptr]*logContext
	stackCacheLock sync.RWMutex
)

func init() {
	wd, err := os.Getwd()
	if err == nil {
		workingDir = filepath.ToSlash(wd) + "/"
	}
	stackCache = make(map[uintptr]*logContext)
}

// Represents a normal runtime caller context.
type logContext struct {
	funcName  string
	line      int
	shortPath string
	fullPath  string
	fileName  string
	callTime  time.Time
	custom    interface{}
}

func fileInfo(skip int) string {
	caller, err := extractCallerInfo(skip + loggerFuncCallDepth + 2)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s:%d", caller.fileName, caller.line)
}

func extractCallerInfo(skip int) (*logContext, error) {
	var stack [1]uintptr
	if runtime.Callers(skip+1, stack[:]) != 1 {
		return nil, errors.New("error  during runtime.Callers")
	}
	pc := stack[0]

	// do we have a cache entry?
	stackCacheLock.RLock()
	ctx, ok := stackCache[pc]
	stackCacheLock.RUnlock()
	if ok {
		return ctx, nil
	}

	// look up the details of the given caller
	funcInfo := runtime.FuncForPC(pc)
	if funcInfo == nil {
		return nil, errors.New("error during runtime.FuncForPC")
	}

	var shortPath string
	fullPath, line := funcInfo.FileLine(pc)
	if strings.HasPrefix(fullPath, workingDir) {
		shortPath = fullPath[len(workingDir):]
	} else {
		shortPath = fullPath
	}
	funcName := funcInfo.Name()
	if strings.HasPrefix(funcName, workingDir) {
		funcName = funcName[len(workingDir):]
	}

	ctx = &logContext{
		funcName:  funcName,
		line:      line,
		shortPath: shortPath,
		fullPath:  fullPath,
		fileName:  filepath.Base(fullPath),
	}

	// save the details in the cache; note that it's possible we might
	// have written an entry into the map in between the test above and
	// this section, but the behaviour is still correct
	stackCacheLock.Lock()
	stackCache[pc] = ctx
	stackCacheLock.Unlock()
	return ctx, nil
}
