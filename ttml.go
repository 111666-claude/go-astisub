package astisub

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/asticode/go-astikit"
)

// https://www.w3.org/TR/ttaf1-dfxp/
// http://www.skynav.com:8080/ttv/check
// https://www.speechpad.com/captions/ttml

// TTML languages
const (
	ttmlLanguageChinese   = "zh"
	ttmlLanguageEnglish   = "en"
	ttmlLanguageJapanese  = "ja"
	ttmlLanguageFrench    = "fr"
	ttmlLanguageNorwegian = "no"
)

// TTML language mapping
var ttmlLanguageMapping = astikit.NewBiMap().
	Set(ttmlLanguageChinese, LanguageChinese).
	Set(ttmlLanguageEnglish, LanguageEnglish).
	Set(ttmlLanguageFrench, LanguageFrench).
	Set(ttmlLanguageJapanese, LanguageJapanese).
	Set(ttmlLanguageNorwegian, LanguageNorwegian)

// TTML Clock Time Frames and Offset Time
var (
	ttmlRegexpClockTimeFrames = regexp.MustCompile(`\:[\d]+$`)
	ttmlRegexpOffsetTime      = regexp.MustCompile(`^(\d+(\.\d+)?)(h|m|s|ms|f|t)$`)
)

type TTMLInBodyDiv struct {
	XMLName   xml.Name         `xml:"div"`
	Subtitles []TTMLInSubtitle `xml:"p"`

	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}
type TTMLInBody struct {
	XMLName xml.Name        `xml:"body"`
	Divs    []TTMLInBodyDiv `xml:"div"`

	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}

// TTMLIn represents an input TTML that must be unmarshaled
// We split it from the output TTML as we can't add strict namespace without breaking retrocompatibility
type TTMLIn struct {
	Framerate int            `xml:"frameRate,attr"`
	Lang      string         `xml:"lang,attr"`
	Metadata  TTMLInMetadata `xml:"head>metadata"`
	Regions   []TTMLInRegion `xml:"head>layout>region"`
	Styles    []TTMLInStyle  `xml:"head>styling>style"`
	Body      TTMLInBody     `xml:"body"`
	Tickrate  int            `xml:"tickRate,attr"`
	XMLName   xml.Name       `xml:"tt"`
}

// metadata returns the Metadata of the TTML
func (t TTMLIn) metadata() (m *Metadata) {
	m = &Metadata{
		Framerate:     t.Framerate,
		Title:         t.Metadata.Title,
		TTMLCopyright: t.Metadata.Copyright,
	}
	if v, ok := ttmlLanguageMapping.Get(astikit.StrPad(t.Lang, ' ', 2, astikit.PadCut)); ok {
		m.Language = v.(string)
	}
	return
}

// TTMLInMetadata represents an input TTML Metadata
type TTMLInMetadata struct {
	Copyright string `xml:"copyright"`
	Title     string `xml:"title"`
}

// TTMLInStyleAttributes represents input TTML style attributes
type TTMLInStyleAttributes struct {
	BackgroundColor *string `xml:"backgroundColor,attr,omitempty"`
	Color           *string `xml:"color,attr,omitempty"`
	Direction       *string `xml:"direction,attr,omitempty"`
	Display         *string `xml:"display,attr,omitempty"`
	DisplayAlign    *string `xml:"displayAlign,attr,omitempty"`
	Extent          *string `xml:"extent,attr,omitempty"`
	FontFamily      *string `xml:"fontFamily,attr,omitempty"`
	FontSize        *string `xml:"fontSize,attr,omitempty"`
	FontStyle       *string `xml:"fontStyle,attr,omitempty"`
	FontWeight      *string `xml:"fontWeight,attr,omitempty"`
	LineHeight      *string `xml:"lineHeight,attr,omitempty"`
	Opacity         *string `xml:"opacity,attr,omitempty"`
	Origin          *string `xml:"origin,attr,omitempty"`
	Overflow        *string `xml:"overflow,attr,omitempty"`
	Padding         *string `xml:"padding,attr,omitempty"`
	ShowBackground  *string `xml:"showBackground,attr,omitempty"`
	TextAlign       *string `xml:"textAlign,attr,omitempty"`
	TextDecoration  *string `xml:"textDecoration,attr,omitempty"`
	TextOutline     *string `xml:"textOutline,attr,omitempty"`
	UnicodeBidi     *string `xml:"unicodeBidi,attr,omitempty"`
	Visibility      *string `xml:"visibility,attr,omitempty"`
	WrapOption      *string `xml:"wrapOption,attr,omitempty"`
	WritingMode     *string `xml:"writingMode,attr,omitempty"`
	ZIndex          *int    `xml:"zIndex,attr,omitempty"`
}

