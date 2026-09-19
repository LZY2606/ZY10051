package gofeed_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

// benchFeedItemCount is the number of items in each generated benchmark feed.
const benchFeedItemCount = 1000

func benchRSSFeed() string {
	var b strings.Builder
	b.WriteString(`<rss version="2.0"><channel><title>bench</title><link>https://example.org/</link>`)
	for i := 0; i < benchFeedItemCount; i++ {
		fmt.Fprintf(&b, `<item><title>Item %d</title><link>https://example.org/%d</link>`+
			`<description>Description %d</description><author>jane@example.org (Jane Doe)</author>`+
			`<guid>urn:bench:%d</guid><pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>`+
			`<category>tech</category>`+
			`<enclosure url="https://example.org/%d.mp3" type="audio/mpeg" length="123"/></item>`,
			i, i, i, i, i)
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

func benchAtomFeed() string {
	var b strings.Builder
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom"><title>bench</title>` +
		`<link rel="alternate" href="https://example.org/"/><updated>2006-01-03T15:04:05Z</updated>`)
	for i := 0; i < benchFeedItemCount; i++ {
		fmt.Fprintf(&b, `<entry><title>Item %d</title><link rel="alternate" href="https://example.org/%d"/>`+
			`<id>urn:bench:%d</id><summary>Description %d</summary>`+
			`<author><name>Jane Doe</name><email>jane@example.org</email></author>`+
			`<published>2006-01-02T15:04:05Z</published><updated>2006-01-03T15:04:05Z</updated>`+
			`<category term="tech"/>`+
			`<link rel="enclosure" href="https://example.org/%d.mp3" type="audio/mpeg" length="123"/></entry>`,
			i, i, i, i, i)
	}
	b.WriteString(`</feed>`)
	return b.String()
}

func benchJSONFeed() string {
	var b strings.Builder
	b.WriteString(`{"version":"https://jsonfeed.org/version/1.1","title":"bench","home_page_url":"https://example.org/","items":[`)
	for i := 0; i < benchFeedItemCount; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"urn:bench:%d","url":"https://example.org/%d","title":"Item %d",`+
			`"summary":"Description %d","content_html":"<p>Content %d</p>",`+
			`"author":{"name":"Jane Doe"},"date_published":"2006-01-02T15:04:05Z",`+
			`"date_modified":"2006-01-03T15:04:05Z","tags":["tech"],`+
			`"attachments":[{"url":"https://example.org/%d.mp3","mime_type":"audio/mpeg","size_in_bytes":123}]}`,
			i, i, i, i, i, i)
	}
	b.WriteString(`]}`)
	return b.String()
}

func benchmarkParse(b *testing.B, feed string) {
	b.ReportAllocs()
	parser := gofeed.NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := parser.ParseString(feed); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse1000ItemsRSS(b *testing.B)  { benchmarkParse(b, benchRSSFeed()) }
func BenchmarkParse1000ItemsAtom(b *testing.B) { benchmarkParse(b, benchAtomFeed()) }
func BenchmarkParse1000ItemsJSON(b *testing.B) { benchmarkParse(b, benchJSONFeed()) }
