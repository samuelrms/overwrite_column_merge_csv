package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

var DiffOutputDir string
var (
	DataOutputDir   string
	FirstCSVPath    string
	SecondCSVPath   string
	KeysFirst       []string
	KeysSecond      []string
	OverwriteColumn string
	SourceColumn    string
	DefaultValue    string
)

var BaseNameFileData = "merged_"

func init() {
	// Use Go layout: year-month-day_hour-minute-second
	now := time.Now().Format("2006-01-02_15-04-05")
	DiffOutputDir = fmt.Sprintf("diff-%s", now)

	// Defining log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// loadEnv loads an environment variable or exits if not set
func loadEnv(key string) string {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env not found, using system environment variables")
	}
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ %s is not set", key)
	}
	return val
}

func main() {
	// Load configuration from environment
	DataOutputDir = loadEnv("DATA_OUTPUT_DIR")
	FirstCSVPath = loadEnv("FIRST_CSV")
	SecondCSVPath = loadEnv("SECOND_CSV")
	KeysFirst = strings.Split(loadEnv("KEY_COLUMNS_FIRST"), ",")
	KeysSecond = strings.Split(loadEnv("KEY_COLUMNS_SECOND"), ",")
	OverwriteColumn = loadEnv("OVERWRITE_COLUMN")
	SourceColumn = loadEnv("SOURCE_COLUMN")
	DefaultValue = loadEnv("DEFAULT")

	// Ensure output directory exists
	if err := os.MkdirAll(DataOutputDir, os.ModePerm); err != nil {
		log.Fatalf("❌ failed to create output directory %s: %v", DataOutputDir, err)
	}

	// Build lookup map from second CSV
	lookup := buildLookup(SecondCSVPath, KeysSecond, SourceColumn)

	// Process first CSV and merge values
	mergeCSV(FirstCSVPath, lookup, KeysFirst, OverwriteColumn, DefaultValue, DataOutputDir)

	log.Println("✅ Merging complete.")
}

// buildLookup reads a CSV and returns a map of composite key to source value
func buildLookup(path string, keyCols []string, srcCol string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("❌ failed to open second CSV %s: %v", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("❌ failed to read second CSV %s: %v", path, err)
	}
	if len(records) < 1 {
		log.Fatalf("❌ second CSV %s is empty", path)
	}

	head := records[0]

	// Determine indices for key columns and source column
	keyIdx := findIndicesOrFail(head, keyCols, "key column")
	srcIdx := findIndexOrFail(head, srcCol, "source column")

	lookup := make(map[string]string, len(records)-1)
	for _, row := range records[1:] {
		key := buildKey(row, keyIdx)
		lookup[key] = row[srcIdx]
	}
	return lookup
}

// mergeCSV reads the first CSV, applies overwrites using the lookup map, and writes output
func mergeCSV(path string, lookup map[string]string, keyCols []string, overwriteCol, defaultVal, outDir string) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("❌ failed to open first CSV %s: %v", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("❌ failed to read first CSV %s: %v", path, err)
	}
	if len(records) < 1 {
		log.Fatalf("❌ first CSV %s is empty", path)
	}

	head := records[0]

	// Determine indices for key columns and overwrite column
	keyIdx := findIndicesOrFail(head, keyCols, "key column")
	owIdx := findIndexOrFail(head, overwriteCol, "overwrite column")

	// Prepare output file
	outPath := filepath.Join(outDir, BaseNameFileData+filepath.Base(path))
	outFile, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("❌ failed to create output CSV %s: %v", outPath, err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	// Write header
	writer.Write(head)

	// Process rows
	for _, row := range records[1:] {
		key := buildKey(row, keyIdx)
		if val, found := lookup[key]; found {
			row[owIdx] = val
		} else {
			row[owIdx] = defaultVal
		}
		writer.Write(row)
	}
}

// findIndicesOrFail finds the indices of multiple headers or exits
func findIndicesOrFail(header []string, cols []string, desc string) []int {
	indices := make([]int, len(cols))
	for i, col := range cols {
		idx := findIndex(header, col)
		if idx == -1 {
			log.Fatalf("❌ %s %s not found in header", desc, col)
		}
		indices[i] = idx
	}
	return indices
}

// findIndexOrFail finds the index of a header or exits
func findIndexOrFail(header []string, col, desc string) int {
	idx := findIndex(header, col)
	if idx == -1 {
		log.Fatalf("❌ %s %s not found in header", desc, col)
	}
	return idx
}

// findIndex returns the index of a string in a slice or -1
func findIndex(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

// buildKey concatenates values from a row at given indices to form a composite key
func buildKey(row []string, indices []int) string {
	parts := make([]string, len(indices))
	for i, idx := range indices {
		if idx < len(row) {
			parts[i] = row[idx]
		} else {
			parts[i] = ""
		}
	}
	return strings.Join(parts, "|")
}