// StyleAttributes converts TTMLInStyleAttributes into a StyleAttributes
func (s TTMLInStyleAttributes) styleAttributes() (o *StyleAttributes) {
	o = &StyleAttributes{
		TTMLDirection:      s.Direction,
		TTMLDisplay:        s.Display,
		TTMLDisplayAlign:   s.DisplayAlign,
		TTMLExtent:         s.Extent,
		TTMLFontFamily:     s.FontFamily,
		TTMLFontSize:       s.FontSize,
		TTMLFontStyle:      s.FontStyle,
		TTMLFontWeight:     s.FontWeight,
		TTMLLineHeight:     s.LineHeight,
		TTMLOpacity:        s.Opacity,
		TTMLOrigin:         s.Origin,
		TTMLOverflow:       s.Overflow,
		TTMLPadding:        s.Padding,
		TTMLShowBackground: s.ShowBackground,
		TTMLTextAlign:      s.TextAlign,
		TTMLTextDecoration: s.TextDecoration,
		TTMLTextOutline:    s.TextOutline,
		TTMLUnicodeBidi:    s.UnicodeBidi,
		TTMLVisibility:     s.Visibility,
		TTMLWrapOption:     s.WrapOption,
		TTMLWritingMode:    s.WritingMode,
		TTMLZIndex:         s.ZIndex,
	}
	// Parse colors if present
	if s.Color != nil {
		o.TTMLColor = newColorFromHTMLString(*s.Color)
	}
	if s.BackgroundColor != nil {
		o.TTMLBackgroundColor = newColorFromHTMLString(*s.BackgroundColor)
	}
	o.propagateTTMLAttributes()
	return
}

// TTMLInHeader represents an input TTML header
type TTMLInHeader struct {
	ID    string `xml:"id,attr,omitempty"`
	Style string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}

// TTMLInRegion represents an input TTML region
type TTMLInRegion struct {
	TTMLInHeader
	XMLName xml.Name `xml:"region"`
}

// TTMLInStyle represents an input TTML style
type TTMLInStyle struct {
	TTMLInHeader
	XMLName xml.Name `xml:"style"`
}

// TTMLInSubtitle represents an input TTML subtitle
type TTMLInSubtitle struct {
	Begin *TTMLInDuration `xml:"begin,attr,omitempty"`
	End   *TTMLInDuration `xml:"end,attr,omitempty"`
	ID    string          `xml:"id,attr,omitempty"`
	// We must store inner XML temporarily here since there's no tag to describe both any tag and chardata
	// Real unmarshal will be done manually afterwards
	Items  string `xml:",innerxml"`
	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLInStyleAttributes
}

// TTMLInItems represents input TTML items
type TTMLInItems []TTMLInItem

// UnmarshalXML implements the XML unmarshaler interface
func (i *TTMLInItems) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	// Get next tokens
	var t xml.Token
	for {
		// Get next token
		if t, err = d.Token(); err != nil {
			if err == io.EOF {
				break
			}
			err = fmt.Errorf("astisub: getting next token failed: %w", err)
			return
		}

		// Start element
		if se, ok := t.(xml.StartElement); ok {
			// A ruby container needs a custom token-stream decode because its
			// "rb"/"rt" children are interleaved siblings.
			if se.Name.Local == "ruby" {
				var e TTMLInItem
				if e, err = decodeTTMLInRuby(d, se); err != nil {
					return
				}
				*i = append(*i, e)
				continue
			}
			var e = TTMLInItem{}
			if err = d.DecodeElement(&e, &se); err != nil {
				err = fmt.Errorf("astisub: decoding xml.StartElement failed: %w", err)
				return
			}
			*i = append(*i, e)
		} else if b, ok := t.(xml.CharData); ok {
			if str := string(b); len(strings.TrimSpace(str)) > 0 {
				*i = append(*i, TTMLInItem{Text: str})
			}
		}
	}
	return nil
}

type ttmlXmlTokenReader struct {
	xmlTokenReader xml.TokenReader
	holdingToken   xml.Token
}

// Token implements the TokenReader interface, when it meets the "br" tag, it will hold the token and return a newline
// instead. This is to work around the fact that the go xml unmarshaler will ignore the "br" tag if it's within a
// character data field.
func (r *ttmlXmlTokenReader) Token() (xml.Token, error) {
	if r.holdingToken != nil {
		returnToken := r.holdingToken
		r.holdingToken = nil
		return returnToken, nil
	}

	t, err := r.xmlTokenReader.Token()
	if err != nil {
		return nil, err
	}

	if se, ok := t.(xml.StartElement); ok && strings.ToLower(se.Name.Local) == "br" {
		r.holdingToken = t
		return xml.CharData("\n"), nil
	}

	return t, nil
}

func newTTMLXmlDecoder(s string) *xml.Decoder {
	return xml.NewTokenDecoder(
		&ttmlXmlTokenReader{
			xmlTokenReader: xml.NewDecoder(strings.NewReader("<p>" + s + "</p>")),
			holdingToken:   nil,
		},
	)
}

// TTMLInItem represents an input TTML item
type TTMLInItem struct {
	Style string `xml:"style,attr,omitempty"`
	Text  string `xml:",chardata"`
	// Ruby is populated only when the item is a "ruby" container. The custom
	// TTMLInItems unmarshaler detects the container and decodes the ordered
	// base/annotation pairs itself since their interleaved "rb"/"rt" siblings
	// cannot be described with struct tags.
	Ruby []TTMLInRubyPair
	TTMLInStyleAttributes
	XMLName xml.Name
}

// TTMLInRubyComponent represents a "rb" (base) or "rt" (annotation) element of
// a ruby container
type TTMLInRubyComponent struct {
	Style string `xml:"style,attr,omitempty"`
	Text  string `xml:",chardata"`
	TTMLInStyleAttributes
}

// TTMLInRubyPair associates a ruby base with its reading. Either component may
// be nil when the container is missing the corresponding element.
type TTMLInRubyPair struct {
	Base *TTMLInRubyComponent
	Text *TTMLInRubyComponent
}

