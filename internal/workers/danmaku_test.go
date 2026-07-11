package workers

import (
	"math"
	"testing"
)

func TestParseDanDanXML(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		expected int
		wantErr  bool
	}{
		{
			name:     "empty xml",
			xml:      "<i></i>",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "single entry",
			xml:      `<i><d p="10.5,1,25,1234567890,1,2,abc123">Hello</d></i>`,
			expected: 1,
			wantErr:  false,
		},
		{
			name: "multiple entries",
			xml: `<i>
				<d p="1.0,1,25,16777215,0,0,abc">First</d>
				<d p="2.5,0,25,16777215,0,0,def">Second</d>
				<d p="3.0,2,25,16711680,0,0,ghi">Third</d>
			</i>`,
			expected: 3,
			wantErr:  false,
		},
		{
			name:     "malformed p attr",
			xml:      `<i><d p="invalid">text</d></i>`,
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "missing fields",
			xml:      `<i><d p="10">text</d></i>`,
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "invalid xml",
			xml:      "not xml",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := parseDanDanXML([]byte(tt.xml))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDanDanXML() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(lines) != tt.expected {
				t.Fatalf("parseDanDanXML() returned %d entries, expected %d", len(lines), tt.expected)
			}
		})
	}
}

func TestParseDanDanXML_VerifyValues(t *testing.T) {
	xml := `<i><d p="10.5,1,25,1234567890,1,2,abc123">Hello World</d></i>`

	lines, err := parseDanDanXML([]byte(xml))
	if err != nil {
		t.Fatalf("parseDanDanXML() error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(lines))
	}

	line := lines[0]

	if math.Abs(line.Time-10.5) > 1e-9 {
		t.Errorf("Time = %f, want 10.5", line.Time)
	}
	if line.Type != 1 {
		t.Errorf("Type = %d, want 1", line.Type)
	}
	if line.Color != 1234567890 {
		t.Errorf("Color = %d, want 1234567890", line.Color)
	}
	if line.Text != "Hello World" {
		t.Errorf("Text = %q, want %q", line.Text, "Hello World")
	}
}
