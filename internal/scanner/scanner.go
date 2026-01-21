package scanner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Drilmo/git-secret-scanner/internal/config"
)

// Secret represents a found secret
type Secret struct {
	File             string         `json:"file"`
	Key              string         `json:"key"`
	Type             string         `json:"type"`
	ChangeCount      int            `json:"changeCount"`
	TotalOccurrences int            `json:"totalOccurrences"`
	Authors          []string       `json:"authors"`
	History          []SecretValue  `json:"history"`
}

// SecretValue represents a specific value of a secret
type SecretValue struct {
	Value       string   `json:"value"`
	MaskedValue string   `json:"maskedValue"`
	Commits     []string `json:"commits"`
	Authors     []string `json:"authors"`
	FirstSeen   string   `json:"firstSeen"`
	LastSeen    string   `json:"lastSeen"`
}

// ScanResult holds the complete scan results
type ScanResult struct {
	Repository   string    `json:"repository"`
	Branch       string    `json:"branch"`
	SecretsFound int       `json:"secretsFound"`
	TotalValues  int       `json:"totalValues"`
	Secrets      []Secret  `json:"secrets"`
	ScanDate     time.Time `json:"scanDate"`
}

// StreamEntry represents a single entry for streaming output
type StreamEntry struct {
	File        string `json:"file"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	MaskedValue string `json:"maskedValue"`
	Type        string `json:"type"`
	Commit      string `json:"commit"`
	Author      string `json:"author"`
	Date        string `json:"date"`
}

// ScanOptions holds scanning options
type ScanOptions struct {
	Branch        string
	ConfigPath    string
	MaxConcurrent int
	OnProgress    func(current, total, found int)
}

// Scanner performs git history scanning
type Scanner struct {
	config             *config.Config
	extractionPatterns []*config.CompiledPattern
}

// New creates a new Scanner
func New(cfg *config.Config) *Scanner {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Scanner{
		config:             cfg,
		extractionPatterns: cfg.GetCompiledPatterns(),
	}
}

// extractKeyValue tries all configured extraction patterns and returns the first match
func (s *Scanner) extractKeyValue(line string) (key, value string, found bool) {
	for _, pattern := range s.extractionPatterns {
		match := pattern.Regex.FindStringSubmatch(line)
		if match != nil && len(match) > pattern.ValueGroup {
			// Group 1 is typically the key, ValueGroup indicates which group contains the value
			key = strings.TrimSpace(match[1])
			value = strings.TrimSpace(match[pattern.ValueGroup])
			return key, value, true
		}
	}
	return "", "", false
}

// Scan performs a full scan of the repository
func (s *Scanner) Scan(repoPath string, opts ScanOptions) (*ScanResult, error) {
	if opts.Branch == "" {
		opts.Branch = "--all"
	}
	if opts.MaxConcurrent == 0 {
		opts.MaxConcurrent = 4
	}

	keywords := s.config.GetAllKeywords()
	secretsIndex := make(map[string]*secretData)
	var mu sync.Mutex
	var totalFound int

	// Process keywords in batches
	sem := make(chan struct{}, opts.MaxConcurrent)
	var wg sync.WaitGroup

	for i, keyword := range keywords {
		wg.Add(1)
		sem <- struct{}{}

		go func(kw string, idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			count := s.searchKeyword(repoPath, kw, opts.Branch, secretsIndex, &mu)

			mu.Lock()
			totalFound += count
			if opts.OnProgress != nil {
				opts.OnProgress(idx+1, len(keywords), totalFound)
			}
			mu.Unlock()
		}(keyword, i)
	}

	wg.Wait()

	// Build result
	secrets := s.buildSecrets(secretsIndex)

	return &ScanResult{
		Repository:   repoPath,
		Branch:       opts.Branch,
		SecretsFound: len(secrets),
		TotalValues:  countTotalValues(secrets),
		Secrets:      secrets,
		ScanDate:     time.Now(),
	}, nil
}

type secretData struct {
	file    string
	key     string
	keyType string
	authors map[string]bool
	values  map[string]*valueData
}

type valueData struct {
	commits   []string
	authors   map[string]bool
	firstSeen time.Time
	lastSeen  time.Time
}

func (s *Scanner) searchKeyword(repoPath, keyword, branch string, index map[string]*secretData, mu *sync.Mutex) int {
	args := []string{
		"log",
		branch,
		fmt.Sprintf("-S%s", keyword),
		"--pretty=format:COMMIT_START|%H|%an|%aI",
		"-p",
	}

	// Add file exclusions (all pathspecs after single --)
	if len(s.config.ExcludeBinaryExtensions) > 0 {
		args = append(args, "--")
		for _, ext := range s.config.ExcludeBinaryExtensions {
			args = append(args, fmt.Sprintf(":!*%s", ext))
		}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0
	}

	if err := cmd.Start(); err != nil {
		return 0
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var currentCommit *commitInfo
	var currentFile string
	var findings int

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "COMMIT_START|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) >= 4 {
				currentCommit = &commitInfo{
					hash:   parts[1],
					author: parts[2],
					date:   parts[3],
				}
				currentFile = ""
			}
			continue
		}

		if strings.HasPrefix(line, "diff --git") {
			if idx := strings.Index(line, " b/"); idx != -1 {
				currentFile = line[idx+3:]
				// Check if file should be ignored
				if s.config.ShouldIgnoreFile(currentFile) {
					currentFile = "" // Reset to skip this file
				}
			}
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && currentCommit != nil && currentFile != "" {
			content := line[1:]

			// Check if contains keyword
			searchIn := content
			searchFor := keyword
			if !s.config.Settings.CaseSensitive {
				searchIn = strings.ToLower(content)
				searchFor = strings.ToLower(keyword)
			}

			if !strings.Contains(searchIn, searchFor) {
				continue
			}

			// Extract key=value using configured patterns
			key, value, found := s.extractKeyValue(content)
			if !found {
				continue
			}

			if s.config.ShouldIgnoreValue(value) {
				continue
			}

			findings++
			secretKey := fmt.Sprintf("%s|%s", currentFile, key)

			mu.Lock()
			if _, exists := index[secretKey]; !exists {
				index[secretKey] = &secretData{
					file:    currentFile,
					key:     key,
					keyType: keyword,
					authors: make(map[string]bool),
					values:  make(map[string]*valueData),
				}
			}

			entry := index[secretKey]
			entry.authors[currentCommit.author] = true

			if _, exists := entry.values[value]; !exists {
				t, _ := time.Parse(time.RFC3339, currentCommit.date)
				entry.values[value] = &valueData{
					commits:   []string{},
					authors:   make(map[string]bool),
					firstSeen: t,
					lastSeen:  t,
				}
			}

			vd := entry.values[value]
			vd.commits = append(vd.commits, currentCommit.hash)
			vd.authors[currentCommit.author] = true

			t, _ := time.Parse(time.RFC3339, currentCommit.date)
			if t.Before(vd.firstSeen) {
				vd.firstSeen = t
			}
			if t.After(vd.lastSeen) {
				vd.lastSeen = t
			}
			mu.Unlock()
		}
	}

	cmd.Wait()
	return findings
}

type commitInfo struct {
	hash   string
	author string
	date   string
}

func (s *Scanner) buildSecrets(index map[string]*secretData) []Secret {
	secrets := make([]Secret, 0, len(index))

	for _, data := range index {
		history := make([]SecretValue, 0, len(data.values))

		for value, vd := range data.values {
			authors := make([]string, 0, len(vd.authors))
			for a := range vd.authors {
				authors = append(authors, a)
			}

			history = append(history, SecretValue{
				Value:       value,
				MaskedValue: maskSecret(value),
				Commits:     vd.commits,
				Authors:     authors,
				FirstSeen:   vd.firstSeen.Format(time.RFC3339),
				LastSeen:    vd.lastSeen.Format(time.RFC3339),
			})
		}

		// Sort by date
		sort.Slice(history, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, history[i].FirstSeen)
			tj, _ := time.Parse(time.RFC3339, history[j].FirstSeen)
			return ti.Before(tj)
		})

		authors := make([]string, 0, len(data.authors))
		for a := range data.authors {
			authors = append(authors, a)
		}

		totalOccurrences := 0
		for _, h := range history {
			totalOccurrences += len(h.Commits)
		}

		secrets = append(secrets, Secret{
			File:             data.file,
			Key:              data.key,
			Type:             data.keyType,
			ChangeCount:      len(history),
			TotalOccurrences: totalOccurrences,
			Authors:          authors,
			History:          history,
		})
	}

	// Sort by change count
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].ChangeCount > secrets[j].ChangeCount
	})

	return secrets
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

func countTotalValues(secrets []Secret) int {
	total := 0
	for _, s := range secrets {
		total += s.ChangeCount
	}
	return total
}

// ScanStream performs streaming scan to file
func (s *Scanner) ScanStream(repoPath, outputPath string, opts ScanOptions) (int, error) {
	if opts.Branch == "" {
		opts.Branch = "--all"
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Deduplication set: tracks seen (file|key|value) combinations
	seen := make(map[string]bool)

	keywords := s.config.GetAllKeywords()
	var count int

	for i, keyword := range keywords {
		c := s.streamKeyword(repoPath, keyword, opts.Branch, file, seen)
		count += c

		if opts.OnProgress != nil {
			opts.OnProgress(i+1, len(keywords), count)
		}
	}

	return count, nil
}

func (s *Scanner) streamKeyword(repoPath, keyword, branch string, file *os.File, seen map[string]bool) int {
	args := []string{
		"log",
		branch,
		fmt.Sprintf("-S%s", keyword),
		"--pretty=format:COMMIT|%H|%an|%aI",
		"-p",
	}

	// Add file exclusions (all pathspecs after single --)
	if len(s.config.ExcludeBinaryExtensions) > 0 {
		args = append(args, "--")
		for _, ext := range s.config.ExcludeBinaryExtensions {
			args = append(args, fmt.Sprintf(":!*%s", ext))
		}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0
	}

	if err := cmd.Start(); err != nil {
		return 0
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var currentCommit *commitInfo
	var currentFile string
	var count int

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "COMMIT|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) >= 4 {
				currentCommit = &commitInfo{
					hash:   parts[1],
					author: parts[2],
					date:   parts[3],
				}
				currentFile = ""
			}
			continue
		}

		if strings.HasPrefix(line, "diff --git") {
			if idx := strings.Index(line, " b/"); idx != -1 {
				currentFile = line[idx+3:]
				// Check if file should be ignored
				if s.config.ShouldIgnoreFile(currentFile) {
					currentFile = "" // Reset to skip this file
				}
			}
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && currentCommit != nil && currentFile != "" {
			content := line[1:]

			searchIn := content
			searchFor := keyword
			if !s.config.Settings.CaseSensitive {
				searchIn = strings.ToLower(content)
				searchFor = strings.ToLower(keyword)
			}

			if !strings.Contains(searchIn, searchFor) {
				continue
			}

			// Extract key=value using configured patterns
			key, value, found := s.extractKeyValue(content)
			if !found {
				continue
			}

			if s.config.ShouldIgnoreValue(value) {
				continue
			}

			// Deduplication: check if we've seen this (file|key|value) combination
			dedupeKey := fmt.Sprintf("%s|%s|%s", currentFile, key, value)
			if seen[dedupeKey] {
				continue
			}
			seen[dedupeKey] = true

			entry := StreamEntry{
				File:        currentFile,
				Key:         key,
				Value:       value,
				MaskedValue: maskSecret(value),
				Type:        keyword,
				Commit:      currentCommit.hash,
				Author:      currentCommit.author,
				Date:        currentCommit.date,
			}

			data, _ := json.Marshal(entry)
			file.WriteString(string(data) + "\n")
			count++
		}
	}

	cmd.Wait()
	return count
}

// GetAllValues extracts all unique secret values from scan result
func GetAllValues(scanResult *ScanResult) []string {
	values := make(map[string]bool)

	for _, secret := range scanResult.Secrets {
		for _, h := range secret.History {
			if h.Value != "" && !strings.Contains(h.Value, "REMOVED") {
				values[h.Value] = true
			}
		}
	}

	valueList := make([]string, 0, len(values))
	for v := range values {
		valueList = append(valueList, v)
	}

	// Sort by length (longest first) for cleaning
	sort.Slice(valueList, func(i, j int) bool {
		return len(valueList[i]) > len(valueList[j])
	})

	return valueList
}

// ScanCurrentStream scans current files and writes to JSONL file as it goes
func (s *Scanner) ScanCurrentStream(repoPath, outputPath string) (int, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Deduplication set: tracks seen (file|key|value) combinations
	seen := make(map[string]bool)

	keywords := s.config.GetAllKeywords()
	var count int

	for _, keyword := range keywords {
		c := s.streamCurrentFiles(repoPath, keyword, file, seen)
		count += c
	}

	return count, nil
}

func (s *Scanner) streamCurrentFiles(repoPath, keyword string, outFile *os.File, seen map[string]bool) int {
	var count int

	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			relPath = path
		}

		if s.config.ShouldIgnoreFile(relPath) {
			return nil
		}

		for _, ext := range s.config.ExcludeBinaryExtensions {
			if strings.HasSuffix(relPath, ext) {
				return nil
			}
		}

		if info.Size() > 1024*1024 {
			return nil
		}

		// Search file and write matches to JSONL
		c := s.streamFileMatches(relPath, path, keyword, outFile, seen)
		count += c
		return nil
	})

	return count
}

func (s *Scanner) streamFileMatches(relPath, fullPath, keyword string, outFile *os.File, seen map[string]bool) int {
	file, err := os.Open(fullPath)
	if err != nil {
		return 0
	}
	defer file.Close()

	fileScanner := bufio.NewScanner(file)
	keywordLower := strings.ToLower(keyword)
	var count int

	for fileScanner.Scan() {
		line := fileScanner.Text()

		if !s.config.Settings.CaseSensitive {
			if !strings.Contains(strings.ToLower(line), keywordLower) {
				continue
			}
		} else {
			if !strings.Contains(line, keyword) {
				continue
			}
		}

		// Extract key=value using configured patterns
		key, value, found := s.extractKeyValue(line)
		if !found {
			continue
		}

		if s.config.ShouldIgnoreValue(value) {
			continue
		}

		// Deduplication: check if we've seen this (file|key|value) combination
		dedupeKey := fmt.Sprintf("%s|%s|%s", relPath, key, value)
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true

		entry := StreamEntry{
			File:        relPath,
			Key:         key,
			Value:       value,
			MaskedValue: maskSecret(value),
			Type:        keyword,
			Commit:      "current",
			Author:      "current",
			Date:        time.Now().Format(time.RFC3339),
		}

		data, _ := json.Marshal(entry)
		outFile.WriteString(string(data) + "\n")
		count++
	}

	return count
}

// ScanCurrent scans only current files (no history) - fast mode
func (s *Scanner) ScanCurrent(repoPath string) (*ScanResult, error) {
	keywords := s.config.GetAllKeywords()
	secretsIndex := make(map[string]*secretData)

	for _, keyword := range keywords {
		s.grepCurrentFiles(repoPath, keyword, secretsIndex)
	}

	secrets := s.buildSecrets(secretsIndex)

	return &ScanResult{
		Repository:   repoPath,
		Branch:       "HEAD (current files)",
		SecretsFound: len(secrets),
		TotalValues:  countTotalValues(secrets),
		Secrets:      secrets,
		ScanDate:     time.Now(),
	}, nil
}

func (s *Scanner) grepCurrentFiles(repoPath, keyword string, index map[string]*secretData) {
	// Walk all files in the repository (including untracked files)
	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			// Skip .git directory
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			relPath = path
		}

		// Check if file should be ignored
		if s.config.ShouldIgnoreFile(relPath) {
			return nil
		}

		// Check if should exclude binary extensions
		for _, ext := range s.config.ExcludeBinaryExtensions {
			if strings.HasSuffix(relPath, ext) {
				return nil
			}
		}

		// Skip large files (> 1MB)
		if info.Size() > 1024*1024 {
			return nil
		}

		// Read and search file
		s.searchFileForKeyword(relPath, path, keyword, index)
		return nil
	})
}

func (s *Scanner) searchFileForKeyword(relPath, fullPath, keyword string, index map[string]*secretData) {
	file, err := os.Open(fullPath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	keywordLower := strings.ToLower(keyword)

	for scanner.Scan() {
		line := scanner.Text()

		// Check if line contains keyword (case insensitive)
		if !s.config.Settings.CaseSensitive {
			if !strings.Contains(strings.ToLower(line), keywordLower) {
				continue
			}
		} else {
			if !strings.Contains(line, keyword) {
				continue
			}
		}

		// Extract key=value using configured patterns
		key, value, found := s.extractKeyValue(line)
		if !found {
			continue
		}

		if s.config.ShouldIgnoreValue(value) {
			continue
		}

		secretKey := fmt.Sprintf("%s|%s", relPath, key)

		if _, exists := index[secretKey]; !exists {
			index[secretKey] = &secretData{
				file:    relPath,
				key:     key,
				keyType: keyword,
				authors: make(map[string]bool),
				values:  make(map[string]*valueData),
			}
		}

		entry := index[secretKey]
		entry.authors["current"] = true

		if _, exists := entry.values[value]; !exists {
			entry.values[value] = &valueData{
				commits:   []string{"current"},
				authors:   map[string]bool{"current": true},
				firstSeen: time.Now(),
				lastSeen:  time.Now(),
			}
		}
	}
}

// ScanBoth scans both current files and git history, combining results
func (s *Scanner) ScanBoth(repoPath string, opts ScanOptions) (*ScanResult, error) {
	// First scan current files
	currentResult, err := s.ScanCurrent(repoPath)
	if err != nil {
		return nil, err
	}

	// Then scan git history
	historyResult, err := s.Scan(repoPath, opts)
	if err != nil {
		return nil, err
	}

	// Merge results: combine secrets from both sources
	secretsMap := make(map[string]*Secret)

	// Add secrets from current files
	for i := range currentResult.Secrets {
		secret := &currentResult.Secrets[i]
		key := fmt.Sprintf("%s|%s", secret.File, secret.Key)
		secretsMap[key] = secret
	}

	// Add/merge secrets from history
	for i := range historyResult.Secrets {
		secret := &historyResult.Secrets[i]
		key := fmt.Sprintf("%s|%s", secret.File, secret.Key)

		if existing, ok := secretsMap[key]; ok {
			// Merge: add history values and authors
			existingValues := make(map[string]bool)
			for _, h := range existing.History {
				existingValues[h.Value] = true
			}

			for _, h := range secret.History {
				if !existingValues[h.Value] {
					existing.History = append(existing.History, h)
					existing.ChangeCount++
				}
			}

			for _, author := range secret.Authors {
				found := false
				for _, a := range existing.Authors {
					if a == author {
						found = true
						break
					}
				}
				if !found {
					existing.Authors = append(existing.Authors, author)
				}
			}
			existing.TotalOccurrences += secret.TotalOccurrences
		} else {
			secretsMap[key] = secret
		}
	}

	// Build result slice
	secrets := make([]Secret, 0, len(secretsMap))
	for _, secret := range secretsMap {
		secrets = append(secrets, *secret)
	}

	// Sort by change count
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].ChangeCount > secrets[j].ChangeCount
	})

	return &ScanResult{
		Repository:   repoPath,
		Branch:       fmt.Sprintf("%s + current files", opts.Branch),
		SecretsFound: len(secrets),
		TotalValues:  countTotalValues(secrets),
		Secrets:      secrets,
		ScanDate:     time.Now(),
	}, nil
}

// ScanBothStream scans both current files and git history to JSONL
func (s *Scanner) ScanBothStream(repoPath, outputPath string, opts ScanOptions) (int, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Deduplication set: tracks seen (file|key|value) combinations
	seen := make(map[string]bool)

	var count int

	// First scan current files
	keywords := s.config.GetAllKeywords()
	for _, keyword := range keywords {
		c := s.streamCurrentFiles(repoPath, keyword, file, seen)
		count += c
	}

	// Then scan git history
	if opts.Branch == "" {
		opts.Branch = "--all"
	}

	for _, keyword := range keywords {
		c := s.streamKeyword(repoPath, keyword, opts.Branch, file, seen)
		count += c
	}

	return count, nil
}
