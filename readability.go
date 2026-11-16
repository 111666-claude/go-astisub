package astisub

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"
)

// Readability check errors
var (
	// ErrInvalidReadabilityThreshold is returned when one of the readability
	// check thresholds is zero or negative.
	ErrInvalidReadabilityThreshold = errors.New("astisub: readability thresholds must be strictly positive")
)

// ReadabilityRule identifies a readability rule that can produce a
// ReadabilityDiagnostic.
type ReadabilityRule string

// Readability rules
const (
	// ReadabilityRuleEmptyText reports an item whose visible text is empty.
	ReadabilityRuleEmptyText ReadabilityRule = "empty_text"
	// ReadabilityRuleNonPositiveDuration reports an item whose duration
	// (EndAt minus StartAt) is zero or negative.
	ReadabilityRuleNonPositiveDuration ReadabilityRule = "non_positive_duration"
	// ReadabilityRuleCrossLineStyle reports a multi-line item whose visible
	// styling changes between lines.
	ReadabilityRuleCrossLineStyle ReadabilityRule = "cross_line_style"
	// ReadabilityRuleCPS reports an item whose visible characters per second
	// exceed the configured maximum.
	ReadabilityRuleCPS ReadabilityRule = "cps"
	// ReadabilityRuleLineWidth reports a line whose display width exceeds the
	// configured maximum, full-width and half-width characters counted
	// separately.
	ReadabilityRuleLineWidth ReadabilityRule = "line_width"
	// ReadabilityRuleOverlap reports two items displayed at the same time for
	// longer than the configured tolerance.
	ReadabilityRuleOverlap ReadabilityRule = "overlap"
)

// Default readability thresholds
const (
	// defaultReadabilityMaxCPS is the commonly used safe reading speed of
	// 21 visible characters per second.
	defaultReadabilityMaxCPS = 21.
	// defaultReadabilityMaxLineWidth is 42 display columns (roughly the
	// 42-character line limit used by most subtitle guidelines).
	defaultReadabilityMaxLineWidth = 42
)

// ReadabilityOptions configures a readability check.
//
// Zero-valued numeric fields are replaced with sensible defaults so callers
// only need to set the limits they want to override.
type ReadabilityOptions struct {
	// MaxCPS is the maximum number of visible characters per second an item
	// may display. Defaults to 21.
	MaxCPS float64
	// MaxLineWidth is the maximum display width of a single line, where
	// full-width characters (e.g. CJK ideographs) count as 2 columns and
	// half-width characters (e.g. ASCII letters) count as 1. Defaults to 42.
	MaxLineWidth int
	// OverlapTolerance is the maximum amount of time two items may be
	// displayed at the same time without being reported. Defaults to 0.
	OverlapTolerance time.Duration
}

// ReadabilityDiagnostic is a single, locatable readability finding.
type ReadabilityDiagnostic struct {
	// Index is the zero-based position of the offending item in
	// Subtitles.Items.
	Index int
	// ItemIndex is the identifier stored on the item itself (Item.Index),
	// e.g. the 1-based cue number parsed from SRT or WebVTT; it is 0 when the
	// source format does not provide one.
	ItemIndex int
	// StartAt is the start time of the offending item.
	StartAt time.Duration
	// EndAt is the end time of the offending item.
	EndAt time.Duration
	// Rule is the rule that produced the diagnostic.
	Rule ReadabilityRule
	// LineIndex is the zero-based index of the offending line within the
	// item, and -1 when the rule applies to the whole item.
	LineIndex int
	// Measured is the measured value that violates the limit:
	// visible characters per second for CPS, display columns for line width,
	// overlap duration in seconds for overlaps, item duration in seconds for
	// non-positive durations and 0 for the remaining rules.
	Measured float64
	// Limit is the configured threshold the measured value violates:
	// MaxCPS for CPS, MaxLineWidth for line width, overlap tolerance in
	// seconds for overlaps and 0 for the remaining rules.
	Limit float64
	// OtherIndex is the zero-based position in Subtitles.Items of the other
	// item involved in an overlap, and -1 for every other rule.
	OtherIndex int
	// OtherItemIndex is the Item.Index of the other item involved in an
	// overlap, and -1 for every other rule.
	OtherItemIndex int
	// Message gives a human-readable description of the finding.
	Message string
}