// decodeTTMLInRuby decodes a "ruby" container by walking its token stream.
//
// "rb" and "rt" siblings are interleaved, so each "rb" opens a new pair and a
// following "rt" attaches to it. An "rt" without a pending base is kept as a
// base-less pair (orphan annotation), while unknown children ("rtc", nested
// "ruby", a "span" inside "rb", ...) are skipped entirely: their content is
// ambiguous and re-emitting it would break the base/annotation pairing.
// Invalid XML surfaces as an error instead of a truncated container.
func decodeTTMLInRuby(d *xml.Decoder, start xml.StartElement) (item TTMLInItem, err error) {
	item.XMLName = start.Name
	if item.TTMLInStyleAttributes, err = startTTMLInStyleAttributes(start); err != nil {
		err = fmt.Errorf("astisub: decoding ruby container attributes failed: %w", err)
		return
	}
	for _, attr := range start.Attr {
		if attr.Name.Local == "style" {
			item.Style = attr.Value
		}
	}

	for {
		var t xml.Token
		if t, err = d.Token(); err != nil {
			err = fmt.Errorf("astisub: decoding ruby container failed: %w", err)
			return
		}
		switch tok := t.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "rb":
				var c TTMLInRubyComponent
				if err = d.DecodeElement(&c, &tok); err != nil {
					err = fmt.Errorf("astisub: decoding ruby base failed: %w", err)
					return
				}
				// An "rb" always starts a new pair, even when its text is
				// empty (empty base is preserved as an empty component).
				item.Ruby = append(item.Ruby, TTMLInRubyPair{Base: &c})
			case "rt":
				var c TTMLInRubyComponent
				if err = d.DecodeElement(&c, &tok); err != nil {
					err = fmt.Errorf("astisub: decoding ruby text failed: %w", err)
					return
				}
				// Attach to the pending base when the last pair is base-only.
				if n := len(item.Ruby); n > 0 && item.Ruby[n-1].Base != nil && item.Ruby[n-1].Text == nil {
					item.Ruby[n-1].Text = &c
				} else {
					// Orphan "rt": no pending base to annotate.
					item.Ruby = append(item.Ruby, TTMLInRubyPair{Text: &c})
				}
			default:
				// Unknown role ("rtc", nested "ruby", ...): discard the whole
				// subtree rather than guessing how to pair its content.
				if err = d.Skip(); err != nil {
					err = fmt.Errorf("astisub: skipping unknown ruby child %q failed: %w", tok.Name.Local, err)
					return
				}
			}
		case xml.EndElement:
			if tok.Name.Local == "ruby" {
				return
			}
		}
	}
}

// startTTMLInStyleAttributes extracts the supported styling attributes of a
// start element into a TTMLInStyleAttributes. Like the struct-tag unmarshaler
// used for regular spans, attributes are matched on their local name whatever
// their namespace (so "tts:color" matches a "color" field); only namespace
// declarations themselves are skipped.
func startTTMLInStyleAttributes(start xml.StartElement) (TTMLInStyleAttributes, error) {
	var a TTMLInStyleAttributes
	for _, attr := range start.Attr {
		if attr.Name.Space == "xmlns" || (attr.Name.Space == "" && attr.Name.Local == "xmlns") {
			continue
		}
		switch attr.Name.Local {
		case "backgroundColor":
			v := attr.Value
			a.BackgroundColor = &v
		case "color":
			v := attr.Value
			a.Color = &v
		case "direction":
			v := attr.Value
			a.Direction = &v
		case "display":
			v := attr.Value
			a.Display = &v
		case "displayAlign":
			v := attr.Value
			a.DisplayAlign = &v
		case "extent":
			v := attr.Value
			a.Extent = &v
		case "fontFamily":
			v := attr.Value
			a.FontFamily = &v
		case "fontSize":
			v := attr.Value
			a.FontSize = &v
		case "fontStyle":
			v := attr.Value
			a.FontStyle = &v
		case "fontWeight":
			v := attr.Value
			a.FontWeight = &v
		case "lineHeight":
			v := attr.Value
			a.LineHeight = &v
		case "opacity":
			v := attr.Value
			a.Opacity = &v
		case "origin":
			v := attr.Value
			a.Origin = &v
		case "overflow":
			v := attr.Value
			a.Overflow = &v
		case "padding":
			v := attr.Value
			a.Padding = &v
		case "showBackground":
			v := attr.Value
			a.ShowBackground = &v
		case "textAlign":
			v := attr.Value
			a.TextAlign = &v
		case "textDecoration":
			v := attr.Value
			a.TextDecoration = &v
		case "textOutline":
			v := attr.Value
			a.TextOutline = &v
		case "unicodeBidi":
			v := attr.Value
			a.UnicodeBidi = &v
		case "visibility":
			v := attr.Value
			a.Visibility = &v
		case "wrapOption":
			v := attr.Value
			a.WrapOption = &v
		case "writingMode":
			v := attr.Value
			a.WritingMode = &v
		case "zIndex":
			// An invalid integer is rejected like the struct-tag decoding
			// used for regular spans does with *int attributes.
			v, err := strconv.Atoi(attr.Value)
			if err != nil {
				return a, fmt.Errorf("astisub: invalid zIndex %q: %w", attr.Value, err)
			}
			a.ZIndex = &v
		}
	}
	return a, nil
}

// TTMLInDuration represents an input TTML duration
type TTMLInDuration struct {
	d                 time.Duration
	frames, framerate int // Framerate is in frame/s
	ticks, tickrate   int // Tickrate is in ticks/s
}

