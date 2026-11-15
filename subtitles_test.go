package astisub_test

import (
	"bytes"
	"io"
	"math"
	"os"
	"testing"
	"time"

	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLine_Text(t *testing.T) {
	var l = astisub.Line{Items: []astisub.LineItem{{Text: "1"}, {Text: "2"}, {Text: "3"}}}
	assert.Equal(t, "123", l.String())
}

func assertSubtitleItems(t *testing.T, i *astisub.Subtitles) {
	// No format
	assert.Len(t, i.Items, 6)
	assert.Equal(t, time.Minute+39*time.Second, i.Items[0].StartAt)
	assert.Equal(t, time.Minute+41*time.Second+40*time.Millisecond, i.Items[0].EndAt)
	assert.Equal(t, "(deep rumbling)", i.Items[0].Lines[0].String())
	assert.Equal(t, 2*time.Minute+4*time.Second+80*time.Millisecond, i.Items[1].StartAt)
	assert.Equal(t, 2*time.Minute+7*time.Second+120*time.Millisecond, i.Items[1].EndAt)
	assert.Equal(t, "MAN:", i.Items[1].Lines[0].String())
	assert.Equal(t, "How did we end up here?", i.Items[1].Lines[1].String())
	assert.Equal(t, 2*time.Minute+12*time.Second+160*time.Millisecond, i.Items[2].StartAt)
	assert.Equal(t, 2*time.Minute+15*time.Second+200*time.Millisecond, i.Items[2].EndAt)
	assert.Equal(t, "This place is horrible.", i.Items[2].Lines[0].String())
	assert.Equal(t, 2*time.Minute+20*time.Second+240*time.Millisecond, i.Items[3].StartAt)
	assert.Equal(t, 2*time.Minute+22*time.Second+280*time.Millisecond, i.Items[3].EndAt)
	assert.Equal(t, "Smells like balls.", i.Items[3].Lines[0].String())
	assert.Equal(t, 2*time.Minute+28*time.Second+320*time.Millisecond, i.Items[4].StartAt)
	assert.Equal(t, 2*time.Minute+31*time.Second+360*time.Millisecond, i.Items[4].EndAt)
	assert.Equal(t, "We don't belong", i.Items[4].Lines[0].String())
	assert.Equal(t, "in this shithole.", i.Items[4].Lines[1].String())
	assert.Equal(t, 2*time.Minute+31*time.Second+400*time.Millisecond, i.Items[5].StartAt)
	assert.Equal(t, 2*time.Minute+33*time.Second+440*time.Millisecond, i.Items[5].EndAt)
	assert.Equal(t, "(computer playing", i.Items[5].Lines[0].String())
	assert.Equal(t, "electronic melody)", i.Items[5].Lines[1].String())
}

func mockSubtitles() *astisub.Subtitles {
	return &astisub.Subtitles{Items: []*astisub.Item{{EndAt: 3 * time.Second, StartAt: time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-1"}}}}}, {EndAt: 7 * time.Second, StartAt: 3 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-2"}}}}}}}
}

func TestSubtitles_Add(t *testing.T) {
	var s = mockSubtitles()
	s.Add(time.Second)
	assert.Len(t, s.Items, 2)
	assert.Equal(t, 2*time.Second, s.Items[0].StartAt)
	assert.Equal(t, 4*time.Second, s.Items[0].EndAt)
	assert.Equal(t, 2*time.Second, s.Items[0].StartAt)
	assert.Equal(t, 4*time.Second, s.Items[0].EndAt)
	s.Add(-3 * time.Second)
	assert.Len(t, s.Items, 2)
	assert.Equal(t, time.Duration(0), s.Items[0].StartAt)
	assert.Equal(t, time.Second, s.Items[0].EndAt)
	s.Add(-2 * time.Second)
	assert.Len(t, s.Items, 1)
	assert.Equal(t, "subtitle-2", s.Items[0].Lines[0].Items[0].Text)
}

func TestSubtitles_Duration(t *testing.T) {
	assert.Equal(t, time.Duration(0), astisub.Subtitles{}.Duration())
	assert.Equal(t, 7*time.Second, mockSubtitles().Duration())
}

func TestSubtitles_IsEmpty(t *testing.T) {
	assert.True(t, astisub.Subtitles{}.IsEmpty())
	assert.False(t, mockSubtitles().IsEmpty())
}

