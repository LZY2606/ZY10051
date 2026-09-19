package gofeed_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

// benchItemCount is the number of items each benchmark feed carries, so
// allocation numbers are directly comparable across formats and across
// revisions of the translators.
const benchItemCount = 1000

func buildRSSBenchFeed(n int) []byte {
	var sb strings.Builder
	sb.WriteString(`<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:content="http://purl.org/rss/1.0/modules/content/">` +
		`<channel><title>bench</title><link>https://example.com/</link>` +
		`<lastBuildDate>2006-01-03T11:00:00Z</lastBuildDate><managingEditor>Jane Doe</managingEditor>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, `<item><title>Post %d</title><link>https://example.com/%d</link>`+
			`<guid>urn:bench:%d</guid><description>Summary %d</description>`+
			`<content:encoded><![CDATA[<p>Content %d</p>]]></content:encoded>`+
			`<author>John Item</author><category>Tech</category>`+
			`<pubDate>2006-01-02T15:04:05Z</pubDate><dc:date>2006-01-03T11:00:00Z</dc:date>`+
			`<enclosure url="https://example.com/%d.mp3" type="audio/mpeg" length="111"/></item>`,
			i, i, i, i, i, i)
	}
	sb.WriteString(`</channel></rss>`)
	return []byte(sb.String())
}

func buildAtomBenchFeed(n int) []byte {
	var sb strings.Builder
	sb.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="en-us"><title>bench</title>` +
		`<link rel="alternate" href="https://example.com/"/><link rel="self" href="https://example.com/feed"/>` +
		`<updated>2006-01-03T11:00:00Z</updated><author><name>Jane Doe</name></author>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, `<entry><title>Post %d</title><link rel="alternate" href="https://example.com/%d"/>`+
			`<id>urn:bench:%d</id><summary>Summary %d</summary>`+
			`<content type="html">&lt;p&gt;Content %d&lt;/p&gt;</content>`+
			`<author><name>John Item</name></author><category term="Tech"/>`+
			`<published>2006-01-02T15:04:05Z</published><updated>2006-01-03T11:00:00Z</updated>`+
			`<link rel="enclosure" href="https://example.com/%d.mp3" type="audio/mpeg" length="111"/></entry>`,
			i, i, i, i, i, i)
	}
	sb.WriteString(`</feed>`)
	return []byte(sb.String())
}

func buildJSONBenchFeed(n int) []byte {
	var sb strings.Builder
	sb.WriteString(`{"version":"https://jsonfeed.org/version/1","title":"bench",` +
		`"home_page_url":"https://example.com/","feed_url":"https://example.com/feed",` +
		`"author":{"name":"Jane Doe"},"items":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"id":"urn:bench:%d","url":"https://example.com/%d",`+
			`"title":"Post %d","summary":"Summary %d","content_html":"<p>Content %d</p>",`+
			`"date_published":"2006-01-02T15:04:05Z","date_modified":"2006-01-03T11:00:00Z",`+
			`"author":{"name":"John Item"},"tags":["Tech"],`+
			`"attachments":[{"url":"https://example.com/%d.mp3","mime_type":"audio/mpeg","size_in_bytes":111}]}`,
			i, i, i, i, i, i)
	}
	sb.WriteString(`]}`)
	return []byte(sb.String())
}

func benchmarkParseTranslate1000Items(b *testing.B, data []byte) {
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	parser := gofeed.NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		feed, err := parser.Parse(bytes.NewReader(data))
		if err != nil {
			b.Fatal(err)
		}
		if len(feed.Items) != benchItemCount {
			b.Fatalf("parsed %d items, want %d", len(feed.Items), benchItemCount)
		}
	}
}

func BenchmarkParseTranslate1000ItemsRSS(b *testing.B) {
	benchmarkParseTranslate1000Items(b, buildRSSBenchFeed(benchItemCount))
}

func BenchmarkParseTranslate1000ItemsAtom(b *testing.B) {
	benchmarkParseTranslate1000Items(b, buildAtomBenchFeed(benchItemCount))
}

func BenchmarkParseTranslate1000ItemsJSON(b *testing.B) {
	benchmarkParseTranslate1000Items(b, buildJSONBenchFeed(benchItemCount))
}
