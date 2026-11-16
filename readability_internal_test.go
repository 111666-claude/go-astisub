package astisub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"你好", 4},       // two wide ideographs
		{"，", 2},        // full-width comma
		{"ｱ", 1},        // half-width katakana U+FF71
		{"a你b好", 6},     // mixed: 1+2+1+2
		{"é", 1},       // base letter + combining acute
		{"a\u200bb", 2}, // zero-width space ignored
		{"a\tb", 2},     // tab is a control character: 0 width
		{"a\u00adb", 2}, // soft hyphen takes no space
		{"abc ", 4},     // spaces count as one column
		{"12:30", 5},    // ASCII punctuation is half width
		{"（）", 4},       // full-width parentheses
		{"😀", 2},        // emoji classified East Asian Wide by the width tables
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, displayWidth(tt.in), "displayWidth(%q)", tt.in)
	}
}

func TestCountVisibleRunes(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"你好", 2},
		{"éé", 2},           // two visible letters, each with a combining mark
		{"a\u200b\u200cb", 2}, // zero-width spaces do not count
		{"a b c", 5},          // spaces count
		{"\t\n\r", 0},         // control characters do not count
		{"a\u00adb", 2},       // soft hyphen does not count
		{"�", 1},              // replacement rune is visible
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, countVisibleRunes(tt.in), "countVisibleRunes(%q)", tt.in)
	}
}

func TestOverlapDuration(t *testing.T) {
	tests := []struct {
		name        string
		aStart      time.Duration
		aEnd        time.Duration
		bStart      time.Duration
		bEnd        time.Duration
		wantOverlap time.Duration
	}{
		{"full containment", 0, 10 * time.Second, 2 * time.Second, 4 * time.Second, 2 * time.Second},
		{"partial overlap", time.Second, 3 * time.Second, 2 * time.Second, 4 * time.Second, time.Second},
		{"touching boundaries", time.Second, 2 * time.Second, 2 * time.Second, 3 * time.Second, 0},
		{"disjoint", 0, time.Second, 2 * time.Second, 3 * time.Second, 0},
		{"reversed order", 5 * time.Second, 6 * time.Second, time.Second, 5500 * time.Millisecond, 500 * time.Millisecond},
		{"same window", time.Second, 2 * time.Second, time.Second, 2 * time.Second, time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Item{StartAt: tt.aStart, EndAt: tt.aEnd}
			b := &Item{StartAt: tt.bStart, EndAt: tt.bEnd}
			assert.Equal(t, tt.wantOverlap, overlapDuration(a, b))
		})
	}
}

func TestReadabilityCheck_NilItems(t *testing.T) {
	// Nil entries must be skipped without panicking, and conflicts between
	// the remaining items must still be reported.
	s := &Subtitles{Items: []*Item{
		nil,
		{Index: 1, StartAt: time.Second, EndAt: 3 * time.Second, Lines: []Line{{Items: []LineItem{{Text: "a"}}}}},
		nil,
		{Index: 2, StartAt: 2 * time.Second, EndAt: 4 * time.Second, Lines: []Line{{Items: []LineItem{{Text: "b"}}}}},
	}}
	ds, err := s.ReadabilityCheck(ReadabilityOptions{})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, ReadabilityRuleOverlap, ds[0].Rule)
	assert.Equal(t, 1, ds[0].Index)
	assert.Equal(t, 3, ds[0].OtherIndex)
}

func TestReadabilityCheck_DefaultsApplied(t *testing.T) {
	s := &Subtitles{Items: []*Item{{StartAt: time.Second, EndAt: 2 * time.Second, Lines: []Line{{Items: []LineItem{{Text: "x"}}}}}}}
	ds, err := s.ReadabilityCheck(ReadabilityOptions{})
	require.NoError(t, err)
	assert.Empty(t, ds)

	// Explicit defaults are equivalent to the zero-value options.
	ds, err = s.ReadabilityCheck(ReadabilityOptions{MaxCPS: defaultReadabilityMaxCPS, MaxLineWidth: defaultReadabilityMaxLineWidth})
	require.NoError(t, err)
	assert.Empty(t, ds)
}