func TestSubtitles_ForceDuration(t *testing.T) {
	var s = mockSubtitles()
	s.ForceDuration(10*time.Second, false)
	assert.Len(t, s.Items, 2)
	s = mockSubtitles()
	s.ForceDuration(10*time.Second, true)
	assert.Len(t, s.Items, 3)
	assert.Equal(t, 10*time.Second, s.Items[2].EndAt)
	assert.Equal(t, 10*time.Second-time.Millisecond, s.Items[2].StartAt)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{Text: "..."}}}}, s.Items[2].Lines)
	s.Items[2].StartAt = 7 * time.Second
	s.Items[2].EndAt = 12 * time.Second
	s.ForceDuration(10*time.Second, true)
	assert.Len(t, s.Items, 3)
	assert.Equal(t, 10*time.Second, s.Items[2].EndAt)
	assert.Equal(t, 7*time.Second, s.Items[2].StartAt)
}

func TestSubtitles_Fragment(t *testing.T) {
	// Init
	var s = mockSubtitles()

	// Fragment
	s.Fragment(2 * time.Second)
	assert.Len(t, s.Items, 5)
	assert.Equal(t, time.Second, s.Items[0].StartAt)
	assert.Equal(t, 2*time.Second, s.Items[0].EndAt)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-1"}}}}, s.Items[0].Lines)
	assert.Equal(t, 2*time.Second, s.Items[1].StartAt)
	assert.Equal(t, 3*time.Second, s.Items[1].EndAt)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-1"}}}}, s.Items[1].Lines)
	assert.Equal(t, 3*time.Second, s.Items[2].StartAt)
	assert.Equal(t, 4*time.Second, s.Items[2].EndAt)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-2"}}}}, s.Items[2].Lines)
	assert.Equal(t, 4*time.Second, s.Items[3].StartAt)
	assert.Equal(t, 6*time.Second, s.Items[3].EndAt)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-2"}}}}, s.Items[3].Lines)
	assert.Equal(t, 6*time.Second, s.Items[4].StartAt)
	assert.Equal(t, 7*time.Second, s.Items[4].EndAt)
	assert.Equal(t, []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-2"}}}}, s.Items[4].Lines)

	// Unfragment
	s.Items = append(s.Items[:4], append([]*astisub.Item{{EndAt: 5 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "subtitle-3"}}}}, StartAt: 4 * time.Second}}, s.Items[4:]...)...)
	s.Unfragment()
	assert.Len(t, s.Items, 3)
	assert.Equal(t, "subtitle-1", s.Items[0].String())
	assert.Equal(t, time.Second, s.Items[0].StartAt)
	assert.Equal(t, 3*time.Second, s.Items[0].EndAt)
	assert.Equal(t, "subtitle-2", s.Items[1].String())
	assert.Equal(t, 3*time.Second, s.Items[1].StartAt)
	assert.Equal(t, 7*time.Second, s.Items[1].EndAt)
	assert.Equal(t, "subtitle-3", s.Items[2].String())
	assert.Equal(t, 4*time.Second, s.Items[2].StartAt)
	assert.Equal(t, 5*time.Second, s.Items[2].EndAt)
}

func TestSubtitles_Unfragment(t *testing.T) {
	itemText := func(s string) []astisub.Line {
		return []astisub.Line{{Items: []astisub.LineItem{{Text: s}}}}
	}
	items := []*astisub.Item{{
		Lines:   itemText("subtitle-1"),
		StartAt: 1 * time.Second,
		EndAt:   2 * time.Second,
	}, {
		Lines:   itemText("subtitle-2"),
		StartAt: 2 * time.Second,
		EndAt:   5 * time.Second,
	}, {
		Lines:   itemText("subtitle-3"),
		StartAt: 3 * time.Second,
		EndAt:   4 * time.Second,
	}, {
		// gap and nested within first subtitle-2; should not override end time
		Lines:   itemText("subtitle-2"),
		StartAt: 3 * time.Second,
		EndAt:   4 * time.Second,
	}, {
		Lines: itemText("subtitle-3"),
		// gap and start time equals previous end time
		StartAt: 4 * time.Second,
		EndAt:   5 * time.Second,
	}, {
		// should not be combined
		Lines:   itemText("subtitle-3"),
		StartAt: 6 * time.Second,
		EndAt:   7 * time.Second,
	}, {
		// test correcting for out-of-orderness
		Lines:   itemText("subtitle-1"),
		StartAt: 0 * time.Second,
		EndAt:   3 * time.Second,
	}}

	s := &astisub.Subtitles{Items: items}

	s.Unfragment()

	expected := []astisub.Item{{
		Lines:   itemText("subtitle-1"),
		StartAt: 0 * time.Second,
		EndAt:   3 * time.Second,
	}, {
		Lines:   itemText("subtitle-2"),
		StartAt: 2 * time.Second,
		EndAt:   5 * time.Second,
	}, {
		Lines:   itemText("subtitle-3"),
		StartAt: 3 * time.Second,
		EndAt:   5 * time.Second,
	}, {
		Lines:   itemText("subtitle-3"),
		StartAt: 6 * time.Second,
		EndAt:   7 * time.Second,
	}}

	assert.Equal(t, len(expected), len(s.Items))
	for i := range expected {
		assert.Equal(t, expected[i], *s.Items[i])
	}
}

