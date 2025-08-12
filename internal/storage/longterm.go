package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"eino_llm_poc/pkg"
	"log"
)

// LongtermManager handles long-term memory (JSON file) operations
type LongtermManager interface {
	LoadMemory(customerID string) ([]pkg.LongtermMemoryEntry, error)
	SaveEntry(entry pkg.LongtermMemoryEntry) error
	GetMemoryStats(customerID string) (*MemoryStats, error)
	CleanupOldEntries(customerID string, maxAge time.Duration) error
}

// JSONLongtermManager implements file-based longterm memory storage
type JSONLongtermManager struct {
	baseDir string
}

// NewJSONLongtermManager creates a new JSON-based longterm memory manager
func NewJSONLongtermManager(baseDir string) LongtermManager {
	return &JSONLongtermManager{
		baseDir: baseDir,
	}
}

// LoadMemory loads all longterm memory entries for a customer
func (j *JSONLongtermManager) LoadMemory(customerID string) ([]pkg.LongtermMemoryEntry, error) {
	filename := fmt.Sprintf("%s.json", customerID)
	filePath := filepath.Join(j.baseDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []pkg.LongtermMemoryEntry{}, nil // Return empty slice if file doesn't exist
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read longterm memory file: %v", err)
	}

	var entries []pkg.LongtermMemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse longterm memory file: %v", err)
	}

	return entries, nil
}

// SaveEntry saves a single entry to longterm memory
func (j *JSONLongtermManager) SaveEntry(entry pkg.LongtermMemoryEntry) error {
	// Ensure directory exists
	if err := os.MkdirAll(j.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create longterm directory: %v", err)
	}

	// Load existing entries
	entries, err := j.LoadMemory(entry.CustomerID)
	if err != nil {
		log.Printf("Warning: Failed to load existing longterm memory, starting fresh: %v", err)
		entries = []pkg.LongtermMemoryEntry{}
	}

	// Add new entry
	entries = append(entries, entry)

	// Write back to file
	filename := fmt.Sprintf("%s.json", entry.CustomerID)
	filePath := filepath.Join(j.baseDir, filename)

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal longterm memory data: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write longterm memory file: %v", err)
	}

	log.Printf("💾 Saved to longterm memory: %s (customer: %s, importance: %.3f)",
		filePath, entry.CustomerID, entry.ImportanceScore)

	return nil
}

// MemoryStats provides statistics about longterm memory
type MemoryStats struct {
	CustomerID       string    `json:"customer_id"`
	TotalEntries     int       `json:"total_entries"`
	AvgImportance    float64   `json:"avg_importance"`
	OldestEntry      time.Time `json:"oldest_entry"`
	NewestEntry      time.Time `json:"newest_entry"`
	TopIntents       []string  `json:"top_intents"`
	FileSizeBytes    int64     `json:"file_size_bytes"`
}

// GetMemoryStats returns statistics about a customer's longterm memory
func (j *JSONLongtermManager) GetMemoryStats(customerID string) (*MemoryStats, error) {
	entries, err := j.LoadMemory(customerID)
	if err != nil {
		return nil, err
	}

	stats := &MemoryStats{
		CustomerID: customerID,
		TopIntents: []string{},
	}

	if len(entries) == 0 {
		return stats, nil
	}

	stats.TotalEntries = len(entries)

	// Calculate average importance
	totalImportance := 0.0
	intentCounts := make(map[string]int)
	
	stats.OldestEntry = entries[0].Timestamp
	stats.NewestEntry = entries[0].Timestamp

	for _, entry := range entries {
		totalImportance += entry.ImportanceScore
		
		// Track intents
		if entry.NLUResponse != nil && entry.NLUResponse.PrimaryIntent != "" {
			intentCounts[entry.NLUResponse.PrimaryIntent]++
		}
		
		// Track time boundaries
		if entry.Timestamp.Before(stats.OldestEntry) {
			stats.OldestEntry = entry.Timestamp
		}
		if entry.Timestamp.After(stats.NewestEntry) {
			stats.NewestEntry = entry.Timestamp
		}
	}

	stats.AvgImportance = totalImportance / float64(len(entries))

	// Get top intents
	stats.TopIntents = getTopIntents(intentCounts, 5)

	// Get file size
	filename := fmt.Sprintf("%s.json", customerID)
	filePath := filepath.Join(j.baseDir, filename)
	if info, err := os.Stat(filePath); err == nil {
		stats.FileSizeBytes = info.Size()
	}

	return stats, nil
}

