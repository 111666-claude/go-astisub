package astisub

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTextWebVTT(t *testing.T) {

	t.Run("When both voice tags are available", func(t *testing.T) {
		testData := `<v Bob>Correct tag</v>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		assert.Equal(t, "Bob", s.VoiceName)
		assert.Equal(t, 1, len(s.Items))
		assert.Equal(t, "Correct tag", s.Items[0].Text)
	})

	t.Run("When there is no end tag", func(t *testing.T) {
		testData := `<v Bob>Text without end tag`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		assert.Equal(t, "Bob", s.VoiceName)
		assert.Equal(t, 1, len(s.Items))
		assert.Equal(t, "Text without end tag", s.Items[0].Text)
	})

	t.Run("When the end tag is correct", func(t *testing.T) {
		testData := `<v Bob>Incorrect end tag</vi>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		assert.Equal(t, "Bob", s.VoiceName)
		assert.Equal(t, 1, len(s.Items))
		assert.Equal(t, "Incorrect end tag", s.Items[0].Text)
	})

	t.Run("When inline timestamps are included", func(t *testing.T) {
		testData := `<00:01:01.000>With inline <00:01:02.000>timestamps`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		assert.Equal(t, 2, len(s.Items))
		assert.Equal(t, "With inline ", s.Items[0].Text)
		assert.Equal(t, time.Minute+time.Second, s.Items[0].StartAt)
		assert.Equal(t, "timestamps", s.Items[1].Text)
		assert.Equal(t, time.Minute+2*time.Second, s.Items[1].StartAt)
	})

	t.Run("When inline timestamps together", func(t *testing.T) {
		testData := `<00:01:01.000><00:01:02.000>With timestamp tags together`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		assert.Equal(t, 1, len(s.Items))
		assert.Equal(t, "With timestamp tags together", s.Items[0].Text)
		assert.Equal(t, time.Minute+2*time.Second, s.Items[0].StartAt)
	})

	t.Run("When inline timestamps is at end", func(t *testing.T) {
		testData := `With end timestamp<00:01:02.000>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		assert.Equal(t, 1, len(s.Items))
		assert.Equal(t, "With end timestamp", s.Items[0].Text)
		assert.Equal(t, time.Duration(0), s.Items[0].StartAt)
	})

	t.Run("When two voices appear in the same line", func(t *testing.T) {
		testData := `<v Alice>Hello,</v> <v Bob>hi!</v>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		assert.Equal(t, "Alice", s.VoiceName)
		require.Len(t, s.Items, 3)
		assert.Equal(t, "Hello,", s.Items[0].Text)
		assert.Equal(t, "Alice", s.Items[0].VoiceName)
		assert.Equal(t, " ", s.Items[1].Text)
		assert.Equal(t, "", s.Items[1].VoiceName)
		assert.Equal(t, "hi!", s.Items[2].Text)
		assert.Equal(t, "Bob", s.Items[2].VoiceName)
	})

	t.Run("When the voice switches three times", func(t *testing.T) {
		testData := `<v Alice>one</v><v Bob>two</v><v Alice>three</v>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		require.Len(t, s.Items, 3)
		assert.Equal(t, "Alice", s.Items[0].VoiceName)
		assert.Equal(t, "Bob", s.Items[1].VoiceName)
		assert.Equal(t, "Alice", s.Items[2].VoiceName)
	})

	t.Run("When voice tags are nested", func(t *testing.T) {
		testData := `<v Alice>a<v Bob>b</v>c</v>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		require.Len(t, s.Items, 3)
		assert.Equal(t, "a", s.Items[0].Text)
		assert.Equal(t, "Alice", s.Items[0].VoiceName)
		assert.Equal(t, "b", s.Items[1].Text)
		assert.Equal(t, "Bob", s.Items[1].VoiceName)
		assert.Equal(t, "c", s.Items[2].Text)
		assert.Equal(t, "Alice", s.Items[2].VoiceName)
	})

	t.Run("When a voice tag is not closed before the next one", func(t *testing.T) {
		testData := `<v Alice>a<v Bob>b`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		require.Len(t, s.Items, 2)
		assert.Equal(t, "Alice", s.Items[0].VoiceName)
		assert.Equal(t, "Bob", s.Items[1].VoiceName)
	})

	t.Run("When the speaker is empty", func(t *testing.T) {
		testData := `<v Alice>a</v><v>b</v>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		require.Len(t, s.Items, 2)
		assert.Equal(t, "Alice", s.Items[0].VoiceName)
		// The text of an empty speaker is attributed to no one, not to Alice
		assert.Equal(t, "b", s.Items[1].Text)
		assert.Equal(t, "", s.Items[1].VoiceName)
	})

	t.Run("When voices are mixed with inline styles and plain text", func(t *testing.T) {
		testData := `<v Alice><i>Hello</i></v> plain <v Bob>hi</v>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		require.Len(t, s.Items, 3)
		assert.Equal(t, "Hello", s.Items[0].Text)
		assert.Equal(t, "Alice", s.Items[0].VoiceName)
		require.NotNil(t, s.Items[0].InlineStyle)
		require.Len(t, s.Items[0].InlineStyle.WebVTTTags, 1)
		assert.Equal(t, "i", s.Items[0].InlineStyle.WebVTTTags[0].Name)
		assert.Equal(t, " plain ", s.Items[1].Text)
		assert.Equal(t, "", s.Items[1].VoiceName)
		assert.Nil(t, s.Items[1].InlineStyle)
		assert.Equal(t, "hi", s.Items[2].Text)
		assert.Equal(t, "Bob", s.Items[2].VoiceName)
	})

	t.Run("When the text contains invalid escapes", func(t *testing.T) {
		testData := `<v Alice>a &invalid; & b</v>`

		s := parseTextWebVTT(testData, &StyleAttributes{})
		require.Len(t, s.Items, 1)
		// Invalid entities are kept as is instead of being dropped
		assert.Equal(t, "a &invalid; & b", s.Items[0].Text)
		assert.Equal(t, "Alice", s.Items[0].VoiceName)
	})
}

