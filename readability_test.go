package astisub_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// item builds an item with plain-text single-line items.
func item(index int, start, end time.Duration, lines ...string) *astisub.Item {
	i := &astisub.Item{Index: index, StartAt: start, EndAt: end}
	for _, l := range lines {
		i.Lines = append(i.Lines, astisub.Line{Items: []astisub.LineItem{{Text: l}}})
	}
	return i
}

func TestSubtitles_ReadabilityCheck_EmptyText(t *testing.T) {
	s := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 2*time.Second),
		{Index: 2, StartAt: 2 * time.Second, EndAt: 3 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: ""}}}}},
		{Index: 3, StartAt: 3 * time.Second, EndAt: 4 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "  \t\u200b"}}}}},
	}}
	ds, err := s.ReadabilityCheck(astisub.ReadabilityOptions{})
	require.NoError(t, err)
	var rules []astisub.ReadabilityRule
	for _, d := range ds {
		rules = append(rules, d.Rule)
	}
	assert.Equal(t, []astisub.ReadabilityRule{
		astisub.ReadabilityRuleEmptyText,
		astisub.ReadabilityRuleEmptyText,
		astisub.ReadabilityRuleEmptyText,
	}, rules)
	// Empty text must not produce a CPS diagnostic and no division happened.
	for _, d := range ds {
		assert.NotEqual(t, astisub.ReadabilityRuleCPS, d.Rule)
		assert.Equal(t, -1, d.LineIndex)
	}
}

func TestSubtitles_ReadabilityCheck_NonPositiveDuration(t *testing.T) {
	s := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, time.Second, "zero duration"),
		item(2, 3*time.Second, 2*time.Second, "negative duration"),
	}}
	ds, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1, MaxLineWidth: 200})
	require.NoError(t, err)
	require.Len(t, ds, 2)
	for _, d := range ds {
		assert.Equal(t, astisub.ReadabilityRuleNonPositiveDuration, d.Rule)
		assert.Equal(t, -1, d.OtherIndex)
	}
	assert.Equal(t, 0., ds[0].Measured)
	assert.Equal(t, -1., ds[1].Measured)
	// Text content is still measured for width, but CPS cannot be computed
	// from a non-positive duration.
	for _, d := range ds {
		assert.NotEqual(t, astisub.ReadabilityRuleCPS, d.Rule)
	}
}

func TestSubtitles_ReadabilityCheck_CPS(t *testing.T) {
	s := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 2*time.Second, "1234567890"), // 10 cps
		item(2, 2*time.Second, 4*time.Second, "abcd"),     // 2 cps
	}}
	ds, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 5, MaxLineWidth: 200})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleCPS, ds[0].Rule)
	assert.Equal(t, 0, ds[0].Index)
	assert.Equal(t, 1, ds[0].ItemIndex)
	assert.InDelta(t, 10., ds[0].Measured, 0.001)
	assert.Equal(t, 5., ds[0].Limit)
	assert.Equal(t, time.Second, ds[0].StartAt)
	assert.Equal(t, 2*time.Second, ds[0].EndAt)

	// Markup is ignored: SRT bold tags are parsed into styles, so the rate is
	// computed from the 5 visible characters only.
	parsed, err := astisub.ReadFromSRT(bytes.NewReader([]byte("1\n00:00:01,000 --> 00:00:02,000\n<b>hello</b>\n")))
	require.NoError(t, err)
	ds, err = parsed.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 4, MaxLineWidth: 200})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleCPS, ds[0].Rule)
	assert.InDelta(t, 5., ds[0].Measured, 0.001)

	// Combining marks do not count as visible characters: "e" + combining
	// acute is a single visible letter.
	combined := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 2*time.Second, "éé"), // U+0065 U+0301, twice
	}}
	ds, err = combined.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1.5, MaxLineWidth: 200})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.InDelta(t, 2., ds[0].Measured, 0.001)
}

func TestSubtitles_ReadabilityCheck_LineWidth(t *testing.T) {
	s := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 2*time.Second, "hello", "你好"), // widths 5 and 4
		item(2, 2*time.Second, 3*time.Second, "a你好，world"), // mixed full/half width
	}}
	ds, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 6})
	require.NoError(t, err)
	// "a你好，world": 1 + 2 + 2 + 2 + 5 = 12 columns, the only offending line.
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleLineWidth, ds[0].Rule)
	assert.Equal(t, 1, ds[0].Index)
	assert.Equal(t, 0, ds[0].LineIndex)
	assert.Equal(t, 12., ds[0].Measured)
	assert.Equal(t, 6., ds[0].Limit)

	// Every offending line is reported.
	twoWide := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 2*time.Second, "aaaaaaaaaa", "bbbbbbbbbb"),
	}}
	ds, err = twoWide.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 4})
	require.NoError(t, err)
	require.Len(t, ds, 2)
	assert.Equal(t, 0, ds[0].LineIndex)
	assert.Equal(t, 1, ds[1].LineIndex)
	for _, d := range ds {
		assert.Equal(t, 10., d.Measured)
	}
}