// UnmarshalText implements the TextUnmarshaler interface
// Possible formats are:
// - hh:mm:ss.mmm
// - hh:mm:ss:fff (fff being frames)
// - [ticks]t ([ticks] being the tick amount)
func (d *TTMLInDuration) UnmarshalText(i []byte) (err error) {
	// Reset duration
	d.d = time.Duration(0)
	d.frames = 0
	d.ticks = 0

	// Check offset time
	text := string(i)
	if matches := ttmlRegexpOffsetTime.FindStringSubmatch(text); matches != nil {
		// Parse value
		var value float64
		if value, err = strconv.ParseFloat(matches[1], 64); err != nil {
			err = fmt.Errorf("astisub: failed to parse value %s", matches[1])
			return
		}

		// Parse metric
		metric := matches[3]

		// Update duration
		if metric == "t" {
			d.ticks = int(value)
		} else if metric == "f" {
			d.frames = int(value)
		} else {
			// Get timebase
			var timebase time.Duration
			switch metric {
			case "h":
				timebase = time.Hour
			case "m":
				timebase = time.Minute
			case "s":
				timebase = time.Second
			case "ms":
				timebase = time.Millisecond
			default:
				err = fmt.Errorf("astisub: invalid metric %s", metric)
				return
			}

			// Update duration
			d.d = time.Duration(value * float64(timebase.Nanoseconds()))
		}
		return
	}

	// Extract clock time frames
	if indexes := ttmlRegexpClockTimeFrames.FindStringIndex(text); indexes != nil {
		// Parse frames
		var s = text[indexes[0]+1 : indexes[1]]
		if d.frames, err = strconv.Atoi(s); err != nil {
			err = fmt.Errorf("astisub: atoi %s failed: %w", s, err)
			return
		}

		// Update text
		text = text[:indexes[0]] + ".000"
	}

	d.d, err = parseDuration(text, ".", 3)
	return
}

// duration returns the input TTML Duration's time.Duration
func (d TTMLInDuration) duration() (o time.Duration) {
	if d.ticks > 0 && d.tickrate > 0 {
		return time.Duration(float64(d.ticks) * 1e9 / float64(d.tickrate))
	}
	o = d.d
	if d.frames > 0 && d.framerate > 0 {
		o += time.Duration(float64(d.frames) / float64(d.framerate) * float64(time.Second.Nanoseconds()))
	}
	return
}

