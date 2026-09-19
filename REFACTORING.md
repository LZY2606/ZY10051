# Translator normalization refactoring

The RSS, Atom and JSON translators (`translator.go`) each implemented their own
copies of the same normalization rules when mapping to the universal `Feed` /
`Item`. This change moves the rules that are *genuinely identical* across
formats into shared, pure helpers (`translate_helpers.go`) and keeps the
format-specific rules where they were. No dependency was added and no public
API was added or changed (every helper is unexported). The source format
structs (`rss.Feed`, `atom.Feed`, `json.Feed`) are never mutated, and no helper
reads global state (`atomExtensionKeys` is an immutable constant list).

## Rules now shared

All in `translate_helpers.go`:

- **Empty-value selection** — `firstNonEmpty` (first non-empty candidate) and
  `firstString` (first entry of a slice). Used for feed/item title, language,
  copyright, updated, content (`content:encoded` vs atom `content`;
  `content_html` vs `content_text`), atom published→updated fallback, and the
  RSS managingEditor→webMaster chain. `dcFallbackValues` flattens the
  dc: fallback fields once per feed/item instead of repeating nil checks.
- **URL candidates** — `isWebLinkRel` (rel ∈ {"", "alternate", "self"}) and
  `webLinkHrefs` replace three copies of the same rel whitelist loop (RSS
  channel links, Atom feed links, Atom entry links). `appendNonEmpty` covers
  the JSON home_page_url/feed_url and url/external_url pairs. `firstImageURL`
  covers the Atom logo→icon and JSON image→banner_image/icon candidates.
- **Author assembly** — `rssPersonAuthor` (plain author → dc:author →
  dc:creator → itunes:author) replaces the near-identical feed and item
  switches; `personFromText`, `singleAuthorList` (Author ↔ Authors wrapping,
  nil-preserving), `atomAuthors` (Authors → primary) and
  `translateJSONAuthors` (legacy `author` fills `authors` only when absent)
  replace four copies of the Author/Authors consistency logic.
- **Date fallback** — `parseDateOrNil` (parse, nil on empty/unparseable)
  replaces seven copies of `if date, err := shared.ParseDate(x); err == nil { y = &date }`.
- **Enclosure construction** — `newEnclosure` for the URL/Length/Type mapping
  shared by RSS enclosures, Atom `rel="enclosure"` links and JSON attachments.
- **Categories** — `rssCategoryValues` and `splitKeywords` (empty string → nil,
  not `[""]`) replace duplicated RSS feed/item loops.

## Rules deliberately kept divergent

These look similar but are format-specific on purpose; unifying them would
change output:

- **FeedLink with multiple `rel="self"` links**: RSS takes the *last*
  atom:link (`translateFeedFeedLink`), Atom takes the *first*
  (`firstLinkWithRel`). Guarded by
  `feed_feedlink_-_rss_channel_atom_link_self_multiple` and
  `feed_feedlink_-_atom10_feed_link_rel_self_multiple`.
- **RSS feed Link fallback** accepts only rel ∈ {"", "alternate"} (no "self"),
  a subset of the shared web-link whitelist.
- **Atom item Published falls back to Updated**; RSS and JSON have no such
  fallback. Guarded by `feed_item_published_-_atom10_feed_entry_updated_fallback`.
- **JSON feed-level Updated/Published mirror the first item's dates**; RSS and
  Atom read feed-level elements. Guarded by
  `json10_feed_dates_from_first_item`.
- **RSS item Published vs PublishedParsed text selection**: the parsed variant
  falls through to the atom `published` extension when dc:date is empty, the
  raw variant does not. Preserved as-is.
- **RSS item description chain** (description → dc:description →
  itunes:summary → atom summary) triggers on dc element *presence*, so an
  empty dc:description still shadows itunes:summary. Preserved as-is.
- **dc author trigger**: a present-but-empty dc:author/dc:creator still wins
  over itunes:author (slice-presence trigger, not content). Kept in
  `rssPersonAuthor`.
- **Whitespace**: the XML parsers trim element text, the JSON parser keeps
  string values verbatim (`"title": "   "` stays `"   "`). Locked by
  `TestCrossFormat_EmptyAndWhitespace`.
- **JSON author object with empty name** still produces a non-nil `Person{}`;
  an empty RSS managingEditor produces no author. Same test.
- **JSON enclosure Length** is `size_in_bytes` formatted as a decimal string,
  only when > 0; RSS/Atom copy the source length verbatim.
- **Image selection**: RSS keeps its itunes/media/enclosure/HTML-scan chain
  (and `DisableContentImageScan`); only the trivial first-non-empty-URL tail
  of the Atom/JSON rules is shared.

## Behavior guards

New tests (all run by `go test ./... -count=1` from the repo root):

- `TestCrossFormat_EquivalentFeeds` — parses the semantically equivalent
  fixtures `testdata/translator/crossformat/equivalent.{rss.xml,atom.xml,json}`
  and requires identical projections of the shared fields (title, description,
  links, feed link, language, updated, image, authors, and per-item title,
  content, link, GUID, dates, authors, categories, enclosures). The fixture
  exercises multiple enclosures and a relative enclosure URL.
- `TestCrossFormat_DateParseFailure` — unparseable dates keep the raw string
  and leave the parsed field nil, in all three formats (feed and item level).
- `TestCrossFormat_RelativeURL` — relative URLs pass through unchanged.
- `TestCrossFormat_MissingAuthorName` — email-only (RSS/Atom) and URL-only
  (JSON) authors.