func TestSubtitles_ReadabilityCheck_Overlap(t *testing.T) {
	s := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 3*time.Second, "a"),   // [1s, 3s]
		item(2, 2*time.Second, 4*time.Second, "b"), // [2s, 4s] overlaps 1s
		item(3, 4*time.Second, 5*time.Second, "c"), // [4s, 5s] only touches item 2
		item(4, 6*time.Second, 7*time.Second, "d"), // no overlap
	}}
	ds, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 200, OverlapTolerance: time.Second})
	require.NoError(t, err)
	// The overlap lasts exactly the tolerance, so it is not reported.
	assert.Empty(t, ds)

	ds, err = s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 200, OverlapTolerance: 500 * time.Millisecond})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleOverlap, ds[0].Rule)
	assert.Equal(t, 0, ds[0].Index)
	assert.Equal(t, 1, ds[0].OtherIndex)
	assert.InDelta(t, 1., ds[0].Measured, 0.001)
	assert.InDelta(t, 0.5, ds[0].Limit, 0.001)

	// Three mutually overlapping items produce one diagnostic per
	// conflicting pair, and a pair stored out of order is anchored on the
	// item that starts first.
	messy := &astisub.Subtitles{Items: []*astisub.Item{
		item(3, 2500*time.Millisecond, 6*time.Second, "c"),
		item(1, time.Second, 4*time.Second, "a"),
		item(2, 2*time.Second, 3*time.Second, "b"),
	}}
	ds, err = messy.ReadabilityCheck(astisub.ReadabilityOptions{})
	require.NoError(t, err)
	var pairs [][2]int
	for _, d := range ds {
		assert.Equal(t, astisub.ReadabilityRuleOverlap, d.Rule)
		pairs = append(pairs, [2]int{d.Index, d.OtherIndex})
	}
	assert.ElementsMatch(t, [][2]int{{1, 0}, {1, 2}, {2, 0}}, pairs)
	// Every pair is anchored on the item that starts first.
	for _, d := range ds {
		assert.True(t, messy.Items[d.Index].StartAt < messy.Items[d.OtherIndex].StartAt)
	}
}

func TestSubtitles_ReadabilityCheck_CrossLineStyle(t *testing.T) {
	bold := true
	plain := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 2*time.Second, "first line", "second line"),
	}}
	ds, err := plain.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 200})
	require.NoError(t, err)
	assert.Empty(t, ds)

	styled := &astisub.Subtitles{Items: []*astisub.Item{
		{
			Index: 2, StartAt: 2 * time.Second, EndAt: 3 * time.Second,
			Lines: []astisub.Line{
				{Items: []astisub.LineItem{{Text: "bold line", InlineStyle: &astisub.StyleAttributes{SRTBold: bold}}}},
				{Items: []astisub.LineItem{{Text: "plain line"}}},
			},
		},
	}}
	ds, err = styled.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 200})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleCrossLineStyle, ds[0].Rule)
	assert.Equal(t, 0, ds[0].Index)
	assert.Equal(t, -1, ds[0].LineIndex)

	// The same scenario reached through the SRT parser: an italic first line
	// followed by a plain second line.
	parsed, err := astisub.ReadFromSRT(bytes.NewReader([]byte("1\n00:00:01,000 --> 00:00:02,000\n<i>styled line here</i>\nplain line text\n")))
	require.NoError(t, err)
	ds, err = parsed.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 200})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleCrossLineStyle, ds[0].Rule)
}