func TestLineStyle(t *testing.T) {
	bold := true
	italic := true
	color := newColorFromHTMLString("#ff0000")

	l1 := Line{Items: []LineItem{{Text: "a", InlineStyle: &StyleAttributes{SRTBold: true, SRTColor: color}}}}
	l2 := Line{Items: []LineItem{{Text: "b", InlineStyle: &StyleAttributes{SRTBold: true, SRTColor: color}}}}
	l3 := Line{Items: []LineItem{{Text: "c", InlineStyle: &StyleAttributes{SRTBold: true}}}}
	l4 := Line{Items: []LineItem{{Text: "d", InlineStyle: &StyleAttributes{SRTBold: bold, SSAItalic: &italic}}}}

	assert.Equal(t, lineStyle(l1), lineStyle(l2))
	assert.NotEqual(t, lineStyle(l1), lineStyle(l3))
	assert.NotEqual(t, lineStyle(l1), lineStyle(l4))

	// Named styles participate in the signature.
	named1 := Line{Items: []LineItem{{Text: "a", Style: &Style{ID: "speech"}}}}
	named2 := Line{Items: []LineItem{{Text: "b", Style: &Style{ID: "narration"}}}}
	assert.NotEqual(t, lineStyle(named1), lineStyle(named2))

	// Underline, colors from every format family, SSA typography and the
	// WebVTT tag stack all contribute to the signature.
	under := Line{Items: []LineItem{{Text: "a", InlineStyle: &StyleAttributes{SRTUnderline: true}}}}
	assert.True(t, lineStyle(under).underline)

	ssaColor := Line{Items: []LineItem{{Text: "a", InlineStyle: &StyleAttributes{SSAPrimaryColour: color}}}}
	ttmlColor := Line{Items: []LineItem{{Text: "a", InlineStyle: &StyleAttributes{TTMLColor: color}}}}
	assert.Equal(t, lineStyle(ssaColor).color, lineStyle(ttmlColor).color)
	assert.NotEqual(t, lineStyle(ssaColor), lineStyle(under))

	size := 48.
	typography := Line{Items: []LineItem{{Text: "a", InlineStyle: &StyleAttributes{SSAFontName: "Arial", SSAFontSize: &size}}}}
	typoSig := lineStyle(typography)
	assert.Equal(t, "Arial", typoSig.fontName)
	assert.Equal(t, 48., typoSig.fontSize)

	tagged := Line{Items: []LineItem{{
		Text:        "a",
		InlineStyle: &StyleAttributes{WebVTTTags: []WebVTTTag{{Name: "b"}, {Name: "i"}, {Name: "u"}}},
	}}}
	tagSig := lineStyle(tagged)
	assert.True(t, tagSig.bold)
	assert.True(t, tagSig.italics)
	assert.True(t, tagSig.underline)

	// A line item without inline style is simply ignored.
	assert.Equal(t, lineStyleSignature{}, lineStyle(Line{Items: []LineItem{{Text: "a"}}}))
}

func TestItemCrossLineStyleDiagnostic(t *testing.T) {
	item := &Item{Index: 1, StartAt: time.Second, EndAt: 2 * time.Second}
	// A single-line item cannot have cross-line styling.
	item.Lines = []Line{{Items: []LineItem{{Text: "only line"}}}}
	assert.Nil(t, itemCrossLineStyleDiagnostic(0, item))

	// Blank lines are ignored: one styled line plus a blank line is fine.
	item.Lines = []Line{
		{Items: []LineItem{{Text: "styled", InlineStyle: &StyleAttributes{SRTBold: true}}}},
		{Items: []LineItem{{Text: "  "}}},
	}
	assert.Nil(t, itemCrossLineStyleDiagnostic(0, item))

	// Two readable lines with different styles are reported.
	item.Lines = []Line{
		{Items: []LineItem{{Text: "bold", InlineStyle: &StyleAttributes{SRTBold: true}}}},
		{Items: []LineItem{{Text: "plain"}}},
	}
	d := itemCrossLineStyleDiagnostic(7, item)
	require.NotNil(t, d)
	assert.Equal(t, 7, d.Index)
	assert.Equal(t, ReadabilityRuleCrossLineStyle, d.Rule)
}

func TestReadabilityDiagnostic_String(t *testing.T) {
	assert.Contains(t, ReadabilityDiagnostic{
		Index: 2, StartAt: time.Second, EndAt: 2 * time.Second,
		Rule: ReadabilityRuleCPS, LineIndex: -1, Message: "too fast",
	}.String(), "item 2")
	assert.Contains(t, ReadabilityDiagnostic{
		Index: 2, StartAt: time.Second, EndAt: 2 * time.Second,
		Rule: ReadabilityRuleLineWidth, LineIndex: 1, Message: "too wide",
	}.String(), "line 1")
}

func TestReadabilityRuleOrder(t *testing.T) {
	// The ordering function is total over the known rules.
	rules := []ReadabilityRule{
		ReadabilityRuleOverlap,
		ReadabilityRuleEmptyText,
		ReadabilityRuleNonPositiveDuration,
		ReadabilityRuleCPS,
		ReadabilityRuleLineWidth,
		ReadabilityRuleCrossLineStyle,
	}
	seen := map[int]bool{}
	for _, r := range rules {
		o := readabilityRuleOrder(r)
		assert.False(t, seen[o], "duplicate order %d", o)
		seen[o] = true
	}
}
