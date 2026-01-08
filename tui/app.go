package tui

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"tcp_lb/backend"
	"tcp_lb/config"
	"tcp_lb/loadbalancer"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App represents the TUI application.
type App struct {
	app            *tview.Application
	lb             *loadbalancer.LoadBalancer
	pool           *backend.Pool
	config         *config.Config
	lbAddr         string

	// UI components
	mainLayout     *tview.Flex
	backendTable   *tview.Table
	logView        *tview.TextView
	statusBar      *tview.TextView
	timersView     *tview.TextView
	serverInfo     *tview.TextView

	// State
	logs            []string
	lastHealthCheck time.Time
	currentAlgo     string
}

// NewApp creates a new TUI application.
func NewApp(lb *loadbalancer.LoadBalancer, cfg *config.Config) *App {
	return &App{
		app:             tview.NewApplication(),
		lb:              lb,
		pool:            lb.GetPool(),
		config:          cfg,
		lbAddr:          cfg.ListenAddr,
		logs:            make([]string, 0),
		lastHealthCheck: time.Now(),
		currentAlgo:     "Round Robin",
	}
}

// Run starts the TUI application.
func (a *App) Run() error {
	// Create header
	header := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText("[yellow::b]TCP LOAD BALANCER DASHBOARD[-:-:-]\n[gray]Press: [white]1[-] +1 conn | [white]2[-] +10 conn | [white]3[-] Algorithm | [white]r[-] Restart sim | [white]q[-] Quit")
	header.SetBorder(true).SetBorderColor(tcell.ColorDarkCyan)

	// Create server info panel
	a.serverInfo = tview.NewTextView().
		SetDynamicColors(true)
	a.serverInfo.SetBorder(true).SetBorderColor(tcell.ColorDarkCyan).SetTitle(" [::b]Load Balancer ")
	a.refreshServerInfo()

	// Create backend table
	a.backendTable = tview.NewTable().
		SetBorders(true).
		SetSelectable(false, false)
	a.backendTable.SetTitle(" [::b]Backends ").SetBorder(true).SetBorderColor(tcell.ColorDarkCyan)
	a.setupTableHeaders()

	// Create timers display (health check + server pause)
	a.timersView = tview.NewTextView().
		SetDynamicColors(true)
	a.timersView.SetTitle(" [::b]Timers ").SetBorder(true).SetBorderColor(tcell.ColorDarkCyan)

	// Create log view
	a.logView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetMaxLines(100)
	a.logView.SetTitle(" [::b]Activity Log ").SetBorder(true).SetBorderColor(tcell.ColorDarkCyan)

	// Create status bar
	a.statusBar = tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	a.updateStatusBar()

	// Left panel with server info and backends
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.serverInfo, 7, 0, false).
		AddItem(a.backendTable, 0, 1, false)

	// Right panel with timers and log
	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.timersView, 8, 0, false).
		AddItem(a.logView, 0, 1, false)

	// Main content area
	content := tview.NewFlex().
		AddItem(leftPanel, 0, 3, false).
		AddItem(rightPanel, 0, 2, false)

	// Main layout
	a.mainLayout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 4, 0, false).
		AddItem(content, 0, 1, false).
		AddItem(a.statusBar, 1, 0, false)

	// Set up keyboard input
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q', 'Q':
				a.app.Stop()
				return nil
			case '1':
				go a.sendTraffic()
				return nil
			case '2':
				go a.sendBurstTraffic(10)
				return nil
			case '3':
				a.showAlgorithmModal()
				return nil
			case 'r', 'R':
				a.restartSimulation()
				return nil
			}
		case tcell.KeyEscape:
			a.app.Stop()
			return nil
		}
		return event
	})

	// Register event callback for pool events (server down/up)
	a.pool.SetEventCallback(func(event backend.PoolEvent) {
		a.app.QueueUpdateDraw(func() {
			switch event.Type {
			case backend.EventBackendDown:
				a.addLog(fmt.Sprintf("[red]💥 Server CRASHED: %s[-] [gray](LB unaware, status still Healthy)[-]", event.Backend))
			case backend.EventBackendRecovered:
				a.addLog(fmt.Sprintf("[yellow]⏳ Server READY: %s[-] [gray](awaiting health check)[-]", event.Backend))
			}
		})
	})

	// Initial data population
	a.refreshBackends()
	a.refreshTimers()
	a.addLog("[green]Dashboard started[-]")
	a.addLog(fmt.Sprintf("[gray]Load balancer on %s[-]", a.lbAddr))

	// Start background refresh
	go a.refreshLoop()

	return a.app.SetRoot(a.mainLayout, true).EnableMouse(true).Run()
}

// setupTableHeaders creates the table header row.
func (a *App) setupTableHeaders() {
	headers := []string{"Address", "Status", "Active", "Share", "Total", "Last Check"}
	for i, h := range headers {
		a.backendTable.SetCell(0, i,
			tview.NewTableCell(h).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignCenter).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold))
	}
}