func TestSubtitles_Merge(t *testing.T) {
	var s1 = &astisub.Subtitles{Items: []*astisub.Item{{EndAt: 3 * time.Second, StartAt: time.Second}, {EndAt: 8 * time.Second, StartAt: 5 * time.Second}, {EndAt: 12 * time.Second, StartAt: 10 * time.Second}}, Regions: map[string]*astisub.Region{"region_0": {ID: "region_0"}, "region_1": {ID: "region_1"}}, Styles: map[string]*astisub.Style{"style_0": {ID: "style_0"}, "style_1": {ID: "style_1"}}}
	var s2 = &astisub.Subtitles{Items: []*astisub.Item{{EndAt: 4 * time.Second, StartAt: 2 * time.Second}, {EndAt: 7 * time.Second, StartAt: 6 * time.Second}, {EndAt: 11 * time.Second, StartAt: 9 * time.Second}, {EndAt: 14 * time.Second, StartAt: 13 * time.Second}}, Regions: map[string]*astisub.Region{"region_1": {ID: "region_1"}, "region_2": {ID: "region_2"}}, Styles: map[string]*astisub.Style{"style_1": {ID: "style_1"}, "style_2": {ID: "style_2"}}}
	s1.Merge(s2)
	assert.Len(t, s1.Items, 7)
	assert.Equal(t, &astisub.Item{EndAt: 3 * time.Second, StartAt: time.Second}, s1.Items[0])
	assert.Equal(t, &astisub.Item{EndAt: 4 * time.Second, StartAt: 2 * time.Second}, s1.Items[1])
	assert.Equal(t, &astisub.Item{EndAt: 8 * time.Second, StartAt: 5 * time.Second}, s1.Items[2])
	assert.Equal(t, &astisub.Item{EndAt: 7 * time.Second, StartAt: 6 * time.Second}, s1.Items[3])
	assert.Equal(t, &astisub.Item{EndAt: 11 * time.Second, StartAt: 9 * time.Second}, s1.Items[4])
	assert.Equal(t, &astisub.Item{EndAt: 12 * time.Second, StartAt: 10 * time.Second}, s1.Items[5])
	assert.Equal(t, &astisub.Item{EndAt: 14 * time.Second, StartAt: 13 * time.Second}, s1.Items[6])
	assert.Equal(t, len(s1.Regions), 3)
	assert.Equal(t, len(s1.Styles), 3)
}

func TestSubtitles_Optimize(t *testing.T) {
	var s = &astisub.Subtitles{
		Items: []*astisub.Item{
			{Region: &astisub.Region{ID: "1"}},
			{Style: &astisub.Style{ID: "1"}},
			{Lines: []astisub.Line{{Items: []astisub.LineItem{{Style: &astisub.Style{ID: "2"}}}}}},
		},
		Regions: map[string]*astisub.Region{
			"1": {ID: "1", Style: &astisub.Style{ID: "3"}},
			"2": {ID: "2", Style: &astisub.Style{ID: "4"}},
		},
		Styles: map[string]*astisub.Style{
			"1": {ID: "1"},
			"2": {ID: "2"},
			"3": {ID: "3"},
			"4": {ID: "4"},
			"5": {ID: "5"},
		},
	}
	s.Optimize()
	assert.Len(t, s.Regions, 1)
	assert.Len(t, s.Styles, 3)
}