func TestTimestampMap(t *testing.T) {
	for i, c := range []struct {
		line           string
		expectedOffset time.Duration
		expectError    bool
	}{
		{
			line:           "X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:180000",
			expectedOffset: 2 * time.Second,
		},
		{
			line:           "X-TIMESTAMP-MAP=LOCAL:00:00:00.500,MPEGTS:180000",
			expectedOffset: 1500 * time.Millisecond,
		},
		{
			line:           "X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:135000",
			expectedOffset: 1500 * time.Millisecond,
		},
		{
			line:           "X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:324090000",
			expectedOffset: time.Hour + time.Second,
		},
		{
			line:        "X-TIMESTAMP-MAP=MPEGTS:foo, LOCAL:00:00:00.000",
			expectError: true,
		},
		{
			line:        "X-TIMESTAMP-MAP=MPEGTS:180000,LOCAL:bar",
			expectError: true,
		},
		{
			line:        "X-TIMESTAMP-MAP=MPEGTS:180000,LOCAL",
			expectError: true,
		},
		{
			line:        "X-TIMESTAMP-MAP=MPEGTS,LOCAL:00:00:00.000",
			expectError: true,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			timestampMap, err := parseWebVTTTimestampMap(c.line)
			assert.Equal(t, c.expectedOffset, timestampMap.Offset())
			if c.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.line, timestampMap.String())
			}
		})
	}
}

func TestCueVoiceSpanRegex(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{
			give: `<v 中文> this is the content</v>`,
			want: `中文`,
		},
		{
			give: `<v 中文> this is the content`,
			want: `中文`,
		},
		{
			give: `<v.abc 中文> this is the content</v>`,
			want: `中文`,
		},
		{
			give: `<v.jp 言語の> this is the content`,
			want: `言語の`,
		},
		{
			give: `<v.ko 언어> this is the content`,
			want: `언어`,
		},
		{
			give: `<v foo bar> this is the content`,
			want: `foo bar`,
		},
		{
			give: `<v هذا عربي> this is the content`,
			want: `هذا عربي`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			results := webVTTRegexpTag.FindStringSubmatch(tt.give)
			assert.True(t, len(results) == 5)
			assert.Equal(t, tt.want, results[4])
		})
	}
}

func TestLineWebVTTBytes(t *testing.T) {
	require.Equal(t, "<t1>1 <t2>2</t2> 3</t1>\n", string(Line{Items: []LineItem{
		{
			InlineStyle: &StyleAttributes{WebVTTTags: []WebVTTTag{
				{Name: "t1"},
			}},
			Text: "1 ",
		},
		{
			InlineStyle: &StyleAttributes{WebVTTTags: []WebVTTTag{
				{Name: "t1"},
				{Name: "t2"},
			}},
			Text: "2",
		},
		{
			InlineStyle: &StyleAttributes{WebVTTTags: []WebVTTTag{
				{Name: "t1"},
			}},
			Text: " 3",
		},
	}}.webVTTBytes()))
}

func TestLineWebVTTBytesWithVoices(t *testing.T) {
	// A single voice keeps the historical output: one opening tag, no closing tag
	require.Equal(t, "<v Alice>1 2\n", string(Line{
		VoiceName: "Alice",
		Items: []LineItem{
			{Text: "1 ", VoiceName: "Alice"},
			{Text: "2", VoiceName: "Alice"},
		},
	}.webVTTBytes()))

	// Multiple voices are written as ordered <v> spans, inline styles are
	// closed and reopened across voice boundaries instead of overlapping them
	require.Equal(t, "<v Alice><i>1</i></v> 2 <v Bob><i>3</i></v>\n", string(Line{
		VoiceName: "Alice",
		Items: []LineItem{
			{
				InlineStyle: &StyleAttributes{WebVTTTags: []WebVTTTag{{Name: "i"}}},
				Text:        "1",
				VoiceName:   "Alice",
			},
			{Text: " 2 "},
			{
				InlineStyle: &StyleAttributes{WebVTTTags: []WebVTTTag{{Name: "i"}}},
				Text:        "3",
				VoiceName:   "Bob",
			},
		},
	}.webVTTBytes()))
}