func TestSubtitles_ReadabilityCheck_InvalidOptionsAndDefaults(t *testing.T) {
	s := &astisub.Subtitles{Items: []*astisub.Item{item(1, time.Second, 2*time.Second, "x")}}

	_, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: -1})
	assert.Equal(t, astisub.ErrInvalidReadabilityThreshold, err)
	_, err = s.ReadabilityCheck(astisub.ReadabilityOptions{MaxLineWidth: 0, MaxCPS: -2})
	assert.Equal(t, astisub.ErrInvalidReadabilityThreshold, err)
	_, err = s.ReadabilityCheck(astisub.ReadabilityOptions{MaxLineWidth: -3})
	assert.Equal(t, astisub.ErrInvalidReadabilityThreshold, err)
	_, err = s.ReadabilityCheck(astisub.ReadabilityOptions{OverlapTolerance: -time.Millisecond})
	assert.Equal(t, astisub.ErrInvalidReadabilityThreshold, err)

	// Zero values fall back to defaults instead of being rejected.
	ds, err := s.ReadabilityCheck(astisub.ReadabilityOptions{})
	require.NoError(t, err)
	assert.NotNil(t, ds)
	assert.Empty(t, ds)

	// Default CPS limit is 21: 22 characters in one second is reported.
	fast := &astisub.Subtitles{Items: []*astisub.Item{item(1, time.Second, 2*time.Second, strings.Repeat("a", 22))}}
	ds, err = fast.ReadabilityCheck(astisub.ReadabilityOptions{})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleCPS, ds[0].Rule)
	assert.Equal(t, 21., ds[0].Limit)

	// Default line width is 42 half-width columns. The CPS limit is raised
	// so the width rule is exercised in isolation.
	wide := &astisub.Subtitles{Items: []*astisub.Item{item(1, time.Second, 2*time.Second, strings.Repeat("a", 43))}}
	ds, err = wide.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000})
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, astisub.ReadabilityRuleLineWidth, ds[0].Rule)
	assert.Equal(t, 42., ds[0].Limit)

	// An empty subtitle set returns a non-nil, empty slice.
	empty := &astisub.Subtitles{}
	ds, err = empty.ReadabilityCheck(astisub.ReadabilityOptions{})
	require.NoError(t, err)
	assert.NotNil(t, ds)
	assert.Empty(t, ds)
}

func TestSubtitles_ReadabilityCheck_StableOrder(t *testing.T) {
	styled := &astisub.StyleAttributes{SRTBold: true}
	s := &astisub.Subtitles{Items: []*astisub.Item{
		{
			Index: 1, StartAt: time.Second, EndAt: time.Second, // non-positive duration
			Lines: []astisub.Line{
				{Items: []astisub.LineItem{{Text: strings.Repeat("a", 50), InlineStyle: styled}}},
				{Items: []astisub.LineItem{{Text: "plain"}}},
			},
		},
		item(2, time.Second, 2*time.Second, "z"),           // overlaps item 3
		item(3, 1500*time.Millisecond, 3*time.Second, "w"), // overlaps item 2
	}}
	first, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 10})
	require.NoError(t, err)
	second, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1000, MaxLineWidth: 10})
	require.NoError(t, err)
	assert.Equal(t, first, second)

	// Per-item, then rule, then line, then other item.
	var order []string
	for _, d := range first {
		order = append(order, string(d.Rule))
	}
	assert.Equal(t, []string{
		string(astisub.ReadabilityRuleNonPositiveDuration),
		string(astisub.ReadabilityRuleLineWidth),
		string(astisub.ReadabilityRuleCrossLineStyle),
		string(astisub.ReadabilityRuleOverlap),
	}, order)

	// The overlap is anchored on the item that starts first.
	var overlap *astisub.ReadabilityDiagnostic
	for idx := range first {
		if first[idx].Rule == astisub.ReadabilityRuleOverlap {
			overlap = &first[idx]
		}
	}
	require.NotNil(t, overlap)
	assert.Equal(t, 1, overlap.Index)
	assert.Equal(t, 2, overlap.OtherIndex)
}

func TestSubtitles_ReadabilityCheck_ReadOnly(t *testing.T) {
	src := "1\r\n00:00:01,000 --> 00:00:02,000\r\n<i>hello</i>\r\n\r\n2\r\n00:00:01,500 --> 00:00:03,000\r\n<b>bold</b> world\r\n"
	s, err := astisub.ReadFromSRT(bytes.NewReader([]byte(src)))
	require.NoError(t, err)

	var before bytes.Buffer
	require.NoError(t, s.WriteToSRT(&before))

	_, err = s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 1, MaxLineWidth: 1, OverlapTolerance: 0})
	require.NoError(t, err)

	var after bytes.Buffer
	require.NoError(t, s.WriteToSRT(&after))
	assert.Equal(t, before.String(), after.String())
	// The styled content round-trips losslessly.
	assert.Contains(t, after.String(), "<i>hello</i>")
	assert.Contains(t, after.String(), "<b>bold</b> world")
}

