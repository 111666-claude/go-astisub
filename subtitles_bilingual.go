package astisub

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"
)

// Bilingual merge errors
var (
	// ErrBilingualMergeInvalidTolerance is returned when the configured tolerance
	// is negative.
	ErrBilingualMergeInvalidTolerance = errors.New("astisub: bilingual merge tolerance must be greater than or equal to 0")
	// ErrBilingualMergeInvalidLineOrder is returned when the configured line
	// order is not one of the BilingualLineOrder constants.
	ErrBilingualMergeInvalidLineOrder = errors.New("astisub: invalid bilingual merge line order")
	// ErrBilingualMergeNilSubtitles is returned when one of the two subtitles
	// tracks is nil.
	ErrBilingualMergeNilSubtitles = errors.New("astisub: bilingual merge requires two non-nil subtitles")
	// ErrBilingualMergeInvalidTiming is returned when an item has missing or
	// invalid time boundaries (negative start, or end not strictly after start).
	ErrBilingualMergeInvalidTiming = errors.New("astisub: bilingual merge item has invalid timing")
	// ErrBilingualMergeStyleConflict is returned when the two tracks define the
	// same style ID with different definitions: merging them would silently
	// re-style one of the tracks.
	ErrBilingualMergeStyleConflict = errors.New("astisub: bilingual merge cannot merge styles sharing the same ID but having different definitions")
	// ErrBilingualMergeRegionConflict is returned when the two tracks define the
	// same region ID with different definitions.
	ErrBilingualMergeRegionConflict = errors.New("astisub: bilingual merge cannot merge regions sharing the same ID but having different definitions")
)

// BilingualLineOrder represents the order in which the lines of the two tracks
// are written inside a merged item.
type BilingualLineOrder int

const (
	// BilingualLineOrderPrimaryFirst writes the lines of the subtitles the
	// method is called on first. This is the zero value.
	BilingualLineOrderPrimaryFirst BilingualLineOrder = iota
	// BilingualLineOrderSecondaryFirst writes the lines of the subtitles passed
	// as argument first.
	BilingualLineOrderSecondaryFirst
)

// BilingualMergeOptions represents the options of a bilingual merge.
type BilingualMergeOptions struct {
	// Tolerance is the maximum gap between two time ranges for their items to
	// be paired together. Items that overlap, touch (end of one equals start of
	// the other) or are separated by a gap lower than or equal to Tolerance can
	// be paired. It must be greater than or equal to 0.
	Tolerance time.Duration
	// LineOrder defines which track's lines come first in a merged item.
	LineOrder BilingualLineOrder
}

// bilingualMergeTrack identifies which track a tagged item belongs to.
type bilingualMergeTrack int

const (
	bilingualMergeTrackPrimary bilingualMergeTrack = iota
	bilingualMergeTrackSecondary
)

// bilingualMergeTaggedItem associates an item with its source track and its
// position in that track.
type bilingualMergeTaggedItem struct {
	track    bilingualMergeTrack
	position int
	item     *Item
}