// ReadFromTTML parses a .ttml content
func ReadFromTTML(i io.Reader) (o *Subtitles, err error) {
	// Init
	o = NewSubtitles()

	// Unmarshal XML
	var ttml TTMLIn
	if err = xml.NewDecoder(i).Decode(&ttml); err != nil {
		err = fmt.Errorf("astisub: xml decoding failed: %w", err)
		return
	}

	// Add metadata
	o.Metadata = ttml.metadata()

	// Loop through styles
	var parentStyles = make(map[string]*Style)
	for _, ts := range ttml.Styles {
		var s = &Style{
			ID:          ts.ID,
			InlineStyle: ts.TTMLInStyleAttributes.styleAttributes(),
		}
		o.Styles[s.ID] = s
		if len(ts.Style) > 0 {
			parentStyles[ts.Style] = s
		}
	}

	// Take care of parent styles
	for id, s := range parentStyles {
		if _, ok := o.Styles[id]; !ok {
			err = fmt.Errorf("astisub: Style %s requested by style %s doesn't exist", id, s.ID)
			return
		}
		s.Style = o.Styles[id]
	}

	// Loop through regions
	for _, tr := range ttml.Regions {
		var r = &Region{
			ID:          tr.ID,
			InlineStyle: tr.TTMLInStyleAttributes.styleAttributes(),
		}
		if len(tr.Style) > 0 {
			if _, ok := o.Styles[tr.Style]; !ok {
				err = fmt.Errorf("astisub: Style %s requested by region %s doesn't exist", tr.Style, r.ID)
				return
			}
			r.Style = o.Styles[tr.Style]
		}
		o.Regions[r.ID] = r
	}

	// Loop through subtitles
	bodyInlineStyle := ttml.Body.TTMLInStyleAttributes.styleAttributes()
	for _, div := range ttml.Body.Divs {
		divInlineStyle := div.TTMLInStyleAttributes.styleAttributes()

		// Propagate styles from Body -> Div
		divInlineStyle.merge(bodyInlineStyle)
		if div.Region == "" {
			div.Region = ttml.Body.Region
		}
		if div.Style == "" {
			div.Style = ttml.Body.Style
		}
		for _, ts := range div.Subtitles {
			// Init item
			ts.Begin.framerate = ttml.Framerate
			ts.Begin.tickrate = ttml.Tickrate
			ts.End.framerate = ttml.Framerate
			ts.End.tickrate = ttml.Tickrate

			itemInlineStyle := ts.TTMLInStyleAttributes.styleAttributes()

			// Propagate styles from Body -> Div -> Item.
			// If the item has its own Region, Style, or InlineStyle, it overrides the Div's.
			// This ensures all relevant styles are preserved at the item level,
			// maintaining compatibility with existing logic that relies on the Subtitles structure.
			itemInlineStyle.merge(divInlineStyle)
			if ts.Region == "" {
				ts.Region = div.Region
			}
			if ts.Style == "" {
				ts.Style = div.Style
			}

			var s = &Item{
				EndAt:       ts.End.duration(),
				InlineStyle: itemInlineStyle,
				StartAt:     ts.Begin.duration(),
			}

			// Add region
			if len(ts.Region) > 0 {
				if _, ok := o.Regions[ts.Region]; !ok {
					err = fmt.Errorf("astisub: Region %s requested by subtitle between %s and %s doesn't exist", ts.Region, s.StartAt, s.EndAt)
					return
				}
				s.Region = o.Regions[ts.Region]
			}

			// Add style
			if len(ts.Style) > 0 {
				if _, ok := o.Styles[ts.Style]; !ok {
					err = fmt.Errorf("astisub: Style %s requested by subtitle between %s and %s doesn't exist", ts.Style, s.StartAt, s.EndAt)
					return
				}
				s.Style = o.Styles[ts.Style]
			}

			// Remove items identation
			lines := strings.Split(ts.Items, "\n")
			for i := 0; i < len(lines); i++ {
				lines[i] = strings.TrimLeftFunc(lines[i], unicode.IsSpace)
			}

			// Unmarshal items
			var items = TTMLInItems{}
			if err = newTTMLXmlDecoder(strings.Join(lines, "")).Decode(&items); err != nil {
				err = fmt.Errorf("astisub: unmarshaling items failed: %w", err)
				return
			}

			// Loop through texts
			var l = &Line{}
			for _, tt := range items {
				// New line specified with the "br" tag
				if strings.ToLower(tt.XMLName.Local) == "br" {
					s.Lines = append(s.Lines, *l)
					l = &Line{}
					continue
				}

				// Ruby container: keep the ordered base/annotation pairs as a
				// single line item. A "br" cannot appear inside a ruby
				// container so it never introduces an extra line here.
				if tt.XMLName.Local == "ruby" {
					var lineItem = LineItem{
						InlineStyle: tt.TTMLInStyleAttributes.styleAttributes(),
						Ruby:        make([]TTMLRubyPair, 0, len(tt.Ruby)),
					}
					if len(tt.Style) > 0 {
						if _, ok := o.Styles[tt.Style]; !ok {
							err = fmt.Errorf("astisub: Style %s requested by ruby container doesn't exist", tt.Style)
							return
						}
						lineItem.Style = o.Styles[tt.Style]
					}
					for _, pair := range tt.Ruby {
						var p = TTMLRubyPair{}
						if pair.Base != nil {
							if p.Base, err = ttmlRubyComponentFromIn(pair.Base, o.Styles); err != nil {
								return
							}
						}
						if pair.Text != nil {
							if p.Text, err = ttmlRubyComponentFromIn(pair.Text, o.Styles); err != nil {
								return
							}
						}
						lineItem.Ruby = append(lineItem.Ruby, p)
					}
					l.Items = append(l.Items, lineItem)
					continue
				}

				// New line decoded as a line break. This can happen if there's a "br" tag within the text since
				// since the go xml unmarshaler will unmarshal a "br" tag as a line break if the field has the
				// chardata xml tag.
				for idx, li := range strings.Split(tt.Text, "\n") {
					// New line
					if idx > 0 {
						s.Lines = append(s.Lines, *l)
						l = &Line{}
					}

					// Init line item
					var t = LineItem{
						InlineStyle: tt.TTMLInStyleAttributes.styleAttributes(),
						Text:        li,
					}

					// Add style
					if len(tt.Style) > 0 {
						if _, ok := o.Styles[tt.Style]; !ok {
							err = fmt.Errorf("astisub: Style %s requested by item with text %s doesn't exist", tt.Style, tt.Text)
							return
						}
						t.Style = o.Styles[tt.Style]
					}

					// Append items
					l.Items = append(l.Items, t)
				}

			}
			s.Lines = append(s.Lines, *l)

			// Append subtitle
			o.Items = append(o.Items, s)
		}
	}
	return
}

// ttmlRubyComponentFromIn converts an input ruby base/annotation into the
// domain model, resolving its referenced style like regular span items do.
func ttmlRubyComponentFromIn(c *TTMLInRubyComponent, styles map[string]*Style) (*TTMLRubyComponent, error) {
	var out = &TTMLRubyComponent{
		InlineStyle: c.TTMLInStyleAttributes.styleAttributes(),
		Text:        c.Text,
	}
	if len(c.Style) > 0 {
		style, ok := styles[c.Style]
		if !ok {
			return nil, fmt.Errorf("astisub: Style %s requested by ruby component with text %s doesn't exist", c.Style, c.Text)
		}
		out.Style = style
	}
	return out, nil
}

// TTMLOut represents an output TTML that must be marshaled
// We split it from the input TTML as this time we'll add strict namespaces
type TTMLOut struct {
	Lang            string            `xml:"xml:lang,attr,omitempty"`
	Metadata        *TTMLOutMetadata  `xml:"head>metadata,omitempty"`
	Styles          []TTMLOutStyle    `xml:"head>styling>style,omitempty"` //!\\ Order is important! Keep Styling above Layout
	Regions         []TTMLOutRegion   `xml:"head>layout>region,omitempty"`
	Subtitles       []TTMLOutSubtitle `xml:"body>div>p,omitempty"`
	XMLName         xml.Name          `xml:"http://www.w3.org/ns/ttml tt"`
	XMLNamespaceTTM string            `xml:"xmlns:ttm,attr"`
	XMLNamespaceTTS string            `xml:"xmlns:tts,attr"`
}

// TTMLOutMetadata represents an output TTML Metadata
type TTMLOutMetadata struct {
	Copyright string `xml:"ttm:copyright,omitempty"`
	Title     string `xml:"ttm:title,omitempty"`
}