// String implements the Stringer interface.
func (d ReadabilityDiagnostic) String() string {
	s := fmt.Sprintf("item %d (%s -> %s): %s: %s",
		d.Index, formatDuration(d.StartAt, ".", 3), formatDuration(d.EndAt, ".", 3), d.Rule, d.Message)
	if d.LineIndex >= 0 {
		s = fmt.Sprintf("item %d line %d (%s -> %s): %s: %s",
			d.Index, d.LineIndex, formatDuration(d.StartAt, ".", 3), formatDuration(d.EndAt, ".", 3), d.Rule, d.Message)
	}
	return s
}

// ReadabilityCheck runs the readability rules over the subtitles and returns
// the resulting diagnostics. It is strictly read-only: the subtitles, their
// items, lines, styles and regions are never modified.
//
// Zero-valued option fields fall back to defaults (MaxCPS = 21,
// MaxLineWidth = 42, OverlapTolerance = 0). Non-positive MaxCPS or
// MaxLineWidth, and a negative OverlapTolerance, make the options invalid and
// ErrInvalidReadabilityThreshold is returned.
//
// Every problem found on an item is reported: an item may produce several
// diagnostics (one per offending line, one per overlapping item, ...). The
// returned slice is always non-nil and deterministically ordered by item
// position, rule and line.
func (s *Subtitles) ReadabilityCheck(o ReadabilityOptions) (ds []ReadabilityDiagnostic, err error) {
	// Validate and default options
	if o.MaxCPS == 0 {
		o.MaxCPS = defaultReadabilityMaxCPS
	}
	if o.MaxLineWidth == 0 {
		o.MaxLineWidth = defaultReadabilityMaxLineWidth
	}
	if o.MaxCPS <= 0 || o.MaxLineWidth <= 0 || o.OverlapTolerance < 0 {
		err = ErrInvalidReadabilityThreshold
		return
	}

	// Make sure the returned slice is non-nil even when nothing is reported.
	ds = []ReadabilityDiagnostic{}

	// Per-item rules. Items are visited in their stored order so the output
	// is deterministic even when the items have not been ordered by time.
	for idx, item := range s.Items {
		if item == nil {
			continue
		}

		// Duration. A zero or negative duration is reported before any
		// rate-based rule so that no division by a non-positive duration can
		// happen.
		duration := item.EndAt - item.StartAt
		if duration <= 0 {
			ds = append(ds, ReadabilityDiagnostic{
				Index:          idx,
				ItemIndex:      item.Index,
				StartAt:        item.StartAt,
				EndAt:          item.EndAt,
				Rule:           ReadabilityRuleNonPositiveDuration,
				LineIndex:      -1,
				Measured:       duration.Seconds(),
				OtherIndex:     -1,
				OtherItemIndex: -1,
				Message:        fmt.Sprintf("duration is %s, expected a strictly positive duration", duration),
			})
		}

		// Empty text. Whitespace-only and zero-width-only items are treated
		// as empty since a viewer has nothing to read.
		text := itemText(item)
		runeCount := countVisibleRunes(text)
		blank := !hasReadableRune(text)
		if blank {
			ds = append(ds, ReadabilityDiagnostic{
				Index:          idx,
				ItemIndex:      item.Index,
				StartAt:        item.StartAt,
				EndAt:          item.EndAt,
				Rule:           ReadabilityRuleEmptyText,
				LineIndex:      -1,
				OtherIndex:     -1,
				OtherItemIndex: -1,
				Message:        "item has no visible text",
			})
		}

		// CPS. Only meaningful with readable text and a positive duration;
		// skipping the other cases avoids a division by zero.
		if !blank && duration > 0 {
			cps := float64(runeCount) / duration.Seconds()
			if cps > o.MaxCPS {
				ds = append(ds, ReadabilityDiagnostic{
					Index:          idx,
					ItemIndex:      item.Index,
					StartAt:        item.StartAt,
					EndAt:          item.EndAt,
					Rule:           ReadabilityRuleCPS,
					LineIndex:      -1,
					Measured:       cps,
					Limit:          o.MaxCPS,
					OtherIndex:     -1,
					OtherItemIndex: -1,
					Message:        fmt.Sprintf("%.2f visible characters per second exceeds the maximum of %.2f (%d characters in %s)", cps, o.MaxCPS, runeCount, duration),
				})
			}
		}

		// Line width. Each line that is too wide is reported individually.
		for lineIdx, line := range item.Lines {
			w := displayWidth(line.String())
			if w > o.MaxLineWidth {
				ds = append(ds, ReadabilityDiagnostic{
					Index:          idx,
					ItemIndex:      item.Index,
					StartAt:        item.StartAt,
					EndAt:          item.EndAt,
					Rule:           ReadabilityRuleLineWidth,
					LineIndex:      lineIdx,
					Measured:       float64(w),
					Limit:          float64(o.MaxLineWidth),
					OtherIndex:     -1,
					OtherItemIndex: -1,
					Message:        fmt.Sprintf("line is %d display columns wide, maximum is %d", w, o.MaxLineWidth),
				})
			}
		}

		// Cross-line style.
		if d := itemCrossLineStyleDiagnostic(idx, item); d != nil {
			ds = append(ds, *d)
		}
	}

	// Overlaps. Every pair is examined so no conflict is missed, including
	// items that share a start or an end time and items stored out of order.
	// Each conflicting pair is reported once, anchored on the earlier item.
	for i := 0; i < len(s.Items); i++ {
		for j := i + 1; j < len(s.Items); j++ {
			if s.Items[i] == nil || s.Items[j] == nil {
				continue
			}
			overlap := overlapDuration(s.Items[i], s.Items[j])
			if overlap <= o.OverlapTolerance {
				continue
			}
			first, second := i, j
			if s.Items[j].StartAt < s.Items[i].StartAt {
				first, second = j, i
			}
			ds = append(ds, ReadabilityDiagnostic{
				Index:          first,
				ItemIndex:      s.Items[first].Index,
				StartAt:        s.Items[first].StartAt,
				EndAt:          s.Items[first].EndAt,
				Rule:           ReadabilityRuleOverlap,
				LineIndex:      -1,
				Measured:       overlap.Seconds(),
				Limit:          o.OverlapTolerance.Seconds(),
				OtherIndex:     second,
				OtherItemIndex: s.Items[second].Index,
				Message:        fmt.Sprintf("overlaps item %d for %s (tolerance %s)", second, overlap, o.OverlapTolerance),
			})
		}
	}

	// Stable ordering: item position, then rule, then line, then the other
	// item involved in an overlap. Equal diagnostics keep their input order.
	sort.SliceStable(ds, func(i, j int) bool {
		if ds[i].Index != ds[j].Index {
			return ds[i].Index < ds[j].Index
		}
		if ri, rj := readabilityRuleOrder(ds[i].Rule), readabilityRuleOrder(ds[j].Rule); ri != rj {
			return ri < rj
		}
		if ds[i].LineIndex != ds[j].LineIndex {
			return ds[i].LineIndex < ds[j].LineIndex
		}
		return ds[i].OtherIndex < ds[j].OtherIndex
	})
	return
}

