package cleaner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	scannerPkg "github.com/Drilmo/git-secret-scanner/internal/scanner"
)

// CleanOptions holds cleaning options
type CleanOptions struct {
	Tool       string          // auto, filter-repo, bfg, filter-branch
	Source     string          // current, history, both
	FilePaths  map[string]bool // Files to clean (only these files will be modified for current files)
	DryRun     bool
	Force      bool
	NoBackup   bool
	OnProgress func(step, total int, message string)
}

// CleanResult holds cleaning results
type CleanResult struct {
	Tool           string
	Source         string
	SecretsRemoved int
	PatternsUsed   int
	FilesModified  int // Number of current files modified
	Success        bool
	Message        string
	BackupBranch   string
	DryRun         bool
	PreviewSecrets []string // First few secrets (masked) for preview
}

// Cleaner performs git history cleaning
type Cleaner struct{}

// New creates a new Cleaner
func New() *Cleaner {
	return &Cleaner{}
}

// HasFilterRepo checks if git-filter-repo is installed
func HasFilterRepo() bool {
	cmd := exec.Command("git", "filter-repo", "--version")
	return cmd.Run() == nil
}

// HasBFG checks if BFG is installed
func HasBFG() bool {
	cmd := exec.Command("bfg", "--version")
	if cmd.Run() == nil {
		return true
	}
	// Try java -jar bfg.jar
	cmd = exec.Command("java", "-jar", "bfg.jar", "--version")
	return cmd.Run() == nil
}

// GetAvailableTools returns list of available cleaning tools
func GetAvailableTools() map[string]bool {
	return map[string]bool{
		"filter-repo":   HasFilterRepo(),
		"bfg":           HasBFG(),
		"filter-branch": true, // Always available
	}
}

