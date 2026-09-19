package gofeed

import (
	"strings"
	"time"

	"github.com/mmcdole/gofeed/atom"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/mmcdole/gofeed/internal/shared"
	"github.com/mmcdole/gofeed/json"
	"github.com/mmcdole/gofeed/rss"
)

// This file holds the normalization rules that are genuinely identical
// across the RSS, Atom and JSON translators. Every helper is a pure function:
// it reads only its arguments, never touches global state, and never mutates
// the source format structs. Rules that differ per format on purpose (see
// REFACTORING.md) stay in translator.go.

// atomExtensionKeys are the extension namespaces that embed Atom elements in
// RSS feeds. Immutable; read-only by convention.
var atomExtensionKeys = []string{"atom", "atom10", "atom03"}

// firstNonEmpty returns the first candidate that is not "", or "" when all
// candidates are empty.
func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

// firstString returns the first entry of a string slice, or "" when empty.
func firstString(entries []string) string {
	if len(entries) == 0 {
		return ""
	}
	return entries[0]
}

// parseDateOrNil parses a date string, returning nil when the text is empty
// or unparseable. Callers always keep the raw text separately.
func parseDateOrNil(text string) *time.Time {
	if date, err := shared.ParseDate(text); err == nil {
		return &date
	}
	return nil
}

// isWebLinkRel reports whether an Atom link relation points at the feed's own
// web presence: an empty rel (which defaults to alternate), "alternate" (the
// site) or "self" (the feed).
func isWebLinkRel(rel string) bool {
	return rel == "" || rel == "alternate" || rel == "self"
}

// webLinkHrefs returns the hrefs of the links whose rel denotes a web link,
// in document order. Returns nil when there are none.
func webLinkHrefs(links []*atom.Link) []string {
	var hrefs []string
	for _, l := range links {
		if isWebLinkRel(l.Rel) {
			hrefs = append(hrefs, l.Href)
		}
	}
	return hrefs
}

// appendNonEmpty appends each non-empty value to list. Appending nothing to a
// nil list keeps it nil.
func appendNonEmpty(list []string, values ...string) []string {
	for _, value := range values {
		if value != "" {
			list = append(list, value)
		}
	}
	return list
}

// firstImageURL returns an Image for the first non-empty URL candidate, or
// nil when all candidates are empty.
func firstImageURL(candidates ...string) *Image {
	if url := firstNonEmpty(candidates...); url != "" {
		return &Image{URL: url}
	}
	return nil
}

// newEnclosure builds a universal Enclosure from the three fields every
// format maps.
func newEnclosure(url, length, enclosureType string) *Enclosure {
	return &Enclosure{URL: url, Length: length, Type: enclosureType}
}

// personFromText builds a Person from a free-form author string like
// "Example Name (example@site.com)".
func personFromText(text string) *Person {
	name, address := shared.ParseNameAddress(text)
	return &Person{Name: name, Email: address}
}

// singleAuthorList wraps a primary author as the Authors list, preserving
// nil when there is no author.
func singleAuthorList(author *Person) []*Person {
	if author == nil {
		return nil
	}
	return []*Person{author}
}

// rssPersonAuthor picks an author from the prioritized RSS sources shared by
// the feed and item translators: a plain author string (managingEditor /
// webMaster for feeds, author for items), then dc:author, dc:creator, then
// itunes:author. A present-but-empty dc element still wins, matching the
// historical trigger on slice presence rather than content.
func rssPersonAuthor(plain string, dc *ext.DublinCoreExtension, itunesAuthor string) *Person {
	switch {
	case plain != "":
		return personFromText(plain)
	case dc != nil && dc.Author != nil:
		return personFromText(firstString(dc.Author))
	case dc != nil && dc.Creator != nil:
		return personFromText(firstString(dc.Creator))
	case itunesAuthor != "":
		return personFromText(itunesAuthor)
	}
	return nil
}

// atomAuthors converts atom persons to the universal Author/Authors pair: the
// primary author is the first entry. Both are nil when there are no authors.
func atomAuthors(persons []*atom.Person) (primary *Person, all []*Person) {
	if len(persons) == 0 {
		return nil, nil
	}
	all = atomPersons(persons)
	return all[0], all
}

// translateJSONAuthors maps the JSON Feed author/authors pair to the
// universal Author/Authors: the legacy single author fills Authors only when
// the authors array is absent.
func translateJSONAuthors(author *json.Author, authors []*json.Author) (primary *Person, all []*Person) {
	if author != nil {
		primary = jsonPerson(author)
	}
	if authors != nil {
		all = jsonPersons(authors)
	} else {
		all = singleAuthorList(primary)
	}
	return
}

// dcFallbacks flattens the dc: fields the translators fall back to into plain
// strings (first value each), so fallback chains don't repeat nil checks.
type dcFallbacks struct {
	Title       string
	Language    string
	Rights      string
	Date        string
	Description string
}

// dcFallbackValues extracts the fallback values from a Dublin Core extension.
// A nil extension yields all empty strings.
func dcFallbackValues(dc *ext.DublinCoreExtension) dcFallbacks {
	if dc == nil {
		return dcFallbacks{}
	}
	return dcFallbacks{
		Title:       firstString(dc.Title),
		Language:    firstString(dc.Language),
		Rights:      firstString(dc.Rights),
		Date:        firstString(dc.Date),
		Description: firstString(dc.Description),
	}
}

// itunesFeedSummary, itunesFeedAuthor and itunesFeedKeywords read the shared
// itunes fallback fields from a feed extension, tolerating a nil extension.
func itunesFeedSummary(e *ext.ITunesFeedExtension) string {
	if e == nil {
		return ""
	}
	return e.Summary
}

func itunesFeedAuthor(e *ext.ITunesFeedExtension) string {
	if e == nil {
		return ""
	}
	return e.Author
}

func itunesFeedKeywords(e *ext.ITunesFeedExtension) string {
	if e == nil {
		return ""
	}
	return e.Keywords
}

// itunesItemSummary, itunesItemAuthor and itunesItemKeywords are the item
// extension counterparts of the feed accessors above.
func itunesItemSummary(e *ext.ITunesItemExtension) string {
	if e == nil {
		return ""
	}
	return e.Summary
}

func itunesItemAuthor(e *ext.ITunesItemExtension) string {
	if e == nil {
		return ""
	}
	return e.Author
}

func itunesItemKeywords(e *ext.ITunesItemExtension) string {
	if e == nil {
		return ""
	}
	return e.Keywords
}

// rssCategoryValues flattens RSS categories to their text values. Returns nil
// for an empty list so callers can distinguish "no categories" from "empty".
func rssCategoryValues(categories []*rss.Category) []string {
	if len(categories) == 0 {
		return nil
	}
	values := make([]string, 0, len(categories))
	for _, c := range categories {
		values = append(values, c.Value)
	}
	return values
}

// splitKeywords splits an itunes keywords string. An empty string yields nil,
// not []string{""}.
func splitKeywords(keywords string) []string {
	if keywords == "" {
		return nil
	}
	return strings.Split(keywords, ",")
}