// readabilityRuleOrder gives each rule a fixed position in the per-item
// diagnostic order.
func readabilityRuleOrder(r ReadabilityRule) int {
	switch r {
	case ReadabilityRuleEmptyText:
		return 0
	case ReadabilityRuleNonPositiveDuration:
		return 1
	case ReadabilityRuleCPS:
		return 2
	case ReadabilityRuleLineWidth:
		return 3
	case ReadabilityRuleCrossLineStyle:
		return 4
	case ReadabilityRuleOverlap:
		return 5
	default:
		return 6
	}
}

// itemText concatenates the visible text of every line of an item, joining
// lines with a line break. Markup is not part of the text at this stage:
// SRT/WebVTT/SSA tags are parsed into StyleAttributes by the readers and HTML
// entities are already decoded, so LineItem.Text only contains visible text.
func itemText(item *Item) string {
	var b strings.Builder
	for lineIdx, line := range item.Lines {
		if lineIdx > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line.String())
	}
	return b.String()
}

// countVisibleRunes counts the runes a viewer actually reads: control
// characters and combining/zero-width marks do not count, but spaces do.
func countVisibleRunes(s string) int {
	n := 0
	for _, r := range s {
		if isZeroWidthRune(r) {
			continue
		}
		n++
	}
	return n
}

// hasReadableRune reports whether s contains at least one rune that is
// neither zero-width nor whitespace. It is used to detect blank text:
// a line made only of spaces, tabs and zero-width marks has nothing to read.
func hasReadableRune(s string) bool {
	for _, r := range s {
		if isZeroWidthRune(r) || unicode.IsSpace(r) {
			continue
		}
		return true
	}
	return false
}

// displayWidth returns the number of display columns occupied by s, treating
// full-width characters (East Asian Wide and Fullwidth, e.g. CJK ideographs
// and full-width punctuation) as 2 columns, half-width characters as 1 and
// combining/zero-width marks as 0. Tabs and other control characters count
// as a single column.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeDisplayWidth(r)
	}
	return w
}

