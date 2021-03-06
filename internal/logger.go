// Copyright 2015 - 2017 Ka-Hing Cheung
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"fmt"
	glog "log"
	"log/syslog"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/kdar/logrus-cloudwatchlogs"
	"github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
)

var mu sync.Mutex
var loggers = make(map[string]*LogHandle)

var log = GetLogger("main")
var fuseLog = GetLogger("fuse")

var syslogHook *logrus_syslog.SyslogHook

func InitLoggers(logToSyslog bool, cwRegion, cwGroup, cwName string) {
	if cwRegion != "" && cwGroup != "" && cwName != "" {
		cfg := aws.NewConfig().WithRegion(cwRegion)
		hook, err := logrus_cloudwatchlogs.NewHook(cwGroup, cwName, cfg)
		if err != nil {
			log.Println(fmt.Sprintf("Could not create cloudwatch log: %s", err.Error()))
		} else {
			for _, l := range loggers {
				l.Hooks.Add(hook)
			}
		}
	}

	if logToSyslog {
		hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_DEBUG, "")
		if err != nil {
			log.Println(fmt.Sprintf("Unable to connect to local syslog daemon: %s", err.Error()))
		} else {
			for _, l := range loggers {
				l.Hooks.Add(hook)
			}
		}
	}
}

type LogHandle struct {
	logrus.Logger

	name string
	Lvl  *logrus.Level
}

func (l *LogHandle) Format(e *logrus.Entry) ([]byte, error) {
	// Mon Jan 2 15:04:05 -0700 MST 2006
	timestamp := ""
	lvl := e.Level
	if l.Lvl != nil {
		lvl = *l.Lvl
	}

	if syslogHook == nil {
		const timeFormat = "2006/01/02 15:04:05.000000"

		timestamp = e.Time.Format(timeFormat) + " "
	}

	str := fmt.Sprintf("%v%v.%v %v",
		timestamp,
		l.name,
		strings.ToUpper(lvl.String()),
		e.Message)

	if len(e.Data) != 0 {
		str += " " + fmt.Sprint(e.Data)
	}

	str += "\n"
	return []byte(str), nil
}

// for aws.Logger
func (l *LogHandle) Log(args ...interface{}) {
	l.Debugln(args...)
}

func NewLogger(name string) *LogHandle {
	l := &LogHandle{name: name}
	l.Out = os.Stderr
	l.Formatter = l
	l.Level = logrus.InfoLevel
	l.Hooks = make(logrus.LevelHooks)
	if syslogHook != nil {
		l.Hooks.Add(syslogHook)
	}
	return l
}

func GetLogger(name string) *LogHandle {
	mu.Lock()
	defer mu.Unlock()

	if logger, ok := loggers[name]; ok {
		return logger
	} else {
		logger := NewLogger(name)
		loggers[name] = logger
		return logger
	}
}

func GetStdLogger(l *LogHandle, lvl logrus.Level) *glog.Logger {
	mu.Lock()
	defer mu.Unlock()

	w := l.Writer()
	l.Formatter.(*LogHandle).Lvl = &lvl
	l.Level = lvl
	return glog.New(w, "", 0)
}
