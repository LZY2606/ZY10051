package gofeed_test

import (
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commonFeedView holds the fields that RSS, Atom and JSON Feed all map onto
// the universal Feed. Format-specific fields (FeedType, FeedVersion,
// extensions, generator, copyright, ...) are deliberately excluded.
type commonFeedView struct {
	Title       string
	Description string
	Link        string
	FeedLink    string
	Links       []string
	Language    string
	ImageURL    string
	Authors     []gofeed.Person
	UpdatedUnix int64
	Items       []commonItemView
}

type commonItemView struct {
	Title         string
	Link          string
	Links         []string
	Description   string
	Content       string
	GUID          string
	Authors       []gofeed.Person
	PublishedUnix int64
	UpdatedUnix   int64
	Categories    []string
	Enclosures    []gofeed.Enclosure
}

// Date strings are excluded: each format preserves its own raw representation
// (RFC1123 for RSS, RFC3339 for Atom/JSON); only the parsed instants are
// required to agree.
func commonView(f *gofeed.Feed) commonFeedView {
	view := commonFeedView{
		Title:       f.Title,
		Description: f.Description,
		Link:        f.Link,
		FeedLink:    f.FeedLink,
		Links:       f.Links,
		Language:    f.Language,
	}
	if f.Image != nil {
		view.ImageURL = f.Image.URL
	}
	if f.UpdatedParsed != nil {
		view.UpdatedUnix = f.UpdatedParsed.Unix()
	}
	for _, p := range f.Authors {
		view.Authors = append(view.Authors, *p)
	}
	for _, i := range f.Items {
		iv := commonItemView{
			Title:       i.Title,
			Link:        i.Link,
			Links:       i.Links,
			Description: i.Description,
			Content:     i.Content,
			GUID:        i.GUID,
			Categories:  i.Categories,
		}
		if i.PublishedParsed != nil {
			iv.PublishedUnix = i.PublishedParsed.Unix()
		}
		if i.UpdatedParsed != nil {
			iv.UpdatedUnix = i.UpdatedParsed.Unix()
		}
		for _, p := range i.Authors {
			iv.Authors = append(iv.Authors, *p)
		}
		for _, e := range i.Enclosures {
			iv.Enclosures = append(iv.Enclosures, *e)
		}
		view.Items = append(view.Items, iv)
	}
	return view
}

// TestCrossFormatParity_CommonFields parses semantically equivalent RSS, Atom
// and JSON feeds and requires the shared normalization rules (empty-value
// selection, URL candidates, author assembly, date fallback, enclosures) to
// produce identical universal fields for all three formats.
func TestCrossFormatParity_CommonFields(t *testing.T) {
	views := map[string]commonFeedView{}
	for name, path := range map[string]string{
		"rss":  "testdata/translator/parity/common_rss.xml",
		"atom": "testdata/translator/parity/common_atom.xml",
		"json": "testdata/translator/parity/common_json.json",
	} {
		feed, err := gofeed.NewParser().ParseString(string(testutil.ReadFile(t, path)))
		require.NoError(t, err, path)
		require.NotNil(t, feed, path)
		views[name] = commonView(feed)
	}

	assert.Equal(t, views["rss"], views["atom"], "rss vs atom common fields")
	assert.Equal(t, views["rss"], views["json"], "rss vs json common fields")

	// Sanity: the fixtures actually exercise the shared rules (non-trivial
	// values, two enclosures in source order, parsed dates).
	rssView := views["rss"]
	require.Len(t, rssView.Items, 1)
	assert.Equal(t, "Parity Feed", rssView.Title)
	assert.Equal(t, "https://example.org/feed", rssView.FeedLink)
	assert.Equal(t, []gofeed.Person{{Name: "Jane Doe"}}, rssView.Authors)
	assert.Equal(t, []gofeed.Enclosure{
		{URL: "https://example.org/ep1.mp3", Type: "audio/mpeg", Length: "123"},
		{URL: "https://example.org/pic.png", Type: "image/png", Length: "456"},
	}, rssView.Items[0].Enclosures)
	assert.NotZero(t, rssView.Items[0].PublishedUnix)
	assert.NotZero(t, rssView.Items[0].UpdatedUnix)
}

// TestCrossFormatDivergence_FormatSpecificRules pins the rules that are
// intentionally NOT shared between formats, so the refactoring cannot
// silently unify them.
func TestCrossFormatDivergence_FormatSpecificRules(t *testing.T) {
	t.Run("rss_itunes_and_dc_fallbacks", func(t *testing.T) {
		in := `<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:dc="http://purl.org/dc/elements/1.1/"><channel>
			<title>t</title>
			<itunes:summary>feed summary</itunes:summary>
			<itunes:keywords>a,b</itunes:keywords>
			<item>
				<itunes:summary>item summary</itunes:summary>
				<dc:creator>Item Writer</dc:creator>
			</item>
		</channel></rss>`
		feed, err := gofeed.NewParser().ParseString(in)
		require.NoError(t, err)
		assert.Equal(t, "feed summary", feed.Description)
		assert.Equal(t, []string{"a", "b"}, feed.Categories)
		require.Len(t, feed.Items, 1)
		assert.Equal(t, "item summary", feed.Items[0].Description)
		require.NotNil(t, feed.Items[0].Author)
		assert.Equal(t, "Item Writer", feed.Items[0].Author.Name)
	})

	t.Run("atom_label_generator_and_published_fallback", func(t *testing.T) {
		in := `<feed xmlns="http://www.w3.org/2005/Atom">
			<title>t</title>
			<updated>2006-01-03T15:04:05Z</updated>
			<icon>https://example.org/icon.png</icon>
			<generator uri="https://gen.example" version="1.0">Gen</generator>
			<category term="tech" label="Technology"/>
			<entry>
				<title>i</title><id>urn:1</id>
				<updated>2006-01-03T15:04:05Z</updated>
			</entry>
		</feed>`
		feed, err := gofeed.NewParser().ParseString(in)
		require.NoError(t, err)
		assert.Equal(t, []string{"Technology"}, feed.Categories)
		assert.Equal(t, "Gen v1.0 https://gen.example", feed.Generator)
		require.NotNil(t, feed.Image)
		assert.Equal(t, "https://example.org/icon.png", feed.Image.URL)
		require.Len(t, feed.Items, 1)
		assert.Equal(t, "2006-01-03T15:04:05Z", feed.Items[0].Published)
		require.NotNil(t, feed.Items[0].PublishedParsed)
	})

	t.Run("json_icon_content_and_feed_date_rules", func(t *testing.T) {
		in := `{"version":"https://jsonfeed.org/version/1.1","title":"t",
			"icon":"https://example.org/icon.png",
			"items":[
				{"id":"1","content_text":"plain","banner_image":"https://example.org/banner.png",
				 "date_published":"2006-01-02T15:04:05Z","date_modified":"2006-01-03T15:04:05Z"}
			]}`
		feed, err := gofeed.NewParser().ParseString(in)
		require.NoError(t, err)
		require.NotNil(t, feed.Image)
		assert.Equal(t, "https://example.org/icon.png", feed.Image.URL)
		assert.Equal(t, "2006-01-03T15:04:05Z", feed.Updated)
		assert.Equal(t, "2006-01-02T15:04:05Z", feed.Published)
		require.Len(t, feed.Items, 1)
		assert.Equal(t, "plain", feed.Items[0].Content)
		require.NotNil(t, feed.Items[0].Image)
		assert.Equal(t, "https://example.org/banner.png", feed.Items[0].Image.URL)
	})
}

// TestTranslatorEdgeCases pins behavior for the inputs most likely to regress
// when normalization rules are shared: unparsable dates, relative URLs,
// multiple enclosures, missing author names, and empty vs whitespace strings.
func TestTranslatorEdgeCases(t *testing.T) {
	t.Run("date_parse_failure_keeps_string_and_nil_parsed", func(t *testing.T) {
		rssIn := `<rss version="2.0"><channel><title>t</title><item>
			<pubDate>not a date</pubDate></item></channel></rss>`
		atomIn := `<feed xmlns="http://www.w3.org/2005/Atom"><title>t</title>
			<entry><title>i</title><id>urn:1</id><published>not a date</published></entry></feed>`
		jsonIn := `{"version":"https://jsonfeed.org/version/1.1","title":"t",
			"items":[{"id":"1","date_published":"not a date"}]}`

		for name, in := range map[string]string{"rss": rssIn, "atom": atomIn, "json": jsonIn} {
			feed, err := gofeed.NewParser().ParseString(in)
			require.NoError(t, err, name)
			require.Len(t, feed.Items, 1, name)
			assert.Equal(t, "not a date", feed.Items[0].Published, name)
			assert.Nil(t, feed.Items[0].PublishedParsed, name)
		}
	})

	t.Run("relative_url_handling_stays_format_specific", func(t *testing.T) {
		// Atom resolves links against xml:base; RSS and JSON pass the raw
		// value through. The refactoring must not unify these.
		atomIn := `<feed xmlns="http://www.w3.org/2005/Atom" xml:base="https://example.org/base/">
			<title>t</title><link rel="alternate" href="home"/></feed>`
		feed, err := gofeed.NewParser().ParseString(atomIn)
		require.NoError(t, err)
		assert.Equal(t, "https://example.org/base/home", feed.Link)

		rssIn := `<rss version="2.0"><channel><title>t</title><link>/home</link></channel></rss>`
		feed, err = gofeed.NewParser().ParseString(rssIn)
		require.NoError(t, err)
		assert.Equal(t, "/home", feed.Link)

		jsonIn := `{"version":"https://jsonfeed.org/version/1.1","title":"t","home_page_url":"/home","items":[]}`
		feed, err = gofeed.NewParser().ParseString(jsonIn)
		require.NoError(t, err)
		assert.Equal(t, "/home", feed.Link)
	})

	t.Run("multiple_enclosures_keep_source_order", func(t *testing.T) {
		rssIn := `<rss version="2.0"><channel><title>t</title><item>
			<enclosure url="https://example.org/a.mp3" type="audio/mpeg" length="1"/>
			<enclosure url="https://example.org/b.mp4" type="video/mp4" length="2"/>
			<enclosure url="https://example.org/c.png" type="image/png" length="3"/>
		</item></channel></rss>`
		atomIn := `<feed xmlns="http://www.w3.org/2005/Atom"><title>t</title><entry>
			<title>i</title><id>urn:1</id>
			<link rel="enclosure" href="https://example.org/a.mp3" type="audio/mpeg" length="1"/>
			<link rel="enclosure" href="https://example.org/b.mp4" type="video/mp4" length="2"/>
			<link rel="enclosure" href="https://example.org/c.png" type="image/png" length="3"/>
		</entry></feed>`
		jsonIn := `{"version":"https://jsonfeed.org/version/1.1","title":"t","items":[{"id":"1","attachments":[
			{"url":"https://example.org/a.mp3","mime_type":"audio/mpeg","size_in_bytes":1},
			{"url":"https://example.org/b.mp4","mime_type":"video/mp4","size_in_bytes":2},
			{"url":"https://example.org/c.png","mime_type":"image/png","size_in_bytes":3}]}]}`

		want := []gofeed.Enclosure{
			{URL: "https://example.org/a.mp3", Type: "audio/mpeg", Length: "1"},
			{URL: "https://example.org/b.mp4", Type: "video/mp4", Length: "2"},
			{URL: "https://example.org/c.png", Type: "image/png", Length: "3"},
		}
		for name, in := range map[string]string{"rss": rssIn, "atom": atomIn, "json": jsonIn} {
			feed, err := gofeed.NewParser().ParseString(in)
			require.NoError(t, err, name)
			require.Len(t, feed.Items, 1, name)
			got := []gofeed.Enclosure{}
			for _, e := range feed.Items[0].Enclosures {
				got = append(got, *e)
			}
			assert.Equal(t, want, got, name)
		}
	})

	t.Run("missing_author_name", func(t *testing.T) {
		rssIn := `<rss version="2.0"><channel><title>t</title>
			<item><author>jane@example.org</author></item></channel></rss>`
		feed, err := gofeed.NewParser().ParseString(rssIn)
		require.NoError(t, err)
		require.NotNil(t, feed.Items[0].Author)
		assert.Equal(t, gofeed.Person{Name: "", Email: "jane@example.org"}, *feed.Items[0].Author)

		atomIn := `<feed xmlns="http://www.w3.org/2005/Atom"><title>t</title>
			<author><email>jane@example.org</email></author></feed>`
		feed, err = gofeed.NewParser().ParseString(atomIn)
		require.NoError(t, err)
		require.NotNil(t, feed.Author)
		assert.Equal(t, gofeed.Person{Name: "", Email: "jane@example.org"}, *feed.Author)

		jsonIn := `{"version":"https://jsonfeed.org/version/1.1","title":"t",
			"author":{"url":"https://example.org/jane"},"items":[]}`
		feed, err = gofeed.NewParser().ParseString(jsonIn)
		require.NoError(t, err)
		require.NotNil(t, feed.Author)
		assert.Equal(t, gofeed.Person{Name: "", URL: "https://example.org/jane"}, *feed.Author)
	})

	t.Run("empty_string_vs_whitespace", func(t *testing.T) {
		// Empty description falls back to itunes:summary; a whitespace-only
		// CDATA description is a real value and must NOT trigger the fallback.
		rssIn := `<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"><channel><title>t</title>
			<item><description></description><itunes:summary>fallback</itunes:summary></item>
			<item><description><![CDATA[   ]]></description><itunes:summary>fallback</itunes:summary></item>
		</channel></rss>`
		feed, err := gofeed.NewParser().ParseString(rssIn)
		require.NoError(t, err)
		require.Len(t, feed.Items, 2)
		assert.Equal(t, "fallback", feed.Items[0].Description)
		assert.Equal(t, "   ", feed.Items[1].Description)

		// JSON strings are preserved verbatim: whitespace stays, empty stays.
		jsonIn := `{"version":"https://jsonfeed.org/version/1.1","title":"t","items":[
			{"id":"1","summary":""},{"id":"2","summary":"   "}]}`
		feed, err = gofeed.NewParser().ParseString(jsonIn)
		require.NoError(t, err)
		require.Len(t, feed.Items, 2)
		assert.Equal(t, "", feed.Items[0].Description)
		assert.Equal(t, "   ", feed.Items[1].Description)
	})
}
