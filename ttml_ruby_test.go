package astisub_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTTMLRubyImport checks that ruby containers are parsed into ordered
// base/annotation pairs alongside ordinary spans, while preserving the cue's
// timing and the existing style machinery.
func TestTTMLRubyImport(t *testing.T) {
	s, err := astisub.OpenFile("./testdata/example-in-ruby.ttml")
	require.NoError(t, err)
	require.Len(t, s.Items, 3)

	// Timing is untouched.
	assert.Equal(t, 1_000_000_000, int(s.Items[0].StartAt))
	assert.Equal(t, 3_500_000_000, int(s.Items[0].EndAt))
	assert.Equal(t, 24, s.Metadata.Framerate)

	// First cue: span, ruby, span / line break / ruby.
	first := s.Items[0]
	require.Len(t, first.Lines, 2)

	// Line 1: "それは" span, ruby container with two pairs, "です。" span.
	line1 := first.Lines[0].Items
	require.Len(t, line1, 3)
	assert.Equal(t, "それは", line1[0].Text)
	assert.Nil(t, line1[0].Ruby)

	require.Len(t, line1[1].Ruby, 2)
	assert.Equal(t, "rubyBase", line1[1].Style.ID)
	pair := line1[1].Ruby[0]
	require.NotNil(t, pair.Base)
	require.NotNil(t, pair.Text)
	assert.Equal(t, "漢字", pair.Base.Text)
	assert.Equal(t, "rubyBase", pair.Base.Style.ID)
	assert.Equal(t, "かんじ", pair.Text.Text)
	assert.Equal(t, "rubyText", pair.Text.Style.ID)
	require.NotNil(t, pair.Text.InlineStyle)
	assert.Equal(t, "#ff0000", pair.Text.InlineStyle.TTMLColor.HTMLString())
	pair = line1[1].Ruby[1]
	assert.Equal(t, "と", pair.Base.Text)
	assert.Equal(t, "と", pair.Text.Text)

	assert.Equal(t, "です。", line1[2].Text)

	// Line 2: a second ruby container keeps the same ordering.
	line2 := first.Lines[1].Items
	require.Len(t, line2, 1)
	require.Len(t, line2[0].Ruby, 2)
	assert.Equal(t, "日", line2[0].Ruby[0].Base.Text)
	assert.Equal(t, "に", line2[0].Ruby[0].Text.Text)
	assert.Equal(t, "本", line2[0].Ruby[1].Base.Text)
	assert.Equal(t, "ほん", line2[0].Ruby[1].Text.Text)

	// Second cue: edge cases.
	second := s.Items[1].Lines[0].Items
	// ruby container + orphan "rt" decoded as an ordinary item.
	require.Len(t, second, 2)
	ruby := second[0]
	require.Len(t, ruby.Ruby, 4)

	// Container-level style attributes (including the integer zIndex) are
	// parsed like on a regular span.
	require.NotNil(t, ruby.InlineStyle)
	require.NotNil(t, ruby.InlineStyle.TTMLZIndex)
	assert.Equal(t, 2, *ruby.InlineStyle.TTMLZIndex)

	// Empty base is preserved as an empty component, annotation still paired.
	assert.Equal(t, "", ruby.Ruby[0].Base.Text)
	require.NotNil(t, ruby.Ruby[0].Text)
	assert.Equal(t, "から", ruby.Ruby[0].Text.Text)

	// Nested conflict: nested markup inside "rb" is flattened to its text.
	assert.Equal(t, "ABCD", ruby.Ruby[1].Base.Text)
	assert.Equal(t, "エックス", ruby.Ruby[1].Text.Text)

	// Unknown "rtc" role is skipped; the following pair remains paired.
	assert.Equal(t, "音", ruby.Ruby[2].Base.Text)
	assert.Equal(t, "おと", ruby.Ruby[2].Text.Text)

	// Base without an annotation survives with a nil Text component.
	assert.Equal(t, "last", ruby.Ruby[3].Base.Text)
	assert.Nil(t, ruby.Ruby[3].Text)

	// Orphan "rt" outside any ruby container is treated as plain text.
	assert.Equal(t, "orphan", second[1].Text)
	assert.Nil(t, second[1].Ruby)

	// Third cue: ordinary subtitles keep working exactly as before.
	third := s.Items[2].Lines[0].Items
	require.Len(t, third, 1)
	assert.Equal(t, "普通の字幕", third[0].Text)
	require.NotNil(t, third[0].InlineStyle)
	assert.Equal(t, "#008000", third[0].InlineStyle.TTMLColor.HTMLString())
}

