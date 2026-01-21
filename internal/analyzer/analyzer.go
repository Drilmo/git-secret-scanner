package analyzer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Analysis holds the complete analysis results
type Analysis struct {
	Stats   Stats    `json:"stats"`
	Secrets []Secret `json:"secrets"`
}

// Stats holds global statistics
type Stats struct {
	TotalEntries  int           `json:"totalEntries"`
	UniqueSecrets int           `json:"uniqueSecrets"`
	UniqueValues  int           `json:"uniqueValues"`
	TopAuthors    []AuthorStat  `json:"topAuthors"`
	TopFiles      []FileStat    `json:"topFiles"`
	TypeBreakdown []TypeStat    `json:"typeBreakdown"`
}

// AuthorStat represents author statistics
type AuthorStat struct {
	Author string `json:"author"`
	Count  int    `json:"count"`
}

// FileStat represents file statistics
type FileStat struct {
	File  string `json:"file"`
	Count int    `json:"count"`
}

// TypeStat represents type statistics
type TypeStat struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// Secret represents an analyzed secret
type Secret struct {
	File             string        `json:"file"`
	Key              string        `json:"key"`
	Type             string        `json:"type"`
	ChangeCount      int           `json:"changeCount"`
	TotalOccurrences int           `json:"totalOccurrences"`
	Authors          []string      `json:"authors"`
	FirstSeen        string        `json:"firstSeen"`
	LastSeen         string        `json:"lastSeen"`
	History          []ValueEntry  `json:"history"`
}

// ValueEntry represents a value in the history
type ValueEntry struct {
	Value       string   `json:"value"`
	MaskedValue string   `json:"maskedValue"`
	Occurrences int      `json:"occurrences"`
	Authors     []string `json:"authors"`
	FirstSeen   string   `json:"firstSeen"`
	LastSeen    string   `json:"lastSeen"`
}

// StreamEntry represents a single entry from JSONL
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

// AnalyzeOptions holds analysis options
type AnalyzeOptions struct {
	ShowValues bool
	MaxSecrets int
	OnProgress func(lines int)
}

// Analyzer performs analysis on scan results
type Analyzer struct{}

// New creates a new Analyzer
func New() *Analyzer {
	return &Analyzer{}
}

// ScanResult represents the structure of a JSON scan result file
type ScanResult struct {
	Repository   string       `json:"repository"`
	Branch       string       `json:"branch"`
	SecretsFound int          `json:"secretsFound"`
	TotalValues  int          `json:"totalValues"`
	Secrets      []ScanSecret `json:"secrets"`
	ScanDate     string       `json:"scanDate"`
}

// ScanSecret represents a secret from the scan result
type ScanSecret struct {
	File             string           `json:"file"`
	Key              string           `json:"key"`
	Type             string           `json:"type"`
	ChangeCount      int              `json:"changeCount"`
	TotalOccurrences int              `json:"totalOccurrences"`
	Authors          []string         `json:"authors"`
	History          []ScanValueEntry `json:"history"`
}

// ScanValueEntry represents a value entry from the scan result
type ScanValueEntry struct {
	Value       string   `json:"value"`
	MaskedValue string   `json:"maskedValue"`
	Commits     []string `json:"commits"`
	Authors     []string `json:"authors"`
	FirstSeen   string   `json:"firstSeen"`
	LastSeen    string   `json:"lastSeen"`
}

