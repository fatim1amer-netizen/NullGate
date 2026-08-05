package waf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel ranks severities for level-based filtering.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func parseLevel(s string) LogLevel {
	switch s {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Logger is a leveled, structured logger with size-based file rotation and an
// in-memory ring buffer of recent security events for the admin dashboard.
// Keeping the ring buffer in the logger means every subsystem's warnings are
// visible in the UI without a separate event bus.
type Logger struct {
	cfg   LoggingConfig
	level LogLevel
	mu    sync.Mutex
	file  *os.File
	size  int64

	events     []Event // ring buffer of recent notable events
	eventsHead int
	eventsCap  int
	eventsLen  int
}

// Event is a structured security event retained for the dashboard.
type Event struct {
	Time    time.Time              `json:"time"`
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// NewLogger constructs a logger, opening the log file if file output is set.
func NewLogger(cfg LoggingConfig) *Logger {
	l := &Logger{
		cfg:       cfg,
		level:     parseLevel(cfg.Level),
		eventsCap: 500,
	}
	l.events = make([]Event, l.eventsCap)

	if cfg.Output == "file" && cfg.FilePath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0o755); err == nil {
			if f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
				l.file = f
				if info, err := f.Stat(); err == nil {
					l.size = info.Size()
				}
			}
		}
	}
	return l
}

// Debug/Info/Warn/Error emit a structured log line at the given level.
func (l *Logger) Debug(msg string, fields map[string]interface{}) { l.log(LevelDebug, msg, fields) }
func (l *Logger) Info(msg string, fields map[string]interface{})  { l.log(LevelInfo, msg, fields) }
func (l *Logger) Warn(msg string, fields map[string]interface{})  { l.log(LevelWarn, msg, fields) }
func (l *Logger) Error(msg string, fields map[string]interface{}) { l.log(LevelError, msg, fields) }

func (l *Logger) log(level LogLevel, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	ev := Event{
		Time:    time.Now(),
		Level:   level.String(),
		Message: msg,
		Fields:  fields,
	}

	// Retain warnings and errors in the dashboard ring buffer.
	if level >= LevelWarn {
		l.pushEvent(ev)
	}

	line := l.format(ev)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.rotateIfNeeded()
		n, _ := l.file.WriteString(line + "\n")
		l.size += int64(n)
	} else {
		fmt.Fprintln(os.Stdout, line)
	}
}

func (l *Logger) format(ev Event) string {
	if l.cfg.Format == "json" {
		entry := map[string]interface{}{
			"timestamp": ev.Time.Format(time.RFC3339Nano),
			"level":     ev.Level,
			"message":   ev.Message,
		}
		for k, v := range ev.Fields {
			entry[k] = v
		}
		b, err := json.Marshal(entry)
		if err != nil {
			return fmt.Sprintf(`{"level":"ERROR","message":"log marshal failed: %v"}`, err)
		}
		return string(b)
	}
	return fmt.Sprintf("%s [%s] %s %v", ev.Time.Format(time.RFC3339), ev.Level, ev.Message, ev.Fields)
}

// pushEvent appends to the ring buffer under its own lock.
func (l *Logger) pushEvent(ev Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events[l.eventsHead] = ev
	l.eventsHead = (l.eventsHead + 1) % l.eventsCap
	if l.eventsLen < l.eventsCap {
		l.eventsLen++
	}
}

// RecentEvents returns up to `n` most-recent events, newest first.
func (l *Logger) RecentEvents(n int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n > l.eventsLen {
		n = l.eventsLen
	}
	out := make([]Event, 0, n)
	for i := 0; i < n; i++ {
		idx := (l.eventsHead - 1 - i + l.eventsCap*2) % l.eventsCap
		out = append(out, l.events[idx])
	}
	return out
}

// rotateIfNeeded rolls the log file over when it exceeds MaxSizeMB, keeping up
// to MaxBackups numbered copies. Caller must hold l.mu.
func (l *Logger) rotateIfNeeded() {
	if l.cfg.MaxSizeMB <= 0 || l.file == nil {
		return
	}
	maxBytes := int64(l.cfg.MaxSizeMB) * 1024 * 1024
	if l.size < maxBytes {
		return
	}

	l.file.Close()
	base := l.cfg.FilePath

	// Shift existing backups: .(*n-1) -> .n, dropping the oldest.
	backups := l.cfg.MaxBackups
	if backups < 1 {
		backups = 1
	}
	oldest := fmt.Sprintf("%s.%d", base, backups)
	os.Remove(oldest)
	for i := backups - 1; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", base, i), fmt.Sprintf("%s.%d", base, i+1))
	}
	os.Rename(base, base+".1")

	if f, err := os.OpenFile(base, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		l.file = f
		l.size = 0
	}
}

// Close flushes and closes the underlying file, if any.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}
