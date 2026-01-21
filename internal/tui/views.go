package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/Drilmo/git-secret-scanner/internal/analyzer"
	"github.com/Drilmo/git-secret-scanner/internal/cleaner"
	"github.com/Drilmo/git-secret-scanner/internal/config"
	"github.com/Drilmo/git-secret-scanner/internal/scanner"
)

// Messages
type scanStartMsg struct{}
type scanProgressMsg struct {
	current int
	total   int
	found   int
}
type scanDoneMsg struct {
	result     interface{}
	err        error
	outputPath string
}
type analyzeDoneMsg struct {
	result     *analyzer.Analysis
	err        error
	csvPath    string
	csvExported bool
}
type cleanDoneMsg struct {
	result *cleaner.CleanResult
	err    error
}

// Scan form handling
func (m Model) updateScanForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			// Go back to menu
			m.view = ViewMenu
			return m, nil
		case "ctrl+e":
			// Open configuration
			m.view = ViewScanConfig
			m.configIndex = 0
			m.configFromScan = true
			return m, nil
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		if m.scanConfirm != nil && *m.scanConfirm {
			// Start scan
			m.view = ViewScanProgress
			return m, tea.Batch(m.spinner.Tick, m.startScan())
		}
		// User cancelled
		m.view = ViewMenu
		return m, nil
	}

	if m.form.State == huh.StateAborted {
		m.view = ViewMenu
	}

	return m, cmd
}

func (m Model) viewScanForm() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🔍 Scan Repository"))
	sb.WriteString("\n\n")

	// Show current configuration with details
	configLabel := "Built-in defaults"
	if m.configPath != "" {
		configLabel = m.configPath
	}
	sb.WriteString(keyStyle.Render("Configuration: "))
	sb.WriteString(configLabel)

	// Show pattern count
	cfg, _ := config.Load(m.configPath)
	if cfg != nil {
		patternCount := 0
		for _, kw := range cfg.Keywords {
			patternCount += len(kw.Patterns)
		}
		sb.WriteString(fmt.Sprintf(" (%d patterns)", patternCount))
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("  (Ctrl+E to change)"))
	sb.WriteString("\n\n")

	sb.WriteString(m.form.View())

	return boxStyle.Render(sb.String())
}

func (m *Model) startScan() tea.Cmd {
	// Capture values from pointers before the closure
	repoPath := "."
	if m.scanRepoPath != nil && *m.scanRepoPath != "" {
		repoPath = *m.scanRepoPath
	}

	outputPath := "secrets.json"
	if m.scanOutputPath != nil && *m.scanOutputPath != "" {
		outputPath = *m.scanOutputPath
	}

	scanMode := "full"
	if m.scanMode != nil {
		scanMode = *m.scanMode
	}

	scanSource := "both"
	if m.scanSource != nil {
		scanSource = *m.scanSource
	}

	branch := "--all"
	if m.scanBranch != nil {
		branch = *m.scanBranch
	}

	configPath := m.scanConfigPath

	return func() tea.Msg {
		cfg, _ := config.Load(configPath)
		s := scanner.New(cfg)

		opts := scanner.ScanOptions{
			Branch:     branch,
			ConfigPath: configPath,
			OnProgress: func(current, total, found int) {
				// Progress updates would need channel communication
				// For now, we'll show completion
			},
		}

		switch scanMode {
		case "stream":
			// Stream mode always uses .jsonl extension
			streamPath := outputPath
			if strings.HasSuffix(streamPath, ".json") {
				streamPath = strings.TrimSuffix(streamPath, ".json") + ".jsonl"
			} else if !strings.HasSuffix(streamPath, ".jsonl") {
				streamPath = streamPath + ".jsonl"
			}

			var count int
			var err error

			switch scanSource {
			case "current":
				count, err = s.ScanCurrentStream(repoPath, streamPath)
			case "history":
				count, err = s.ScanStream(repoPath, streamPath, opts)
			default: // both
				count, err = s.ScanBothStream(repoPath, streamPath, opts)
			}

			if err != nil {
				return scanDoneMsg{err: err}
			}
			return scanDoneMsg{
				result: map[string]interface{}{
					"mode":   "stream",
					"source": scanSource,
					"count":  count,
				},
				outputPath: streamPath,
			}

		case "fast":
			// Fast mode uses .json extension
			jsonPath := outputPath
			if strings.HasSuffix(jsonPath, ".jsonl") {
				jsonPath = strings.TrimSuffix(jsonPath, ".jsonl") + ".json"
			} else if !strings.HasSuffix(jsonPath, ".json") {
				jsonPath = jsonPath + ".json"
			}

			var result *scanner.ScanResult
			var err error

			switch scanSource {
			case "current":
				result, err = s.ScanCurrent(repoPath)
			case "history":
				result, err = s.Scan(repoPath, opts)
			default: // both
				result, err = s.ScanBoth(repoPath, opts)
			}

			if err != nil {
				return scanDoneMsg{err: err}
			}
			// Save results to file
			if err := saveResultToFile(result, jsonPath); err != nil {
				return scanDoneMsg{err: err}
			}
			return scanDoneMsg{result: result, outputPath: jsonPath}

		default: // full
			// Full mode uses .json extension
			jsonPath := outputPath
			if strings.HasSuffix(jsonPath, ".jsonl") {
				jsonPath = strings.TrimSuffix(jsonPath, ".jsonl") + ".json"
			} else if !strings.HasSuffix(jsonPath, ".json") {
				jsonPath = jsonPath + ".json"
			}

			var result *scanner.ScanResult
			var err error

			switch scanSource {
			case "current":
				result, err = s.ScanCurrent(repoPath)
			case "history":
				result, err = s.Scan(repoPath, opts)
			default: // both
				result, err = s.ScanBoth(repoPath, opts)
			}

			if err != nil {
				return scanDoneMsg{err: err}
			}
			// Save results to file
			if err := saveResultToFile(result, jsonPath); err != nil {
				return scanDoneMsg{err: err}
			}
			return scanDoneMsg{result: result, outputPath: jsonPath}
		}
	}
}

