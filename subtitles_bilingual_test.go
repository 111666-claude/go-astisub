package astisub_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bilingualTestLine builds a single unstyled line.
func bilingualTestLine(text string) astisub.Line {
	return astisub.Line{Items: []astisub.LineItem{{Text: text}}}
}

// bilingualTestStyledLine builds a single line whose item carries SRT styling.
func bilingualTestStyledLine(text string, bold bool, color *astisub.Color) astisub.Line {
	return astisub.Line{Items: []astisub.LineItem{{
		Text:        text,
		InlineStyle: &astisub.StyleAttributes{SRTBold: bold, SRTColor: color},
	}}}
}

func bilingualTestItem(start, end time.Duration, lines ...astisub.Line) *astisub.Item {
	return &astisub.Item{StartAt: start, EndAt: end, Lines: lines}
}

// assertBilingualMergeImmutable fails if either input track differs from its
// snapshot.
func assertBilingualMergeImmutable(t *testing.T, primarySnap, secondarySnap, primary, secondary *astisub.Subtitles) {
	t.Helper()
	assert.Equal(t, primarySnap, primary)
	assert.Equal(t, secondarySnap, secondary)
}

func TestSubtitles_MergeBilingual_TimeOffsetAndSingleSideGap(t *testing.T) {
	// Two tracks with slightly offset timings. The last primary item has no
	// counterpart in the secondary track and must be preserved as-is.
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1000*time.Millisecond, 2000*time.Millisecond, bilingualTestLine("你好")),
		bilingualTestItem(3000*time.Millisecond, 4000*time.Millisecond, bilingualTestLine("再见")),
		bilingualTestItem(6000*time.Millisecond, 7000*time.Millisecond, bilingualTestLine("无匹配")),
	}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1100*time.Millisecond, 2050*time.Millisecond, bilingualTestLine("Hello")),
		bilingualTestItem(2950*time.Millisecond, 4100*time.Millisecond, bilingualTestLine("Goodbye")),
	}}
	primarySnap, secondarySnap := primary, secondary

	out, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{Tolerance: 200 * time.Millisecond})
	require.NoError(t, err)

	// Inputs are not modified
	assertBilingualMergeImmutable(t, primarySnap, secondarySnap, primary, secondary)

	require.Len(t, out.Items, 3)

	// First merged item: primary line first by default, union timing
	assert.Equal(t, 1000*time.Millisecond, out.Items[0].StartAt)
	assert.Equal(t, 2050*time.Millisecond, out.Items[0].EndAt)
	require.Len(t, out.Items[0].Lines, 2)
	assert.Equal(t, "你好", out.Items[0].Lines[0].String())
	assert.Equal(t, "Hello", out.Items[0].Lines[1].String())

	// Second merged item: union boundaries
	assert.Equal(t, 2950*time.Millisecond, out.Items[1].StartAt)
	assert.Equal(t, 4100*time.Millisecond, out.Items[1].EndAt)
	require.Len(t, out.Items[1].Lines, 2)
	assert.Equal(t, "再见", out.Items[1].Lines[0].String())
	assert.Equal(t, "Goodbye", out.Items[1].Lines[1].String())

	// Unmatched item preserved with its exact original timing
	assert.Equal(t, 6000*time.Millisecond, out.Items[2].StartAt)
	assert.Equal(t, 7000*time.Millisecond, out.Items[2].EndAt)
	require.Len(t, out.Items[2].Lines, 1)
	assert.Equal(t, "无匹配", out.Items[2].Lines[0].String())

	// Output is sorted and re-indexed
	for idx, item := range out.Items {
		assert.Equal(t, idx+1, item.Index)
		if idx > 0 {
			assert.True(t, out.Items[idx-1].StartAt <= item.StartAt)
		}
	}
}

func TestSubtitles_MergeBilingual_SecondaryFirstLineOrder(t *testing.T) {
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("中文")),
	}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("English")),
	}}

	out, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{LineOrder: astisub.BilingualLineOrderSecondaryFirst})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	require.Len(t, out.Items[0].Lines, 2)
	assert.Equal(t, "English", out.Items[0].Lines[0].String())
	assert.Equal(t, "中文", out.Items[0].Lines[1].String())
}