// CleanupOldEntries removes entries older than maxAge
func (j *JSONLongtermManager) CleanupOldEntries(customerID string, maxAge time.Duration) error {
	entries, err := j.LoadMemory(customerID)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	filteredEntries := []pkg.LongtermMemoryEntry{}

	for _, entry := range entries {
		if entry.Timestamp.After(cutoff) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	if len(filteredEntries) == len(entries) {
		return nil // No cleanup needed
	}

	// Write filtered entries back
	filename := fmt.Sprintf("%s.json", customerID)
	filePath := filepath.Join(j.baseDir, filename)

	data, err := json.MarshalIndent(filteredEntries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cleaned longterm memory data: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cleaned longterm memory file: %v", err)
	}

	removed := len(entries) - len(filteredEntries)
	log.Printf("🧹 Cleaned up longterm memory: removed %d old entries for customer %s", removed, customerID)

	return nil
}

// ShouldSaveToLongterm determines if an entry should be saved to longterm memory
func ShouldSaveToLongterm(response *pkg.NLUResponse, threshold float64) bool {
	return response.ImportanceScore >= threshold
}

// CreateLongtermEntry creates a longterm memory entry from request and response
func CreateLongtermEntry(request pkg.NLURequest, response *pkg.NLUResponse) pkg.LongtermMemoryEntry {
	return pkg.LongtermMemoryEntry{
		CustomerID:      request.CustomerID,
		Timestamp:       response.Timestamp,
		InputText:       request.Text,
		NLUResponse:     response,
		ImportanceScore: response.ImportanceScore,
	}
}

// GetBusinessInsights extracts business insights from NLU analysis
func GetBusinessInsights(response *pkg.NLUResponse) map[string]any {
	insights := make(map[string]any)

	// Intent insights
	if len(response.Intents) > 0 {
		intentData := make([]map[string]any, len(response.Intents))
		for i, intent := range response.Intents {
			intentData[i] = map[string]any{
				"name":       intent.Name,
				"confidence": intent.Confidence,
			}
		}
		insights["intents"] = intentData
	}

	// Entity insights
	if len(response.Entities) > 0 {
		entityData := make([]map[string]any, len(response.Entities))
		for i, entity := range response.Entities {
			entityData[i] = map[string]any{
				"type":       entity.Type,
				"value":      entity.Value,
				"confidence": entity.Confidence,
			}
		}
		insights["entities"] = entityData
	}

	// Language insights
	if len(response.Languages) > 0 {
		languageData := make([]map[string]any, len(response.Languages))
		for i, lang := range response.Languages {
			languageData[i] = map[string]any{
				"code":       lang.Code,
				"confidence": lang.Confidence,
				"is_primary": lang.IsPrimary,
			}
		}
		insights["languages"] = languageData
	}

	// Sentiment insights
	if response.Sentiment.Label != "" {
		insights["sentiment"] = map[string]any{
			"label":      response.Sentiment.Label,
			"confidence": response.Sentiment.Confidence,
		}
	}

	return insights
}

// Helper function to get top intents from counts
func getTopIntents(intentCounts map[string]int, limit int) []string {
	type intentCount struct {
		intent string
		count  int
	}

	var counts []intentCount
	for intent, count := range intentCounts {
		counts = append(counts, intentCount{intent, count})
	}

	// Simple sorting by count (descending)
	for i := 0; i < len(counts)-1; i++ {
		for j := 0; j < len(counts)-i-1; j++ {
			if counts[j].count < counts[j+1].count {
				counts[j], counts[j+1] = counts[j+1], counts[j]
			}
		}
	}

	var topIntents []string
	maxItems := limit
	if len(counts) < limit {
		maxItems = len(counts)
	}

	for i := 0; i < maxItems; i++ {
		topIntents = append(topIntents, counts[i].intent)
	}

	return topIntents
}