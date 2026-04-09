package repl

import "strings"

// TryParseRange parses range syntax and returns concrete indices.
// Open-ended ranges are materialized only up to lastLen.
func TryParseRange(line string, lastLen int) ([]int, bool) {
	if lastLen < 0 {
		lastLen = 0
	}
	idx := strings.Index(line, "..")
	if idx < 0 {
		return nil, false
	}

	startStr, endPart := line[:idx], line[idx:]
	if strings.HasPrefix(endPart, "..=") {
		return nil, false
	}
	endStr := endPart[2:]

	start := 0
	if startStr != "" {
		n, ok := parseUint(startStr)
		if !ok {
			return nil, false
		}
		start = n
	}

	if endStr == "" {
		return materializeRange(start, lastLen), true
	}

	end, ok := parseUint(endStr)
	if !ok {
		return nil, false
	}
	return materializeBoundedRange(start, end+1, lastLen), true
}

// TryParseCSL parses a comma-separated list of non-negative integer indices.
func TryParseCSL(line string) ([]int, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, false
	}
	parts := strings.Split(line, ",")
	indices := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		n, ok := parseUint(part)
		if !ok {
			return nil, false
		}
		indices = append(indices, n)
	}
	return indices, true
}

func parseUint(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func materializeRange(start, lastLen int) []int {
	if start >= lastLen {
		return nil
	}
	indices := make([]int, 0, lastLen-start)
	for i := start; i < lastLen; i++ {
		indices = append(indices, i)
	}
	return indices
}

func materializeBoundedRange(start, end, lastLen int) []int {
	if start >= end || start >= lastLen {
		return nil
	}
	if end > lastLen {
		end = lastLen
	}
	indices := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		indices = append(indices, i)
	}
	return indices
}