func TestSubtitles_MergeBilingual_OneToMany(t *testing.T) {
	// One primary line translated as two secondary lines: the three items are
	// transitively paired and produce one merged bilingual item.
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1000*time.Millisecond, 4000*time.Millisecond, bilingualTestLine("这是一整句话")),
	}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1050*time.Millisecond, 2500*time.Millisecond, bilingualTestLine("This is")),
		bilingualTestItem(2500*time.Millisecond, 4050*time.Millisecond, bilingualTestLine("one full sentence")),
	}}

	out, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{Tolerance: 100 * time.Millisecond})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	merged := out.Items[0]
	assert.Equal(t, 1000*time.Millisecond, merged.StartAt)
	assert.Equal(t, 4050*time.Millisecond, merged.EndAt)
	require.Len(t, merged.Lines, 3)
	assert.Equal(t, "这是一整句话", merged.Lines[0].String())
	assert.Equal(t, "This is", merged.Lines[1].String())
	assert.Equal(t, "one full sentence", merged.Lines[2].String())

	// The one-to-many case is symmetric: many primary lines, one secondary line
	primary2 := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("第一行")),
		bilingualTestItem(2*time.Second, 3*time.Second, bilingualTestLine("第二行")),
	}}
	secondary2 := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 3*time.Second, bilingualTestLine("One line")),
	}}
	out2, err := primary2.MergeBilingual(secondary2, astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out2.Items, 1)
	require.Len(t, out2.Items[0].Lines, 3)
	assert.Equal(t, "第一行", out2.Items[0].Lines[0].String())
	assert.Equal(t, "第二行", out2.Items[0].Lines[1].String())
	assert.Equal(t, "One line", out2.Items[0].Lines[2].String())
}

func TestSubtitles_MergeBilingual_TouchingBoundaryAndTolerance(t *testing.T) {
	// Touching ranges (end == start) pair even with a zero tolerance.
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("A")),
	}}
	secondaryTouch := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(2*time.Second, 3*time.Second, bilingualTestLine("B")),
	}}
	out, err := primary.MergeBilingual(secondaryTouch, astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	assert.Equal(t, time.Second, out.Items[0].StartAt)
	assert.Equal(t, 3*time.Second, out.Items[0].EndAt)

	// A gap strictly wider than the tolerance prevents pairing: both items are
	// preserved separately.
	secondaryGap := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(2101*time.Millisecond, 3*time.Second, bilingualTestLine("B")),
	}}
	out, err = primary.MergeBilingual(secondaryGap, astisub.BilingualMergeOptions{Tolerance: 100 * time.Millisecond})
	require.NoError(t, err)
	require.Len(t, out.Items, 2)
	assert.Equal(t, "A", out.Items[0].Lines[0].String())
	assert.Equal(t, "B", out.Items[1].Lines[0].String())

	// A gap equal to the tolerance still pairs (inclusive boundary).
	secondaryEqual := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(2100*time.Millisecond, 3*time.Second, bilingualTestLine("C")),
	}}
	out, err = primary.MergeBilingual(secondaryEqual, astisub.BilingualMergeOptions{Tolerance: 100 * time.Millisecond})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	require.Len(t, out.Items[0].Lines, 2)
}

func TestSubtitles_MergeBilingual_OverlappingSubtitlesWithinTrack(t *testing.T) {
	// Two overlapping primary items both overlap one secondary item. The
	// connected component rule merges them into one deterministic item rather
	// than dropping either primary line.
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 3*time.Second, bilingualTestLine("P1")),
		bilingualTestItem(2*time.Second, 4*time.Second, bilingualTestLine("P2")),
	}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1500*time.Millisecond, 3500*time.Millisecond, bilingualTestLine("S1")),
	}}

	out, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	assert.Equal(t, time.Second, out.Items[0].StartAt)
	assert.Equal(t, 4*time.Second, out.Items[0].EndAt)
	require.Len(t, out.Items[0].Lines, 3)
	assert.Equal(t, "P1", out.Items[0].Lines[0].String())
	assert.Equal(t, "P2", out.Items[0].Lines[1].String())
	assert.Equal(t, "S1", out.Items[0].Lines[2].String())

	// Overlapping items confined to one track cannot be merged across tracks
	// and are both preserved untouched.
	secondaryAlone := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(8*time.Second, 9*time.Second, bilingualTestLine("lonely")),
	}}
	primary2 := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 3*time.Second, bilingualTestLine("P1")),
		bilingualTestItem(2*time.Second, 4*time.Second, bilingualTestLine("P2")),
	}}
	out, err = primary2.MergeBilingual(secondaryAlone, astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out.Items, 3)
	assert.Equal(t, "P1", out.Items[0].Lines[0].String())
	assert.Equal(t, "P2", out.Items[1].Lines[0].String())
	assert.Equal(t, "lonely", out.Items[2].Lines[0].String())
	assert.Equal(t, 8*time.Second, out.Items[2].StartAt)
}

