package astisub

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// TTML layout defaults, as defined by the TTML1 specification
const (
	ttmlLayoutDefaultCellResolutionColumns = 32
	ttmlLayoutDefaultCellResolutionRows    = 15
	ttmlLayoutDefaultDisplayAlign          = "before"
	ttmlLayoutDefaultTextAlign             = "start"
)

// TTML lengths, e.g. "10%", "100px" or "2c"
var ttmlLayoutRegexpLength = regexp.MustCompile(`^([+-]?[\d]+(\.[\d]+)?)(px|%|c)?$`)

// TTMLRect represents a rectangle in pixels within the video frame
type TTMLRect struct {
	Height float64
	Width  float64
	X      float64
	Y      float64
}

// TTMLLayout represents the evaluated layout of a TTML item for a given video
// size. All dimensions are in pixels within the video frame.
type TTMLLayout struct {
	// Content is the region rectangle shrunk by the resolved padding. This is
	// the area text should be laid out in.
	Content TTMLRect
	// DisplayAlign is the resolved tts:displayAlign value ("before", "center"
	// or "after"), defaulting to "before".
	DisplayAlign string
	// Padding is the resolved padding in pixels, in top, right, bottom, left
	// order.
	Padding [4]float64
	// Region is the pixel rectangle of the item's region, clamped to the video
	// frame.
	Region TTMLRect
	// TextAlign is the resolved tts:textAlign value, defaulting to "start".
	TextAlign string
}

// ttmlLayoutLength represents a parsed TTML length
type ttmlLayoutLength struct {
	unit  string // "", "px", "%" or "c"
	value float64
}

// parseTTMLLayoutLength parses a TTML length. Supported units are "%", "px"
// and "c" (cells). A unitless value is treated as pixels. Any other unit
// (e.g. "em", "pt") is rejected.
func parseTTMLLayoutLength(i string) (l ttmlLayoutLength, err error) {
	m := ttmlLayoutRegexpLength.FindStringSubmatch(strings.TrimSpace(i))
	if m == nil {
		err = fmt.Errorf("astisub: invalid TTML length %q", i)
		return
	}
	if l.value, err = strconv.ParseFloat(m[1], 64); err != nil {
		err = fmt.Errorf("astisub: parsing TTML length %q failed: %w", i, err)
		return
	}
	l.unit = m[3]
	return
}

// parseTTMLLayoutLengths parses a whitespace-separated list of TTML lengths
func parseTTMLLayoutLengths(i string) (o []ttmlLayoutLength, err error) {
	for _, p := range strings.Fields(i) {
		var l ttmlLayoutLength
		if l, err = parseTTMLLayoutLength(p); err != nil {
			return
		}
		o = append(o, l)
	}
	return
}

// ttmlLayoutResolver resolves TTML lengths into video pixels
type ttmlLayoutResolver struct {
	cellHeight  float64 // in root extent units
	cellWidth   float64 // in root extent units
	rootHeight  float64
	rootWidth   float64
	videoHeight float64
	videoWidth  float64
}

// newTTMLLayoutResolver builds a resolver for the given video size and
// metadata. When the metadata carries no root extent, the root extent defaults
// to the video size (pixel values map 1:1). When it carries no cell
// resolution, it defaults to 32x15 as per the TTML1 specification.
func newTTMLLayoutResolver(m *Metadata, videoWidth, videoHeight int) (r *ttmlLayoutResolver, err error) {
	if videoWidth <= 0 || videoHeight <= 0 {
		err = fmt.Errorf("astisub: invalid video size %dx%d", videoWidth, videoHeight)
		return
	}
	r = &ttmlLayoutResolver{
		rootWidth:   float64(videoWidth),
		rootHeight:  float64(videoHeight),
		videoWidth:  float64(videoWidth),
		videoHeight: float64(videoHeight),
	}

	// Root extent
	if m != nil && strings.TrimSpace(m.TTMLRootExtent) != "" {
		var ls []ttmlLayoutLength
		if ls, err = parseTTMLLayoutLengths(m.TTMLRootExtent); err != nil {
			err = fmt.Errorf("astisub: parsing TTML root extent failed: %w", err)
			return
		}
		if len(ls) != 2 {
			err = fmt.Errorf("astisub: TTML root extent %q must contain exactly 2 lengths", m.TTMLRootExtent)
			return
		}
		if ls[0].unit == "c" || ls[1].unit == "c" {
			err = fmt.Errorf("astisub: TTML root extent %q cannot use cell units", m.TTMLRootExtent)
			return
		}
		// Percentages in the root extent are relative to the video frame
		r.rootWidth, r.rootHeight = r.resolve(ls[0], true), r.resolve(ls[1], false)
		if r.rootWidth <= 0 || r.rootHeight <= 0 {
			err = fmt.Errorf("astisub: TTML root extent %q must be positive", m.TTMLRootExtent)
			return
		}
	}

	// Cell resolution
	var columns, rows = ttmlLayoutDefaultCellResolutionColumns, ttmlLayoutDefaultCellResolutionRows
	if m != nil && strings.TrimSpace(m.TTMLCellResolution) != "" {
		var parts = strings.Fields(m.TTMLCellResolution)
		if len(parts) != 2 {
			err = fmt.Errorf("astisub: TTML cell resolution %q must contain exactly 2 integers", m.TTMLCellResolution)
			return
		}
		if columns, err = strconv.Atoi(parts[0]); err != nil {
			err = fmt.Errorf("astisub: parsing TTML cell resolution %q failed: %w", m.TTMLCellResolution, err)
			return
		}
		if rows, err = strconv.Atoi(parts[1]); err != nil {
			err = fmt.Errorf("astisub: parsing TTML cell resolution %q failed: %w", m.TTMLCellResolution, err)
			return
		}
		if columns <= 0 || rows <= 0 {
			err = fmt.Errorf("astisub: TTML cell resolution %q must be positive", m.TTMLCellResolution)
			return
		}
	}
	r.cellWidth = r.rootWidth / float64(columns)
	r.cellHeight = r.rootHeight / float64(rows)
	return
}

