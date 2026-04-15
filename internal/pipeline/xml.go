package pipeline

import (
	"regexp"
	"strconv"
	"strings"
)

// getXMLTag extracts the content of a single XML tag from text.
// Example: getXMLTag("<title>Hello</title>", "title") returns "Hello"
func getXMLTag(text, tag string) string {
	pattern := regexp.MustCompile(`(?s)<` + tag + `>(.*?)</` + tag + `>`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// getXMLChildren extracts all children of a repeated tag.
// Example: getXMLChildren("<facts><fact>A</fact><fact>B</fact></facts>", "facts", "fact") returns ["A", "B"]
func getXMLChildren(text, parent, child string) []string {
	parentContent := getXMLTag(text, parent)
	if parentContent == "" {
		return nil
	}
	pattern := regexp.MustCompile(`(?s)<` + child + `>(.*?)</` + child + `>`)
	matches := pattern.FindAllStringSubmatch(parentContent, -1)
	var result []string
	for _, m := range matches {
		if len(m) >= 2 {
			s := strings.TrimSpace(m[1])
			if s != "" {
				result = append(result, s)
			}
		}
	}
	return result
}

// splitXMLBlocks splits text into individual blocks of the given tag.
// Example: splitXMLBlocks("<a>1</a><a>2</a>", "a") returns ["<a>1</a>", "<a>2</a>"]
func splitXMLBlocks(text, tag string) []string {
	pattern := regexp.MustCompile(`(?s)<` + tag + `>.*?</` + tag + `>`)
	return pattern.FindAllString(text, -1)
}

// getXMLInt extracts an integer from an XML tag. Returns 0 if not found or invalid.
func getXMLInt(text, tag string) int {
	s := getXMLTag(text, tag)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}
