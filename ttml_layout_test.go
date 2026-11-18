package astisub_test

import (
	"strings"
	"testing"

	"github.com/asticode/go-astisub"
	"github.com/stretchr/testify/assert"
)

func TestTTMLLayout(t *testing.T) {
	// Open
	s, err := astisub.OpenFile("./testdata/example-in-layout.ttml")
	assert.NoError(t, err)
	assert.Equal(t, 3, len(s.Items))
	assert.Equal(t, "1920px 1080px", s.Metadata.TTMLRootExtent)
	assert.Equal(t, "40 20", s.Metadata.TTMLCellResolution)

	// Evaluate at 1280x720: the root extent is 1920x1080 so px and cell
	// values are scaled by 2/3, and cells are 1920/40 x 1080/20 = 48x54 root
	// units, i.e. 32x36 video pixels

	// Item with an inline region mixing %, px and cell units
	l, err := s.TTMLLayout(s.Items[0], 1280, 720)
	assert.NoError(t, err)
	assert.InDelta(t, 128, l.Region.X, 1e-9)
	assert.InDelta(t, 576, l.Region.Y, 1e-9)
	assert.InDelta(t, 1024, l.Region.Width, 1e-9)
	assert.InDelta(t, 108, l.Region.Height, 1e-9)
	assert.InDelta(t, 36, l.Padding[0], 1e-9) // top: 1c
	assert.InDelta(t, 64, l.Padding[1], 1e-9) // right: 2c
	assert.InDelta(t, 36, l.Padding[2], 1e-9) // bottom: 1c
	assert.InDelta(t, 64, l.Padding[3], 1e-9) // left: 2c
	assert.InDelta(t, 192, l.Content.X, 1e-9) // 128 + 64
	assert.InDelta(t, 612, l.Content.Y, 1e-9) // 576 + 36
	assert.InDelta(t, 896, l.Content.Width, 1e-9)
	assert.InDelta(t, 36, l.Content.Height, 1e-9)
	assert.Equal(t, "after", l.DisplayAlign)
	assert.Equal(t, "center", l.TextAlign)

	// Item whose region inherits its origin and text alignment through a
	// style chain, with a px extent
	l, err = s.TTMLLayout(s.Items[1], 1280, 720)
	assert.NoError(t, err)
	assert.InDelta(t, 320, l.Region.X, 1e-9)
	assert.InDelta(t, 36, l.Region.Y, 1e-9)
	assert.InDelta(t, 640, l.Region.Width, 1e-9) // 960px scaled by 2/3
	assert.InDelta(t, 72, l.Region.Height, 1e-9) // 108px scaled by 2/3
	assert.Equal(t, "before", l.DisplayAlign)
	assert.Equal(t, "center", l.TextAlign)

	// Item without a region uses the default region covering the whole frame
	l, err = s.TTMLLayout(s.Items[2], 1280, 720)
	assert.NoError(t, err)
	assert.Equal(t, astisub.TTMLRect{X: 0, Y: 0, Width: 1280, Height: 720}, l.Region)
	assert.Equal(t, l.Region, l.Content)
	assert.Equal(t, "before", l.DisplayAlign)
	assert.Equal(t, "start", l.TextAlign)

	// The same TTML scales with the video size
	l, err = s.TTMLLayout(s.Items[0], 1920, 1080)
	assert.NoError(t, err)
	assert.Equal(t, astisub.TTMLRect{X: 192, Y: 864, Width: 1536, Height: 162}, l.Region)
	assert.Equal(t, [4]float64{54, 96, 54, 96}, l.Padding)
	assert.Equal(t, astisub.TTMLRect{X: 288, Y: 918, Width: 1344, Height: 54}, l.Content)

	l, err = s.TTMLLayout(s.Items[0], 640, 360)
	assert.NoError(t, err)
	assert.Equal(t, astisub.TTMLRect{X: 64, Y: 288, Width: 512, Height: 54}, l.Region)
	assert.Equal(t, [4]float64{18, 32, 18, 32}, l.Padding)
	assert.Equal(t, astisub.TTMLRect{X: 96, Y: 306, Width: 448, Height: 18}, l.Content)

	// Invalid video size
	_, err = s.TTMLLayout(s.Items[0], 0, 720)
	assert.EqualError(t, err, "astisub: invalid video size 0x720")

	// Nil item
	_, err = s.TTMLLayout(nil, 1280, 720)
	assert.EqualError(t, err, "astisub: item is nil")
}