func (m Model) updateScanProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scanProgressMsg:
		m.scanProgress = msg.current
		m.scanTotal = msg.total
		m.scanFound = msg.found
		return m, nil

	case scanDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		m.scanResult = msg.result
		m.view = ViewScanResults
		return m, nil

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m Model) viewScanProgress() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🔍 Scanning Repository"))
	sb.WriteString("\n\n")

	sb.WriteString(m.spinner.View())
	sb.WriteString(" Searching for secrets...\n\n")

	if m.scanTotal > 0 {
		progress := float64(m.scanProgress) / float64(m.scanTotal) * 100
		sb.WriteString(fmt.Sprintf("Progress: %d/%d keywords (%.0f%%)\n", m.scanProgress, m.scanTotal, progress))
		sb.WriteString(fmt.Sprintf("Secrets found: %d\n", m.scanFound))
	}

	return boxStyle.Render(sb.String())
}

func (m Model) viewScanResults() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("✅ Scan Complete"))
	sb.WriteString("\n\n")

	// Get output path from scanDoneMsg (stored in scanResult if available)
	outputPath := "secrets.json"
	if m.scanOutputPath != nil && *m.scanOutputPath != "" {
		outputPath = *m.scanOutputPath
	}

	if m.err != nil {
		sb.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	} else if result, ok := m.scanResult.(*scanner.ScanResult); ok {
		// Show config used
		configUsed := "Built-in defaults"
		if m.scanConfigPath != "" {
			configUsed = m.scanConfigPath
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Config used:"), configUsed))
		sb.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Secrets found:"), result.SecretsFound))
		sb.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Total values:"), result.TotalValues))
		sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Repository:"), result.Repository))
		sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Branch:"), result.Branch))
		sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Output file:"), successStyle.Render(outputPath)))

		if len(result.Secrets) > 0 {
			sb.WriteString("\n" + keyStyle.Render("Top secrets by change frequency:") + "\n")
			for i, secret := range result.Secrets {
				if i >= 5 {
					sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(result.Secrets)-5))
					break
				}
				sb.WriteString(fmt.Sprintf("  • %s (%d changes)\n",
					maskedValueStyle.Render(secret.File+"/"+secret.Key),
					secret.ChangeCount))
			}
		}
	} else if streamResult, ok := m.scanResult.(map[string]interface{}); ok {
		sb.WriteString(fmt.Sprintf("%s stream\n", keyStyle.Render("Mode:")))
		if source, ok := streamResult["source"]; ok {
			sb.WriteString(fmt.Sprintf("%s %v\n", keyStyle.Render("Source:"), source))
		}
		sb.WriteString(fmt.Sprintf("%s %v\n", keyStyle.Render("Secrets found:"), streamResult["count"]))
		sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Output file:"), successStyle.Render(outputPath)))
	}

	help := helpStyle.Render("esc: back to menu")
	sb.WriteString("\n\n" + help)

	return successBoxStyle.Render(sb.String())
}