func TestSubtitles_Order(t *testing.T) {
	var s = &astisub.Subtitles{Items: []*astisub.Item{{StartAt: 4 * time.Second, EndAt: 5 * time.Second}, {StartAt: 2 * time.Second, EndAt: 3 * time.Second}, {StartAt: 3 * time.Second, EndAt: 4 * time.Second}, {StartAt: time.Second, EndAt: 2 * time.Second}}}
	s.Order()
	assert.Equal(t, time.Second, s.Items[0].StartAt)
	assert.Equal(t, 2*time.Second, s.Items[0].EndAt)
	assert.Equal(t, 2*time.Second, s.Items[1].StartAt)
	assert.Equal(t, 3*time.Second, s.Items[1].EndAt)
	assert.Equal(t, 3*time.Second, s.Items[2].StartAt)
	assert.Equal(t, 4*time.Second, s.Items[2].EndAt)
	assert.Equal(t, 4*time.Second, s.Items[3].StartAt)
	assert.Equal(t, 5*time.Second, s.Items[3].EndAt)
}

func TestSubtitles_RemoveStyling(t *testing.T) {
	s := &astisub.Subtitles{
		Items: []*astisub.Item{
			{
				Lines: []astisub.Line{{
					Items: []astisub.LineItem{{
						InlineStyle: &astisub.StyleAttributes{},
						Style:       &astisub.Style{},
					}},
				}},
				InlineStyle: &astisub.StyleAttributes{},
				Region:      &astisub.Region{},
				Style:       &astisub.Style{},
			},
		},
		Regions: map[string]*astisub.Region{"region": {}},
		Styles:  map[string]*astisub.Style{"style": {}},
	}
	s.RemoveStyling()
	assert.Equal(t, &astisub.Subtitles{
		Items: []*astisub.Item{
			{
				Lines: []astisub.Line{{
					Items: []astisub.LineItem{{}},
				}},
			},
		},
		Regions: map[string]*astisub.Region{},
		Styles:  map[string]*astisub.Style{},
	}, s)
}

func TestSubtitles_ApplyLinearCorrection(t *testing.T) {
	s := &astisub.Subtitles{Items: []*astisub.Item{
		{
			EndAt:   2 * time.Second,
			StartAt: 1 * time.Second,
		},
		{
			EndAt:   5 * time.Second,
			StartAt: 3 * time.Second,
		},
		{
			EndAt:   10 * time.Second,
			StartAt: 7 * time.Second,
		},
	}}
	s.ApplyLinearCorrection(3*time.Second, 5*time.Second, 5*time.Second, 8*time.Second)
	require.Equal(t, 2*time.Second, s.Items[0].StartAt)
	require.Equal(t, 3500*time.Millisecond, s.Items[0].EndAt)
	require.Equal(t, 5*time.Second, s.Items[1].StartAt)
	require.Equal(t, 8*time.Second, s.Items[1].EndAt)
	require.Equal(t, 11*time.Second, s.Items[2].StartAt)
	require.Equal(t, 15500*time.Millisecond, s.Items[2].EndAt)
}

func TestSubtitles_ApplyAnchoredCorrection(t *testing.T) {
	// Anchors describing 3 segments with different drifts:
	//   [0s, 100s]   -> [0s, 100s]   (no drift)
	//   [100s, 200s] -> [100s, 150s] (slope 0.5)
	//   [200s, 300s] -> [150s, 225s] (slope 0.75)
	var anchors = []astisub.TimeAnchor{
		{Source: 0, Target: 0},
		{Source: 100 * time.Second, Target: 100 * time.Second},
		{Source: 200 * time.Second, Target: 150 * time.Second},
		{Source: 300 * time.Second, Target: 225 * time.Second},
	}
	var s = &astisub.Subtitles{
		Items: []*astisub.Item{
			// Fully inside the first segment
			{StartAt: 10 * time.Second, EndAt: 20 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "1"}}}}},
			// Exactly on anchors
			{StartAt: 100 * time.Second, EndAt: 200 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "4"}}}}},
			// Spanning the anchor at 200s
			{StartAt: 150 * time.Second, EndAt: 250 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "2"}}}}},
			// Overlapping the previous item
			{StartAt: 160 * time.Second, EndAt: 170 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "3"}}}}},
			// After the last anchor, extrapolated
			{StartAt: 400 * time.Second, EndAt: 410 * time.Second, Lines: []astisub.Line{{Items: []astisub.LineItem{{Text: "5"}}}}},
		},
		Metadata: &astisub.Metadata{Comments: []string{"comment"}},
	}
	n, err := s.ApplyAnchoredCorrection(anchors)
	require.NoError(t, err)

	// Mapped times
	require.Len(t, n.Items, 5)
	require.Equal(t, 10*time.Second, n.Items[0].StartAt)
	require.Equal(t, 20*time.Second, n.Items[0].EndAt)
	require.Equal(t, 100*time.Second, n.Items[1].StartAt)
	require.Equal(t, 150*time.Second, n.Items[1].EndAt)
	require.Equal(t, 125*time.Second, n.Items[2].StartAt)
	require.Equal(t, 187500*time.Millisecond, n.Items[2].EndAt)
	require.Equal(t, 130*time.Second, n.Items[3].StartAt)
	require.Equal(t, 135*time.Second, n.Items[3].EndAt)
	require.Equal(t, 300*time.Second, n.Items[4].StartAt)
	require.Equal(t, 307500*time.Millisecond, n.Items[4].EndAt)

	// Order is preserved
	for idx := 1; idx < len(n.Items); idx++ {
		require.True(t, n.Items[idx].StartAt >= n.Items[idx-1].StartAt)
	}

	// Text and metadata are kept
	require.Equal(t, "2", n.Items[2].Lines[0].String())
	require.NotNil(t, n.Metadata)
	require.Equal(t, []string{"comment"}, n.Metadata.Comments)

	// The original is left unmodified
	require.Equal(t, 150*time.Second, s.Items[2].StartAt)
	require.Equal(t, 250*time.Second, s.Items[2].EndAt)
	require.Equal(t, 400*time.Second, s.Items[4].StartAt)
}