// AnalyzeJSON analyzes a JSON scan result file
func (a *Analyzer) AnalyzeJSON(inputPath string, opts AnalyzeOptions) (*Analysis, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}

	var scanResult ScanResult
	if err := json.Unmarshal(data, &scanResult); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// Build stats
	stats := Stats{
		TotalEntries:  scanResult.TotalValues,
		UniqueSecrets: scanResult.SecretsFound,
		UniqueValues:  0,
		TopAuthors:    []AuthorStat{},
		TopFiles:      []FileStat{},
		TypeBreakdown: []TypeStat{},
	}

	// Count stats
	authorCounts := make(map[string]int)
	fileCounts := make(map[string]int)
	typeCounts := make(map[string]int)

	secrets := make([]Secret, 0, len(scanResult.Secrets))
	for _, s := range scanResult.Secrets {
		// Count file
		fileCounts[s.File]++

		// Count type
		typeCounts[s.Type]++

		// Count authors
		for _, author := range s.Authors {
			authorCounts[author]++
		}

		// Build history
		history := make([]ValueEntry, 0, len(s.History))
		firstSeen := ""
		lastSeen := ""
		for _, h := range s.History {
			history = append(history, ValueEntry{
				Value:       h.Value,
				MaskedValue: h.MaskedValue,
				Occurrences: len(h.Commits),
				Authors:     h.Authors,
				FirstSeen:   h.FirstSeen,
				LastSeen:    h.LastSeen,
			})
			if firstSeen == "" || compareDates(h.FirstSeen, firstSeen) < 0 {
				firstSeen = h.FirstSeen
			}
			if lastSeen == "" || compareDates(h.LastSeen, lastSeen) > 0 {
				lastSeen = h.LastSeen
			}
		}

		stats.UniqueValues += len(history)

		secrets = append(secrets, Secret{
			File:             s.File,
			Key:              s.Key,
			Type:             s.Type,
			ChangeCount:      s.ChangeCount,
			TotalOccurrences: s.TotalOccurrences,
			Authors:          s.Authors,
			FirstSeen:        firstSeen,
			LastSeen:         lastSeen,
			History:          history,
		})
	}

	// Sort and limit stats
	stats.TopAuthors = sortMapToStats(authorCounts, 10)
	stats.TopFiles = sortMapToFileStats(fileCounts, 10)
	stats.TypeBreakdown = sortMapToTypeStats(typeCounts)

	// Sort secrets by change count
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].ChangeCount > secrets[j].ChangeCount
	})

	return &Analysis{
		Stats:   stats,
		Secrets: secrets,
	}, nil
}

// AnalyzeJSONL analyzes a JSONL file
func (a *Analyzer) AnalyzeJSONL(inputPath string, opts AnalyzeOptions) (*Analysis, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Index by key (file + key)
	secretsIndex := make(map[string]*secretData)
	stats := &statsData{
		authors: make(map[string]int),
		files:   make(map[string]int),
		types:   make(map[string]int),
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		if lineCount%1000 == 0 && opts.OnProgress != nil {
			opts.OnProgress(lineCount)
		}

		var entry StreamEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		stats.totalEntries++
		secretKey := fmt.Sprintf("%s|%s", entry.File, entry.Key)

		// Index secret
		if _, exists := secretsIndex[secretKey]; !exists {
			secretsIndex[secretKey] = &secretData{
				file:      entry.File,
				key:       entry.Key,
				secretType: entry.Type,
				values:    make(map[string]*valueData),
				authors:   make(map[string]bool),
				firstSeen: entry.Date,
				lastSeen:  entry.Date,
			}
		}

		secret := secretsIndex[secretKey]

		// Add value
		if _, exists := secret.values[entry.Value]; !exists {
			secret.values[entry.Value] = &valueData{
				count:     0,
				authors:   make(map[string]bool),
				firstSeen: entry.Date,
				lastSeen:  entry.Date,
			}
		}

		vd := secret.values[entry.Value]
		vd.count++
		vd.authors[entry.Author] = true

		// Update dates
		if compareDates(entry.Date, vd.firstSeen) < 0 {
			vd.firstSeen = entry.Date
		}
		if compareDates(entry.Date, vd.lastSeen) > 0 {
			vd.lastSeen = entry.Date
		}

		// Update secret
		secret.authors[entry.Author] = true
		if compareDates(entry.Date, secret.firstSeen) < 0 {
			secret.firstSeen = entry.Date
		}
		if compareDates(entry.Date, secret.lastSeen) > 0 {
			secret.lastSeen = entry.Date
		}

		// Global stats
		stats.authors[entry.Author]++
		stats.files[entry.File]++
		stats.types[entry.Type]++
	}

	// Build result
	return a.buildAnalysis(secretsIndex, stats), nil
}