// Clean performs the cleaning operation
func (c *Cleaner) Clean(repoPath string, secrets []string, opts CleanOptions) (*CleanResult, error) {
	if len(secrets) == 0 {
		return &CleanResult{
			Success: true,
			Message: "No secrets to clean",
			DryRun:  opts.DryRun,
		}, nil
	}

	// Default source to "both"
	source := opts.Source
	if source == "" {
		source = "both"
	}

	// Select tool for history cleaning
	tool := opts.Tool
	if tool == "" || tool == "auto" {
		tool = selectBestTool()
	}

	// Group secrets into patterns
	patterns := groupSecretsIntoPatterns(secrets)

	// For dry run, prepare preview and return early
	if opts.DryRun {
		preview := make([]string, 0, min(10, len(secrets)))
		for i, s := range secrets {
			if i >= 10 {
				break
			}
			preview = append(preview, maskSecret(s))
		}

		var msg string
		switch source {
		case "current":
			msg = fmt.Sprintf("[DRY-RUN] Would clean %d secrets in current files only", len(secrets))
		case "history":
			msg = fmt.Sprintf("[DRY-RUN] Would clean %d secrets in git history using %s", len(secrets), tool)
		default:
			msg = fmt.Sprintf("[DRY-RUN] Would clean %d secrets in current files + git history using %s", len(secrets), tool)
		}

		return &CleanResult{
			Tool:           tool,
			Source:         source,
			SecretsRemoved: len(secrets),
			PatternsUsed:   len(patterns),
			Success:        true,
			Message:        msg,
			DryRun:         true,
			PreviewSecrets: preview,
		}, nil
	}

	// Create backup unless disabled (only for history cleaning)
	var backupBranch string
	if !opts.NoBackup && (source == "history" || source == "both") {
		backupBranch = fmt.Sprintf("backup-before-clean-%d", os.Getpid())
		cmd := exec.Command("git", "branch", backupBranch)
		cmd.Dir = repoPath
		cmd.Run()
	}

	var result *CleanResult
	var err error
	var filesModified int

	// Clean current files if needed
	if source == "current" || source == "both" {
		if opts.OnProgress != nil {
			opts.OnProgress(1, 3, "Cleaning current files...")
		}
		filesModified, err = c.cleanCurrentFiles(repoPath, secrets, opts.FilePaths)
		if err != nil {
			return &CleanResult{
				Success: false,
				Source:  source,
				Message: fmt.Sprintf("Failed to clean current files: %v", err),
			}, nil
		}
	}

	// Clean git history if needed
	if source == "history" || source == "both" {
		if opts.OnProgress != nil {
			step := 1
			if source == "both" {
				step = 2
			}
			opts.OnProgress(step, 3, fmt.Sprintf("Cleaning git history using %s with %d patterns", tool, len(patterns)))
		}

		switch tool {
		case "filter-repo":
			result, err = c.cleanWithFilterRepo(repoPath, patterns, opts)
		case "bfg":
			result, err = c.cleanWithBFG(repoPath, secrets, opts)
		default:
			result, err = c.cleanWithFilterBranch(repoPath, patterns, opts)
		}

		if err != nil {
			return nil, err
		}

		// Run git gc after history rewrite
		if result.Success {
			if opts.OnProgress != nil {
				opts.OnProgress(3, 3, "Running git gc...")
			}
			cmd := exec.Command("git", "reflog", "expire", "--expire=now", "--all")
			cmd.Dir = repoPath
			cmd.Run()

			cmd = exec.Command("git", "gc", "--prune=now", "--aggressive")
			cmd.Dir = repoPath
			cmd.Run()
		}
	} else {
		// Current files only - create simple success result
		result = &CleanResult{
			Success: true,
			Message: fmt.Sprintf("Successfully cleaned %d files", filesModified),
		}
	}

	result.Tool = tool
	result.Source = source
	result.SecretsRemoved = len(secrets)
	result.PatternsUsed = len(patterns)
	result.FilesModified = filesModified
	result.BackupBranch = backupBranch
	result.DryRun = false

	// Update message based on source
	if result.Success {
		switch source {
		case "current":
			result.Message = fmt.Sprintf("Successfully cleaned %d secrets in %d files (current files only)", len(secrets), filesModified)
		case "history":
			result.Message = fmt.Sprintf("Successfully cleaned %d secrets in git history using %s", len(secrets), tool)
		default:
			result.Message = fmt.Sprintf("Successfully cleaned %d secrets in %d files + git history using %s", len(secrets), filesModified, tool)
		}
	}

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maskSecret(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	maskLen := len(value) - 4
	if maskLen > 16 {
		maskLen = 16
	}
	return value[:2] + strings.Repeat("*", maskLen) + value[len(value)-2:]
}

func selectBestTool() string {
	if HasFilterRepo() {
		return "filter-repo"
	}
	if HasBFG() {
		return "bfg"
	}
	return "filter-branch"
}

// Group secrets into regex patterns (max 100 per pattern)
func groupSecretsIntoPatterns(secrets []string) []string {
	// Sort by length (longest first)
	sorted := make([]string, len(secrets))
	copy(sorted, secrets)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	var patterns []string
	batchSize := 100

	for i := 0; i < len(sorted); i += batchSize {
		end := i + batchSize
		if end > len(sorted) {
			end = len(sorted)
		}

		batch := sorted[i:end]
		escaped := make([]string, len(batch))
		for j, s := range batch {
			escaped[j] = regexp.QuoteMeta(s)
		}

		pattern := "(" + strings.Join(escaped, "|") + ")"
		patterns = append(patterns, pattern)
	}

	return patterns
}

// cleanCurrentFiles replaces secrets in current files without rewriting git history
// Only files listed in allowedFiles will be modified (if nil, no files are modified)
func (c *Cleaner) cleanCurrentFiles(repoPath string, secrets []string, allowedFiles map[string]bool) (int, error) {
	filesModified := 0

	// If no allowed files specified, don't modify anything
	if allowedFiles == nil || len(allowedFiles) == 0 {
		return 0, nil
	}

	// Only process files that are in the allowed list
	for filePath := range allowedFiles {
		// Build full path
		fullPath := filepath.Join(repoPath, filePath)

		// Check if file exists
		info, err := os.Stat(fullPath)
		if err != nil {
			continue // Skip files that don't exist
		}

		// Skip directories
		if info.IsDir() {
			continue
		}

		// Skip large files (> 1MB)
		if info.Size() > 1024*1024 {
			continue
		}

		// Read file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		// Check if file contains any secrets and replace them
		modified := false
		contentStr := string(content)
		for _, secret := range secrets {
			if strings.Contains(contentStr, secret) {
				contentStr = strings.ReplaceAll(contentStr, secret, "***REMOVED***")
				modified = true
			}
		}

		// Write back if modified
		if modified {
			if err := os.WriteFile(fullPath, []byte(contentStr), info.Mode()); err != nil {
				continue
			}
			filesModified++
		}
	}

	return filesModified, nil
}

func (c *Cleaner) cleanWithFilterRepo(repoPath string, patterns []string, opts CleanOptions) (*CleanResult, error) {
	// Create replacements file
	replacementsFile := fmt.Sprintf("/tmp/replacements-%d.txt", os.Getpid())
	f, err := os.Create(replacementsFile)
	if err != nil {
		return nil, err
	}

	for _, pattern := range patterns {
		f.WriteString(fmt.Sprintf("regex:%s==>***REMOVED***\n", pattern))
	}
	f.Close()
	defer os.Remove(replacementsFile)

	args := []string{"filter-repo", "--replace-text", replacementsFile}
	if opts.Force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return &CleanResult{
			Success: false,
			Message: fmt.Sprintf("git-filter-repo failed: %v", err),
		}, nil
	}

	return &CleanResult{
		Success: true,
		Message: "Successfully cleaned with git-filter-repo",
	}, nil
}