// Analyze form handling
func (m Model) updateAnalyzeForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle ESC to go back to menu
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		m.view = ViewMenu
		return m, nil
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		if m.analyzeConfirm != nil && *m.analyzeConfirm {
			m.view = ViewAnalyzeProgress
			return m, tea.Batch(m.spinner.Tick, m.startAnalyze())
		}
		// User cancelled
		m.view = ViewMenu
		return m, nil
	}

	if m.form.State == huh.StateAborted {
		m.view = ViewMenu
	}

	return m, cmd
}

func (m Model) viewAnalyzeForm() string {
	return boxStyle.Render(
		titleStyle.Render("📊 Analyze Results") + "\n\n" +
			m.form.View(),
	)
}

func (m *Model) startAnalyze() tea.Cmd {
	// Capture values from pointers before closure
	inputPath := "secrets.json"
	if m.analyzeInputPath != nil {
		inputPath = *m.analyzeInputPath
	}
	outputPath := "secrets_analysis.csv"
	if m.analyzeOutputPath != nil {
		outputPath = *m.analyzeOutputPath
	}

	return func() tea.Msg {
		a := analyzer.New()
		var result *analyzer.Analysis
		var err error

		// Use AnalyzeJSON for .json files, AnalyzeJSONL for .jsonl files
		if strings.HasSuffix(inputPath, ".jsonl") {
			result, err = a.AnalyzeJSONL(inputPath, analyzer.AnalyzeOptions{})
		} else {
			result, err = a.AnalyzeJSON(inputPath, analyzer.AnalyzeOptions{})
		}

		if err != nil {
			return analyzeDoneMsg{result: result, err: err}
		}

		// Export to CSV
		csvExported := false
		if outputPath != "" && result != nil {
			if csvErr := analyzer.ExportCSV(result, outputPath); csvErr == nil {
				csvExported = true
			}
		}

		return analyzeDoneMsg{result: result, err: err, csvPath: outputPath, csvExported: csvExported}
	}
}

func (m Model) updateAnalyzeProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case analyzeDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		m.analyzeResult = msg.result
		m.analyzeCsvExported = msg.csvExported
		m.view = ViewAnalyzeResults
		return m, nil

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m Model) viewAnalyzeProgress() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📊 Analyzing Results"))
	sb.WriteString("\n\n")

	sb.WriteString(m.spinner.View())
	sb.WriteString(" Analyzing scan results...\n")

	return boxStyle.Render(sb.String())
}

func (m Model) viewAnalyzeResults() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📊 Analysis Results"))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	} else if result, ok := m.analyzeResult.(*analyzer.Analysis); ok {
		// Stats
		sb.WriteString(keyStyle.Render("Statistics") + "\n")
		sb.WriteString(fmt.Sprintf("  Total entries:     %d\n", result.Stats.TotalEntries))
		sb.WriteString(fmt.Sprintf("  Unique secrets:    %d\n", result.Stats.UniqueSecrets))
		sb.WriteString(fmt.Sprintf("  Unique values:     %d\n\n", result.Stats.UniqueValues))

		// Top authors
		if len(result.Stats.TopAuthors) > 0 {
			sb.WriteString(keyStyle.Render("Top Authors") + "\n")
			for _, a := range result.Stats.TopAuthors[:min(5, len(result.Stats.TopAuthors))] {
				sb.WriteString(fmt.Sprintf("  • %-20s %d\n", a.Author, a.Count))
			}
			sb.WriteString("\n")
		}

		// Top secrets
		if len(result.Secrets) > 0 {
			sb.WriteString(keyStyle.Render("Most Changed Secrets") + "\n")
			for i, s := range result.Secrets {
				if i >= 5 {
					break
				}
				sb.WriteString(fmt.Sprintf("  • %s\n", maskedValueStyle.Render(s.File+"/"+s.Key)))
				sb.WriteString(fmt.Sprintf("    %d changes, %d occurrences, authors: %s\n",
					s.ChangeCount, s.TotalOccurrences, strings.Join(s.Authors, ", ")))
			}
		}

		// CSV export status
		sb.WriteString("\n")
		if m.analyzeCsvExported && m.analyzeOutputPath != nil && *m.analyzeOutputPath != "" {
			sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("CSV exported:"), successStyle.Render(*m.analyzeOutputPath)))
		}
	}

	help := helpStyle.Render("esc: back to menu")
	sb.WriteString("\n\n" + help)

	return boxStyle.Render(sb.String())
}