type secretData struct {
	file       string
	key        string
	secretType string
	values     map[string]*valueData
	authors    map[string]bool
	firstSeen  string
	lastSeen   string
}

type valueData struct {
	count     int
	authors   map[string]bool
	firstSeen string
	lastSeen  string
}

type statsData struct {
	totalEntries int
	authors      map[string]int
	files        map[string]int
	types        map[string]int
}

func (a *Analyzer) buildAnalysis(index map[string]*secretData, stats *statsData) *Analysis {
	secrets := make([]Secret, 0, len(index))

	for _, data := range index {
		history := make([]ValueEntry, 0, len(data.values))

		for value, vd := range data.values {
			authors := make([]string, 0, len(vd.authors))
			for author := range vd.authors {
				authors = append(authors, author)
			}

			history = append(history, ValueEntry{
				Value:       value,
				MaskedValue: maskSecret(value),
				Occurrences: vd.count,
				Authors:     authors,
				FirstSeen:   vd.firstSeen,
				LastSeen:    vd.lastSeen,
			})
		}

		// Sort by date
		sort.Slice(history, func(i, j int) bool {
			return compareDates(history[i].FirstSeen, history[j].FirstSeen) < 0
		})

		authors := make([]string, 0, len(data.authors))
		for author := range data.authors {
			authors = append(authors, author)
		}

		totalOccurrences := 0
		for _, h := range history {
			totalOccurrences += h.Occurrences
		}

		secrets = append(secrets, Secret{
			File:             data.file,
			Key:              data.key,
			Type:             data.secretType,
			ChangeCount:      len(history),
			TotalOccurrences: totalOccurrences,
			Authors:          authors,
			FirstSeen:        data.firstSeen,
			LastSeen:         data.lastSeen,
			History:          history,
		})
	}

	// Sort by change count
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].ChangeCount > secrets[j].ChangeCount
	})

	// Build stats
	topAuthors := sortMapToStats(stats.authors, 10)
	topFiles := sortMapToFileStats(stats.files, 10)
	typeBreakdown := sortMapToTypeStats(stats.types)

	uniqueValues := 0
	for _, s := range secrets {
		uniqueValues += s.ChangeCount
	}

	return &Analysis{
		Stats: Stats{
			TotalEntries:  stats.totalEntries,
			UniqueSecrets: len(secrets),
			UniqueValues:  uniqueValues,
			TopAuthors:    topAuthors,
			TopFiles:      topFiles,
			TypeBreakdown: typeBreakdown,
		},
		Secrets: secrets,
	}
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

func compareDates(a, b string) int {
	ta, _ := time.Parse(time.RFC3339, a)
	tb, _ := time.Parse(time.RFC3339, b)
	if ta.Before(tb) {
		return -1
	}
	if ta.After(tb) {
		return 1
	}
	return 0
}

func sortMapToStats(m map[string]int, limit int) []AuthorStat {
	type kv struct {
		key   string
		value int
	}

	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].value > sorted[j].value
	})

	result := make([]AuthorStat, 0, limit)
	for i, kv := range sorted {
		if i >= limit {
			break
		}
		result = append(result, AuthorStat{Author: kv.key, Count: kv.value})
	}
	return result
}

func sortMapToFileStats(m map[string]int, limit int) []FileStat {
	type kv struct {
		key   string
		value int
	}

	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].value > sorted[j].value
	})

	result := make([]FileStat, 0, limit)
	for i, kv := range sorted {
		if i >= limit {
			break
		}
		result = append(result, FileStat{File: kv.key, Count: kv.value})
	}
	return result
}

func sortMapToTypeStats(m map[string]int) []TypeStat {
	type kv struct {
		key   string
		value int
	}

	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].value > sorted[j].value
	})

	result := make([]TypeStat, 0, len(sorted))
	for _, kv := range sorted {
		result = append(result, TypeStat{Type: kv.key, Count: kv.value})
	}
	return result
}