// refreshLoop updates the UI periodically.
func (a *App) refreshLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		a.app.QueueUpdateDraw(func() {
			a.refreshBackends()
			a.refreshTimers()
			a.updateStatusBar()
		})
	}
}

// refreshBackends updates the backend table.
func (a *App) refreshBackends() {
	backends := a.pool.GetBackends()

	// Calculate total active connections first
	totalActive := 0
	for _, b := range backends {
		totalActive += b.GetActiveConnections()
	}

	for i, b := range backends {
		row := i + 1
		addr, alive, active, total := b.GetStats()
		lastCheck := b.GetLastHealthCheck()

		// Update last health check time
		if lastCheck.After(a.lastHealthCheck) {
			a.lastHealthCheck = lastCheck
		}

		// Address
		a.backendTable.SetCell(row, 0,
			tview.NewTableCell(addr).
				SetAlign(tview.AlignCenter))

		// Status with color
		status := "[green]Healthy[-]"
		if !alive {
			status = "[red]Down[-]"
		}
		a.backendTable.SetCell(row, 1,
			tview.NewTableCell(status).
				SetAlign(tview.AlignCenter))

		// Active connections with highlight if > 0
		activeStr := fmt.Sprintf("%d", active)
		if active > 0 {
			activeStr = fmt.Sprintf("[yellow::b]%d[-:-:-]", active)
		}
		a.backendTable.SetCell(row, 2,
			tview.NewTableCell(activeStr).
				SetAlign(tview.AlignCenter))

		// Share of total active connections
		shareStr := "-"
		if totalActive > 0 {
			share := float64(active) / float64(totalActive) * 100
			shareStr = fmt.Sprintf("%.1f%%", share)
		}
		a.backendTable.SetCell(row, 3,
			tview.NewTableCell(shareStr).
				SetAlign(tview.AlignCenter))

		// Total connections
		a.backendTable.SetCell(row, 4,
			tview.NewTableCell(fmt.Sprintf("%d", total)).
				SetAlign(tview.AlignCenter))

		// Last check (relative time)
		ago := time.Since(lastCheck).Round(time.Second)
		a.backendTable.SetCell(row, 5,
			tview.NewTableCell(fmt.Sprintf("%v ago", ago)).
				SetAlign(tview.AlignCenter).
				SetTextColor(tcell.ColorGray))
	}
}

// refreshTimers updates both the health check and server pause timers.
func (a *App) refreshTimers() {
	var text strings.Builder

	// Health Check Timer
	elapsed := time.Since(a.lastHealthCheck)
	remaining := a.config.HealthCheckInterval - elapsed
	if remaining < 0 {
		remaining = 0
	}

	progress := float64(elapsed) / float64(a.config.HealthCheckInterval)
	if progress > 1 {
		progress = 1
	}
	barWidth := 16
	filled := int(progress * float64(barWidth))
	healthBar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	healthColor := "green"
	if remaining < 2*time.Second {
		healthColor = "yellow"
	}

	text.WriteString(fmt.Sprintf("[yellow::b]Health Check[-:-:-]\n"))
	text.WriteString(fmt.Sprintf("[%s]%v[-] %s\n\n", healthColor, remaining.Round(time.Second), healthBar))

	// Server Pause Timer
	pausedBackend, pauseStart, pauseDuration, nextPause := a.pool.GetPauseState()

	text.WriteString("[yellow::b]Server Pause[-:-:-]\n")
	if pausedBackend != "" {
		// Currently paused - show recovery countdown
		pauseElapsed := time.Since(pauseStart)
		pauseRemaining := pauseDuration - pauseElapsed
		if pauseRemaining < 0 {
			pauseRemaining = 0
		}

		pauseProgress := float64(pauseElapsed) / float64(pauseDuration)
		if pauseProgress > 1 {
			pauseProgress = 1
		}
		pauseFilled := int(pauseProgress * float64(barWidth))
		pauseBar := strings.Repeat("█", pauseFilled) + strings.Repeat("░", barWidth-pauseFilled)

		text.WriteString(fmt.Sprintf("[red]%s[-] paused\n", pausedBackend))
		text.WriteString(fmt.Sprintf("[cyan]%v[-] %s", pauseRemaining.Round(time.Second), pauseBar))
	} else {
		// Not paused - show next pause countdown
		untilNextPause := time.Until(nextPause)
		if untilNextPause < 0 {
			untilNextPause = 0
		}
		text.WriteString(fmt.Sprintf("[gray]Next pause in %v[-]", untilNextPause.Round(time.Second)))
	}

	a.timersView.SetText(text.String())
}