// Clean form handling
func (m Model) updateCleanForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle ESC to go back to menu
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		m.view = ViewMenu
		return m, nil
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		if m.cleanConfirm == nil || !*m.cleanConfirm {
			// User cancelled
			m.view = ViewMenu
			return m, nil
		}
		if m.cleanDryRun != nil && *m.cleanDryRun {
			m.view = ViewCleanProgress
			return m, tea.Batch(m.spinner.Tick, m.startClean())
		}
		m.view = ViewCleanConfirm
		m.form = m.createCleanConfirmForm()
		return m, m.form.Init()
	}

	if m.form.State == huh.StateAborted {
		m.view = ViewMenu
	}

	return m, cmd
}

func (m Model) viewCleanForm() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🧹 Clean History"))
	sb.WriteString("\n\n")

	// Information panel
	sb.WriteString(warningStyle.Render("ℹ️  What this operation does:"))
	sb.WriteString("\n\n")
	sb.WriteString(keyStyle.Render("Preserved:"))
	sb.WriteString("\n")
	sb.WriteString("  ✓ All commits, messages, authors, dates\n")
	sb.WriteString("  ✓ Branches and tags structure\n")
	sb.WriteString("  ✓ File structure and non-secret content\n\n")
	sb.WriteString(keyStyle.Render("Modified:"))
	sb.WriteString("\n")
	sb.WriteString("  ⚡ Secrets replaced with ***REMOVED***\n")
	sb.WriteString("  ⚡ Commit SHA hashes will change (git consequence)\n\n")
	sb.WriteString(errorStyle.Render("⚠️  Important:"))
	sb.WriteString("\n")
	sb.WriteString("  • This rewrites git history permanently\n")
	sb.WriteString("  • All collaborators must re-clone after push\n")
	sb.WriteString("  • Always rotate exposed credentials\n")
	sb.WriteString("\n\n")

	sb.WriteString(m.form.View())

	return boxStyle.Render(sb.String())
}

func (m Model) updateCleanConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle ESC to go back to menu
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		m.view = ViewMenu
		return m, nil
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		if m.cleanConfirm != nil && *m.cleanConfirm {
			m.view = ViewCleanProgress
			return m, tea.Batch(m.spinner.Tick, m.startClean())
		}
		// User cancelled
		m.view = ViewMenu
		return m, nil
	}

	if m.form.State == huh.StateAborted {
		m.view = ViewMenu
	}

	return m, cmd
}

func (m Model) viewCleanConfirm() string {
	return errorBoxStyle.Render(
		titleStyle.Render("⚠️  Confirm Clean") + "\n\n" +
			m.form.View(),
	)
}

func (m *Model) startClean() tea.Cmd {
	// Capture values from pointers before closure
	inputPath := "secrets.json"
	if m.cleanInputPath != nil {
		inputPath = *m.cleanInputPath
	}
	repoPath := "."
	if m.cleanRepoPath != nil && *m.cleanRepoPath != "" {
		repoPath = *m.cleanRepoPath
	}
	tool := "auto"
	if m.cleanTool != nil {
		tool = *m.cleanTool
	}
	dryRun := m.cleanDryRun != nil && *m.cleanDryRun

	return func() tea.Msg {
		// Load secrets and detect source automatically
		var loadResult *cleaner.LoadSecretsResult
		var err error

		if strings.HasSuffix(inputPath, ".jsonl") {
			loadResult, err = cleaner.LoadSecretsFromJSONL(inputPath)
		} else {
			loadResult, err = cleaner.LoadSecretsFromJSON(inputPath)
		}

		if err != nil {
			return cleanDoneMsg{err: err}
		}

		c := cleaner.New()
		result, err := c.Clean(repoPath, loadResult.Secrets, cleaner.CleanOptions{
			Tool:      tool,
			Source:    loadResult.Source,    // Auto-detected from scan file
			FilePaths: loadResult.FileMap,   // Only clean files listed in scan results
			DryRun:    dryRun,
		})

		return cleanDoneMsg{result: result, err: err}
	}
}