// TTMLOutStyleAttributes represents output TTML style attributes
type TTMLOutStyleAttributes struct {
	BackgroundColor *string `xml:"tts:backgroundColor,attr,omitempty"`
	Color           *string `xml:"tts:color,attr,omitempty"`
	Direction       *string `xml:"tts:direction,attr,omitempty"`
	Display         *string `xml:"tts:display,attr,omitempty"`
	DisplayAlign    *string `xml:"tts:displayAlign,attr,omitempty"`
	Extent          *string `xml:"tts:extent,attr,omitempty"`
	FontFamily      *string `xml:"tts:fontFamily,attr,omitempty"`
	FontSize        *string `xml:"tts:fontSize,attr,omitempty"`
	FontStyle       *string `xml:"tts:fontStyle,attr,omitempty"`
	FontWeight      *string `xml:"tts:fontWeight,attr,omitempty"`
	LineHeight      *string `xml:"tts:lineHeight,attr,omitempty"`
	Opacity         *string `xml:"tts:opacity,attr,omitempty"`
	Origin          *string `xml:"tts:origin,attr,omitempty"`
	Overflow        *string `xml:"tts:overflow,attr,omitempty"`
	Padding         *string `xml:"tts:padding,attr,omitempty"`
	ShowBackground  *string `xml:"tts:showBackground,attr,omitempty"`
	TextAlign       *string `xml:"tts:textAlign,attr,omitempty"`
	TextDecoration  *string `xml:"tts:textDecoration,attr,omitempty"`
	TextOutline     *string `xml:"tts:textOutline,attr,omitempty"`
	UnicodeBidi     *string `xml:"tts:unicodeBidi,attr,omitempty"`
	Visibility      *string `xml:"tts:visibility,attr,omitempty"`
	WrapOption      *string `xml:"tts:wrapOption,attr,omitempty"`
	WritingMode     *string `xml:"tts:writingMode,attr,omitempty"`
	ZIndex          *int    `xml:"tts:zIndex,attr,omitempty"`
}

// ttmlOutStyleAttributesFromStyleAttributes converts StyleAttributes into a TTMLOutStyleAttributes
func ttmlOutStyleAttributesFromStyleAttributes(s *StyleAttributes) TTMLOutStyleAttributes {
	if s == nil {
		return TTMLOutStyleAttributes{}
	}
	var color *string
	if s.TTMLColor != nil {
		// Try to convert to named color first, fall back to hex
		color = astikit.StrPtr(s.TTMLColor.HTMLString())
	}
	var backgroundColor *string
	if s.TTMLBackgroundColor != nil {
		backgroundColor = astikit.StrPtr(s.TTMLBackgroundColor.HTMLString())
	}
	return TTMLOutStyleAttributes{
		BackgroundColor: backgroundColor,
		Color:           color,
		Direction:       s.TTMLDirection,
		Display:         s.TTMLDisplay,
		DisplayAlign:    s.TTMLDisplayAlign,
		Extent:          s.TTMLExtent,
		FontFamily:      s.TTMLFontFamily,
		FontSize:        s.TTMLFontSize,
		FontStyle:       s.TTMLFontStyle,
		FontWeight:      s.TTMLFontWeight,
		LineHeight:      s.TTMLLineHeight,
		Opacity:         s.TTMLOpacity,
		Origin:          s.TTMLOrigin,
		Overflow:        s.TTMLOverflow,
		Padding:         s.TTMLPadding,
		ShowBackground:  s.TTMLShowBackground,
		TextAlign:       s.TTMLTextAlign,
		TextDecoration:  s.TTMLTextDecoration,
		TextOutline:     s.TTMLTextOutline,
		UnicodeBidi:     s.TTMLUnicodeBidi,
		Visibility:      s.TTMLVisibility,
		WrapOption:      s.TTMLWrapOption,
		WritingMode:     s.TTMLWritingMode,
		ZIndex:          s.TTMLZIndex,
	}
}

// ttmlOutRubyComponent converts a domain ruby component into its output form
func ttmlOutRubyComponent(c *TTMLRubyComponent) *TTMLOutRubyComponent {
	var out = &TTMLOutRubyComponent{
		Text:                   c.Text,
		TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(c.InlineStyle),
	}
	if c.Style != nil {
		out.Style = c.Style.ID
	}
	return out
}

// TTMLOutHeader represents an output TTML header
type TTMLOutHeader struct {
	ID    string `xml:"xml:id,attr,omitempty"`
	Style string `xml:"style,attr,omitempty"`
	TTMLOutStyleAttributes
}

// TTMLOutRegion represents an output TTML region
type TTMLOutRegion struct {
	TTMLOutHeader
	XMLName xml.Name `xml:"region"`
}

// TTMLOutStyle represents an output TTML style
type TTMLOutStyle struct {
	TTMLOutHeader
	XMLName xml.Name `xml:"style"`
}