func TestSubtitles_ApplyAnchoredCorrection_Errors(t *testing.T) {
	var item = &astisub.Item{StartAt: time.Second, EndAt: 2 * time.Second}
	var s = &astisub.Subtitles{Items: []*astisub.Item{item}}

	// Not enough anchors
	for _, anchors := range [][]astisub.TimeAnchor{
		{},
		{{Source: 0, Target: 0}},
	} {
		_, err := s.ApplyAnchoredCorrection(anchors)
		require.Error(t, err)
	}

	// Sources not strictly increasing
	_, err := s.ApplyAnchoredCorrection([]astisub.TimeAnchor{
		{Source: 0, Target: 0},
		{Source: 10 * time.Second, Target: 10 * time.Second},
		{Source: 10 * time.Second, Target: 20 * time.Second},
	})
	require.Error(t, err)

	// Targets not strictly increasing
	_, err = s.ApplyAnchoredCorrection([]astisub.TimeAnchor{
		{Source: 0, Target: 0},
		{Source: 10 * time.Second, Target: 10 * time.Second},
		{Source: 20 * time.Second, Target: 10 * time.Second},
	})
	require.Error(t, err)

	// Mapped to a negative time
	_, err = s.ApplyAnchoredCorrection([]astisub.TimeAnchor{
		{Source: 10 * time.Second, Target: 0},
		{Source: 20 * time.Second, Target: time.Second},
	})
	require.Error(t, err)

	// Mapped to a time that overflows
	_, err = s.ApplyAnchoredCorrection([]astisub.TimeAnchor{
		{Source: 0, Target: 0},
		{Source: time.Second, Target: time.Duration(math.MaxInt64) / 2},
	})
	require.Error(t, err)

	// End mapped before start
	_, err = (&astisub.Subtitles{Items: []*astisub.Item{{StartAt: 2 * time.Second, EndAt: time.Second}}}).ApplyAnchoredCorrection([]astisub.TimeAnchor{
		{Source: 0, Target: 0},
		{Source: 10 * time.Second, Target: 10 * time.Second},
	})
	require.Error(t, err)
}

func TestSubtitles_ApplyAnchoredCorrection_RoundTrip(t *testing.T) {
	var anchors = []astisub.TimeAnchor{
		{Source: 0, Target: 0},
		{Source: 100 * time.Second, Target: 100 * time.Second},
		{Source: 200 * time.Second, Target: 150 * time.Second},
	}
	var s = &astisub.Subtitles{Items: []*astisub.Item{
		{
			StartAt: 120 * time.Second,
			EndAt:   130 * time.Second,
			Lines: []astisub.Line{{Items: []astisub.LineItem{
				{Text: "first"},
				{StartAt: 125 * time.Second, Text: "second"},
			}}},
		},
	}}
	n, err := s.ApplyAnchoredCorrection(anchors)
	require.NoError(t, err)
	require.Equal(t, 110*time.Second, n.Items[0].StartAt)
	require.Equal(t, 115*time.Second, n.Items[0].EndAt)
	require.Equal(t, 112500*time.Millisecond, n.Items[0].Lines[0].Items[1].StartAt)

	// The original line items are left unmodified
	require.Equal(t, 125*time.Second, s.Items[0].Lines[0].Items[1].StartAt)

	// Times are consistent after writing and reading back
	for _, e := range []struct {
		ext   string
		read  func(r io.Reader) (*astisub.Subtitles, error)
		write func(s *astisub.Subtitles, w io.Writer) error
	}{
		{ext: "srt", read: astisub.ReadFromSRT, write: func(s *astisub.Subtitles, w io.Writer) error { return s.WriteToSRT(w) }},
		{ext: "vtt", read: astisub.ReadFromWebVTT, write: func(s *astisub.Subtitles, w io.Writer) error { return s.WriteToWebVTT(w) }},
	} {
		var b = &bytes.Buffer{}
		require.NoError(t, e.write(n, b), e.ext)
		r, err := e.read(b)
		require.NoError(t, err, e.ext)
		require.Len(t, r.Items, 1, e.ext)
		require.Equal(t, n.Items[0].StartAt, r.Items[0].StartAt, e.ext)
		require.Equal(t, n.Items[0].EndAt, r.Items[0].EndAt, e.ext)
		require.Equal(t, "firstsecond", r.Items[0].Lines[0].String(), e.ext)
	}
}