// MergeBilingual pairs the items of two subtitle tracks (e.g. a Chinese and an
// English track whose timings are slightly offset) and builds a new single
// track in which paired items are combined into one bilingual item instead of
// being appended next to each other.
//
// Pairing rules:
//   - Items are paired when their time ranges overlap, touch (an item ends
//     exactly when the other starts) or are separated by a gap lower than or
//     equal to o.Tolerance.
//   - Pairing is transitive: if item A pairs with B and B pairs with C, A, B
//     and C end up in the same group, which gives a deterministic result for
//     one-to-many cases (one line translated as several lines) and for
//     overlapping subtitles within a single track.
//   - A group containing items from both tracks produces a single merged item.
//     Its lines keep their original per-line styling and are ordered track by
//     track according to o.LineOrder. Its time boundaries are the union of the
//     group's ranges (earliest start, latest end), which is always a valid,
//     non-empty range.
//   - A group containing items of a single track means those items found no
//     match: every item is preserved untouched (deep-copied, with its original
//     timing), nothing is silently dropped.
//
// The returned subtitles are independent: neither of the two input subtitles,
// nor their items, is modified. The output is sorted by start time and its
// items are re-indexed. An error is returned (without returning subtitles) on
// an invalid tolerance or line order, a nil input, an item with missing or
// invalid timing, or a style/region ID defined differently in both tracks.
func (s *Subtitles) MergeBilingual(secondary *Subtitles, o BilingualMergeOptions) (*Subtitles, error) {
	// Validate options
	if o.Tolerance < 0 {
		return nil, ErrBilingualMergeInvalidTolerance
	}
	if o.LineOrder != BilingualLineOrderPrimaryFirst && o.LineOrder != BilingualLineOrderSecondaryFirst {
		return nil, ErrBilingualMergeInvalidLineOrder
	}
	if s == nil || secondary == nil {
		return nil, ErrBilingualMergeNilSubtitles
	}

	// Validate timing of every item up front so that no subtitle is silently
	// dropped or paired on a zero-length range.
	if err := validateBilingualMergeTiming(bilingualMergeTrackPrimary, s.Items); err != nil {
		return nil, err
	}
	if err := validateBilingualMergeTiming(bilingualMergeTrackSecondary, secondary.Items); err != nil {
		return nil, err
	}

	// Merge styles and regions, rejecting same-ID/different-definition conflicts
	styles, err := mergeBilingualMergeStyles(s.Styles, secondary.Styles)
	if err != nil {
		return nil, err
	}
	regions, err := mergeBilingualMergeRegions(s.Regions, secondary.Regions)
	if err != nil {
		return nil, err
	}

	// Tag and sort all items by start time. Ties are broken by the primary
	// track first, then by source position, so the result is deterministic
	// regardless of the input ordering.
	tagged := make([]bilingualMergeTaggedItem, 0, len(s.Items)+len(secondary.Items))
	for idx, item := range s.Items {
		tagged = append(tagged, bilingualMergeTaggedItem{track: bilingualMergeTrackPrimary, position: idx, item: item})
	}
	for idx, item := range secondary.Items {
		tagged = append(tagged, bilingualMergeTaggedItem{track: bilingualMergeTrackSecondary, position: idx, item: item})
	}
	sort.SliceStable(tagged, func(i, j int) bool {
		if tagged[i].item.StartAt != tagged[j].item.StartAt {
			return tagged[i].item.StartAt < tagged[j].item.StartAt
		}
		if tagged[i].track != tagged[j].track {
			return tagged[i].track < tagged[j].track
		}
		return tagged[i].position < tagged[j].position
	})

	// Build connected components of pairable items using a union-find. Two
	// items pair when the second one starts before or at the first one's end
	// plus the tolerance.
	uf := newBilingualMergeUnionFind(len(tagged))
	for i := 0; i < len(tagged); i++ {
		for j := i + 1; j < len(tagged); j++ {
			// Items are sorted by start time: once j is out of reach, every
			// later item is too.
			if tagged[j].item.StartAt > tagged[i].item.EndAt+o.Tolerance {
				break
			}
			uf.union(i, j)
		}
	}

	// Emit one output item (or several) per component, keeping component order
	groups := make(map[int][]bilingualMergeTaggedItem)
	var roots []int
	for idx, t := range tagged {
		root := uf.find(idx)
		if _, ok := groups[root]; !ok {
			roots = append(roots, root)
		}
		groups[root] = append(groups[root], t)
	}

	out := NewSubtitles()
	out.Styles = styles
	out.Regions = regions
	if s.Metadata != nil {
		metadata := *s.Metadata
		out.Metadata = &metadata
	}
	for _, root := range roots {
		group := groups[root]
		hasPrimary, hasSecondary := false, false
		for _, t := range group {
			switch t.track {
			case bilingualMergeTrackPrimary:
				hasPrimary = true
			case bilingualMergeTrackSecondary:
				hasSecondary = true
			}
		}

		// Items from a single track found no match: preserve all of them,
		// untouched and independently copied.
		if !hasPrimary || !hasSecondary {
			for _, t := range group {
				out.Items = append(out.Items, copyBilingualMergeItem(t.item))
			}
			continue
		}

		// Both tracks are present: build one merged item from the whole group.
		out.Items = append(out.Items, buildBilingualMergeItem(group, o.LineOrder))
	}

	// Guarantee the ordering and re-index the output
	sort.SliceStable(out.Items, func(i, j int) bool {
		return out.Items[i].StartAt < out.Items[j].StartAt
	})
	for idx, item := range out.Items {
		item.Index = idx + 1
	}
	return out, nil
}

// validateBilingualMergeTiming makes sure every item carries a usable,
// non-empty time range.
func validateBilingualMergeTiming(track bilingualMergeTrack, items []*Item) error {
	for idx, item := range items {
		if item == nil || item.hasInvalidTiming() {
			var start, end time.Duration
			if item != nil {
				start, end = item.StartAt, item.EndAt
			}
			return fmt.Errorf("%w: %s track item #%d has start %s and end %s",
				ErrBilingualMergeInvalidTiming,
				bilingualMergeTrackName(track),
				idx,
				start,
				end)
		}
	}
	return nil
}

func (i *Item) hasInvalidTiming() bool {
	return i.StartAt < 0 || i.EndAt <= i.StartAt
}

func bilingualMergeTrackName(track bilingualMergeTrack) string {
	if track == bilingualMergeTrackPrimary {
		return "primary"
	}
	return "secondary"
}

// mergeBilingualMergeStyles merges the two style maps, keeping the primary
// definition on identical IDs and failing on incompatible definitions.
func mergeBilingualMergeStyles(primary, secondary map[string]*Style) (map[string]*Style, error) {
	out := make(map[string]*Style, len(primary)+len(secondary))
	for id, style := range primary {
		out[id] = style
	}
	for id, style := range secondary {
		if existing, ok := out[id]; ok {
			if !reflect.DeepEqual(existing, style) {
				return nil, fmt.Errorf("%w: style %q", ErrBilingualMergeStyleConflict, id)
			}
			continue
		}
		out[id] = style
	}
	return out, nil
}

