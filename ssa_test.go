package astisub_test

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
)

func assertSSAStyle(t *testing.T, e, a astisub.Style) {
	assert.Equal(t, e.ID, a.ID)
	assertSSAStyleAttributes(t, *e.InlineStyle, *a.InlineStyle)
}

func assertSSAStyleAttributes(t *testing.T, e, a astisub.StyleAttributes) {
	if e.SSAAlignment != nil {
		assert.Equal(t, *e.SSAAlignment, *a.SSAAlignment)
	}
	if e.SSAAlphaLevel != nil {
		assert.Equal(t, *e.SSAAlphaLevel, *a.SSAAlphaLevel)
	}
	if e.SSABackColour != nil {
		assert.Equal(t, *e.SSABackColour, *a.SSABackColour)
	}
	if e.SSABold != nil {
		assert.Equal(t, *e.SSABold, *a.SSABold)
	}
	if e.SSABorderStyle != nil {
		assert.Equal(t, *e.SSABorderStyle, *a.SSABorderStyle)
	}
	if e.SSAFontSize != nil {
		assert.Equal(t, e.SSAFontName, a.SSAFontName)
	}
	if e.SSAFontSize != nil {
		assert.Equal(t, *e.SSAFontSize, *a.SSAFontSize)
	}
	if e.SSALayer != nil {
		assert.Equal(t, *e.SSALayer, *a.SSALayer)
	}
	if e.SSAMarked != nil {
		assert.Equal(t, *e.SSAMarked, *a.SSAMarked)
	}
	if e.SSAMarginLeft != nil {
		assert.Equal(t, *e.SSAMarginLeft, *a.SSAMarginLeft)
	}
	if e.SSAMarginRight != nil {
		assert.Equal(t, *e.SSAMarginRight, *a.SSAMarginRight)
	}
	if e.SSAMarginVertical != nil {
		assert.Equal(t, *e.SSAMarginVertical, *a.SSAMarginVertical)
	}
	if e.SSAOutline != nil {
		assert.Equal(t, *e.SSAOutline, *a.SSAOutline)
	}
	if e.SSAOutlineColour != nil {
		assert.Equal(t, *e.SSAOutlineColour, *a.SSAOutlineColour)
	}
	if e.SSAPrimaryColour != nil {
		assert.Equal(t, *e.SSAPrimaryColour, *a.SSAPrimaryColour)
	}
	if e.SSASecondaryColour != nil {
		assert.Equal(t, *e.SSASecondaryColour, *a.SSASecondaryColour)
	}
	if e.SSAShadow != nil {
		assert.Equal(t, *e.SSAShadow, *a.SSAShadow)
	}
}