func (m Model) updateCleanProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cleanDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		m.cleanResult = msg.result
		m.view = ViewCleanResults
		return m, nil

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m Model) viewCleanProgress() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🧹 Cleaning Repository"))
	sb.WriteString("\n\n")

	sb.WriteString(m.spinner.View())
	sb.WriteString(" Cleaning secrets...\n\n")

	sb.WriteString(warningStyle.Render("This may take a while for large repositories."))

	return boxStyle.Render(sb.String())
}

func (m Model) viewCleanResults() string {
	var sb strings.Builder

	if m.err != nil {
		sb.WriteString(titleStyle.Render("❌ Clean Failed"))
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	} else if result, ok := m.cleanResult.(*cleaner.CleanResult); ok {
		if result.Success {
			if result.DryRun {
				// Dry run results
				sb.WriteString(titleStyle.Render("🔍 Dry Run Results"))
				sb.WriteString("\n\n")
				sb.WriteString(warningStyle.Render("No changes were made. This is a preview."))
				sb.WriteString("\n\n")

				// Show source
				sourceLabel := "both (current files + git history)"
				if result.Source == "current" {
					sourceLabel = "current files only"
				} else if result.Source == "history" {
					sourceLabel = "git history only"
				}
				sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Target:"), sourceLabel))

				if result.Source != "current" {
					sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Tool:"), result.Tool))
				}
				sb.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Secrets to remove:"), result.SecretsRemoved))
				sb.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Patterns to use:"), result.PatternsUsed))

				// Show preview of secrets
				if len(result.PreviewSecrets) > 0 {
					sb.WriteString("\n" + keyStyle.Render("Preview of values to remove:") + "\n")
					for _, s := range result.PreviewSecrets {
						sb.WriteString(fmt.Sprintf("  • %s\n", maskedValueStyle.Render(s)))
					}
					if result.SecretsRemoved > len(result.PreviewSecrets) {
						sb.WriteString(fmt.Sprintf("  ... and %d more\n", result.SecretsRemoved-len(result.PreviewSecrets)))
					}
				}

				sb.WriteString("\n" + keyStyle.Render("To apply changes:") + "\n")
				sb.WriteString("  Run Clean again with 'Dry Run: No'\n")
			} else {
				// Actual clean results
				sb.WriteString(titleStyle.Render("✅ Clean Complete"))
				sb.WriteString("\n\n")

				// Show source
				sourceLabel := "both (current files + git history)"
				if result.Source == "current" {
					sourceLabel = "current files only"
				} else if result.Source == "history" {
					sourceLabel = "git history only"
				}
				sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Target:"), sourceLabel))

				if result.Source != "current" {
					sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Tool:"), result.Tool))
				}
				sb.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Secrets removed:"), result.SecretsRemoved))

				if result.Source == "current" || result.Source == "both" {
					sb.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Files modified:"), result.FilesModified))
				}
				if result.Source != "current" {
					sb.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Patterns used:"), result.PatternsUsed))
				}

				if result.BackupBranch != "" {
					sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("Backup branch:"), result.BackupBranch))
				}

				// Show appropriate next steps based on source
				sb.WriteString("\n" + warningStyle.Render("⚠️  Next steps:") + "\n")
				if result.Source == "current" {
					sb.WriteString("  1. Review changes: git diff\n")
					sb.WriteString("  2. Commit if satisfied: git add -A && git commit -m 'Remove secrets'\n")
					sb.WriteString("  3. Rotate all exposed credentials\n")
				} else {
					sb.WriteString("  1. Verify the cleaning: git log -p -S 'old_secret'\n")
					sb.WriteString("  2. Force push: git push --force --all\n")
					sb.WriteString("  3. Force push tags: git push --force --tags\n")
					sb.WriteString("  4. All collaborators must re-clone\n")
					sb.WriteString("  5. Rotate all exposed credentials\n")
				}
			}
		} else {
			sb.WriteString(titleStyle.Render("❌ Clean Failed"))
			sb.WriteString("\n\n")
			sb.WriteString(errorStyle.Render(result.Message))
		}
	}

	help := helpStyle.Render("esc: back to menu")
	sb.WriteString("\n\n" + help)

	if m.err != nil || (m.cleanResult != nil && !m.cleanResult.(*cleaner.CleanResult).Success) {
		return errorBoxStyle.Render(sb.String())
	}
	return successBoxStyle.Render(sb.String())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// saveResultToFile saves scan result to JSON file
func saveResultToFile(result *scanner.ScanResult, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