// runeDisplayWidth returns the display width of a single rune.
func runeDisplayWidth(r rune) int {
	if isZeroWidthRune(r) {
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

// isZeroWidthRune reports whether r takes no horizontal space when rendered:
// format/control characters (including tabs and line breaks), combining
// marks and the soft hyphen.
func isZeroWidthRune(r rune) bool {
	if r == utf8.RuneError {
		return false
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cc, r) {
		return true
	}
	// Soft hyphen (U+00AD) is rendered only at line breaks and otherwise
	// takes no space; some Unicode versions classify it outside Cf.
	return r == '\u00ad'
}

// overlapDuration returns how long two items are displayed at the same time.
// It is 0 when the items only touch at a boundary or do not overlap.
func overlapDuration(a, b *Item) time.Duration {
	start := a.StartAt
	if b.StartAt > start {
		start = b.StartAt
	}
	end := a.EndAt
	if b.EndAt < end {
		end = b.EndAt
	}
	if end <= start {
		return 0
	}
	return end - start
}

// lineStyleSignature is a value-shaped summary of the styling applied to the
// visible text of a line, so two signatures can be compared with ==.
type lineStyleSignature struct {
	// bold, italics and underline cover the SRT, WebVTT, SSA and TTML
	// boolean text decorations.
	bold      bool
	italics   bool
	underline bool
	// color is the rendered text color, expressed with its lossless HTML
	// form (empty when no color is set).
	color string
	// fontName and fontSize capture the SSA/TTML typography.
	fontName string
	fontSize float64
	// namedStyles holds the IDs of the named styles referenced by the line
	// items, in reference order.
	namedStyles string
}

// itemCrossLineStyleDiagnostic checks whether the visible styling changes
// between the lines of a multi-line item. It returns nil when no change is
// detected, including for single-line items.
func itemCrossLineStyleDiagnostic(idx int, item *Item) *ReadabilityDiagnostic {
	if len(item.Lines) < 2 {
		return nil
	}

	var signatures []lineStyleSignature
	for _, line := range item.Lines {
		// Blank lines carry no visible styling and cannot be compared.
		if !hasReadableRune(line.String()) {
			continue
		}
		signatures = append(signatures, lineStyle(line))
	}
	if len(signatures) < 2 {
		return nil
	}

	for i := 1; i < len(signatures); i++ {
		if signatures[i] != signatures[0] {
			return &ReadabilityDiagnostic{
				Index:          idx,
				ItemIndex:      item.Index,
				StartAt:        item.StartAt,
				EndAt:          item.EndAt,
				Rule:           ReadabilityRuleCrossLineStyle,
				LineIndex:      -1,
				OtherIndex:     -1,
				OtherItemIndex: -1,
				Message:        "styling is not consistent across the lines of the item",
			}
		}
	}
	return nil
}

// lineStyle builds the style signature of a line from its line items.
func lineStyle(line Line) lineStyleSignature {
	var sig lineStyleSignature
	var named []string
	for _, li := range line.Items {
		if li.Style != nil && li.Style.ID != "" {
			named = append(named, li.Style.ID)
		}
		sa := li.InlineStyle
		if sa == nil {
			continue
		}
		// WebVTT keeps its inline markup in a tag stack rather than in the
		// boolean decoration fields.
		for _, tag := range sa.WebVTTTags {
			switch tag.Name {
			case "b":
				sig.bold = true
			case "i":
				sig.italics = true
			case "u":
				sig.underline = true
			}
		}
		if sa.SRTBold || sa.WebVTTBold {
			sig.bold = true
		}
		if sa.SRTItalics || sa.WebVTTItalics || sa.SSAItalic != nil && *sa.SSAItalic {
			sig.italics = true
		}
		if sa.SRTUnderline || sa.WebVTTUnderline || sa.SSAUnderline != nil && *sa.SSAUnderline {
			sig.underline = true
		}
		if sa.SRTColor != nil {
			sig.color = sa.SRTColor.HTMLString()
		} else if sa.TTMLColor != nil {
			sig.color = sa.TTMLColor.HTMLString()
		} else if sa.SSAPrimaryColour != nil {
			sig.color = sa.SSAPrimaryColour.HTMLString()
		}
		if sa.SSAFontName != "" {
			sig.fontName = sa.SSAFontName
		}
		if sa.SSAFontSize != nil {
			sig.fontSize = *sa.SSAFontSize
		}
	}
	sig.namedStyles = strings.Join(named, ",")
	return sig
}
