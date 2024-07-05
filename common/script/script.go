package script

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type Event string

const (
	EventBeforeStart Event = "before_start"
	EventAfterStart  Event = "after_start"
	EventBeforeClose Event = "before_close"
	EventAfterClose  Event = "after_close"
	EventStartFailed Event = "start_failed"
)

var allAcceptEventMap = map[Event]struct{}{
	EventBeforeStart: {},
	EventAfterStart:  {},
	EventBeforeClose: {},
	EventAfterClose:  {},
	EventStartFailed: {},
}

type Script struct {
	ctx             context.Context
	logger          log.ContextLogger
	command         string
	args            []string
	envs            map[string]string
	currentDir      string
	asService       bool
	acceptEventMap  map[Event]struct{}
	stdoutLogWriter *logWriter
	stderrLogWriter *logWriter
}

func NewScript(ctx context.Context, logger log.ContextLogger, options option.ScriptOptions) (*Script, error) {
	s := &Script{
		ctx:    ctx,
		logger: logger,
	}
	if options.Command == "" {
		return nil, E.New("missing command")
	}
	s.command = options.Command
	s.args = options.Args
	s.envs = options.Envs
	s.currentDir = options.CurrentDir
	if len(options.AcceptEvents) > 0 {
		s.acceptEventMap = make(map[Event]struct{}, len(options.AcceptEvents))
		for _, event := range options.AcceptEvents {
			_, loaded := allAcceptEventMap[Event(event)]
			if !loaded {
				return nil, E.New("unknown event: ", event)
			}
			_, loaded = s.acceptEventMap[Event(event)]
			if loaded {
				return nil, E.New("duplicate event: ", event)
			}
			s.acceptEventMap[Event(event)] = struct{}{}
		}
	} else {
		s.acceptEventMap = allAcceptEventMap
	}
	s.asService = options.AsService
	if options.AsService {
		_, loaded1 := s.acceptEventMap[EventBeforeStart]
		_, loaded2 := s.acceptEventMap[EventAfterStart]
		if loaded1 || loaded2 {
			return nil, E.New("before_start and after_start events are not allowed when as_service is true")
		}
	}
	var err error
	s.stdoutLogWriter, err = parseLogLevelToWriter(logger, "parse stdout log level", options.StdoutLogLevel)
	if err != nil {
		return nil, err
	}
	s.stderrLogWriter, err = parseLogLevelToWriter(logger, "parse stderr log level", options.StderrLogLevel)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func parseLogLevelToWriter(logger log.ContextLogger, errMsg string, level string) (*logWriter, error) {
	if level == "" {
		return nil, nil
	}
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		return nil, E.Cause(err, errMsg)
	}
	var logFunc func(...any)
	switch logLevel {
	case log.LevelPanic:
		logFunc = logger.Panic
	case log.LevelFatal:
		logFunc = logger.Fatal
	case log.LevelError:
		logFunc = logger.Error
	case log.LevelWarn:
		logFunc = logger.Warn
	case log.LevelInfo:
		logFunc = logger.Info
	case log.LevelDebug:
		logFunc = logger.Debug
	case log.LevelTrace:
		logFunc = logger.Trace
	default:
		panic("unreachable")
	}
	writer := &logWriter{
		logFunc: logFunc,
	}
	return writer, nil
}

func (s *Script) buildCommand(ctx context.Context, event Event) *exec.Cmd {
	if ctx == nil {
		ctx = s.ctx
	}
	cmd := exec.CommandContext(ctx, s.command, s.args...)
	cmd.Path = s.currentDir
	if s.envs != nil && len(s.envs) > 0 {
		osEnvs := os.Environ()
		cmd.Env = make([]string, 0, len(osEnvs)+len(s.envs)+1)
		cmd.Env = append(cmd.Env, osEnvs...)
		for k, v := range s.envs {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	} else {
		osEnvs := os.Environ()
		cmd.Env = make([]string, 0, len(osEnvs)+1)
		cmd.Env = append(cmd.Env, osEnvs...)
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("SING_EVENT=%s", event))
	cmd.Stdout = s.stdoutLogWriter
	cmd.Stderr = s.stderrLogWriter
	return cmd
}

func (s *Script) CallWithEvent(ctx context.Context, event Event) error {
	_, loaded := s.acceptEventMap[event]
	if !loaded {
		return nil
	}
	if ctx == nil {
		ctx = s.ctx
	}
	cmd := s.buildCommand(ctx, event)
	cmdString := cmd.String()
	s.logger.Debug("call script: [", cmdString, "], event: ", event)
	var err error
	if s.asService {
		err = cmd.Start()
		if err != nil {
			err = E.Cause(err, "start command: [", cmdString, "]")
		} else {
			go cmd.Wait()
		}
	} else {
		err = cmd.Run()
	}
	if err != nil {
		s.logger.Error("call script: [", cmdString, "] failed: ", err)
	} else {
		s.logger.Debug("call script: [", cmdString, "] success")
	}
	return err
}

type logWriter struct {
	logFunc func(...any)
}

func (w *logWriter) Write(b []byte) (int, error) {
	s := strings.Replace(string(b), "\n", "", -1)
	w.logFunc(s)
	return len(b), nil
}
