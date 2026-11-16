package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintProgressColorAndPlainOutput(t *testing.T) {
	tests := []struct {
		name        string
		color       bool
		success     bool
		integrity   float64
		symbol      string
		wantColored []string
	}{
		{
			name:      "plain success",
			success:   true,
			integrity: 95,
			symbol:    "🟢",
		},
		{
			name:      "colored success",
			color:     true,
			success:   true,
			integrity: 95,
			symbol:    "🟢",
			wantColored: []string{
				sgrGreen + ";1m✅" + sgrReset,
				sgrGray + "m[1/1]" + sgrReset,
				sgrBlue + ";1m" + "documento.docx",
			},
		},
		{
			name:      "colored good integrity",
			color:     true,
			success:   true,
			integrity: 80,
			symbol:    "🟡",
			wantColored: []string{
				sgrYellow + "m🟡" + sgrReset,
				sgrYellow + "m80.0%" + sgrReset,
			},
		},
		{
			name:      "colored acceptable integrity",
			color:     true,
			success:   true,
			integrity: 50,
			symbol:    "🟠",
			wantColored: []string{
				"\x1b[38;5;208m🟠" + sgrReset,
				"\x1b[38;5;208m50.0%" + sgrReset,
			},
		},
		{
			name:  "colored error",
			color: true,
			wantColored: []string{
				sgrRed + ";1m❌" + sgrReset,
				sgrGray + "m[1/1]" + sgrReset,
				sgrBlue + ";1m" + "documento.docx",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldColorEnabled := colorEnabled
			colorEnabled = tt.color
			defer func() { colorEnabled = oldColorEnabled }()

			output := captureStdout(t, func() {
				printProgress(1, 1, "documento.docx", tt.success, tt.integrity)
			})

			wantPlain := fmt.Sprintf("\r✅ [1/1] %s Convertido: %-40s %.1f%%\n",
				tt.symbol, "documento.docx", tt.integrity)
			if !tt.success {
				wantPlain = fmt.Sprintf("\r❌ [1/1] Error: %-40s\n", "documento.docx")
			}
			plain := stripANSI(output)
			if plain != wantPlain {
				t.Fatalf("plain output mismatch:\n got: %q\nwant: %q", plain, wantPlain)
			}
			if !tt.color {
				if output != wantPlain {
					t.Fatalf("unexpected ANSI in plain output: %q", output)
				}
				return
			}
			for _, want := range tt.wantColored {
				if !strings.Contains(output, want) {
					t.Errorf("colored output %q does not contain %q", output, want)
				}
			}
		})
	}
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writeEnd
	run()
	os.Stdout = oldStdout
	if err := writeEnd.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	output, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return string(output)
}

func stripANSI(text string) string {
	var plain strings.Builder
	inEscape := false
	for _, r := range text {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		plain.WriteRune(r)
	}
	return plain.String()
}