func TestSubtitles_MergeBilingual_PreservesPerLineStyling(t *testing.T) {
	primary := &astisub.Subtitles{Items: []*astisub.Item{{
		StartAt:     time.Second,
		EndAt:       2 * time.Second,
		InlineStyle: &astisub.StyleAttributes{SRTItalics: true},
		Lines: []astisub.Line{
			bilingualTestStyledLine("粗体中文", true, astisub.ColorRed),
		},
	}}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 2*time.Second,
			bilingualTestStyledLine("plain English", false, nil)),
	}}

	out, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	require.Len(t, out.Items[0].Lines, 2)

	primaryLineItem := out.Items[0].Lines[0].Items[0]
	require.NotNil(t, primaryLineItem.InlineStyle)
	assert.True(t, primaryLineItem.InlineStyle.SRTBold)
	assert.Equal(t, astisub.ColorRed, primaryLineItem.InlineStyle.SRTColor)

	secondaryLineItem := out.Items[0].Lines[1].Items[0]
	require.NotNil(t, secondaryLineItem.InlineStyle)
	assert.False(t, secondaryLineItem.InlineStyle.SRTBold)
	assert.Nil(t, secondaryLineItem.InlineStyle.SRTColor)

	// Cue-level styling is carried over and independently owned
	require.NotNil(t, out.Items[0].InlineStyle)
	assert.True(t, out.Items[0].InlineStyle.SRTItalics)

	// Styling must be a copy: changing the output does not touch the input
	out.Items[0].Lines[0].Items[0].InlineStyle.SRTBold = false
	out.Items[0].InlineStyle.SRTItalics = false
	assert.True(t, primary.Items[0].Lines[0].Items[0].InlineStyle.SRTBold)
	assert.True(t, primary.Items[0].InlineStyle.SRTItalics)
}

func TestSubtitles_MergeBilingual_Errors(t *testing.T) {
	valid := func() *astisub.Subtitles {
		return &astisub.Subtitles{Items: []*astisub.Item{
			bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("ok")),
		}}
	}

	t.Run("negative tolerance", func(t *testing.T) {
		_, err := valid().MergeBilingual(valid(), astisub.BilingualMergeOptions{Tolerance: -time.Millisecond})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeInvalidTolerance))
	})

	t.Run("invalid line order", func(t *testing.T) {
		_, err := valid().MergeBilingual(valid(), astisub.BilingualMergeOptions{LineOrder: astisub.BilingualLineOrder(42)})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeInvalidLineOrder))
	})

	t.Run("nil subtitles", func(t *testing.T) {
		_, err := valid().MergeBilingual(nil, astisub.BilingualMergeOptions{})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeNilSubtitles))
	})

	t.Run("zero-duration item", func(t *testing.T) {
		bad := &astisub.Subtitles{Items: []*astisub.Item{
			bilingualTestItem(2*time.Second, 2*time.Second, bilingualTestLine("zero")),
		}}
		_, err := valid().MergeBilingual(bad, astisub.BilingualMergeOptions{})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeInvalidTiming))
	})

	t.Run("end before start", func(t *testing.T) {
		bad := &astisub.Subtitles{Items: []*astisub.Item{
			bilingualTestItem(3*time.Second, 2*time.Second, bilingualTestLine("reversed")),
		}}
		_, err := bad.MergeBilingual(valid(), astisub.BilingualMergeOptions{})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeInvalidTiming))
	})

	t.Run("nil item", func(t *testing.T) {
		bad := &astisub.Subtitles{Items: []*astisub.Item{nil}}
		_, err := valid().MergeBilingual(bad, astisub.BilingualMergeOptions{})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeInvalidTiming))
	})

	t.Run("conflicting style ids", func(t *testing.T) {
		p := &astisub.Subtitles{
			Items:  []*astisub.Item{bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("p"))},
			Styles: map[string]*astisub.Style{"x": {ID: "x", InlineStyle: &astisub.StyleAttributes{WebVTTBold: true}}},
		}
		s := &astisub.Subtitles{
			Items:  []*astisub.Item{bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("s"))},
			Styles: map[string]*astisub.Style{"x": {ID: "x", InlineStyle: &astisub.StyleAttributes{WebVTTItalics: true}}},
		}
		_, err := p.MergeBilingual(s, astisub.BilingualMergeOptions{})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeStyleConflict))
	})

	t.Run("conflicting region ids", func(t *testing.T) {
		p := &astisub.Subtitles{
			Items:   []*astisub.Item{bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("p"))},
			Regions: map[string]*astisub.Region{"r": {ID: "r"}},
		}
		s := &astisub.Subtitles{
			Items:   []*astisub.Item{bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("s"))},
			Regions: map[string]*astisub.Region{"r": {ID: "r", InlineStyle: &astisub.StyleAttributes{WebVTTLine: "5%"}}},
		}
		_, err := p.MergeBilingual(s, astisub.BilingualMergeOptions{})
		assert.True(t, errors.Is(err, astisub.ErrBilingualMergeRegionConflict))
	})

	t.Run("identical style ids are merged", func(t *testing.T) {
		style := &astisub.Style{ID: "x", InlineStyle: &astisub.StyleAttributes{WebVTTBold: true}}
		p := &astisub.Subtitles{Items: []*astisub.Item{bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("p"))}, Styles: map[string]*astisub.Style{"x": style}}
		s := &astisub.Subtitles{Items: []*astisub.Item{bilingualTestItem(3*time.Second, 4*time.Second, bilingualTestLine("s"))}, Styles: map[string]*astisub.Style{"x": style}}
		out, err := p.MergeBilingual(s, astisub.BilingualMergeOptions{})
		require.NoError(t, err)
		assert.Len(t, out.Styles, 1)
	})
}