- `TestCrossFormat_EmptyAndWhitespace` — empty vs whitespace-only values and
  the nil/non-nil author distinction.
- New format-specific fixtures under `testdata/translator/{rss,atom,json}`
  (listed above) prove the divergent rules were not unified. Expectations were
  generated from the pre-refactoring code, so they lock byte-compatible output.
- The pre-existing `testdata/translator/**` fixture suite (full-struct
  equality) covers output fields, nil vs empty slices, candidate priority,
  extension content and error text.
- Differential check: `go run ./cmd/ftest <file>` was run over all 1122
  fixtures under `testdata/parser` and `testdata/translator` (stdout and
  stderr) with the pre-refactoring `translator.go` (git HEAD) and with the
  refactored code; every output is byte-identical, including error text for
  the invalid-feed fixtures.

## Duplication reduction

Metric: lines in the translator code implementing the four duplicated idioms
(empty-value selection, URL candidates, author assembly, date fallback),
counted with:

```
# before (translator.go @ git HEAD):
git show HEAD:translator.go | grep -cE 'shared\.ParseDate|== "" \|\||personFromText\(|\[\]\*Person\{|firstString\(|if [a-zA-Z.]+ == ""'
# after (translator.go + translate_helpers.go):
cat translator.go translate_helpers.go | grep -cE 'shared\.ParseDate|== "" \|\||personFromText\(|\[\]\*Person\{|firstString\(|if [a-zA-Z.]+ == ""'
```

Result: **48 → 22 lines (−54.2%)**, well above the 30% target. Per category
(each counted with its own sub-pattern, so a line matching two categories is
counted in both):

| category                    | pattern              | before | after | reduction |
|-----------------------------|----------------------|--------|-------|-----------|
| date fallback               | `shared\.ParseDate`  | 7      | 1     | −86%      |
| URL candidates              | `== "" \|\|`         | 4      | 2     | −50%      |
| author assembly             | `personFromText\(` + `\[\]\*Person\{` | 15 | 7 | −53% |
| empty-value selection       | `firstString\(` + `if [a-zA-Z.]+ == ""` | 29 | 14 | −52% |

The surviving URL-candidate instances are the deliberate RSS-only divergences
described above; surviving author/empty-value instances are the single shared
implementations plus call sites.

## Benchmark (1000 items)

Command (run from the repo root, no external services or network):

```
go test ./ -run xxx -bench BenchmarkParseTranslate1000Items -benchmem -count=5
```

Environment (machine-dependent results): `go version go1.26.4 darwin/arm64`,
Apple M5 Max, macOS; benchmarks parse and translate a generated 1000-item feed
per iteration (`BenchmarkParseTranslate1000Items{RSS,Atom,JSON}` in
`translator_bench_test.go`).

Median of 5 runs, allocs/op (requirement: increase ≤ 8%):

| benchmark | before  | after   | Δ allocs/op | Δ B/op   |
|-----------|---------|---------|-------------|----------|
| RSS       | 220,248 | 220,251 | +3 (+0.001%) | +0.11%  |
| Atom      | 216,183 | 216,182 | −1 (−0.000%) | +0.000% |
| JSON      | 54,088  | 54,090  | +2 (+0.004%) | +0.19%  |

Raw summaries (one line per run, `allocs/op` is the last column):

```
# before
BenchmarkParseTranslate1000ItemsRSS-18    82  13831483 ns/op  22353851 B/op  220251 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    93  12670203 ns/op  22335846 B/op  220248 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    94  12512363 ns/op  22330980 B/op  220247 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    97  12919393 ns/op  22330132 B/op  220246 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    94  12869917 ns/op  22339961 B/op  220248 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   97  12027360 ns/op  17076732 B/op  216183 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18  100  12276081 ns/op  17076695 B/op  216183 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   92  12636326 ns/op  17076698 B/op  216183 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   99  12036340 ns/op  17076716 B/op  216183 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18  100  15365668 ns/op  17076877 B/op  216185 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  158   8045553 ns/op   3570959 B/op   54087 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  141   7862643 ns/op   3570674 B/op   54087 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  170   7253275 ns/op   3580296 B/op   54089 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  172   6598637 ns/op   3581467 B/op   54089 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  170   6843062 ns/op   3579327 B/op   54088 allocs/op
# after
BenchmarkParseTranslate1000ItemsRSS-18    84  13781347 ns/op  22364574 B/op  220252 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    87  13799091 ns/op  22346995 B/op  220250 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    81  13482158 ns/op  22361098 B/op  220252 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    87  14614053 ns/op  22365956 B/op  220251 allocs/op
BenchmarkParseTranslate1000ItemsRSS-18    63  16146260 ns/op  22337772 B/op  220249 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   76  13814586 ns/op  17076730 B/op  216182 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   82  15863956 ns/op  17076832 B/op  216183 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   97  13291654 ns/op  17076710 B/op  216182 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   96  12900184 ns/op  17076695 B/op  216182 allocs/op
BenchmarkParseTranslate1000ItemsAtom-18   90  12815454 ns/op  17076726 B/op  216182 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  190   6238048 ns/op   3587339 B/op   54090 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  195   6044960 ns/op   3585043 B/op   54090 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  193   6166844 ns/op   3578439 B/op   54089 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  198   6244016 ns/op   3586376 B/op   54090 allocs/op
BenchmarkParseTranslate1000ItemsJSON-18  189   6098168 ns/op   3585949 B/op   54090 allocs/op
```

No feature was removed and no default input was shrunk to meet the budget; the
benchmark feeds carry the same 1000 fully-populated items before and after.
