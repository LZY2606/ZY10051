package gofeed

import (
	"strings"
	"time"

	"github.com/mmcdole/gofeed/atom"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/mmcdole/gofeed/internal/shared"
	"github.com/mmcdole/gofeed/rss"
)

// This file holds the normalization rules that are genuinely identical
// across the RSS, Atom and JSON translators. Every helper is pure: it reads
// only its arguments, never mutates the source-format structs, and touches no
// global state. Rules that differ per format on purpose stay in
// translator.go; see REFACTORING.md for the split.

// firstNonEmpty returns the first value that is not "", or "".
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseDateOrNil parses a feed date string, returning nil for empty or
// unparsable input. The raw string is always preserved by the callers in the
// sibling string field; only the parsed companion is best-effort.
func parseDateOrNil(text string) *time.Time {
	if text == "" {
		return nil
	}
	if date, err := shared.ParseDate(text); err == nil {
		return &date
	}
	return nil
}

// isWebLinkRel reports whether a link relation denotes a navigable web link:
// empty (alternate by spec default), "alternate" or "self". It is used for
// both atom.Link rels and atom:link elements embedded in RSS extensions.
func isWebLinkRel(rel string) bool {
	return rel == "" || rel == "alternate" || rel == "self"
}

// atomWebHrefs returns the hrefs of the links that denote web pages or feeds,
// in document order. It returns nil when no link qualifies.
func atomWebHrefs(links []*atom.Link) []string {
	var hrefs []string
	for _, l := range links {
		if isWebLinkRel(l.Rel) {
			hrefs = append(hrefs, l.Href)
		}
	}
	return hrefs
}

// firstImageURL wraps the first non-empty candidate URL in an Image, or
// returns nil when every candidate is empty.
func firstImageURL(candidates ...string) *Image {
	if url := firstNonEmpty(candidates...); url != "" {
		return &Image{URL: url}
	}
	return nil
}

// rssPerson picks a Person from the candidate set shared by RSS channels and
// items: a primary text field, then dc:author, dc:creator, then
// itunes:author. Text candidates must be non-empty; the dc candidates only
// require a non-nil slice, matching the historical behavior.
func rssPerson(primary string, dc *ext.DublinCoreExtension, itunesAuthor string) *Person {
	switch {
	case primary != "":
		return personFromText(primary)
	case dc != nil && dc.Author != nil:
		return personFromText(firstString(dc.Author))
	case dc != nil && dc.Creator != nil:
		return personFromText(firstString(dc.Creator))
	case itunesAuthor != "":
		return personFromText(itunesAuthor)
	}
	return nil
}

// appendRSSCategories appends the values of plain RSS categories to dst.
func appendRSSCategories(dst []string, categories []*rss.Category) []string {
	for _, c := range categories {
		dst = append(dst, c.Value)
	}
	return dst
}

// appendItunesKeywords splits an itunes:keywords value on commas and appends
// the parts to dst. An empty value appends nothing.
func appendItunesKeywords(dst []string, keywords string) []string {
	if keywords != "" {
		dst = append(dst, strings.Split(keywords, ",")...)
	}
	return dst
}

// The dc* and itunes* getters below give nil-safe access to optional
// extension structs so call sites can use firstNonEmpty without pre-checks.

func dcTitle(dc *ext.DublinCoreExtension) string {
	if dc == nil {
		return ""
	}
	return firstString(dc.Title)
}

func dcLanguage(dc *ext.DublinCoreExtension) string {
	if dc == nil {
		return ""
	}
	return firstString(dc.Language)
}

func dcRights(dc *ext.DublinCoreExtension) string {
	if dc == nil {
		return ""
	}
	return firstString(dc.Rights)
}

func dcDate(dc *ext.DublinCoreExtension) string {
	if dc == nil {
		return ""
	}
	return firstString(dc.Date)
}

func dcDescription(dc *ext.DublinCoreExtension) string {
	if dc == nil {
		return ""
	}
	return firstString(dc.Description)
}

func dcSubjects(dc *ext.DublinCoreExtension) []string {
	if dc == nil {
		return nil
	}
	return dc.Subject
}

func itunesFeedAuthor(it *ext.ITunesFeedExtension) string {
	if it == nil {
		return ""
	}
	return it.Author
}

func itunesItemAuthor(it *ext.ITunesItemExtension) string {
	if it == nil {
		return ""
	}
	return it.Author
}