func TestHTMLEntity(t *testing.T) {
	exts := []string{"srt", "vtt"}
	for _, ext := range exts {
		// Read input with entities
		s, err := astisub.OpenFile("./testdata/example-in-html-entities." + ext)
		assert.NoError(t, err)

		assert.Len(t, s.Items, 3)
		assert.Equal(t, 331*time.Millisecond, s.Items[0].StartAt)
		assert.Equal(t, 3*time.Second+750*time.Millisecond, s.Items[0].EndAt)
		assert.Equal(t, "The man in black fled across the desert, \u00A0", s.Items[0].Lines[0].String())
		assert.Equal(t, "& the gunslinger followed.", s.Items[0].Lines[1].String())
		assert.Equal(t, 4*time.Second+101*time.Millisecond, s.Items[1].StartAt)
		assert.Equal(t, 5*time.Second+430*time.Millisecond, s.Items[1].EndAt)
		assert.Equal(t, "Go,\u00A0then,", s.Items[1].Lines[0].String())
		assert.Equal(t, 6*time.Second+331*time.Millisecond, s.Items[2].StartAt)
		assert.Equal(t, 9*time.Second+675*time.Millisecond, s.Items[2].EndAt)
		assert.Equal(t, "there are other < worlds than these.", s.Items[2].Lines[0].String())

		//Write to srt
		w := &bytes.Buffer{}
		c, err := os.ReadFile("./testdata/example-out-html-entities.srt")
		assert.NoError(t, err)
		err = s.WriteToSRT(w)
		assert.NoError(t, err)
		assert.Equal(t, string(c), w.String())

		//Write WebVTT
		w = &bytes.Buffer{}
		c, err = os.ReadFile("./testdata/example-out-html-entities.vtt")
		assert.NoError(t, err)
		err = s.WriteToWebVTT(w)
		assert.NoError(t, err)
		assert.Equal(t, string(c), w.String())
	}
}

func TestNewScanner(t *testing.T) {
	exts := []string{"vtt", "srt", "ssa"}
	for _, ext := range exts {
		s, err := astisub.OpenFile("./testdata/example-in-carriage-return." + ext)
		assert.NoError(t, err)
		assert.Len(t, s.Items, 3)
		assert.Equal(t, time.Duration(0), s.Items[0].StartAt)
		assert.Equal(t, 3*time.Second+766*time.Millisecond, s.Items[0].EndAt)
		assert.Equal(t, "Did one of the last stories strike you as", s.Items[0].Lines[0].String())
		assert.Equal(t, "more interesting than the other?", s.Items[0].Lines[1].String())

		assert.Equal(t, 3*time.Second+767*time.Millisecond, s.Items[1].StartAt)
		assert.Equal(t, 10*time.Second+732*time.Millisecond, s.Items[1].EndAt)
		assert.Equal(t, "That's true. You don’t often find 632", s.Items[1].Lines[0].String())
		assert.Equal(t, "pieces of gum stuck on a sidewalk", s.Items[1].Lines[1].String())

		assert.Equal(t, 10*time.Second+733*time.Millisecond, s.Items[2].StartAt)
		assert.Equal(t, 14*time.Second+66*time.Millisecond, s.Items[2].EndAt)
		assert.Equal(t, "at a busy bus stop or anywhere", s.Items[2].Lines[0].String())
		assert.Equal(t, "else for that matter.", s.Items[2].Lines[1].String())
	}
}
