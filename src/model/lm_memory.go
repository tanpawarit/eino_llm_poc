package model

import "time"

// LongtermMemoryEntry represents a memory entry to be stored in JSON
type LongtermMemoryEntry struct {
	CustomerID      string       `json:"customer_id"`
	Timestamp       time.Time    `json:"timestamp"`
	InputText       string       `json:"input_text"`
	NLUResponse     *NLUResponse `json:"nlu_response"`
	ImportanceScore float64      `json:"importance_score"`
}

// LongtermMemoryStore manages longterm memory persistence
type LongtermMemoryStore struct {
	FilePath string `json:"file_path"`
}