func TestSubtitles_MergeBilingual_UnmatchedItemIsDeepCopied(t *testing.T) {
	primary := &astisub.Subtitles{Items: []*astisub.Item{{
		StartAt:     8 * time.Second,
		EndAt:       9 * time.Second,
		InlineStyle: &astisub.StyleAttributes{SRTItalics: true},
		Lines:       []astisub.Line{bilingualTestStyledLine("lonely", true, astisub.ColorGreen)},
	}}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("far away")),
	}}

	out, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out.Items, 2)

	// Restyling the unmatched copy must not reach the input
	unmatched := out.Items[1]
	unmatched.InlineStyle.SRTItalics = false
	unmatched.Lines[0].Items[0].InlineStyle.SRTBold = false
	assert.True(t, primary.Items[0].InlineStyle.SRTItalics)
	assert.True(t, primary.Items[0].Lines[0].Items[0].InlineStyle.SRTBold)
}

func TestSubtitles_MergeBilingual_OrdersUnsortedInput(t *testing.T) {
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(5*time.Second, 6*time.Second, bilingualTestLine("late")),
		bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("early")),
	}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1100*time.Millisecond, 1900*time.Millisecond, bilingualTestLine("early-en")),
		bilingualTestItem(5050*time.Millisecond, 6050*time.Millisecond, bilingualTestLine("late-en")),
	}}

	out, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{Tolerance: 150 * time.Millisecond})
	require.NoError(t, err)
	require.Len(t, out.Items, 2)
	assert.Equal(t, time.Second, out.Items[0].StartAt)
	assert.Equal(t, 5*time.Second, out.Items[1].StartAt)
	assert.Equal(t, "early", out.Items[0].Lines[0].String())
	assert.Equal(t, "late", out.Items[1].Lines[0].String())
}

