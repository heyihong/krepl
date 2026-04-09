package repl

import (
	"reflect"
	"testing"
)

func TestTryParseRange(t *testing.T) {
	tests := []struct {
		input   string
		lastLen int
		want    []int
		ok      bool
	}{
		{"1..3", 10, []int{1, 2, 3}, true},
		{"..4", 10, []int{0, 1, 2, 3, 4}, true},
		{"3..", 6, []int{3, 4, 5}, true},
		{"..", 3, []int{0, 1, 2}, true},
		{"6..4", 10, nil, true},
		{"6..6", 10, []int{6}, true},
		{"1..=3", 10, nil, false},
		{"..=4", 10, nil, false},
		{"1..==2", 10, nil, false},
		{"1", 10, nil, false},
		{" pods ", 10, nil, false},
	}

	for _, tt := range tests {
		got, ok := TryParseRange(tt.input, tt.lastLen)
		if ok != tt.ok {
			t.Fatalf("TryParseRange(%q): ok=%v want %v", tt.input, ok, tt.ok)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("TryParseRange(%q): got %v want %v", tt.input, got, tt.want)
		}
	}
}

func TestTryParseCSL(t *testing.T) {
	tests := []struct {
		input string
		want  []int
		ok    bool
	}{
		{"1,2,3", []int{1, 2, 3}, true},
		{"1, 2, 3", []int{1, 2, 3}, true},
		{"1", []int{1}, true},
		{"", nil, false},
		{",", nil, false},
		{"1,,2", nil, false},
		{",1,2,", nil, false},
		{"1,x,2", nil, false},
	}

	for _, tt := range tests {
		got, ok := TryParseCSL(tt.input)
		if ok != tt.ok {
			t.Fatalf("TryParseCSL(%q): ok=%v want %v", tt.input, ok, tt.ok)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("TryParseCSL(%q): got %v want %v", tt.input, got, tt.want)
		}
	}
}