func TestSubtitles_ReadabilityCheck_SRTAndWebVTTConsistency(t *testing.T) {
	// The same five cues in both formats:
	// 1. fast CPS, 2. overlaps cue 1, 3. zero duration, 4. cross-line style,
	// 5. wide CJK line.
	srtIn := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"abcdefghijklmnopqrstuvwxyz",
		"",
		"2",
		"00:00:01,500 --> 00:00:03,000",
		"hi",
		"",
		"3",
		"00:00:03,000 --> 00:00:03,000",
		"zero",
		"",
		"4",
		"00:00:04,000 --> 00:00:05,000",
		"<i>styled</i>",
		"plain",
		"",
		"5",
		"00:00:06,000 --> 00:00:08,000",
		"你好世界你好世界你好世界，你好",
		"",
	}, "\r\n")
	vttIn := strings.Join([]string{
		"WEBVTT",
		"",
		"1",
		"00:00:01.000 --> 00:00:02.000",
		"abcdefghijklmnopqrstuvwxyz",
		"",
		"2",
		"00:00:01.500 --> 00:00:03.000",
		"hi",
		"",
		"3",
		"00:00:03.000 --> 00:00:03.000",
		"zero",
		"",
		"4",
		"00:00:04.000 --> 00:00:05.000",
		"<i>styled</i>",
		"plain",
		"",
		"5",
		"00:00:06.000 --> 00:00:08.000",
		"你好世界你好世界你好世界，你好",
		"",
	}, "\n")

	opts := astisub.ReadabilityOptions{MaxCPS: 21, MaxLineWidth: 20, OverlapTolerance: 100 * time.Millisecond}

	srtSubs, err := astisub.ReadFromSRT(bytes.NewReader([]byte(srtIn)))
	require.NoError(t, err)
	vttSubs, err := astisub.ReadFromWebVTT(bytes.NewReader([]byte(vttIn)))
	require.NoError(t, err)

	srtDiags, err := srtSubs.ReadabilityCheck(opts)
	require.NoError(t, err)
	vttDiags, err := vttSubs.ReadabilityCheck(opts)
	require.NoError(t, err)

	require.Len(t, vttDiags, len(srtDiags))
	for idx := range srtDiags {
		assert.Equal(t, srtDiags[idx], vttDiags[idx], "diagnostic %d differs between SRT and WebVTT", idx)
	}

	// Sanity check on the rules actually exercised by the sample.
	var rules []astisub.ReadabilityRule
	for _, d := range srtDiags {
		rules = append(rules, d.Rule)
	}
	assert.Contains(t, rules, astisub.ReadabilityRuleCPS)
	assert.Contains(t, rules, astisub.ReadabilityRuleOverlap)
	assert.Contains(t, rules, astisub.ReadabilityRuleNonPositiveDuration)
	assert.Contains(t, rules, astisub.ReadabilityRuleCrossLineStyle)
	assert.Contains(t, rules, astisub.ReadabilityRuleLineWidth)

	// The original read/write behavior is unchanged: round-tripping either
	// format still works.
	var srtOut, vttOut bytes.Buffer
	require.NoError(t, srtSubs.WriteToSRT(&srtOut))
	require.NoError(t, vttSubs.WriteToWebVTT(&vttOut))
	assert.Contains(t, srtOut.String(), "abcdefghijklmnopqrstuvwxyz")
	assert.Contains(t, vttOut.String(), "abcdefghijklmnopqrstuvwxyz")
}

func TestSubtitles_ReadabilityCheck_ChineseEnglishMixed(t *testing.T) {
	// 3 CJK ideographs (6 columns) + a full-width comma (2 columns) +
	// 3 ASCII letters (3 columns) = 11 columns, 7 visible runes.
	s := &astisub.Subtitles{Items: []*astisub.Item{
		item(1, time.Second, 2*time.Second, "你好吗，abc"),
	}}
	ds, err := s.ReadabilityCheck(astisub.ReadabilityOptions{MaxCPS: 6, MaxLineWidth: 10})
	require.NoError(t, err)
	// CPS uses runes: 7 cps > 6. Width is 11 > 10. Both reported.
	var rules []astisub.ReadabilityRule
	for _, d := range ds {
		rules = append(rules, d.Rule)
	}
	assert.ElementsMatch(t, []astisub.ReadabilityRule{
		astisub.ReadabilityRuleCPS,
		astisub.ReadabilityRuleLineWidth,
	}, rules)
	for _, d := range ds {
		switch d.Rule {
		case astisub.ReadabilityRuleCPS:
			assert.InDelta(t, 7., d.Measured, 0.001)
		case astisub.ReadabilityRuleLineWidth:
			assert.Equal(t, 11., d.Measured)
		}
	}
}
