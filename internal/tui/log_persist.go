package tui

// log_persist.go — 异步将日志行追加写入 ~/.local/share/ops-agent/logs/tui.log。

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const (
	maxLogEntries     = 200
	logPersistBufSize = 256
)

type logEntry struct {
	text string
}

var (
	logPersistOnce sync.Once
	logPersistCh   chan string
)

func startLogPersistWorker() {
	logPersistOnce.Do(func() {
		logPersistCh = make(chan string, logPersistBufSize)
		go func() {
			path := config.LogFilePath()
			for line := range logPersistCh {
				_ = appendLogFile(path, line)
			}
		}()
	})
}

func appendLogFile(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	ts := time.Now().Format("15:04:05")
	_, err = fmt.Fprintf(f, "%s %s\n", ts, text)
	return err
}

func queueLogPersist(text string) {
	startLogPersistWorker()
	select {
	case logPersistCh <- text:
	default:
		recordPersistDrop()
	}
}

func trimLogEntries(entries []logEntry, max int) []logEntry {
	if len(entries) <= max {
		return entries
	}
	return append([]logEntry(nil), entries[len(entries)-max:]...)
}