// GenerateReport generates a text report
func GenerateReport(analysis *Analysis, showValues bool, maxSecrets int) string {
	var sb strings.Builder

	sb.WriteString(strings.Repeat("═", 80) + "\n")
	sb.WriteString("                     RAPPORT D'ANALYSE DES SECRETS\n")
	sb.WriteString(strings.Repeat("═", 80) + "\n\n")

	// Global stats
	sb.WriteString("STATISTIQUES GLOBALES\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")
	sb.WriteString(fmt.Sprintf("  Entrées analysées:     %d\n", analysis.Stats.TotalEntries))
	sb.WriteString(fmt.Sprintf("  Secrets uniques:       %d\n", analysis.Stats.UniqueSecrets))
	sb.WriteString(fmt.Sprintf("  Valeurs différentes:   %d\n\n", analysis.Stats.UniqueValues))

	// Top authors
	sb.WriteString("TOP AUTEURS (qui modifie le plus de secrets)\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")
	for _, stat := range analysis.Stats.TopAuthors {
		bar := strings.Repeat("█", min(stat.Count*50/max(analysis.Stats.TotalEntries, 1), 30))
		sb.WriteString(fmt.Sprintf("  %-25s %5d %s\n", stat.Author, stat.Count, bar))
	}
	sb.WriteString("\n")

	// Top files
	sb.WriteString("TOP FICHIERS (les plus impactés)\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")
	for _, stat := range analysis.Stats.TopFiles {
		file := stat.File
		if len(file) > 50 {
			file = file[:50]
		}
		sb.WriteString(fmt.Sprintf("  %-50s %d\n", file, stat.Count))
	}
	sb.WriteString("\n")

	// Types
	sb.WriteString("TYPES DE SECRETS\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")
	for _, stat := range analysis.Stats.TypeBreakdown {
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", stat.Type, stat.Count))
	}
	sb.WriteString("\n")

	// Secrets details
	sb.WriteString(strings.Repeat("═", 80) + "\n")
	sb.WriteString("SECRETS TRIÉS PAR FRÉQUENCE DE CHANGEMENT\n")
	sb.WriteString(strings.Repeat("═", 80) + "\n\n")

	displayed := analysis.Secrets
	if maxSecrets > 0 && len(displayed) > maxSecrets {
		displayed = displayed[:maxSecrets]
	}

	for _, secret := range displayed {
		sb.WriteString(fmt.Sprintf("┌%s┐\n", strings.Repeat("─", 78)))
		sb.WriteString(fmt.Sprintf("│ %-76s │\n", truncate(secret.File, 76)))
		sb.WriteString(fmt.Sprintf("│ Clé: %-71s │\n", truncate(secret.Key, 71)))
		sb.WriteString(fmt.Sprintf("├%s┤\n", strings.Repeat("─", 78)))
		sb.WriteString(fmt.Sprintf("│ Type: %-15s Changements: %-5d Occurrences: %-10d │\n",
			secret.Type, secret.ChangeCount, secret.TotalOccurrences))
		sb.WriteString(fmt.Sprintf("│ Auteurs: %-67s │\n", truncate(strings.Join(secret.Authors, ", "), 67)))
		sb.WriteString(fmt.Sprintf("│ Période: %s → %-53s │\n", secret.FirstSeen[:10], secret.LastSeen[:10]))
		sb.WriteString(fmt.Sprintf("├%s┤\n", strings.Repeat("─", 78)))
		sb.WriteString(fmt.Sprintf("│ %-76s │\n", "Historique des valeurs:"))

		for _, h := range secret.History {
			val := h.MaskedValue
			if showValues {
				val = h.Value
			}
			authors := strings.Join(h.Authors, ", ")
			sb.WriteString(fmt.Sprintf("│   • %-40s (%dx par %s)%s │\n",
				truncate(val, 40), h.Occurrences, truncate(authors, 20), strings.Repeat(" ", 5)))
		}
		sb.WriteString(fmt.Sprintf("└%s┘\n\n", strings.Repeat("─", 78)))
	}

	if len(analysis.Secrets) > maxSecrets && maxSecrets > 0 {
		sb.WriteString(fmt.Sprintf("... et %d autres secrets\n", len(analysis.Secrets)-maxSecrets))
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ExportCSV exports the analysis results to a CSV file
func ExportCSV(analysis *Analysis, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write BOM for Excel compatibility
	file.WriteString("\xEF\xBB\xBF")

	// Write header
	header := []string{
		"File",
		"Key",
		"Type",
		"ChangeCount",
		"TotalOccurrences",
		"Authors",
		"AuthorCount",
		"FirstSeen",
		"LastSeen",
		"DaysActive",
		"Values",
	}
	file.WriteString(strings.Join(header, ";") + "\n")

	// Write data rows
	for _, secret := range analysis.Secrets {
		// Calculate days active
		daysActive := 0
		if secret.FirstSeen != "" && secret.LastSeen != "" {
			if first, err := time.Parse(time.RFC3339, secret.FirstSeen); err == nil {
				if last, err := time.Parse(time.RFC3339, secret.LastSeen); err == nil {
					daysActive = int(last.Sub(first).Hours() / 24)
				}
			}
		}

		// Collect masked values
		var values []string
		for _, h := range secret.History {
			values = append(values, h.MaskedValue)
		}

		row := []string{
			escapeCSV(secret.File),
			escapeCSV(secret.Key),
			escapeCSV(secret.Type),
			fmt.Sprintf("%d", secret.ChangeCount),
			fmt.Sprintf("%d", secret.TotalOccurrences),
			escapeCSV(strings.Join(secret.Authors, ", ")),
			fmt.Sprintf("%d", len(secret.Authors)),
			formatDate(secret.FirstSeen),
			formatDate(secret.LastSeen),
			fmt.Sprintf("%d", daysActive),
			escapeCSV(strings.Join(values, " | ")),
		}
		file.WriteString(strings.Join(row, ";") + "\n")
	}

	return nil
}

// ExportStatsCSV exports summary statistics to a separate CSV file
func ExportStatsCSV(analysis *Analysis, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write BOM for Excel compatibility
	file.WriteString("\xEF\xBB\xBF")

	// Summary stats
	file.WriteString("=== SUMMARY ===\n")
	file.WriteString("Metric;Value\n")
	file.WriteString(fmt.Sprintf("Total Entries;%d\n", analysis.Stats.TotalEntries))
	file.WriteString(fmt.Sprintf("Unique Secrets;%d\n", analysis.Stats.UniqueSecrets))
	file.WriteString(fmt.Sprintf("Unique Values;%d\n", analysis.Stats.UniqueValues))
	file.WriteString("\n")

	// Authors breakdown
	file.WriteString("=== AUTHORS ===\n")
	file.WriteString("Author;Count\n")
	for _, a := range analysis.Stats.TopAuthors {
		file.WriteString(fmt.Sprintf("%s;%d\n", escapeCSV(a.Author), a.Count))
	}
	file.WriteString("\n")

	// Files breakdown
	file.WriteString("=== FILES ===\n")
	file.WriteString("File;Count\n")
	for _, f := range analysis.Stats.TopFiles {
		file.WriteString(fmt.Sprintf("%s;%d\n", escapeCSV(f.File), f.Count))
	}
	file.WriteString("\n")

	// Types breakdown
	file.WriteString("=== SECRET TYPES ===\n")
	file.WriteString("Type;Count\n")
	for _, t := range analysis.Stats.TypeBreakdown {
		file.WriteString(fmt.Sprintf("%s;%d\n", escapeCSV(t.Type), t.Count))
	}

	return nil
}

func escapeCSV(s string) string {
	// Replace semicolons and newlines for CSV compatibility
	s = strings.ReplaceAll(s, ";", ",")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Quote if contains special characters
	if strings.ContainsAny(s, ",\"") {
		s = "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func formatDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("2006-01-02")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