// TestTTMLRubyRoundTrip writes the parsed subtitles back and compares the
// result byte-for-byte with the golden fixture, then re-reads the output and
// asserts the ruby pairs survive the second import.
func TestTTMLRubyRoundTrip(t *testing.T) {
	s, err := astisub.OpenFile("./testdata/example-in-ruby.ttml")
	require.NoError(t, err)

	w := &bytes.Buffer{}
	require.NoError(t, s.WriteToTTML(w))

	golden, err := ioutil.ReadFile("./testdata/example-out-ruby.ttml")
	require.NoError(t, err)
	assert.Equal(t, string(golden), w.String())

	// Re-reading the written document must preserve the model.
	reRead, err := astisub.ReadFromTTML(bytes.NewReader(w.Bytes()))
	require.NoError(t, err)
	require.Len(t, reRead.Items, 3)
	require.Len(t, reRead.Items[0].Lines, 2)
	pairs := reRead.Items[0].Lines[0].Items[1].Ruby
	require.Len(t, pairs, 2)
	assert.Equal(t, "漢字", pairs[0].Base.Text)
	assert.Equal(t, "かんじ", pairs[0].Text.Text)
	assert.Equal(t, "と", pairs[1].Base.Text)
	assert.Equal(t, "と", pairs[1].Text.Text)
}

// TestTTMLRubyInvalidXML makes sure malformed XML in a cue containing ruby is
// reported instead of silently producing a truncated container.
func TestTTMLRubyInvalidXML(t *testing.T) {
	input := []byte(`<tt xmlns="http://www.w3.org/ns/ttml"><body><div>
		<p begin="00:00:01.000" end="00:00:02.000">
			<ruby><rb>漢字</rb><rt>かんじ</rt>
		</p>
	</div></body></tt>`)
	_, err := astisub.ReadFromTTML(bytes.NewReader(input))
	assert.Error(t, err)
}

// TestTTMLRubyMissingStyle makes sure a style referenced by a ruby container
// or component that does not exist produces an explicit error.
func TestTTMLRubyMissingStyle(t *testing.T) {
	input := []byte(`<tt xmlns="http://www.w3.org/ns/ttml"><body><div>
		<p begin="00:00:01.000" end="00:00:02.000">
			<ruby style="missing">
				<rb style="alsoMissing">漢字</rb>
				<rt>かんじ</rt>
			</ruby>
		</p>
	</div></body></tt>`)
	_, err := astisub.ReadFromTTML(bytes.NewReader(input))
	assert.Error(t, err)
}

// TestTTMLRubyInvalidZIndex makes sure a non-integer zIndex is rejected rather
// than silently dropped, matching regular span decoding.
func TestTTMLRubyInvalidZIndex(t *testing.T) {
	input := []byte(`<tt xmlns="http://www.w3.org/ns/ttml"><body><div>
		<p begin="00:00:01.000" end="00:00:02.000">
			<ruby tts:zIndex="not-a-number"><rb>漢字</rb><rt>かんじ</rt></ruby>
		</p>
	</div></body></tt>`)
	_, err := astisub.ReadFromTTML(bytes.NewReader(input))
	assert.Error(t, err)
}

// TestTTMLRubyOptimizeKeepsStyles checks that styles referenced only by ruby
// components are retained when unused styles are pruned.
func TestTTMLRubyOptimizeKeepsStyles(t *testing.T) {
	s, err := astisub.OpenFile("./testdata/example-in-ruby.ttml")
	require.NoError(t, err)
	assert.Contains(t, s.Styles, "rubyBase")
	assert.Contains(t, s.Styles, "rubyText")

	s.Optimize()
	assert.Contains(t, s.Styles, "rubyBase")
	assert.Contains(t, s.Styles, "rubyText")
}
