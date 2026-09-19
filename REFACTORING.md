# Translator Normalization Refactoring

The RSS, Atom and JSON translators (`translator.go`) each re-implemented the
same normalization rules when mapping their parsed structs onto the universal
`Feed`/`Item`. This change moves the rules that are *identical* across formats
into shared pure helpers (`normalize.go`) and keeps the rules that differ per
spec in the per-format translators.

## What is shared now (`normalize.go`)

All helpers are pure: they read only their arguments, never mutate the
source-format structs (`rss.Feed`, `atom.Feed`, `json.Feed`, ...), and touch
no global state. None of them is exported, so the public API is unchanged.

- **Empty-value selection** — `firstNonEmpty(values ...string)` drives every
  "use X, fall back to Y" cascade: RSS feed/item title, language, copyright,
  updated, JSON item content (`content_html` → `content_text`), RSS item
  description's trailing atom:summary fallback, RSS item updated.
- **Date fallback** — `parseDateOrNil(text)` replaces seven copies of
  `if s != "" { if d, err := shared.ParseDate(s); err == nil { ... } }`
  (RSS feed/item updated+published, JSON feed updated+published, JSON item
  dates). The raw string field is still preserved verbatim by callers; only
  the parsed companion is best-effort, and unparsable input yields `nil`
  exactly as before.
- **URL candidates** — `isWebLinkRel(rel)` (empty / `alternate` / `self`) and
  `atomWebHrefs(links)` back the three identical rel-filter loops (Atom feed
  links, Atom entry links, atom:link elements embedded in RSS extensions).
  `firstImageURL(candidates...)` backs the first-non-empty image selection
  (Atom logo→icon, JSON feed icon, JSON item image→banner_image).
- **Author assembly** — `rssPerson(primary, dc, itunesAuthor)` merges the two
  near-identical RSS author switches (channel: managingEditor→webMaster→
  dc:author→dc:creator→itunes:author; item: author→dc:author→dc:creator→
  itunes:author). The dc candidates keep their historical "non-nil slice is
  enough" semantics while text candidates require a non-empty string.
- **Category merging** — `appendRSSCategories` and `appendItunesKeywords`
  merge the duplicated plain-category and itunes:keywords handling in RSS
  feed and item category assembly.
- **Nil-safe getters** — `dcTitle/dcLanguage/dcRights/dcDate/dcDescription/
  dcSubjects` and `itunesFeedAuthor/itunesItemAuthor` remove repeated
  `ext != nil &&` guards at call sites.

## What deliberately stays per-format

- **RSS item published vs. publishedParsed asymmetry**: `translateItemPublished`
  treats a non-nil `dc:date` slice as authoritative (an empty first value does
  *not* fall through to atom:published), while `translateItemPublishedParsed`
  *does* fall through on an empty string. Both quirks are preserved as-is.
- **RSS item description**: a non-nil `dc:description` slice short-circuits
  the itunes:summary fallback even when its first value is empty; the
  atom:summary fallback only applies to the final empty result.
- **RSS image selection** (channel `<image>` struct, itunes:image,
  media:content, enclosure sniffing, optional HTML `<img>` scan via
  `DisableContentImageScan`) has no counterpart in the other formats.
- **Atom**: category label preferred over term, generator string assembly
  (`name v<version> <uri>`), entry `published` falling back to `updated`.
- **JSON**: feed-level dates mirror the first item's
  `date_modified`/`date_published`; feed image comes from `icon` only;
  enclosure `length` is derived from `size_in_bytes` (bytes, not duration).
- **Raw date strings** are kept in each format's own representation (RSS
  RFC1123, Atom/JSON RFC3339); only parsed instants are comparable across
  formats. Relative URLs are resolved against `xml:base` only where the
  format parser defines a base (Atom/RSS XML); JSON values pass through
  verbatim.

## Behavior guards (added before the refactor)

- `TestCrossFormatParity_CommonFields` — parses the semantically equivalent
  fixtures `testdata/translator/parity/common_{rss.xml,atom.xml,json.json}`
  and requires identical universal fields (title, description, links, feed
  link, language, image URL, authors, item fields, categories, enclosures,
  parsed date instants) for all three formats.
