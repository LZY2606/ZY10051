package gofeed_test

import (
	"bytes"
	"sort"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commonPerson/commonEnclosure/commonItem/commonFeed project the universal
// Feed into the fields that RSS, Atom and JSON all express, so semantically
// equivalent feeds in the three formats can be compared field by field.
// Fields only some formats map (feed-level Published, feed Categories,
// copyright, item Image) are intentionally excluded here and guarded by
// format-specific fixtures instead.
type commonPerson struct {
	Name  string
	Email string
	URL   string
}

type commonEnclosure struct {
	URL    string
	Length string
	Type   string
}

type commonItem struct {
	Title           string
	Description     string
	Content         string
	Link            string
	Links           []string
	GUID            string
	Published       string
	PublishedParsed string
	Updated         string
	UpdatedParsed   string
	Authors         []commonPerson
	Categories      []string
	Enclosures      []commonEnclosure
}

type commonFeed struct {
	Title         string
	Description   string
	Link          string
	FeedLink      string
	Links         []string
	Language      string
	Updated       string
	UpdatedParsed string
	Image         string
	Author        *commonPerson
	Authors       []commonPerson
	Items         []commonItem
}

func projectTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func projectPersons(persons []*gofeed.Person) []commonPerson {
	if persons == nil {
		return nil
	}
	out := make([]commonPerson, 0, len(persons))
	for _, p := range persons {
		out = append(out, commonPerson{Name: p.Name, Email: p.Email, URL: p.URL})
	}
	return out
}

func projectFeed(f *gofeed.Feed) commonFeed {
	projected := commonFeed{
		Title:         f.Title,
		Description:   f.Description,
		Link:          f.Link,
		FeedLink:      f.FeedLink,
		Links:         append([]string(nil), f.Links...),
		Language:      f.Language,
		Updated:       f.Updated,
		UpdatedParsed: projectTime(f.UpdatedParsed),
		Authors:       projectPersons(f.Authors),
	}
	if f.Author != nil {
		projected.Author = &commonPerson{Name: f.Author.Name, Email: f.Author.Email, URL: f.Author.URL}
	}
	if f.Image != nil {
		projected.Image = f.Image.URL
	}
	sort.Strings(projected.Links)
	for _, item := range f.Items {
		ci := commonItem{
			Title:           item.Title,
			Description:     item.Description,
			Content:         item.Content,
			Link:            item.Link,
			Links:           append([]string(nil), item.Links...),
			GUID:            item.GUID,
			Published:       item.Published,
			PublishedParsed: projectTime(item.PublishedParsed),
			Updated:         item.Updated,
			UpdatedParsed:   projectTime(item.UpdatedParsed),
			Authors:         projectPersons(item.Authors),
			Categories:      item.Categories,
		}
		sort.Strings(ci.Links)
		for _, enc := range item.Enclosures {
			ci.Enclosures = append(ci.Enclosures, commonEnclosure{URL: enc.URL, Length: enc.Length, Type: enc.Type})
		}
		projected.Items = append(projected.Items, ci)
	}
	return projected
}

func parseFixture(t *testing.T, path string) *gofeed.Feed {
	t.Helper()
	feed, err := gofeed.NewParser().Parse(bytes.NewReader(testutil.ReadFile(t, path)))
	require.NoError(t, err, "parse %s", path)
	require.NotNil(t, feed)
	return feed
}

// TestCrossFormat_EquivalentFeeds parses semantically equivalent RSS, Atom
// and JSON feeds and requires the shared normalization rules (link/URL
// candidates, author assembly, date fallback, enclosures, extensions-free
// common fields) to produce identical projections.
func TestCrossFormat_EquivalentFeeds(t *testing.T) {
	rssFeed := projectFeed(parseFixture(t, "testdata/translator/crossformat/equivalent.rss.xml"))
	atomFeed := projectFeed(parseFixture(t, "testdata/translator/crossformat/equivalent.atom.xml"))
	jsonFeed := projectFeed(parseFixture(t, "testdata/translator/crossformat/equivalent.json"))

	assert.Equal(t, rssFeed, atomFeed, "RSS and Atom projections diverged")
	assert.Equal(t, rssFeed, jsonFeed, "RSS and JSON projections diverged")

	// Sanity: the fixture really exercises the shared rules.
	require.Len(t, rssFeed.Items, 2)
	require.Equal(t, "https://example.com/feed", rssFeed.FeedLink)
	require.Equal(t, []commonEnclosure{
		{URL: "https://example.com/audio1.mp3", Length: "111", Type: "audio/mpeg"},
		{URL: "/relative/video.mp4", Length: "222", Type: "video/mp4"},
	}, rssFeed.Items[0].Enclosures, "multiple enclosures, order and relative URL preserved")
	require.Nil(t, rssFeed.Items[1].Enclosures, "item without enclosures keeps nil slice")
	require.Nil(t, rssFeed.Items[1].Authors, "item without author keeps nil slice")
}

// TestCrossFormat_DateParseFailure locks the date fallback behavior when a
// date string cannot be parsed: the raw text is kept and the parsed field
// stays nil, in all three formats.
func TestCrossFormat_DateParseFailure(t *testing.T) {
	cases := map[string]string{
		"rss": `<rss version="2.0"><channel><item>
			<pubDate>not a real date</pubDate>
		</item></channel></rss>`,
		"atom": `<feed xmlns="http://www.w3.org/2005/Atom"><entry>
			<id>x</id><published>not a real date</published>
		</entry></feed>`,
		"json": `{"version":"https://jsonfeed.org/version/1","title":"t","items":[
			{"id":"x","date_published":"not a real date","date_modified":"also not a date"}
		]}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			feed, err := gofeed.NewParser().ParseString(doc)
			require.NoError(t, err)
			require.Len(t, feed.Items, 1)
			item := feed.Items[0]
			assert.Equal(t, "not a real date", item.Published, "raw text preserved")
			assert.Nil(t, item.PublishedParsed, "parsed stays nil on failure")
		})
	}

	// JSON feed-level dates mirror the first item; an unparseable
	// date_modified must leave UpdatedParsed nil while keeping the raw text.
	feed, err := gofeed.NewParser().ParseString(cases["json"])
	require.NoError(t, err)
	assert.Equal(t, "also not a date", feed.Updated)
	assert.Nil(t, feed.UpdatedParsed)
}

// TestCrossFormat_RelativeURL locks that relative URLs pass through
// translation unchanged (no resolution against the feed URL) in all formats.
func TestCrossFormat_RelativeURL(t *testing.T) {
	cases := map[string]string{
		"rss":  `<rss version="2.0"><channel><item><link>/relative/path</link></item></channel></rss>`,
		"atom": `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>x</id><link rel="alternate" href="/relative/path"/></entry></feed>`,
		"json": `{"version":"https://jsonfeed.org/version/1","title":"t","items":[{"id":"x","url":"/relative/path"}]}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			feed, err := gofeed.NewParser().ParseString(doc)
			require.NoError(t, err)
			require.Len(t, feed.Items, 1)
			assert.Equal(t, "/relative/path", feed.Items[0].Link)
			assert.Equal(t, []string{"/relative/path"}, feed.Items[0].Links)
		})
	}
}