// resolve resolves a length along an axis (horizontal if x is true, vertical
// otherwise) into video pixels
func (r *ttmlLayoutResolver) resolve(l ttmlLayoutLength, x bool) float64 {
	var root, video, cell float64
	if x {
		root, video, cell = r.rootWidth, r.videoWidth, r.cellWidth
	} else {
		root, video, cell = r.rootHeight, r.videoHeight, r.cellHeight
	}
	switch l.unit {
	case "%":
		return l.value / 100 * video
	case "c":
		return l.value * cell * video / root
	default: // "px" and unitless
		return l.value * video / root
	}
}

// ttmlLayoutStyleChain returns the chain of style attributes a style inherits
// from, most specific first, and detects circular references
func ttmlLayoutStyleChain(s *Style) (o []*StyleAttributes, err error) {
	var visited = make(map[*Style]bool)
	for s != nil {
		if visited[s] {
			err = fmt.Errorf("astisub: circular TTML style reference detected at style %s", s.ID)
			return
		}
		visited[s] = true
		if s.InlineStyle != nil {
			o = append(o, s.InlineStyle)
		}
		s = s.Style
	}
	return
}

// ttmlLayoutStringAttribute returns the first non nil attribute in the chain
func ttmlLayoutStringAttribute(chain []*StyleAttributes, f func(*StyleAttributes) *string) (o *string) {
	for _, sa := range chain {
		if sa == nil {
			continue
		}
		if o = f(sa); o != nil {
			return
		}
	}
	return
}

