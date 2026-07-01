package tui

// diag.go — TUI 性能诊断：消息计数、慢 Update/View 日志，写入 tui-diag.log。
import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const (
	diagInterval       = 30 * time.Second
	diagSlowUpdate     = 50 * time.Millisecond
	diagSlowView       = 50 * time.Millisecond
	diagSlowViewLog    = 200 * time.Millisecond
	diagSlowBridgeFwd  = 100 * time.Millisecond
	diagQueueWarn      = 128
	diagBufSize        = 512
)

type diagTickMsg struct{}

var (
	diagOnce sync.Once
	diagCh   chan string

	diagUpdateTotal atomic.Int64
	diagUpdateSlow  atomic.Int64
	diagViewSlow    atomic.Int64
	diagPersistDrop atomic.Int64

	diagViewSlowLogMu sync.Mutex
	diagLastViewLog   time.Time

	diagMsgHistMu sync.Mutex
	diagMsgHist   map[string]int64
)

func startDiagWorker() {
	diagOnce.Do(func() {
		diagCh = make(chan string, diagBufSize)
		go func() {
			path := config.DiagLogFilePath()
			for line := range diagCh {
				_ = appendDiagFile(path, line)
			}
		}()
	})
}

func appendDiagFile(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	_, err = fmt.Fprintf(f, "%s [DIAG] %s\n", ts, text)
	return err
}

func diagEnabled() bool {
	return os.Getenv("OPS_AGENT_DIAG") != "0"
}

func queueDiag(text string) {
	if !diagEnabled() || text == "" {
		return
	}
	startDiagWorker()
	select {
	case diagCh <- text:
	default:
	}
}

func diagMsgType(msg tea.Msg) string {
	switch m := msg.(type) {
	case LogLineMsg:
		return "LogLine"
	case refreshTickMsg:
		return "refreshTick"
	case invStatusMsg:
		return "invStatus"
	case workerTickMsg:
		return "workerTick"
	case libTestTickMsg:
		return "libTestTick"
	case spinnerTickMsg:
		return "spinnerTick"
	case workerDoneMsg:
		return "workerDone"
	case libTestDoneMsg:
		return "libTestDone"
	case commandDoneMsg:
		return "commandDone"
	case tea.KeyMsg:
		return "Key:" + m.String()
	case tea.WindowSizeMsg:
		return "WindowSize"
	case tea.MouseMsg:
		return "Mouse"
	case diagTickMsg:
		return "diagTick"
	case startupDoneMsg:
		return "startupDone"
	case webhookStartedMsg:
		return "webhookStarted"
	default:
		t := fmt.Sprintf("%T", msg)
		if i := strings.LastIndex(t, "."); i >= 0 {
			return t[i+1:]
		}
		return t
	}
}

func recordUpdate(msgType string, d time.Duration) {
	if !diagEnabled() {
		return
	}
	diagUpdateTotal.Add(1)
	bumpMsgType(msgType)
	if d < diagSlowUpdate {
		return
	}
	diagUpdateSlow.Add(1)
	queueDiag(fmt.Sprintf("update slow type=%s took=%s", msgType, d.Round(time.Millisecond)))
}

func bumpMsgType(typ string) {
	if typ == "" {
		return
	}
	diagMsgHistMu.Lock()
	if diagMsgHist == nil {
		diagMsgHist = make(map[string]int64)
	}
	diagMsgHist[typ]++
	diagMsgHistMu.Unlock()
}

func formatMsgHistTop(n int) string {
	diagMsgHistMu.Lock()
	defer diagMsgHistMu.Unlock()
	if len(diagMsgHist) == 0 {
		return ""
	}
	type kv struct {
		k string
		v int64
	}
	items := make([]kv, 0, len(diagMsgHist))
	for k, v := range diagMsgHist {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].v == items[j].v {
			return items[i].k < items[j].k
		}
		return items[i].v > items[j].v
	})
	if n > len(items) {
		n = len(items)
	}
	var b strings.Builder
	b.WriteString(" msg_top=")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(items[i].k)
		b.WriteByte(':')
		b.WriteString(itoa64(items[i].v))
	}
	return b.String()
}

func recordViewSlow(d time.Duration, logN, outN int) {
	if !diagEnabled() || d < diagSlowView {
		return
	}
	diagViewSlow.Add(1)
	if d < diagSlowViewLog {
		return
	}
	diagViewSlowLogMu.Lock()
	if time.Since(diagLastViewLog) < 10*time.Second {
		diagViewSlowLogMu.Unlock()
		return
	}
	diagLastViewLog = time.Now()
	diagViewSlowLogMu.Unlock()
	queueDiag(fmt.Sprintf("view slow took=%s log_entries=%d output_bytes=%d", d.Round(time.Millisecond), logN, outN))
}

func recordPersistDrop() {
	if !diagEnabled() {
		return
	}
	n := diagPersistDrop.Add(1)
	if n == 1 || n%100 == 0 {
		queueDiag(fmt.Sprintf("log persist queue full, dropped lines total=%d", n))
	}
}

func (m *Model) diagTickCmd() tea.Cmd {
	if !diagEnabled() {
		return nil
	}
	return tea.Tick(diagInterval, func(time.Time) tea.Msg {
		return diagTickMsg{}
	})
}

func (m *Model) handleDiagTick() tea.Cmd {
	emitDiagSnapshot(m)
	return m.diagTickCmd()
}

func emitDiagSnapshot(m *Model) {
	if !diagEnabled() {
		return
	}
	var b strings.Builder
	b.WriteString("snapshot")
	if br := activeBridge; br != nil {
		q, posted, dropLog, dropOther, blocked, fwdSlow, fwdMax := br.stats()
		b.WriteString(fmt.Sprintf(" bridge_queue=%d/%d posted=%d drop_log=%d drop_other=%d blocked=%d fwd_slow=%d fwd_max=%s",
			q, externalMsgBuffer, posted, dropLog, dropOther, blocked, fwdSlow, fwdMax.Round(time.Millisecond)))
		if q >= diagQueueWarn {
			queueDiag(fmt.Sprintf("bridge queue high depth=%d/%d", q, externalMsgBuffer))
		}
	}
	b.WriteString(fmt.Sprintf(" updates=%d update_slow=%d view_slow=%d persist_drop=%d",
		diagUpdateTotal.Load(), diagUpdateSlow.Load(), diagViewSlow.Load(), diagPersistDrop.Load()))
	b.WriteString(formatMsgHistTop(6))
	b.WriteString(fmt.Sprintf(" log_file=%s output_bytes=%d spinner=%v todos=%d libtests=%d",
		config.LogFilePath(), len(m.outputContent), m.spinnerActive, m.store.ActiveCount(), len(m.activeLibTests())))
	queueDiag(b.String())
}

func diagStartupNote() {
	if !diagEnabled() {
		return
	}
	queueDiag(fmt.Sprintf("diag enabled path=%s interval=%s (OPS_AGENT_DIAG=0 to disable)", config.DiagLogFilePath(), diagInterval))
}
