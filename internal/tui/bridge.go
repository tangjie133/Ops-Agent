package tui

// bridge.go — 后台 goroutine 与 bubbletea 之间的有界消息桥，避免 p.Send 阻塞。

import (
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const externalMsgBuffer = 512

var activeBridge *programBridge

// programBridge 将后台 goroutine 的消息经有界队列转发到 bubbletea，避免无缓冲 p.Send 阻塞 smee/webhook/worker。
type programBridge struct {
	incoming chan tea.Msg
	closed   atomic.Bool

	posted       atomic.Int64
	droppedLog   atomic.Int64
	droppedOther atomic.Int64
	blockedDrop  atomic.Int64
	forwardSlow  atomic.Int64
	forwardMaxNs atomic.Int64
}

func newProgramBridge(post func(tea.Msg)) *programBridge {
	b := &programBridge{incoming: make(chan tea.Msg, externalMsgBuffer)}
	activeBridge = b
	go func() {
		for msg := range b.incoming {
			start := time.Now()
			post(msg)
			d := time.Since(start)
			if d >= diagSlowBridgeFwd {
				b.forwardSlow.Add(1)
				queueDiag(fmtBridgeSlow(diagMsgType(msg), d, len(b.incoming)))
			}
			for {
				old := b.forwardMaxNs.Load()
				ns := d.Nanoseconds()
				if ns <= old || b.forwardMaxNs.CompareAndSwap(old, ns) {
					break
				}
			}
		}
	}()
	return b
}

func (b *programBridge) Close() {
	if b.closed.Swap(true) {
		return
	}
	close(b.incoming)
}

func (b *programBridge) postable() bool {
	return b != nil && !b.closed.Load()
}

func fmtBridgeSlow(typ string, d time.Duration, queued int) string {
	return "bridge forward slow type=" + typ + " took=" + d.Round(time.Millisecond).String() +
		" queue_after=" + itoa(queued)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func (b *programBridge) stats() (queued int, posted, dropLog, dropOther, blocked, fwdSlow int64, fwdMax time.Duration) {
	if b == nil {
		return 0, 0, 0, 0, 0, 0, 0
	}
	return len(b.incoming),
		b.posted.Load(),
		b.droppedLog.Load(),
		b.droppedOther.Load(),
		b.blockedDrop.Load(),
		b.forwardSlow.Load(),
		time.Duration(b.forwardMaxNs.Load())
}

func (b *programBridge) Post(msg tea.Msg) {
	if msg == nil || !b.postable() {
		return
	}
	typ := diagMsgType(msg)
	select {
	case b.incoming <- msg:
		b.posted.Add(1)
	default:
		switch msg.(type) {
		case LogLineMsg, invStatusMsg, refreshTickMsg:
			b.droppedLog.Add(1)
			if n := b.droppedLog.Load(); n == 1 || n%50 == 0 {
				queueDiag("bridge drop log type=" + typ + " total=" + itoa64(n) + " queue=" + itoa(len(b.incoming)))
			}
			return
		default:
			select {
			case b.incoming <- msg:
				b.posted.Add(1)
			case <-time.After(50 * time.Millisecond):
				b.blockedDrop.Add(1)
				queueDiag("bridge blocked drop type=" + typ + " wait=50ms queue=" + itoa(len(b.incoming)))
			}
		}
	}
}

func (b *programBridge) PostImportant(msg tea.Msg) {
	if msg == nil || !b.postable() {
		return
	}
	typ := diagMsgType(msg)
	select {
	case b.incoming <- msg:
		b.posted.Add(1)
	default:
		select {
		case b.incoming <- msg:
			b.posted.Add(1)
		case <-time.After(200 * time.Millisecond):
			b.blockedDrop.Add(1)
			b.droppedOther.Add(1)
			queueDiag("bridge important drop type=" + typ + " wait=200ms queue=" + itoa(len(b.incoming)))
		}
	}
}

func itoa64(n int64) string {
	return itoa(int(n))
}