// TestCrossFormat_MissingAuthorName locks author assembly when the name is
// absent: RSS/Atom can still carry an email, JSON can still carry a URL.
func TestCrossFormat_MissingAuthorName(t *testing.T) {
	t.Run("rss_email_only", func(t *testing.T) {
		feed, err := gofeed.NewParser().ParseString(
			`<rss version="2.0"><channel><managingEditor>jane@example.com</managingEditor></channel></rss>`)
		require.NoError(t, err)
		require.NotNil(t, feed.Author)
		assert.Equal(t, "", feed.Author.Name)
		assert.Equal(t, "jane@example.com", feed.Author.Email)
	})
	t.Run("atom_email_only", func(t *testing.T) {
		feed, err := gofeed.NewParser().ParseString(
			`<feed xmlns="http://www.w3.org/2005/Atom"><author><email>jane@example.com</email></author></feed>`)
		require.NoError(t, err)
		require.NotNil(t, feed.Author)
		assert.Equal(t, "", feed.Author.Name)
		assert.Equal(t, "jane@example.com", feed.Author.Email)
	})
	t.Run("json_url_only", func(t *testing.T) {
		feed, err := gofeed.NewParser().ParseString(
			`{"version":"https://jsonfeed.org/version/1","title":"t","author":{"url":"https://example.com/jane"},"items":[]}`)
		require.NoError(t, err)
		require.NotNil(t, feed.Author)
		assert.Equal(t, "", feed.Author.Name)
		assert.Equal(t, "", feed.Author.Email)
		assert.Equal(t, "https://example.com/jane", feed.Author.URL)
	})
}

// TestCrossFormat_EmptyAndWhitespace locks the deliberate difference in
// whitespace handling: the XML parsers trim element text, the JSON parser
// keeps string values verbatim. Empty strings and nil slices are distinct.
func TestCrossFormat_EmptyAndWhitespace(t *testing.T) {
	t.Run("xml_whitespace_title_trims_to_empty", func(t *testing.T) {
		for _, doc := range []string{
			`<rss version="2.0"><channel><item><title>   </title></item></channel></rss>`,
			`<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>x</id><title>   </title></entry></feed>`,
		} {
			feed, err := gofeed.NewParser().ParseString(doc)
			require.NoError(t, err)
			require.Len(t, feed.Items, 1)
			assert.Equal(t, "", feed.Items[0].Title)
		}
	})
	t.Run("json_whitespace_title_preserved", func(t *testing.T) {
		feed, err := gofeed.NewParser().ParseString(
			`{"version":"https://jsonfeed.org/version/1","title":"t","items":[{"id":"x","title":"   "}]}`)
		require.NoError(t, err)
		require.Len(t, feed.Items, 1)
		assert.Equal(t, "   ", feed.Items[0].Title)
	})
	t.Run("empty_author_object_stays_present", func(t *testing.T) {
		// JSON: an author object with an empty name still produces a Person
		// (non-nil, all fields empty). RSS: an empty managingEditor produces
		// no author at all. Both behaviors are intentional and must not be
		// unified.
		jsonFeed, err := gofeed.NewParser().ParseString(
			`{"version":"https://jsonfeed.org/version/1","title":"t","author":{"name":""},"items":[]}`)
		require.NoError(t, err)
		require.NotNil(t, jsonFeed.Author, "empty JSON author object stays a non-nil Person")
		assert.Equal(t, gofeed.Person{}, *jsonFeed.Author)
		require.Len(t, jsonFeed.Authors, 1)

		rssFeed, err := gofeed.NewParser().ParseString(
			`<rss version="2.0"><channel><managingEditor></managingEditor></channel></rss>`)
		require.NoError(t, err)
		assert.Nil(t, rssFeed.Author, "empty RSS managingEditor yields no author")
		assert.Nil(t, rssFeed.Authors)
	})
}
