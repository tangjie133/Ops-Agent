package tui

// signals.go — SIGINT 双次退出与 runCtx 取消，避免 bubbletea 事件队列阻塞时无法退出。
import (
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// installSignalHandler 在独立 goroutine 监听 SIGINT/SIGTERM，TUI 卡死时仍可退出。
// 第一次信号：取消后台任务并 Kill 程序；第二次或 2s 后仍存活：os.Exit。
func installSignalHandler(p *tea.Program, shutdown func()) {
	sigCh := make(chan os.Signal, 3)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for n := 1; ; n++ {
			sig := <-sigCh
			queueDiag("signal " + sig.String() + ": cancel background + Kill TUI")
			shutdown()
			p.Kill()
			if n >= 2 {
				os.Exit(128 + int(syscall.SIGINT))
			}
			go func() {
				time.Sleep(2 * time.Second)
				queueDiag("signal: Kill timeout, os.Exit")
				os.Exit(128 + int(syscall.SIGINT))
			}()
		}
	}()
}