func TestSubtitles_MergeBilingual_EmptyTracks(t *testing.T) {
	empty := astisub.NewSubtitles()
	other := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(time.Second, 2*time.Second, bilingualTestLine("only")),
	}}

	// Empty primary: every secondary item is unmatched and preserved
	out, err := empty.MergeBilingual(other, astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	assert.Equal(t, "only", out.Items[0].Lines[0].String())

	// Empty secondary: every primary item is unmatched and preserved
	out, err = other.MergeBilingual(astisub.NewSubtitles(), astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	assert.Equal(t, "only", out.Items[0].Lines[0].String())

	// Both empty: writing must report the usual "no subtitles" error instead of
	// silently producing an empty file
	out, err = empty.MergeBilingual(astisub.NewSubtitles(), astisub.BilingualMergeOptions{})
	require.NoError(t, err)
	require.Empty(t, out.Items)
	assert.True(t, errors.Is(out.WriteToSRT(&bytes.Buffer{}), astisub.ErrNoSubtitlesToWrite))
}

func TestSubtitles_MergeBilingual_SRTRoundTrip(t *testing.T) {
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1000*time.Millisecond, 2000*time.Millisecond,
			bilingualTestStyledLine("你好世界", true, nil)),
		bilingualTestItem(5000*time.Millisecond, 6000*time.Millisecond,
			bilingualTestLine("只有中文")),
	}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1050*time.Millisecond, 1950*time.Millisecond,
			bilingualTestStyledLine("Hello world", false, astisub.ColorBlue)),
	}}

	merged, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{Tolerance: 100 * time.Millisecond})
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, merged.WriteToSRT(&buf))

	// Sanity check the serialized payload: both languages on separate lines of
	// cue 1, styling tags present, unmatched cue preserved.
	payload := buf.String()
	assert.True(t, strings.Contains(payload, "你好世界"))
	assert.True(t, strings.Contains(payload, "Hello world"))
	assert.True(t, strings.Contains(payload, "<b>你好世界</b>"))
	assert.True(t, strings.Contains(payload, "只有中文"))
	assert.True(t, strings.Contains(payload, "<font color=\"#0000ff\">Hello world</font>"))

	// Re-read: cues, timing and line counts must survive
	reread, err := astisub.ReadFromSRT(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Len(t, reread.Items, 2)
	assert.Equal(t, 1000*time.Millisecond, reread.Items[0].StartAt)
	assert.Equal(t, 2000*time.Millisecond, reread.Items[0].EndAt)
	require.Len(t, reread.Items[0].Lines, 2)
	assert.Equal(t, "你好世界", reread.Items[0].Lines[0].String())
	assert.Equal(t, "Hello world", reread.Items[0].Lines[1].String())
	assert.Equal(t, 5000*time.Millisecond, reread.Items[1].StartAt)
	require.Len(t, reread.Items[1].Lines, 1)
	assert.Equal(t, "只有中文", reread.Items[1].Lines[0].String())

	// Per-line bold styling survives the SRT round-trip
	assert.True(t, reread.Items[0].Lines[0].Items[0].InlineStyle.SRTBold)
}

func TestSubtitles_MergeBilingual_WebVTTRoundTrip(t *testing.T) {
	primary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1000*time.Millisecond, 2000*time.Millisecond,
			bilingualTestStyledLine("你好", true, nil)),
	}}
	secondary := &astisub.Subtitles{Items: []*astisub.Item{
		bilingualTestItem(1020*time.Millisecond, 2020*time.Millisecond,
			bilingualTestLine("Hello")),
	}}

	merged, err := primary.MergeBilingual(secondary, astisub.BilingualMergeOptions{Tolerance: 50 * time.Millisecond})
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, merged.WriteToWebVTT(&buf))
	payload := buf.String()
	assert.True(t, strings.HasPrefix(payload, "WEBVTT"))
	assert.True(t, strings.Contains(payload, "你好"))
	assert.True(t, strings.Contains(payload, "Hello"))

	reread, err := astisub.ReadFromWebVTT(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Len(t, reread.Items, 1)
	assert.Equal(t, 1000*time.Millisecond, reread.Items[0].StartAt)
	assert.Equal(t, 2020*time.Millisecond, reread.Items[0].EndAt)
	require.Len(t, reread.Items[0].Lines, 2)
	assert.Equal(t, "你好", reread.Items[0].Lines[0].String())
	assert.Equal(t, "Hello", reread.Items[0].Lines[1].String())
}

// TestSubtitles_MergeBilingual_DoesNotChangeLegacyMerge makes sure the new
// entry point coexists with the original append-based Merge without altering
// its behavior.
func TestSubtitles_MergeBilingual_DoesNotChangeLegacyMerge(t *testing.T) {
	s1 := &astisub.Subtitles{
		Items:   []*astisub.Item{bilingualTestItem(time.Second, 3*time.Second, bilingualTestLine("a"))},
		Regions: map[string]*astisub.Region{"r1": {ID: "r1"}},
		Styles:  map[string]*astisub.Style{"s1": {ID: "s1"}},
	}
	s2 := &astisub.Subtitles{
		Items:   []*astisub.Item{bilingualTestItem(2*time.Second, 4*time.Second, bilingualTestLine("b"))},
		Regions: map[string]*astisub.Region{"r2": {ID: "r2"}},
		Styles:  map[string]*astisub.Style{"s2": {ID: "s2"}},
	}
	s1.Merge(s2)

	// Legacy Merge still appends instead of pairing: two interleaved cues
	require.Len(t, s1.Items, 2)
	assert.Equal(t, "a", s1.Items[0].Lines[0].String())
	assert.Equal(t, "b", s1.Items[1].Lines[0].String())
	assert.Len(t, s1.Regions, 2)
	assert.Len(t, s1.Styles, 2)
}