func TestSSA(t *testing.T) {
	// Open
	s, err := astisub.OpenFile("./testdata/example-in.ssa")
	assert.NoError(t, err)
	assertSubtitleItems(t, s)
	// Metadata
	assert.Equal(t, &astisub.Metadata{Comments: []string{"Comment 1", "Comment 2"}, SSACollisions: "Normal", SSAOriginalScript: "asticode", SSAPlayDepth: astikit.IntPtr(0), SSAPlayResY: astikit.IntPtr(600), SSAScriptType: "v4.00", SSAScriptUpdatedBy: "version 2.8.01", SSATimer: astikit.Float64Ptr(100), Title: "SSA test"}, s.Metadata)
	// Styles
	assert.Equal(t, 3, len(s.Styles))
	assertSSAStyle(t, astisub.Style{ID: "1", InlineStyle: &astisub.StyleAttributes{SSAAlignment: astikit.IntPtr(7), SSAAlphaLevel: astikit.Float64Ptr(0.1), SSABackColour: &astisub.Color{Alpha: 128, Red: 8}, SSABold: astikit.BoolPtr(true), SSABorderStyle: astikit.IntPtr(7), SSAFontName: "f1", SSAFontSize: astikit.Float64Ptr(4), SSAOutline: astikit.Float64Ptr(1), SSAOutlineColour: &astisub.Color{Green: 255, Red: 255}, SSAMarginLeft: astikit.IntPtr(1), SSAMarginRight: astikit.IntPtr(4), SSAMarginVertical: astikit.IntPtr(7), SSAPrimaryColour: &astisub.Color{Green: 255, Red: 255}, SSASecondaryColour: &astisub.Color{Green: 255, Red: 255}, SSAShadow: astikit.Float64Ptr(4)}}, *s.Styles["1"])
	assertSSAStyle(t, astisub.Style{ID: "2", InlineStyle: &astisub.StyleAttributes{SSAAlignment: astikit.IntPtr(8), SSAAlphaLevel: astikit.Float64Ptr(0.2), SSABackColour: &astisub.Color{Blue: 15, Green: 15, Red: 15}, SSABold: astikit.BoolPtr(true), SSABorderStyle: astikit.IntPtr(8), SSAEncoding: astikit.IntPtr(1), SSAFontName: "f2", SSAFontSize: astikit.Float64Ptr(5), SSAOutline: astikit.Float64Ptr(2), SSAOutlineColour: &astisub.Color{Green: 255, Red: 255}, SSAMarginLeft: astikit.IntPtr(2), SSAMarginRight: astikit.IntPtr(5), SSAMarginVertical: astikit.IntPtr(8), SSAPrimaryColour: &astisub.Color{Blue: 239, Green: 239, Red: 239}, SSASecondaryColour: &astisub.Color{Green: 255, Red: 255}, SSAShadow: astikit.Float64Ptr(5)}}, *s.Styles["2"])
	assertSSAStyle(t, astisub.Style{ID: "3", InlineStyle: &astisub.StyleAttributes{SSAAlignment: astikit.IntPtr(9), SSAAlphaLevel: astikit.Float64Ptr(0.3), SSABackColour: &astisub.Color{Red: 8}, SSABorderStyle: astikit.IntPtr(9), SSAEncoding: astikit.IntPtr(2), SSAFontName: "f3", SSAFontSize: astikit.Float64Ptr(6), SSAOutline: astikit.Float64Ptr(3), SSAOutlineColour: &astisub.Color{Red: 8}, SSAMarginLeft: astikit.IntPtr(3), SSAMarginRight: astikit.IntPtr(6), SSAMarginVertical: astikit.IntPtr(9), SSAPrimaryColour: &astisub.Color{Blue: 180, Green: 252, Red: 252}, SSASecondaryColour: &astisub.Color{Blue: 180, Green: 252, Red: 252}, SSAShadow: astikit.Float64Ptr(6)}}, *s.Styles["3"])
	// Items
	assertSSAStyleAttributes(t, astisub.StyleAttributes{SSAEffect: "test", SSAMarked: astikit.BoolPtr(false), SSAMarginLeft: astikit.IntPtr(1234), SSAMarginRight: astikit.IntPtr(2345), SSAMarginVertical: astikit.IntPtr(3456)}, *s.Items[0].InlineStyle)
	assert.Equal(t, s.Styles["1"], s.Items[0].Style)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{InlineStyle: &astisub.StyleAttributes{SSAEffect: "{\\pos(400,570)}"}, Text: "(deep rumbling)"}}, VoiceName: "Cher"}}, s.Items[0].Lines)
	assert.Equal(t, s.Styles["2"], s.Items[1].Style)
	assert.Equal(t, s.Styles["3"], s.Items[2].Style)
	assert.Equal(t, s.Styles["1"], s.Items[3].Style)
	assert.Equal(t, s.Styles["2"], s.Items[4].Style)
	assert.Equal(t, s.Styles["3"], s.Items[5].Style)

	// No subtitles to write
	w := &bytes.Buffer{}
	err = astisub.Subtitles{}.WriteToSSA(w)
	assert.EqualError(t, err, astisub.ErrNoSubtitlesToWrite.Error())

	// Write
	c, err := ioutil.ReadFile("./testdata/example-out.ssa")
	assert.NoError(t, err)
	err = s.WriteToSSA(w)
	assert.NoError(t, err)
	assert.Equal(t, string(c), w.String())
}

func TestSSAv4plus(t *testing.T) {
	// Open
	s, err := astisub.OpenFile("./testdata/example-in.ssa")
	assert.NoError(t, err)
	assertSubtitleItems(t, s)
	// Metadata
	s.Metadata.SSAScriptType = "v4.00+"

	// Write
	w := &bytes.Buffer{}
	err = s.WriteToSSA(w)
	assert.NoError(t, err)

	c, err := ioutil.ReadFile("./testdata/example-out-v4plus.ssa")
	assert.NoError(t, err)
	assert.Equal(t, string(c), w.String())
}

func TestWriteToSSANilMetadata(t *testing.T) {
	// Metadata intentionally left nil
	s := &astisub.Subtitles{
		Items: []*astisub.Item{
			{
				StartAt: time.Second,
				EndAt:   3 * time.Second,
				Lines:   []astisub.Line{{Items: []astisub.LineItem{{Text: "Hello world"}}}},
			},
		},
	}

	w := &bytes.Buffer{}
	assert.NotPanics(t, func() {
		s.WriteToSSA(w)
	})
}

func TestInBetweenSSAEffect(t *testing.T) {
	s, err := astisub.ReadFromSSA(bytes.NewReader([]byte(`[Events]
Format: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: Marked=0,0:01:39.00,0:01:41.04,,Cher,1234,2345,3456,test,First item{\pos(400,570)}Second item`)))
	assert.NoError(t, err)
	assert.Len(t, s.Items[0].Lines[0].Items, 2)
	assert.Equal(t, astisub.LineItem{Text: "First item"}, s.Items[0].Lines[0].Items[0])
	assert.Equal(t, astisub.LineItem{
		InlineStyle: &astisub.StyleAttributes{SSAEffect: "{\\pos(400,570)}"},
		Text:        "Second item",
	}, s.Items[0].Lines[0].Items[1])
}

func TestWriteToSSAKeepsInlineStyleSpacing(t *testing.T) {
	s, err := astisub.ReadFromSSA(bytes.NewReader([]byte(`[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:03.50,Default,,0,0,0,,Hello {\i1}world{\i0}!`)))
	assert.NoError(t, err)
	w := &bytes.Buffer{}
	assert.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `,Hello {\i1}world{\i0}!`)
}

