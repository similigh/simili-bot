// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Brand color
var (
	primaryColor = lipgloss.Color("#ff7300")
	subtleColor  = lipgloss.Color("#626262")
	successColor = lipgloss.Color("#04B575")
	errorColor   = lipgloss.Color("#FF0000")

	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	activeStepStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	doneStepStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorStepStyle = lipgloss.NewStyle().
			Foreground(errorColor)
)

// PipelineStatusMsg indicates a status update from the pipeline.
type PipelineStatusMsg struct {
	Step    string
	Status  string // "started", "success", "error", "skipped"
	Message string
}

// ResultMsg indicates the final result.
type ResultMsg struct {
	Success bool
	Output  string
}

// Model for the TUI.
type Model struct {
	spinner    spinner.Model
	steps      []string
	current    int
	status     map[string]string // step -> status
	logs       []string
	quitting   bool
	err        error
	statusChan <-chan PipelineStatusMsg
}

// NewModel creates a new TUI model.
func NewModel(steps []string, statusChan <-chan PipelineStatusMsg) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	return Model{
		spinner:    s,
		steps:      steps,
		current:    0,
		status:     make(map[string]string),
		statusChan: statusChan,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.waitForActivity(),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case PipelineStatusMsg:
		m.status[msg.Step] = msg.Status
		if msg.Message != "" {
			m.logs = append(m.logs, fmt.Sprintf("[%s] %s: %s", time.Now().Format("15:04:05"), msg.Step, msg.Message))
		}

		// Find current step index
		for i, s := range m.steps {
			if s == msg.Step {
				m.current = i
				break
			}
		}

		if msg.Status == "error" {
			m.err = fmt.Errorf("step %s failed: %s", msg.Step, msg.Message)
		}

		return m, m.waitForActivity()

	case ResultMsg:
		// Print the final output before quitting so the user can see the result
		if msg.Output != "" {
			fmt.Println("\n" + msg.Output)
		}
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) waitForActivity() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.statusChan:
			if !ok {
				return ResultMsg{Success: true}
			}
			return msg
		case <-time.After(30 * time.Second):
			// Timeout waiting for pipeline activity
			return ResultMsg{
				Success: false,
				Output:  "pipeline timed out waiting for activity",
			}
		}
	}
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(titleStyle.Render("Simili-Bot Pipeline"))
	s.WriteString("\n\n")

	for i, step := range m.steps {
		status := m.status[step]
		var line string

		prefix := "  "
		style := stepStyle

		if i == m.current {
			prefix = m.spinner.View() + " "
			style = activeStepStyle
		}

		switch status {
		case "success":
			prefix = "✓ "
			style = doneStepStyle
		case "error":
			prefix = "✗ "
			style = errorStepStyle
		case "skipped":
			prefix = "○ "
			style = stepStyle.Faint(true)
		}

		line = fmt.Sprintf("%s%s\n", prefix, step)
		s.WriteString(style.Render(line))
	}

	s.WriteString("\nLogs:\n")
	// Show last 5 logs
	start := 0
	if len(m.logs) > 5 {
		start = len(m.logs) - 5
	}
	for _, log := range m.logs[start:] {
		s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(log) + "\n")
	}

	if m.err != nil {
		s.WriteString("\n" + errorStepStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n")
	}

	s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render("\nPress q to quit\n"))

	return s.String()
}
