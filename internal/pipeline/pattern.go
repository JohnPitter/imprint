package pipeline

import (
	"fmt"
	"strings"

	"imprint/internal/store"
)

// DetectedPattern represents a pattern found by analyzing compressed observations.
type DetectedPattern struct {
	Type        string   `json:"type"` // "co_change", "error_repeat", "frequent_file"
	Description string   `json:"description"`
	Files       []string `json:"files"`
	Concepts    []string `json:"concepts"`
	Frequency   int      `json:"frequency"`
	Confidence  float64  `json:"confidence"`
}

// PatternDetector analyzes co-change patterns and error repeats across observations.
// Pure data analysis — no LLM needed.
type PatternDetector struct{}

// NewPatternDetector creates a new PatternDetector.
func NewPatternDetector() *PatternDetector {
	return &PatternDetector{}
}

// DetectPatterns analyzes compressed observations for co-change, error repeat,
// and frequent file patterns.
func (p *PatternDetector) DetectPatterns(observations []store.CompressedObservationRow) []DetectedPattern {
	var patterns []DetectedPattern

	patterns = append(patterns, p.detectCoChanges(observations)...)
	patterns = append(patterns, p.detectErrorRepeats(observations)...)
	patterns = append(patterns, p.detectFrequentFiles(observations)...)
	patterns = append(patterns, p.detectFrequentConcepts(observations)...)

	return patterns
}

// detectCoChanges finds file pairs that frequently appear together.
func (p *PatternDetector) detectCoChanges(observations []store.CompressedObservationRow) []DetectedPattern {
	filePairs := map[string]int{} // "fileA|fileB" -> count

	for _, obs := range observations {
		for i, f1 := range obs.Files {
			for _, f2 := range obs.Files[i+1:] {
				key := f1 + "|" + f2
				if f1 > f2 {
					key = f2 + "|" + f1
				}
				filePairs[key]++
			}
		}
	}

	var patterns []DetectedPattern
	for pair, count := range filePairs {
		if count >= 3 {
			files := strings.SplitN(pair, "|", 2)
			patterns = append(patterns, DetectedPattern{
				Type:        "co_change",
				Description: fmt.Sprintf("Files %s and %s are frequently modified together", files[0], files[1]),
				Files:       files,
				Frequency:   count,
				Confidence:  minFloat(float64(count)/10.0, 1.0),
			})
		}
	}
	return patterns
}

// detectErrorRepeats finds error types that appear multiple times.
func (p *PatternDetector) detectErrorRepeats(observations []store.CompressedObservationRow) []DetectedPattern {
	type errorInfo struct {
		count    int
		files    []string
		concepts []string
	}
	errorCounts := map[string]*errorInfo{}

	for _, obs := range observations {
		if obs.Type != "error" {
			continue
		}
		info, ok := errorCounts[obs.Title]
		if !ok {
			info = &errorInfo{}
			errorCounts[obs.Title] = info
		}
		info.count++
		info.files = appendUnique(info.files, obs.Files...)
		info.concepts = appendUnique(info.concepts, obs.Concepts...)
	}

	var patterns []DetectedPattern
	for title, info := range errorCounts {
		if info.count >= 2 {
			patterns = append(patterns, DetectedPattern{
				Type:        "error_repeat",
				Description: fmt.Sprintf("Error '%s' occurred %d times", title, info.count),
				Files:       info.files,
				Concepts:    info.concepts,
				Frequency:   info.count,
				Confidence:  minFloat(float64(info.count)/5.0, 1.0),
			})
		}
	}
	return patterns
}

// detectFrequentFiles finds files that appear across many observations.
func (p *PatternDetector) detectFrequentFiles(observations []store.CompressedObservationRow) []DetectedPattern {
	fileCounts := map[string]int{}
	for _, obs := range observations {
		for _, f := range obs.Files {
			fileCounts[f]++
		}
	}

	var patterns []DetectedPattern
	for file, count := range fileCounts {
		if count >= 5 {
			patterns = append(patterns, DetectedPattern{
				Type:        "frequent_file",
				Description: fmt.Sprintf("File '%s' appears in %d observations", file, count),
				Files:       []string{file},
				Frequency:   count,
				Confidence:  minFloat(float64(count)/15.0, 1.0),
			})
		}
	}
	return patterns
}

// detectFrequentConcepts finds concepts that appear across many observations.
func (p *PatternDetector) detectFrequentConcepts(observations []store.CompressedObservationRow) []DetectedPattern {
	conceptCounts := map[string]int{}
	for _, obs := range observations {
		for _, c := range obs.Concepts {
			conceptCounts[c]++
		}
	}

	var patterns []DetectedPattern
	for concept, count := range conceptCounts {
		if count >= 5 {
			patterns = append(patterns, DetectedPattern{
				Type:        "frequent_concept",
				Description: fmt.Sprintf("Concept '%s' appears in %d observations", concept, count),
				Concepts:    []string{concept},
				Frequency:   count,
				Confidence:  minFloat(float64(count)/15.0, 1.0),
			})
		}
	}
	return patterns
}

// minFloat returns the smaller of two float64 values.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// appendUnique appends values to a slice, skipping duplicates.
func appendUnique(slice []string, values ...string) []string {
	seen := map[string]bool{}
	for _, s := range slice {
		seen[s] = true
	}
	for _, v := range values {
		if !seen[v] {
			slice = append(slice, v)
			seen[v] = true
		}
	}
	return slice
}