func TestSSAKaraoke(t *testing.T) {
	// Read multi word, multi line and mixed \k, \kf and \ko samples
	s, err := astisub.ReadFromSSA(bytes.NewReader([]byte(`[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,{\k50}Hello {\kf100}beautiful {\ko80}world
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,{\k40}first\N{\k60}second
Dialogue: 0,0:00:09.00,0:00:10.00,Default,,0,0,0,,No karaoke`)))
	assert.NoError(t, err)
	assert.Len(t, s.Items, 3)

	// Timings are relative to the item's start
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 0, EndAt: 500 * time.Millisecond}, Text: "Hello "},
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectKF, StartAt: 500 * time.Millisecond, EndAt: 1500 * time.Millisecond}, Text: "beautiful "},
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectKO, StartAt: 1500 * time.Millisecond, EndAt: 2300 * time.Millisecond}, Text: "world"},
	}}}, s.Items[0].Lines)

	// Each line gets its own timing
	assert.Equal(t, []astisub.Line{
		{Items: []astisub.LineItem{{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 0, EndAt: 400 * time.Millisecond}, Text: "first"}}},
		{Items: []astisub.LineItem{{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 0, EndAt: 600 * time.Millisecond}, Text: "second"}}},
	}, s.Items[1].Lines)

	// No karaoke
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{Text: "No karaoke"}}}}, s.Items[2].Lines)

	// Write
	w := &bytes.Buffer{}
	assert.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `,{\k50}Hello {\kf100}beautiful {\ko80}world`)
	assert.Contains(t, w.String(), `,{\k40}first\N{\k60}second`)
	assert.Contains(t, w.String(), `,No karaoke`)

	// Read back
	s2, err := astisub.ReadFromSSA(bytes.NewReader(w.Bytes()))
	assert.NoError(t, err)
	assert.Len(t, s2.Items, len(s.Items))
	for idx := range s.Items {
		assert.Equal(t, s.Items[idx].StartAt, s2.Items[idx].StartAt)
		assert.Equal(t, s.Items[idx].EndAt, s2.Items[idx].EndAt)
		assert.Equal(t, s.Items[idx].Lines, s2.Items[idx].Lines)
	}
}

func TestSSAKaraokeWithOtherTags(t *testing.T) {
	// Karaoke tags mixed with other tags, consecutive tags and escaped characters
	s, err := astisub.ReadFromSSA(bytes.NewReader([]byte(`[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:03.00,Default,,0,0,0,,{\k50\b1}foo{\b0}{\k30}bar\h baz`)))
	assert.NoError(t, err)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{
		{InlineStyle: &astisub.StyleAttributes{SSAEffect: `{\b1}`}, Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 0, EndAt: 500 * time.Millisecond}, Text: "foo"},
		{InlineStyle: &astisub.StyleAttributes{SSAEffect: `{\b0}`}, Text: ""},
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 500 * time.Millisecond, EndAt: 800 * time.Millisecond}, Text: `bar\h baz`},
	}}}, s.Items[0].Lines)

	// Non karaoke tags and escaped characters are preserved
	w := &bytes.Buffer{}
	assert.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `,{\k50}{\b1}foo{\b0}{\k30}bar\h baz`)
}

func TestSSAKaraokeEdgeCases(t *testing.T) {
	// Invalid durations, cumulated length overflowing the item's end and empty syllables
	s, err := astisub.ReadFromSSA(bytes.NewReader([]byte(`[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\k-5}foo{\k}bar
Dialogue: 0,0:00:03.00,0:00:04.00,Default,,0,0,0,,{\k50}foo{\k200}bar
Dialogue: 0,0:00:05.00,0:00:07.00,Default,,0,0,0,,{\k50}{\k70}foo`)))
	assert.NoError(t, err)

	// Invalid durations are left untouched
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{
		{InlineStyle: &astisub.StyleAttributes{SSAEffect: `{\k-5}`}, Text: "foo"},
		{InlineStyle: &astisub.StyleAttributes{SSAEffect: `{\k}`}, Text: "bar"},
	}}}, s.Items[0].Lines)

	// Syllables cumulating past the item's end are clamped to its duration
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 0, EndAt: 500 * time.Millisecond}, Text: "foo"},
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 500 * time.Millisecond, EndAt: time.Second}, Text: "bar"},
	}}}, s.Items[1].Lines)

	// Empty syllables are kept and still consume their duration
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 0, EndAt: 500 * time.Millisecond}, Text: ""},
		{Karaoke: &astisub.Karaoke{Effect: astisub.KaraokeEffectK, StartAt: 500 * time.Millisecond, EndAt: 1200 * time.Millisecond}, Text: "foo"},
	}}}, s.Items[2].Lines)

	// Write
	w := &bytes.Buffer{}
	assert.NoError(t, s.WriteToSSA(w))
	assert.Contains(t, w.String(), `,{\k-5}foo{\k}bar`)
	assert.Contains(t, w.String(), `,{\k50}foo{\k50}bar`)
	assert.Contains(t, w.String(), `,{\k50}{\k70}foo`)
}