// TTMLOutSubtitle represents an output TTML subtitle
type TTMLOutSubtitle struct {
	Begin  TTMLOutDuration `xml:"begin,attr"`
	End    TTMLOutDuration `xml:"end,attr"`
	ID     string          `xml:"id,attr,omitempty"`
	Items  []TTMLOutItem
	Region string `xml:"region,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	TTMLOutStyleAttributes
}

// TTMLOutRubyComponent represents an output ruby base ("rb") or annotation
// ("rt") element
type TTMLOutRubyComponent struct {
	Style string `xml:"style,attr,omitempty"`
	Text  string `xml:",chardata"`
	TTMLOutStyleAttributes
}

// MarshalXML emits the component under the element name selected by its parent
// pair ("rb" or "rt"). The start element provided by the encoder carries no
// attributes, so they are added here.
func (c TTMLOutRubyComponent) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if c.Style != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "style"}, Value: c.Style})
	}
	start.Attr = append(start.Attr, ttmlOutStyleAttributesTokens(c.TTMLOutStyleAttributes)...)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if c.Text != "" {
		if err := e.EncodeToken(xml.CharData([]byte(c.Text))); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// TTMLOutRubyPair associates an output ruby base with its annotation
type TTMLOutRubyPair struct {
	Base *TTMLOutRubyComponent `xml:"rb"`
	Text *TTMLOutRubyComponent `xml:"rt,omitempty"`
}

// TTMLOutItem represents an output TTML Item
type TTMLOutItem struct {
	Style string `xml:"style,attr,omitempty"`
	Text  string `xml:",chardata"`
	// Ruby holds the pairs of a ruby container. When set, the item marshals as
	// a "ruby" element and Text is ignored; otherwise it marshals as before.
	Ruby []TTMLOutRubyPair `xml:"-"`
	TTMLOutStyleAttributes
	XMLName xml.Name
}

// ttmlOutItemAlias is an alias without MarshalXML so plain (non-ruby) items
// keep using the default struct-tag encoding, preserving their exact output.
type ttmlOutItemAlias TTMLOutItem

// MarshalXML renders a plain item through the default encoder and a ruby
// container by emitting its ordered "rb"/"rt" children itself.
func (i TTMLOutItem) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = i.XMLName
	if len(i.Ruby) == 0 {
		return e.EncodeElement(ttmlOutItemAlias(i), start)
	}
	if i.Style != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "style"}, Value: i.Style})
	}
	start.Attr = append(start.Attr, ttmlOutStyleAttributesTokens(i.TTMLOutStyleAttributes)...)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, pair := range i.Ruby {
		if pair.Base != nil {
			if err := e.EncodeElement(*pair.Base, xml.StartElement{Name: xml.Name{Local: "rb"}}); err != nil {
				return err
			}
		}
		if pair.Text != nil {
			if err := e.EncodeElement(*pair.Text, xml.StartElement{Name: xml.Name{Local: "rt"}}); err != nil {
				return err
			}
		}
	}
	return e.EncodeToken(start.End())
}

// ttmlOutStyleAttributesTokens converts the supported output style attributes
// into encoder tokens. Only set attributes are emitted, matching the
// "omitempty" behavior of the struct-tag encoder.
func ttmlOutStyleAttributesTokens(s TTMLOutStyleAttributes) (attrs []xml.Attr) {
	add := func(local, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "tts:" + local}, Value: value})
		}
	}
	if s.BackgroundColor != nil {
		add("backgroundColor", *s.BackgroundColor)
	}
	if s.Color != nil {
		add("color", *s.Color)
	}
	if s.Direction != nil {
		add("direction", *s.Direction)
	}
	if s.Display != nil {
		add("display", *s.Display)
	}
	if s.DisplayAlign != nil {
		add("displayAlign", *s.DisplayAlign)
	}
	if s.Extent != nil {
		add("extent", *s.Extent)
	}
	if s.FontFamily != nil {
		add("fontFamily", *s.FontFamily)
	}
	if s.FontSize != nil {
		add("fontSize", *s.FontSize)
	}
	if s.FontStyle != nil {
		add("fontStyle", *s.FontStyle)
	}
	if s.FontWeight != nil {
		add("fontWeight", *s.FontWeight)
	}
	if s.LineHeight != nil {
		add("lineHeight", *s.LineHeight)
	}
	if s.Opacity != nil {
		add("opacity", *s.Opacity)
	}
	if s.Origin != nil {
		add("origin", *s.Origin)
	}
	if s.Overflow != nil {
		add("overflow", *s.Overflow)
	}
	if s.Padding != nil {
		add("padding", *s.Padding)
	}
	if s.ShowBackground != nil {
		add("showBackground", *s.ShowBackground)
	}
	if s.TextAlign != nil {
		add("textAlign", *s.TextAlign)
	}
	if s.TextDecoration != nil {
		add("textDecoration", *s.TextDecoration)
	}
	if s.TextOutline != nil {
		add("textOutline", *s.TextOutline)
	}
	if s.UnicodeBidi != nil {
		add("unicodeBidi", *s.UnicodeBidi)
	}
	if s.Visibility != nil {
		add("visibility", *s.Visibility)
	}
	if s.WrapOption != nil {
		add("wrapOption", *s.WrapOption)
	}
	if s.WritingMode != nil {
		add("writingMode", *s.WritingMode)
	}
	if s.ZIndex != nil {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "tts:zIndex"}, Value: strconv.Itoa(*s.ZIndex)})
	}
	return
}

// TTMLOutDuration represents an output TTML duration
type TTMLOutDuration time.Duration

// MarshalText implements the TextMarshaler interface
func (t TTMLOutDuration) MarshalText() ([]byte, error) {
	return []byte(formatDuration(time.Duration(t), ".", 3)), nil
}

// WriteToTTMLOptions represents TTML write options.
type WriteToTTMLOptions struct {
	Indent string // Default is 4 spaces.
}

// WriteToTTMLOption represents a WriteToTTML option.
type WriteToTTMLOption func(o *WriteToTTMLOptions)

// WriteToTTMLWithIndentOption sets the indent option.
func WriteToTTMLWithIndentOption(indent string) WriteToTTMLOption {
	return func(o *WriteToTTMLOptions) {
		o.Indent = indent
	}
}

// WriteToTTML writes subtitles in .ttml format
func (s Subtitles) WriteToTTML(o io.Writer, opts ...WriteToTTMLOption) (err error) {
	// Create write options
	wo := &WriteToTTMLOptions{Indent: "    "}
	for _, opt := range opts {
		opt(wo)
	}

	// Do not write anything if no subtitles
	if len(s.Items) == 0 {
		return ErrNoSubtitlesToWrite
	}

	// Init TTML
	var ttml = TTMLOut{
		XMLNamespaceTTM: "http://www.w3.org/ns/ttml#metadata",
		XMLNamespaceTTS: "http://www.w3.org/ns/ttml#styling",
	}

	// Add metadata
	if s.Metadata != nil {
		if v, ok := ttmlLanguageMapping.GetInverse(s.Metadata.Language); ok {
			ttml.Lang = v.(string)
		}
		if len(s.Metadata.TTMLCopyright) > 0 || len(s.Metadata.Title) > 0 {
			ttml.Metadata = &TTMLOutMetadata{
				Copyright: s.Metadata.TTMLCopyright,
				Title:     s.Metadata.Title,
			}
		}
	}

	// Add regions
	var k []string
	for _, region := range s.Regions {
		k = append(k, region.ID)
	}
	sort.Strings(k)
	for _, id := range k {
		var ttmlRegion = TTMLOutRegion{TTMLOutHeader: TTMLOutHeader{
			ID:                     s.Regions[id].ID,
			TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(s.Regions[id].InlineStyle),
		}}
		if s.Regions[id].Style != nil {
			ttmlRegion.Style = s.Regions[id].Style.ID
		}
		ttml.Regions = append(ttml.Regions, ttmlRegion)
	}

	// Add styles
	k = []string{}
	for _, style := range s.Styles {
		k = append(k, style.ID)
	}
	sort.Strings(k)
	for _, id := range k {
		var ttmlStyle = TTMLOutStyle{TTMLOutHeader: TTMLOutHeader{
			ID:                     s.Styles[id].ID,
			TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(s.Styles[id].InlineStyle),
		}}
		if s.Styles[id].Style != nil {
			ttmlStyle.Style = s.Styles[id].Style.ID
		}
		ttml.Styles = append(ttml.Styles, ttmlStyle)
	}

	// Add items
	for _, item := range s.Items {
		// Init subtitle
		var ttmlSubtitle = TTMLOutSubtitle{
			Begin:                  TTMLOutDuration(item.StartAt),
			End:                    TTMLOutDuration(item.EndAt),
			TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(item.InlineStyle),
		}

		// Add region
		if item.Region != nil {
			ttmlSubtitle.Region = item.Region.ID
		}

		// Add style
		if item.Style != nil {
			ttmlSubtitle.Style = item.Style.ID
		}

		// Add lines
		for _, line := range item.Lines {
			// Loop through line items
			for _, lineItem := range line.Items {
				// Ruby container: write its ordered base/annotation pairs
				if len(lineItem.Ruby) > 0 {
					var ttmlItem = TTMLOutItem{
						TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(lineItem.InlineStyle),
						XMLName:                xml.Name{Local: "ruby"},
						Ruby:                   make([]TTMLOutRubyPair, 0, len(lineItem.Ruby)),
					}

					// Add style
					if lineItem.Style != nil {
						ttmlItem.Style = lineItem.Style.ID
					}

					for _, pair := range lineItem.Ruby {
						var p TTMLOutRubyPair
						if pair.Base != nil {
							p.Base = ttmlOutRubyComponent(pair.Base)
						}
						if pair.Text != nil {
							p.Text = ttmlOutRubyComponent(pair.Text)
						}
						ttmlItem.Ruby = append(ttmlItem.Ruby, p)
					}

					// Add ttml item
					ttmlSubtitle.Items = append(ttmlSubtitle.Items, ttmlItem)
					continue
				}

				// Init ttml item
				var ttmlItem = TTMLOutItem{
					Text:                   lineItem.Text,
					TTMLOutStyleAttributes: ttmlOutStyleAttributesFromStyleAttributes(lineItem.InlineStyle),
					XMLName:                xml.Name{Local: "span"},
				}

				// Add style
				if lineItem.Style != nil {
					ttmlItem.Style = lineItem.Style.ID
				}

				// Add ttml item
				ttmlSubtitle.Items = append(ttmlSubtitle.Items, ttmlItem)
			}

			// Add line break
			ttmlSubtitle.Items = append(ttmlSubtitle.Items, TTMLOutItem{XMLName: xml.Name{Local: "br"}})
		}

		// Remove last line break
		if len(ttmlSubtitle.Items) > 0 {
			ttmlSubtitle.Items = ttmlSubtitle.Items[:len(ttmlSubtitle.Items)-1]
		}

		// Append subtitle
		ttml.Subtitles = append(ttml.Subtitles, ttmlSubtitle)
	}

	// Marshal XML
	var e = xml.NewEncoder(o)

	// Set indent
	e.Indent("", wo.Indent)

	if err = e.Encode(ttml); err != nil {
		err = fmt.Errorf("astisub: xml encoding failed: %w", err)
		return
	}
	return
}