// mergeBilingualMergeRegions merges the two region maps, keeping the primary
// definition on identical IDs and failing on incompatible definitions.
func mergeBilingualMergeRegions(primary, secondary map[string]*Region) (map[string]*Region, error) {
	out := make(map[string]*Region, len(primary)+len(secondary))
	for id, region := range primary {
		out[id] = region
	}
	for id, region := range secondary {
		if existing, ok := out[id]; ok {
			if !reflect.DeepEqual(existing, region) {
				return nil, fmt.Errorf("%w: region %q", ErrBilingualMergeRegionConflict, id)
			}
			continue
		}
		out[id] = region
	}
	return out, nil
}

// buildBilingualMergeItem combines a group containing items from both tracks
// into a single item. Lines are deep-copied so the input items stay untouched;
// their per-line styling travels with them.
func buildBilingualMergeItem(group []bilingualMergeTaggedItem, order BilingualLineOrder) *Item {
	merged := &Item{}

	// Time boundaries are the union of the group's ranges. Every input item is
	// a valid non-empty range, so the union is valid as well.
	merged.StartAt = group[0].item.StartAt
	merged.EndAt = group[0].item.EndAt
	for _, t := range group {
		if t.item.StartAt < merged.StartAt {
			merged.StartAt = t.item.StartAt
		}
		if t.item.EndAt > merged.EndAt {
			merged.EndAt = t.item.EndAt
		}
	}

	// Cue-level styling comes from the first primary item, deterministically.
	// Line-level styling of every track is preserved on the copied lines.
	firstPrimary := group[0].item
	for _, t := range group {
		if t.track == bilingualMergeTrackPrimary {
			firstPrimary = t.item
			break
		}
	}
	merged.Region = firstPrimary.Region
	merged.Style = firstPrimary.Style
	merged.InlineStyle = copyBilingualMergeStyleAttributes(firstPrimary.InlineStyle)

	// Append the lines in the configured track order. Within a track the
	// source item order is kept.
	appendTrack := func(track bilingualMergeTrack) {
		for _, t := range group {
			if t.track != track {
				continue
			}
			for _, line := range t.item.Lines {
				merged.Lines = append(merged.Lines, copyBilingualMergeLine(line))
			}
		}
	}
	if order == BilingualLineOrderSecondaryFirst {
		appendTrack(bilingualMergeTrackSecondary)
		appendTrack(bilingualMergeTrackPrimary)
	} else {
		appendTrack(bilingualMergeTrackPrimary)
		appendTrack(bilingualMergeTrackSecondary)
	}
	return merged
}

// copyBilingualMergeItem deep-copies an item. The copy owns its comments,
// inline style and lines; style and region pointers are shared read-only
// (nothing in the merge mutates them) and remain valid for writing.
func copyBilingualMergeItem(i *Item) *Item {
	c := &Item{}
	*c = *i
	c.Comments = append([]string(nil), i.Comments...)
	c.InlineStyle = copyBilingualMergeStyleAttributes(i.InlineStyle)
	c.Lines = make([]Line, len(i.Lines))
	for idx, line := range i.Lines {
		c.Lines[idx] = copyBilingualMergeLine(line)
	}
	return c
}

// copyBilingualMergeStyleAttributes returns an independent copy of the style
// attributes, or nil when the input is nil.
func copyBilingualMergeStyleAttributes(sa *StyleAttributes) *StyleAttributes {
	if sa == nil {
		return nil
	}
	c := *sa
	return &c
}

// copyBilingualMergeLine deep-copies a line including its line items.
func copyBilingualMergeLine(l Line) Line {
	c := Line{VoiceName: l.VoiceName}
	c.Items = make([]LineItem, len(l.Items))
	for idx, li := range l.Items {
		c.Items[idx] = li
		// InlineStyle is a pointer: it has to be copied so that restyling the
		// output cannot mutate the input subtitles.
		c.Items[idx].InlineStyle = copyBilingualMergeStyleAttributes(li.InlineStyle)
	}
	return c
}

// bilingualMergeUnionFind is a small union-find used to build the connected
// components of pairable items.
type bilingualMergeUnionFind struct {
	parents []int
	ranks   []int
}

func newBilingualMergeUnionFind(n int) *bilingualMergeUnionFind {
	uf := &bilingualMergeUnionFind{parents: make([]int, n), ranks: make([]int, n)}
	for idx := range uf.parents {
		uf.parents[idx] = idx
	}
	return uf
}

func (uf *bilingualMergeUnionFind) find(i int) int {
	for uf.parents[i] != i {
		uf.parents[i] = uf.parents[uf.parents[i]]
		i = uf.parents[i]
	}
	return i
}

func (uf *bilingualMergeUnionFind) union(i, j int) {
	ri, rj := uf.find(i), uf.find(j)
	if ri == rj {
		return
	}
	switch {
	case uf.ranks[ri] < uf.ranks[rj]:
		uf.parents[ri] = rj
	case uf.ranks[ri] > uf.ranks[rj]:
		uf.parents[rj] = ri
	default:
		uf.parents[rj] = ri
		uf.ranks[ri]++
	}
}