// TTMLLayout evaluates the layout of an item for the given video size in
// pixels. It resolves the item's region and style inheritance chain and
// returns the region rectangle, padding, content rectangle and text alignment
// information in video pixels.
//
// The following rules apply:
//   - an item without a region uses the default region, which covers the whole
//     video frame
//   - percentages are relative to the video frame, px and cell values are
//     expressed in the root extent coordinate system (the tts:extent of the
//     root element, defaulting to the video size) and scaled to the video size
//   - cell values use the ttp:cellResolution of the document, defaulting to
//     32x15
//   - a negative extent or padding is an error; a negative origin is clamped
//     to 0
//   - a region overflowing the video frame is clamped to it
func (s *Subtitles) TTMLLayout(item *Item, videoWidth, videoHeight int) (o *TTMLLayout, err error) {
	// Init
	if item == nil {
		err = fmt.Errorf("astisub: item is nil")
		return
	}
	var r *ttmlLayoutResolver
	if r, err = newTTMLLayoutResolver(s.Metadata, videoWidth, videoHeight); err != nil {
		return
	}
	o = &TTMLLayout{
		DisplayAlign: ttmlLayoutDefaultDisplayAlign,
		TextAlign:    ttmlLayoutDefaultTextAlign,
	}

	// Build the region attributes chain: the region's inline attributes take
	// precedence over its style inheritance chain
	var regionChain []*StyleAttributes
	if item.Region != nil {
		if item.Region.InlineStyle != nil {
			regionChain = append(regionChain, item.Region.InlineStyle)
		}
		var styleChain []*StyleAttributes
		if styleChain, err = ttmlLayoutStyleChain(item.Region.Style); err != nil {
			return
		}
		regionChain = append(regionChain, styleChain...)
	}

	// Origin
	var origin = "0% 0%"
	if v := ttmlLayoutStringAttribute(regionChain, func(sa *StyleAttributes) *string { return sa.TTMLOrigin }); v != nil {
		origin = *v
	}
	var originLengths []ttmlLayoutLength
	if originLengths, err = parseTTMLLayoutLengths(origin); err != nil {
		err = fmt.Errorf("astisub: parsing TTML origin failed: %w", err)
		return
	}
	if len(originLengths) != 2 {
		err = fmt.Errorf("astisub: TTML origin %q must contain exactly 2 lengths", origin)
		return
	}
	o.Region.X = r.resolve(originLengths[0], true)
	o.Region.Y = r.resolve(originLengths[1], false)

	// Extent
	var extent = "100% 100%"
	if v := ttmlLayoutStringAttribute(regionChain, func(sa *StyleAttributes) *string { return sa.TTMLExtent }); v != nil {
		extent = *v
	}
	var extentLengths []ttmlLayoutLength
	if extentLengths, err = parseTTMLLayoutLengths(extent); err != nil {
		err = fmt.Errorf("astisub: parsing TTML extent failed: %w", err)
		return
	}
	if len(extentLengths) != 2 {
		err = fmt.Errorf("astisub: TTML extent %q must contain exactly 2 lengths", extent)
		return
	}
	if extentLengths[0].value < 0 || extentLengths[1].value < 0 {
		err = fmt.Errorf("astisub: TTML extent %q must not be negative", extent)
		return
	}
	o.Region.Width = r.resolve(extentLengths[0], true)
	o.Region.Height = r.resolve(extentLengths[1], false)

	// Clamp the region to the video frame
	if o.Region.X < 0 {
		o.Region.Width += o.Region.X
		o.Region.X = 0
	}
	if o.Region.Y < 0 {
		o.Region.Height += o.Region.Y
		o.Region.Y = 0
	}
	if o.Region.X+o.Region.Width > r.videoWidth {
		o.Region.Width = r.videoWidth - o.Region.X
	}
	if o.Region.Y+o.Region.Height > r.videoHeight {
		o.Region.Height = r.videoHeight - o.Region.Y
	}
	if o.Region.Width < 0 {
		o.Region.Width = 0
	}
	if o.Region.Height < 0 {
		o.Region.Height = 0
	}

	// Padding, in top, right, bottom, left order following the CSS shorthand
	// rules
	if v := ttmlLayoutStringAttribute(regionChain, func(sa *StyleAttributes) *string { return sa.TTMLPadding }); v != nil {
		var paddingLengths []ttmlLayoutLength
		if paddingLengths, err = parseTTMLLayoutLengths(*v); err != nil {
			err = fmt.Errorf("astisub: parsing TTML padding failed: %w", err)
			return
		}
		if len(paddingLengths) == 0 || len(paddingLengths) > 4 {
			err = fmt.Errorf("astisub: TTML padding %q must contain between 1 and 4 lengths", *v)
			return
		}
		for _, l := range paddingLengths {
			if l.value < 0 {
				err = fmt.Errorf("astisub: TTML padding %q must not be negative", *v)
				return
			}
		}
		var top, right, bottom, left ttmlLayoutLength
		switch len(paddingLengths) {
		case 1:
			top, right, bottom, left = paddingLengths[0], paddingLengths[0], paddingLengths[0], paddingLengths[0]
		case 2:
			top, bottom = paddingLengths[0], paddingLengths[0]
			right, left = paddingLengths[1], paddingLengths[1]
		case 3:
			top, right, bottom = paddingLengths[0], paddingLengths[1], paddingLengths[2]
			left = right
		case 4:
			top, right, bottom, left = paddingLengths[0], paddingLengths[1], paddingLengths[2], paddingLengths[3]
		}
		o.Padding = [4]float64{
			r.resolve(top, false),
			r.resolve(right, true),
			r.resolve(bottom, false),
			r.resolve(left, true),
		}
	}

	// Content rectangle
	o.Content = TTMLRect{
		X:      o.Region.X + o.Padding[3],
		Y:      o.Region.Y + o.Padding[0],
		Width:  o.Region.Width - o.Padding[1] - o.Padding[3],
		Height: o.Region.Height - o.Padding[0] - o.Padding[2],
	}
	if o.Content.Width < 0 {
		o.Content.Width = 0
	}
	if o.Content.Height < 0 {
		o.Content.Height = 0
	}

	// Display align
	if v := ttmlLayoutStringAttribute(regionChain, func(sa *StyleAttributes) *string { return sa.TTMLDisplayAlign }); v != nil {
		o.DisplayAlign = *v
	}

	// Text align: the item's inline attributes take precedence over its style
	// inheritance chain, which takes precedence over the region's
	var itemChain []*StyleAttributes
	if item.InlineStyle != nil {
		itemChain = append(itemChain, item.InlineStyle)
	}
	var itemStyleChain []*StyleAttributes
	if itemStyleChain, err = ttmlLayoutStyleChain(item.Style); err != nil {
		return
	}
	itemChain = append(itemChain, itemStyleChain...)
	if v := ttmlLayoutStringAttribute(itemChain, func(sa *StyleAttributes) *string { return sa.TTMLTextAlign }); v != nil {
		o.TextAlign = *v
	} else if v := ttmlLayoutStringAttribute(regionChain, func(sa *StyleAttributes) *string { return sa.TTMLTextAlign }); v != nil {
		o.TextAlign = *v
	}
	return
}
