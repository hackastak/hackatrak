package vault

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

func TestSpliceInsertOrReplace(t *testing.T) {
	note := string(readFixture(t, "2026-W45.md"))
	chicago := mustLocation(t, "America/Chicago")
	monday := time.Date(2026, 11, 2, 18, 0, 0, 0, chicago)
	tuesday := monday.AddDate(0, 0, 1)

	t.Run("replaces the marked line and keeps nested hand-written lines", func(t *testing.T) {
		opt, buf := testLog(t)
		before, ok, err := FindMarkedLine(note, "SAP Business Network & ANT", "supply-chain-monitor", monday)
		if err != nil || !ok {
			t.Fatalf("find = %v, %v", ok, err)
		}
		bullet := Bullet{
			Slug: "supply-chain-monitor",
			Text: "- [[supply-chain-monitor]] (NEXTGEN-60297): narrowed the batch window",
		}
		got, err := Splice(note, "SAP Business Network & ANT", monday, bullet, opt)
		if err != nil {
			t.Fatal(err)
		}
		if buf.Len() != 0 {
			t.Fatalf("success logged %q", buf.String())
		}
		want := bullet.Text + " <!-- ht:supply-chain-monitor -->"
		after, ok, err := FindMarkedLine(got, "SAP Business Network & ANT", "supply-chain-monitor", monday)
		if err != nil || !ok {
			t.Fatalf("find after = %v, %v", ok, err)
		}
		if after.Text != want || after.Index != before.Index {
			t.Fatalf("replaced line = %#v, want text %q at %d", after, want, before.Index)
		}
		if strings.Contains(got, "root-caused the shared batch failure") {
			t.Fatal("old marked text still present")
		}
		for _, kept := range []string{
			"\t- (NEXTGEN-60297): reviewed the unentitled read path",
			"\t- mentioned Monday in a nested note, not a heading",
			"- OOO - advisory board meeting",
			"- Staksmith: restored what-next and backlog-review",
		} {
			if strings.Count(got, kept) != 1 {
				t.Fatalf("hand-written line %q count changed", kept)
			}
		}
		assertUnmarkedEqual(t, note, got, "supply-chain-monitor")
		if countSlug(got, "supply-chain-monitor") != 1 {
			t.Fatalf("marker count = %d, want 1", countSlug(got, "supply-chain-monitor"))
		}
		hack, err := LocateSection(got, "Hackastak", monday)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(spanText(got, hack), "supply-chain-monitor") {
			t.Fatal("SAP bullet leaked into Hackastak")
		}
	})

	t.Run("inserts under today without touching an unmarked day or another bucket", func(t *testing.T) {
		bullet := Bullet{
			Slug: "supply-chain-monitor",
			Text: "- [[supply-chain-monitor]] (NEXTGEN-1): checked provisioned accounts",
		}
		got, err := Splice(note, "SAP Business Network & ANT", tuesday, bullet, SpliceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		want := bullet.Text + " <!-- ht:supply-chain-monitor -->"
		found, ok, err := FindMarkedLine(got, "SAP Business Network & ANT", "supply-chain-monitor", tuesday)
		if err != nil || !ok || found.Text != want {
			t.Fatalf("inserted = %#v, %v, %v", found, ok, err)
		}
		section, err := LocateSection(got, "SAP Business Network & ANT", tuesday)
		if err != nil {
			t.Fatal(err)
		}
		body := spanText(got, section)
		hand := "- [[supply-chain-monitor]] : tried to track down provisioned accounts"
		nested := "\t- Picked up another bug ticket that was created in the past 24 hours"
		if !strings.Contains(body, hand) || !strings.Contains(body, nested) || !strings.Contains(body, want) {
			t.Fatalf("tuesday =\n%s", body)
		}
		if strings.Index(body, hand) > strings.Index(body, want) || strings.Index(body, nested) > strings.Index(body, want) {
			t.Fatalf("insert reordered hand-written lines\n%s", body)
		}
		mondaySection, err := LocateSection(got, "SAP Business Network & ANT", monday)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(spanText(got, mondaySection), "checked provisioned accounts") {
			t.Fatal("tuesday bullet landed on monday")
		}
		assertUnmarkedEqual(t, note, got, "supply-chain-monitor")
	})

	t.Run("same slug outside today is not the line we replace", func(t *testing.T) {
		checklist := "- [ ] Finish the Hackatrak work tracker <!-- ht:hackatrak -->"
		bullet := Bullet{Slug: "hackatrak", Text: "- [[hackatrak]]: resolved the weekly note path"}
		got, err := Splice(note, "Hackastak", monday, bullet, SpliceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(got, checklist) != 1 {
			t.Fatal("checklist marker was rewritten")
		}
		found, ok, err := FindMarkedLine(got, "Hackastak", "hackatrak", monday)
		if err != nil || !ok {
			t.Fatalf("monday marker = %v, %v", ok, err)
		}
		if found.Text != bullet.Text+" <!-- ht:hackatrak -->" {
			t.Fatalf("monday line = %q", found.Text)
		}
		again, err := Splice(got, "Hackastak", monday, bullet, SpliceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if again != got {
			t.Fatal("re-run duplicated or moved the monday bullet")
		}
		if strings.Count(again, checklist) != 1 {
			t.Fatal("re-run touched the checklist")
		}
	})
}

func TestSpliceIdempotentAndPrefixSafe(t *testing.T) {
	note := string(readFixture(t, "2026-W45.md"))
	day := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	first := Bullet{Slug: "hackatrak", Text: "- Hackatrak: parsed git log"}
	second := Bullet{Slug: "staksmith", Text: "- [[staksmith]] (NEXTGEN-9): ported a skill"}

	once, err := Splice(note, "Hackastak", day, first, SpliceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Splice(once, "Hackastak", day, first, SpliceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if twice != once {
		t.Fatal("same bullet was not idempotent")
	}
	// The fixture already has two hackatrak markers on Tuesday. Replace the
	// first; leave the nested marked line alone.
	if strings.Count(twice, "\t- handled empty repos and detached HEAD <!-- ht:hackatrak -->") != 1 {
		t.Fatal("nested marked line was modified")
	}
	if strings.Contains(twice, "added git log parsing") {
		t.Fatal("old first line survived replacement")
	}

	both, err := Splice(twice, "Hackastak", day, second, SpliceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if countSlug(both, "staksmith") != 1 || !strings.Contains(both, first.Text+" <!-- ht:hackatrak -->") {
		t.Fatal("second project disturbed the first bullet")
	}
	again, err := Splice(both, "Hackastak", day, second, SpliceOptions{})
	if err != nil || again != both {
		t.Fatal("second project re-run was not idempotent")
	}

	prefix := Bullet{Slug: "hack", Text: "- [[hack]]: not hackatrak"}
	withPrefix, err := Splice(note, "Hackastak", day, prefix, SpliceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(withPrefix, "added git log parsing <!-- ht:hackatrak -->") != 1 {
		t.Fatal("prefix slug replaced hackatrak")
	}
	if countSlug(withPrefix, "hack") != 1 {
		t.Fatalf("hack count = %d", countSlug(withPrefix, "hack"))
	}
}

func TestSpliceKeepsCallerMarkerAndRejectsBadBullets(t *testing.T) {
	note := "# Hackastak\n\n#### Daily Journal\nMonday\n- hand written\nTuesday\n"
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	text := "- [[staksmith]] (NEXTGEN-123): reworked docs flow <!-- ht:staksmith -->"
	got, err := Splice(note, "Hackastak", day, Bullet{Slug: "staksmith", Text: text}, SpliceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, text) != 1 || strings.Contains(got, text+" <!--") {
		t.Fatalf("marker doubled:\n%s", got)
	}
	if !strings.Contains(got, "- hand written\n") {
		t.Fatalf("unmarked line changed:\n%s", got)
	}

	opt, buf := testLog(t, CreateMissing)
	rejects := []Bullet{
		{Slug: "foo--bar", Text: "- nope"},
		{Slug: "hackatrak", Text: ""},
		{Slug: "hackatrak", Text: "- line\n- two"},
		{Slug: "hackatrak", Text: "- wrong <!-- ht:other -->"},
		{Slug: "hackatrak", Text: "- two <!-- ht:hackatrak --> <!-- ht:other -->"},
	}
	for _, bullet := range rejects {
		got, err := Splice(note, "Hackastak", day, bullet, opt)
		if err == nil {
			t.Fatalf("Splice(%q) succeeded", bullet.Text)
		}
		if got != note {
			t.Fatalf("rejected splice modified the note: %q", bullet.Text)
		}
	}
	if buf.Len() != 0 {
		t.Fatalf("validation logged %q", buf.String())
	}

	_, err = Splice(note, "  ", day, Bullet{Slug: "hackatrak", Text: "- x"}, SpliceOptions{})
	if err == nil {
		t.Fatal("empty bucket succeeded")
	}
}

func TestSplicePreservesCRLFOnUnmarkedLines(t *testing.T) {
	note := "# Hackastak\r\n\r\n#### Daily Journal\r\nMonday\r\n- hand written\r\n\r\nTuesday\r\n"
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	got, err := Splice(note, "Hackastak", day, Bullet{Slug: "hackatrak", Text: "- added"}, SpliceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- hand written\r\n") || !strings.Contains(got, "Tuesday\r\n") {
		t.Fatalf("CRLF lines rewritten:\n%q", got)
	}
	if !strings.Contains(got, "- added <!-- ht:hackatrak -->\n") {
		t.Fatalf("bullet = %q", got)
	}
}

func TestSpliceSkipMissingHeading(t *testing.T) {
	note := string(readFixture(t, "2026-W45.md"))
	day := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC) // Saturday
	bullet := Bullet{Slug: "supply-chain-monitor", Text: "- [[supply-chain-monitor]]: Saturday work that must not land"}
	opt, buf := testLog(t, SkipMissing)

	got, err := Splice(note, "SAP Business Network & ANT", day, bullet, opt)
	if !errors.Is(err, ErrWeekdayNotFound) {
		t.Fatalf("err = %v, want ErrWeekdayNotFound", err)
	}
	if got != note {
		t.Fatal("skip modified the note")
	}
	assertSkipLog(t, buf.String(), "supply-chain-monitor", "SAP Business Network & ANT", "Saturday")
	if strings.Contains(buf.String(), "Saturday work") {
		t.Fatal("log included the bullet body")
	}

	buf.Reset()
	got, err = Splice(note, "Personal", day, bullet, opt)
	if !errors.Is(err, ErrBucketNotFound) || got != note {
		t.Fatalf("missing bucket = %v, changed=%v", err, got != note)
	}
	assertSkipLog(t, buf.String(), "supply-chain-monitor", "Personal", "Saturday")

	buf.Reset()
	journal := "# Hackastak\n\n- checklist only\n"
	got, err = Splice(journal, "Hackastak", day, Bullet{Slug: "hackatrak", Text: "- x"}, opt)
	if !errors.Is(err, ErrJournalNotFound) || got != journal {
		t.Fatalf("missing journal = %v, changed=%v", err, got != journal)
	}
	if !strings.Contains(buf.String(), "skipping splice") || !strings.Contains(buf.String(), "daily journal") {
		t.Fatalf("log = %q", buf.String())
	}

	buf.Reset()
	// Zero-value Missing is SkipMissing. Log is set only so the test can read it.
	got, err = Splice("", "Hackastak", day, bullet, SpliceOptions{Log: opt.Log})
	if !errors.Is(err, ErrBucketNotFound) || got != "" {
		t.Fatalf("empty note = %q, %v", got, err)
	}
	if !strings.Contains(buf.String(), "skipping splice") {
		t.Fatal("zero-value policy did not log a skip")
	}
}

func TestSpliceCreateMissingHeading(t *testing.T) {
	raw := string(readFixture(t, "Weekly_note.md"))
	note, err := fillDateLine(raw, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	sunday := time.Date(2026, 1, 11, 15, 0, 0, 0, time.UTC)
	bullet := Bullet{Slug: "supply-chain-monitor", Text: "- [[supply-chain-monitor]]: closed the Saturday gap"}
	opt, buf := testLog(t, CreateMissing)

	got, err := Splice(note, "SAP Business Network & ANT", sunday, bullet, opt)
	if err != nil {
		t.Fatal(err)
	}
	logText := buf.String()
	if !strings.Contains(logText, "created weekday heading \"Sunday\"") || !strings.Contains(logText, "SAP Business Network & ANT") {
		t.Fatalf("log = %q", logText)
	}
	if strings.Contains(logText, "closed the Saturday gap") {
		t.Fatal("log included the bullet body")
	}
	section, err := LocateSection(got, "SAP Business Network & ANT", sunday)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(spanText(got, section), "<!-- ht:supply-chain-monitor -->") {
		t.Fatalf("sunday =\n%s", spanText(got, section))
	}
	if strings.Index(got, "\nSunday\n") > strings.Index(got, "\n# __\n") {
		t.Fatal("SAP Sunday was created after the bucket separator")
	}
	hackSunday, err := LocateSection(got, "Hackastak", sunday)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spanText(got, hackSunday), "supply-chain-monitor") {
		t.Fatal("created the weekday in the wrong bucket")
	}
	assertOriginalSubsequence(t, note, got)

	buf.Reset()
	again, err := Splice(got, "SAP Business Network & ANT", sunday, bullet, opt)
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Fatal("create re-run duplicated the weekday or the bullet")
	}
	if buf.Len() != 0 {
		t.Fatalf("second create logged %q", buf.String())
	}
	if strings.Count(again, "\nSunday\n") != 2 { // SAP created + Hackastak template
		t.Fatalf("Sunday count = %d", strings.Count(again, "\nSunday\n"))
	}
}

func TestSpliceCreateOrdersWeekdayAndBucket(t *testing.T) {
	note := "# Hackastak\n\n#### Daily Journal\nMonday\n- hand\n\nWednesday\n- mid\n\n# Other\nkeep\n"
	tuesday := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	opt, buf := testLog(t, CreateMissing)
	bullet := Bullet{Slug: "hackatrak", Text: "- tuesday work"}

	got, err := Splice(note, "Hackastak", tuesday, bullet, opt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(got, "\nMonday\n") > strings.Index(got, "\nTuesday\n") || strings.Index(got, "\nTuesday\n") > strings.Index(got, "\nWednesday\n") {
		t.Fatalf("weekday order wrong:\n%s", got)
	}
	if !strings.Contains(got, "- hand\n") || !strings.Contains(got, "- mid\n") || !strings.Contains(got, "# Other\nkeep\n") {
		t.Fatalf("existing lines changed:\n%s", got)
	}
	if !strings.Contains(buf.String(), "created weekday heading \"Tuesday\" under \"Hackastak\"") {
		t.Fatalf("log = %q", buf.String())
	}

	buf.Reset()
	noJournal := "# Hackastak\n\n- checklist only\n# Other\nkeep\n"
	got, err = Splice(noJournal, "Hackastak", tuesday, bullet, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- checklist only\n") || !strings.Contains(buf.String(), "created daily journal under \"Hackastak\"") {
		t.Fatalf("journal create =\n%s\nlog %q", got, buf.String())
	}
	if strings.Index(got, "- checklist only") > strings.Index(got, "#### Daily Journal") {
		t.Fatal("journal was inserted above existing bucket content")
	}
	if strings.Contains(got, "# Other\n#### Daily Journal") {
		t.Fatal("journal landed in the next bucket")
	}

	buf.Reset()
	w45 := string(readFixture(t, "2026-W45.md"))
	personal := Bullet{Slug: "hackatrak", Text: "- personal bucket work"}
	got, err = Splice(w45, "Personal", tuesday, personal, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, strings.TrimRight(w45, "\n")) {
		t.Fatal("creating a bucket rewrote the existing note")
	}
	if strings.Count(got, "\n# Personal\n") != 1 {
		t.Fatalf("Personal heading count wrong:\n%s", got[len(got)-400:])
	}
	for _, kind := range []string{"created bucket heading \"Personal\"", "created daily journal under \"Personal\"", "created weekday heading \"Tuesday\" under \"Personal\""} {
		if !strings.Contains(buf.String(), kind) {
			t.Fatalf("log %q missing %s", buf.String(), kind)
		}
	}
	found, ok, err := FindMarkedLine(got, "Personal", "hackatrak", tuesday)
	if err != nil || !ok || !strings.Contains(found.Text, "personal bucket work") {
		t.Fatalf("created bucket marker = %#v, %v, %v", found, ok, err)
	}

	buf.Reset()
	empty, err := Splice("", "Hackastak", tuesday, bullet, opt)
	if err != nil {
		t.Fatal(err)
	}
	wantEmpty := "# Hackastak\n\n#### Daily Journal\nTuesday\n- tuesday work <!-- ht:hackatrak -->\n"
	if empty != wantEmpty {
		t.Fatalf("empty create =\n%q", empty)
	}
}

func testLog(t *testing.T, missing ...MissingHeading) (SpliceOptions, *bytes.Buffer) {
	t.Helper()
	var policy MissingHeading
	if len(missing) > 0 {
		policy = missing[0]
	}
	var buf bytes.Buffer
	return SpliceOptions{Missing: policy, Log: log.New(&buf, "", 0)}, &buf
}

func assertSkipLog(t *testing.T, msg, slug, bucket, weekday string) {
	t.Helper()
	for _, want := range []string{"skipping splice", slug, bucket, weekday, "not found"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("log %q missing %q", msg, want)
		}
	}
}

func assertUnmarkedEqual(t *testing.T, before, after, slug string) {
	t.Helper()
	got := strings.Join(linesWithout(after, slug), "\n")
	want := strings.Join(linesWithout(before, slug), "\n")
	if got != want {
		t.Fatalf("unmarked lines changed\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func assertOriginalSubsequence(t *testing.T, before, after string) {
	t.Helper()
	want := strings.Split(before, "\n")
	got := strings.Split(after, "\n")
	i := 0
	for _, line := range got {
		if i < len(want) && line == want[i] {
			i++
		}
	}
	if i != len(want) {
		t.Fatalf("original note not preserved in order, stuck at %q", want[i])
	}
}

func linesWithout(note, slug string) []string {
	var out []string
	for _, line := range strings.Split(note, "\n") {
		if lineHasSlug(line, slug) {
			continue
		}
		out = append(out, line)
	}
	return out
}

func countSlug(note, slug string) int {
	n := 0
	for _, line := range strings.Split(note, "\n") {
		if lineHasSlug(line, slug) {
			n++
		}
	}
	return n
}