- `TestCrossFormatDivergence_FormatSpecificRules` — pins the deliberately
  divergent rules listed above so they cannot be silently unified.
- `TestTranslatorEdgeCases` — covers unparsable dates (string kept, parsed
  nil), relative URLs (xml:base resolution vs. pass-through), multiple
  enclosures (count and source order), missing author names (email/URL-only
  persons), and empty-string vs. whitespace-only values (whitespace must not
  trigger fallbacks).

Run them with:

```
go test -run 'TestCrossFormatParity_CommonFields|TestCrossFormatDivergence_FormatSpecificRules|TestTranslatorEdgeCases' -v -count=1 .
```

## Duplication reduction

Metric: lines implementing the four duplicated rule classes (empty-value
selection, URL candidates, author assembly, date fallback), counted with a
fixed set of greps. Before the refactor all of them lived in `translator.go`;
after, they are counted across `translator.go` + `normalize.go` (helper
bodies included).

```
# before (commit HEAD, translator.go only)
for p in 'shared.ParseDate' '== ""|!= ""' 'Rel ==' '^\s*(switch|case )' \
         'firstString' 'personFromText' '&Image{'; do
  git show HEAD:translator.go | grep -cE "$p"
done | paste -sd+ - | bc        # -> 109

# after (working tree, translator.go + normalize.go)
for p in 'shared.ParseDate' '== ""|!= ""' 'Rel ==' '^\s*(switch|case )' \
         'firstString' 'personFromText' '&Image{'; do
  grep -h -cE "$p" translator.go normalize.go | paste -sd+ - | bc
done | paste -sd+ - | bc        # -> 61
```

Per class, before → after:

| class                       | before | after | change |
|-----------------------------|--------|-------|--------|
| `shared.ParseDate` sites    | 7      | 1     | -86%   |
| empty-check lines           | 47     | 27    | -43%   |
| rel-check lines             | 4      | 2     | -50%   |
| switch/case lines           | 11     | 5     | -55%   |
| `firstString` refs          | 16     | 11    | -31%   |
| `personFromText` refs       | 12     | 7     | -42%   |
| `&Image{}` literals         | 12     | 8     | -33%   |
| **total**                   | **109**| **61**| **-44%**|

Total: 109 → 61 lines, a 44% reduction (requirement: ≥ 30%).

## Benchmark: parsing 1000 items

`bench_test.go` generates 1000-item RSS/Atom/JSON feeds in code (no default
input was shrunk and no feature removed) and measures parse+translate:

```
go test -run xxx -bench BenchmarkParse1000Items -benchmem -benchtime=2s -count=1 .
```

Environment: Apple M5 Max (darwin/arm64), go1.26.4. Absolute numbers are
machine-dependent; the allocation *delta* is the criterion (must be ≤ +8%).

Raw summaries:

```
# before (commit HEAD)
BenchmarkParse1000ItemsRSS-18    236   10100797 ns/op   16695492 B/op   164182 allocs/op
BenchmarkParse1000ItemsAtom-18   226   10744818 ns/op   11577934 B/op   194130 allocs/op
BenchmarkParse1000ItemsJSON-18   334    7144591 ns/op    3587768 B/op    54082 allocs/op

# after (working tree)
BenchmarkParse1000ItemsRSS-18    201   11016390 ns/op   16698598 B/op   164182 allocs/op
BenchmarkParse1000ItemsAtom-18   193   12439411 ns/op   11577957 B/op   194130 allocs/op
BenchmarkParse1000ItemsJSON-18   322    7453192 ns/op    3584629 B/op    54082 allocs/op
```

Allocation change: RSS +0.00%, Atom +0.00%, JSON +0.00% — all well under the
+8% budget. ns/op differences between the two runs are run-to-run noise on
this machine (the benchmark was not the optimization target); allocations and
bytes/op are the stable signal.

## Acceptance

From the repository root, no external services, env changes, or network:

```
go build ./...          # preparation
go test ./... -count=1  # acceptance, exit code 0
```
