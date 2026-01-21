package tui

import (
	"os/exec"

	"github.com/charmbracelet/huh"
)

func (m *Model) createScanForm() *huh.Form {
	// Allocate pointers for values (shared across Model copies)
	// This is necessary because Bubble Tea copies the Model on each Update
	if m.scanRepoPath == nil {
		repoPath := "."
		m.scanRepoPath = &repoPath
	}
	if m.scanMode == nil {
		mode := "full"
		m.scanMode = &mode
	}
	if m.scanBranch == nil {
		branch := "--all"
		m.scanBranch = &branch
	}
	if m.scanSource == nil {
		source := "both"
		m.scanSource = &source
	}
	if m.scanOutputPath == nil {
		outputPath := "secrets.json"
		m.scanOutputPath = &outputPath
	}
	// Use the selected config path
	m.scanConfigPath = m.configPath

	// Allocate pointer for confirm (shared across Model copies)
	// Default to false (Cancel) - user must explicitly choose to start
	confirm := false
	m.scanConfirm = &confirm

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Repository Path").
				Description("Path to the git repository to scan").
				Value(m.scanRepoPath),

			huh.NewSelect[string]().
				Title("Scan Mode").
				Description("Choose the scanning method").
				Options(
					huh.NewOption("Full (all history, aggregated)", "full"),
					huh.NewOption("Stream (large repos, writes to file)", "stream"),
					huh.NewOption("Fast (quick check)", "fast"),
				).
				Value(m.scanMode),

			huh.NewSelect[string]().
				Title("Source").
				Description("What to scan").
				Options(
					huh.NewOption("Both (current files + git history)", "both"),
					huh.NewOption("Current files (HEAD + untracked)", "current"),
					huh.NewOption("Git history only", "history"),
				).
				Value(m.scanSource),

			huh.NewInput().
				Title("Branch").
				Description("Branch to scan (for git history)").
				Value(m.scanBranch),

			huh.NewInput().
				Title("Output File").
				Description("Where to save the results").
				Value(m.scanOutputPath),

			huh.NewConfirm().
				Title("Start Scan?").
				Affirmative("Start").
				Negative("Cancel").
				Value(m.scanConfirm),
		),
	).WithTheme(huh.ThemeDracula())
}

func (m *Model) createAnalyzeForm() *huh.Form {
	// Allocate pointers for values (shared across Model copies)
	if m.analyzeInputPath == nil {
		inputPath := "secrets.json"
		m.analyzeInputPath = &inputPath
	}
	if m.analyzeOutputPath == nil {
		outputPath := "secrets_analysis.csv"
		m.analyzeOutputPath = &outputPath
	}
	// Allocate pointer for confirm (shared across Model copies)
	// Default to false (Cancel) - user must explicitly choose to start
	confirm := false
	m.analyzeConfirm = &confirm

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Input File").
				Description("JSONL file from scan-stream or JSON from scan").
				Value(m.analyzeInputPath),

			huh.NewInput().
				Title("CSV Output File").
				Description("Where to save the CSV report for statistics").
				Value(m.analyzeOutputPath),

			huh.NewConfirm().
				Title("Start Analysis?").
				Affirmative("Analyze").
				Negative("Cancel").
				Value(m.analyzeConfirm),
		),
	).WithTheme(huh.ThemeDracula())
}

func (m *Model) createCleanForm() *huh.Form {
	// Allocate pointers for values (shared across Model copies)
	if m.cleanInputPath == nil {
		inputPath := "secrets.json"
		m.cleanInputPath = &inputPath
	}
	if m.cleanRepoPath == nil {
		repoPath := "."
		m.cleanRepoPath = &repoPath
	}
	if m.cleanTool == nil {
		tool := "auto"
		m.cleanTool = &tool
	}
	// Allocate pointers for confirm values (shared across Model copies)
	// Default dryRun to true for safety, but confirm to false (Cancel)
	dryRun := true
	m.cleanDryRun = &dryRun
	confirm := false
	m.cleanConfirm = &confirm

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Scan Results File").
				Description("JSON or JSONL file with secrets to remove").
				Value(m.cleanInputPath),

			huh.NewInput().
				Title("Repository Path").
				Description("Path to the git repository to clean").
				Value(m.cleanRepoPath),

			huh.NewSelect[string]().
				Title("History Tool").
				Description("Tool to rewrite git history (if applicable)").
				Options(
					huh.NewOption("Auto (best available)", "auto"),
					huh.NewOption("git-filter-repo (recommended)", "filter-repo"),
					huh.NewOption("BFG Repo Cleaner", "bfg"),
					huh.NewOption("git-filter-branch (slow)", "filter-branch"),
				).
				Value(m.cleanTool),

			huh.NewConfirm().
				Title("Dry Run?").
				Description("Simulate without making changes").
				Affirmative("Yes, dry run first").
				Negative("No, clean directly").
				Value(m.cleanDryRun),

			huh.NewConfirm().
				Title("Proceed?").
				Affirmative("Continue").
				Negative("Cancel").
				Value(m.cleanConfirm),
		),
	).WithTheme(huh.ThemeDracula())
}

func (m *Model) createCleanConfirmForm() *huh.Form {
	// Allocate pointer for confirm (shared across Model copies)
	confirm := false // Default to false for safety
	m.cleanConfirm = &confirm
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("⚠️  WARNING: This will rewrite git history!").
				Description("This operation cannot be undone. Make sure you have a backup.\nAll collaborators will need to re-clone the repository.").
				Affirmative("Yes, clean history").
				Negative("Cancel").
				Value(m.cleanConfirm),
		),
	).WithTheme(huh.ThemeDracula())
}

// Helper functions
func hasFilterRepo() bool {
	cmd := exec.Command("git", "filter-repo", "--version")
	return cmd.Run() == nil
}

func hasBFG() bool {
	cmd := exec.Command("bfg", "--version")
	if cmd.Run() == nil {
		return true
	}
	cmd = exec.Command("java", "-jar", "bfg.jar", "--version")
	return cmd.Run() == nil
}