func (c *Cleaner) cleanWithBFG(repoPath string, secrets []string, opts CleanOptions) (*CleanResult, error) {
	// Create replacements file
	replacementsFile := fmt.Sprintf("/tmp/bfg-replacements-%d.txt", os.Getpid())
	f, err := os.Create(replacementsFile)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		f.WriteString(secret + "\n")
	}
	f.Close()
	defer os.Remove(replacementsFile)

	cmd := exec.Command("bfg", "--replace-text", replacementsFile, repoPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return &CleanResult{
			Success: false,
			Message: fmt.Sprintf("BFG failed: %v", err),
		}, nil
	}

	return &CleanResult{
		Success: true,
		Message: "Successfully cleaned with BFG",
	}, nil
}

func (c *Cleaner) cleanWithFilterBranch(repoPath string, patterns []string, opts CleanOptions) (*CleanResult, error) {
	// Build sed command
	sedParts := make([]string, len(patterns))
	for i, pattern := range patterns {
		sedParts[i] = fmt.Sprintf("s/%s/***REMOVED***/g", pattern)
	}
	sedCommand := strings.Join(sedParts, "; ")

	filterCommand := fmt.Sprintf(`git ls-files -z | xargs -0 sed -i '' '%s' 2>/dev/null || true`, sedCommand)

	cmd := exec.Command("git", "filter-branch", "-f", "--tree-filter", filterCommand, "--", "--all")
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return &CleanResult{
			Success: false,
			Message: fmt.Sprintf("git-filter-branch failed: %v", err),
		}, nil
	}

	return &CleanResult{
		Success: true,
		Message: "Successfully cleaned with git-filter-branch (slow method)",
	}, nil
}

// LoadSecretsResult holds secrets and detected source
type LoadSecretsResult struct {
	Secrets   []string
	FilePaths []string          // List of file paths containing secrets
	FileMap   map[string]bool   // Map of file paths for quick lookup
	Source    string            // "current", "history", or "both"
}

// LoadSecretsFromJSONL loads secrets from a JSONL file and detects source
func LoadSecretsFromJSONL(path string) (*LoadSecretsResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]bool)
	filePaths := make(map[string]bool)
	hasCurrent := false
	hasHistory := false
	fileScanner := bufio.NewScanner(file)

	for fileScanner.Scan() {
		var entry scannerPkg.StreamEntry
		if err := json.Unmarshal(fileScanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Value != "" && !strings.Contains(entry.Value, "REMOVED") {
			values[entry.Value] = true

			// Track file path for current files
			if entry.File != "" {
				filePaths[entry.File] = true
			}

			// Detect source from commit field
			if entry.Commit == "current" {
				hasCurrent = true
			} else if entry.Commit != "" {
				hasHistory = true
			}
		}
	}

	secrets := make([]string, 0, len(values))
	for v := range values {
		secrets = append(secrets, v)
	}

	paths := make([]string, 0, len(filePaths))
	for p := range filePaths {
		paths = append(paths, p)
	}

	// Determine source
	source := "both"
	if hasCurrent && !hasHistory {
		source = "current"
	} else if hasHistory && !hasCurrent {
		source = "history"
	}

	return &LoadSecretsResult{
		Secrets:   secrets,
		FilePaths: paths,
		FileMap:   filePaths,
		Source:    source,
	}, nil
}

// LoadSecretsFromJSON loads secrets from a JSON scan result file and detects source
func LoadSecretsFromJSON(path string) (*LoadSecretsResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result scannerPkg.ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// Detect source from commits in history and collect file paths
	hasCurrent := false
	hasHistory := false
	filePaths := make(map[string]bool)

	for _, secret := range result.Secrets {
		// Track file path
		if secret.File != "" {
			filePaths[secret.File] = true
		}

		for _, h := range secret.History {
			for _, commit := range h.Commits {
				if commit == "current" {
					hasCurrent = true
				} else if commit != "" {
					hasHistory = true
				}
			}
		}
	}

	paths := make([]string, 0, len(filePaths))
	for p := range filePaths {
		paths = append(paths, p)
	}

	// Determine source
	source := "both"
	if hasCurrent && !hasHistory {
		source = "current"
	} else if hasHistory && !hasCurrent {
		source = "history"
	}

	return &LoadSecretsResult{
		Secrets:   scannerPkg.GetAllValues(&result),
		FilePaths: paths,
		FileMap:   filePaths,
		Source:    source,
	}, nil
}