// updateStatusBar updates the bottom status bar.
func (a *App) updateStatusBar() {
	backends := a.pool.GetBackends()
	healthy := 0
	totalConns := 0
	for _, b := range backends {
		if b.IsAlive() {
			healthy++
		}
		totalConns += b.GetActiveConnections()
	}

	status := fmt.Sprintf(" [green]●[-] %d/%d backends | [yellow]%d[-] active connections | Algorithm: [cyan]%s[-] ",
		healthy, len(backends), totalConns, a.currentAlgo)
	a.statusBar.SetText(status)
}

// refreshServerInfo updates the server info panel.
func (a *App) refreshServerInfo() {
	a.serverInfo.SetText(fmt.Sprintf(
		"[yellow::b]Server Info[-:-:-]\n"+
			"[white]Listen Address:[gray]  %s\n"+
			"[white]Algorithm:[gray]       %s\n"+
			"[white]Health Interval:[gray] %v\n"+
			"[white]Connect Timeout:[gray] %v",
		a.lbAddr,
		a.currentAlgo,
		a.config.HealthCheckInterval,
		a.config.ConnectTimeout,
	))
}

// sendTraffic sends a test connection through the load balancer.
// The connection is held for a random duration (10-70 seconds) to simulate real traffic.
func (a *App) sendTraffic() {
	a.addLog("[yellow]→ Connecting...[-]")

	conn, err := net.DialTimeout("tcp", a.lbAddr, 5*time.Second)
	if err != nil {
		a.addLog(fmt.Sprintf("[red]✗ Connection failed: %v[-]", err))
		return
	}

	// Read welcome message
	reader := bufio.NewReader(conn)
	welcome, err := reader.ReadString('\n')
	if err != nil {
		a.addLog(fmt.Sprintf("[red]✗ Read failed: %v[-]", err))
		conn.Close()
		return
	}

	welcome = strings.TrimSpace(welcome)
	backendAddr := strings.TrimPrefix(welcome, "Connected to Backend ")

	// Random duration 10-70 seconds
	duration := time.Duration(10+rand.Intn(61)) * time.Second
	a.addLog(fmt.Sprintf("[green]✓ Routed → %s[-] [gray](%ds)[-]", backendAddr, int(duration.Seconds())))

	// Hold connection for the duration
	time.Sleep(duration)
	conn.Close()
	a.addLog(fmt.Sprintf("[gray]↩ Disconnected from %s[-]", backendAddr))
}

// sendBurstTraffic sends multiple connections at once.
func (a *App) sendBurstTraffic(count int) {
	a.addLog(fmt.Sprintf("[yellow]→ Sending %d connections...[-]", count))
	for i := 0; i < count; i++ {
		go a.sendTraffic()
		// Small delay between connections to avoid overwhelming
		time.Sleep(50 * time.Millisecond)
	}
}

// restartSimulation restarts the backend failure simulation.
func (a *App) restartSimulation() {
	a.pool.RestartSimulation()
	a.addLog("[cyan]↻ Simulation restarted[-]")
}

// showAlgorithmModal displays the algorithm selection modal.
func (a *App) showAlgorithmModal() {
	algorithms := []struct {
		name string
		algo loadbalancer.Algorithm
	}{
		{"Round Robin", loadbalancer.NewRoundRobin()},
		{"Least Connections", loadbalancer.NewLeastConnections()},
		{"Weighted Round Robin", loadbalancer.NewWeightedRoundRobin()},
	}

	list := tview.NewList()
	for i, alg := range algorithms {
		name := alg.name
		if name == a.currentAlgo {
			name = "[cyan]" + name + " (active)[-]"
		}
		list.AddItem(name, "", rune('1'+i), nil)
	}

	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		selected := algorithms[index]
		a.lb.SetAlgorithm(selected.algo)
		a.currentAlgo = selected.name
		a.refreshServerInfo()
		a.addLog(fmt.Sprintf("[green]Algorithm changed to: %s[-]", selected.name))
		a.app.SetRoot(a.mainLayout, true)
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.app.SetRoot(a.mainLayout, true)
			return nil
		}
		return event
	})

	list.SetBorder(true).SetTitle(" Select Algorithm (ESC to cancel) ")

	// Center the list in a modal-like layout
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, 7, 0, true).
			AddItem(nil, 0, 1, false), 40, 0, true).
		AddItem(nil, 0, 1, false)

	pages := tview.NewPages().
		AddPage("main", a.mainLayout, true, true).
		AddPage("modal", modal, true, true)

	a.app.SetRoot(pages, true).SetFocus(list)
}

// addLog adds a timestamped message to the log view.
func (a *App) addLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logLine := fmt.Sprintf("[gray]%s[-] %s", timestamp, message)
	a.logs = append(a.logs, logLine)

	// Write directly - QueueUpdateDraw will be called by refresh loop
	fmt.Fprintln(a.logView, logLine)
	a.logView.ScrollToEnd()
}