func ttmlLayoutFromString(t *testing.T, i string, videoWidth, videoHeight int) (*astisub.TTMLLayout, error) {
	s, err := astisub.ReadFromTTML(strings.NewReader(i))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(s.Items))
	return s.TTMLLayout(s.Items[0], videoWidth, videoHeight)
}

func TestTTMLLayoutCircularStyle(t *testing.T) {
	_, err := ttmlLayoutFromString(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling">
		<head>
			<styling>
				<style xml:id="a" style="b" tts:origin="0% 0%"/>
				<style xml:id="b" style="a" tts:extent="100% 100%"/>
			</styling>
			<layout><region xml:id="r" style="a"/></layout>
		</head>
		<body><div><p begin="00:00:01.000" end="00:00:02.000" region="r">text</p></div></body>
	</tt>`, 1280, 720)
	assert.EqualError(t, err, "astisub: circular TTML style reference detected at style a")
}

func TestTTMLLayoutInvalidUnit(t *testing.T) {
	_, err := ttmlLayoutFromString(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling">
		<head><layout><region xml:id="r" tts:origin="10em 20%" tts:extent="80% 60%"/></layout></head>
		<body><div><p begin="00:00:01.000" end="00:00:02.000" region="r">text</p></div></body>
	</tt>`, 1280, 720)
	assert.EqualError(t, err, `astisub: parsing TTML origin failed: astisub: invalid TTML length "10em"`)
}

func TestTTMLLayoutNegativeExtent(t *testing.T) {
	_, err := ttmlLayoutFromString(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling">
		<head><layout><region xml:id="r" tts:origin="10% 10%" tts:extent="-10% 20%"/></layout></head>
		<body><div><p begin="00:00:01.000" end="00:00:02.000" region="r">text</p></div></body>
	</tt>`, 1280, 720)
	assert.EqualError(t, err, `astisub: TTML extent "-10% 20%" must not be negative`)
}

func TestTTMLLayoutOverflowingRegion(t *testing.T) {
	// A region overflowing the video frame is clamped to it
	l, err := ttmlLayoutFromString(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling">
		<head><layout><region xml:id="r" tts:origin="90% 90%" tts:extent="50% 50%"/></layout></head>
		<body><div><p begin="00:00:01.000" end="00:00:02.000" region="r">text</p></div></body>
	</tt>`, 1000, 1000)
	assert.NoError(t, err)
	assert.Equal(t, astisub.TTMLRect{X: 900, Y: 900, Width: 100, Height: 100}, l.Region)

	// A negative origin is clamped to 0 and the extent shrunk accordingly
	l, err = ttmlLayoutFromString(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling">
		<head><layout><region xml:id="r" tts:origin="-10% 10%" tts:extent="50% 50%"/></layout></head>
		<body><div><p begin="00:00:01.000" end="00:00:02.000" region="r">text</p></div></body>
	</tt>`, 1000, 1000)
	assert.NoError(t, err)
	assert.Equal(t, astisub.TTMLRect{X: 0, Y: 100, Width: 400, Height: 500}, l.Region)
}

func TestTTMLLayoutMissingRootExtent(t *testing.T) {
	// Without a root extent, px values map 1:1 to video pixels and cells use
	// the default 32x15 cell resolution
	l, err := ttmlLayoutFromString(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling">
		<head><layout><region xml:id="r" tts:origin="10px 2c" tts:extent="320px 200px"/></layout></head>
		<body><div><p begin="00:00:01.000" end="00:00:02.000" region="r">text</p></div></body>
	</tt>`, 800, 600)
	assert.NoError(t, err)
	assert.Equal(t, astisub.TTMLRect{X: 10, Y: 80, Width: 320, Height: 200}, l.Region)
}
