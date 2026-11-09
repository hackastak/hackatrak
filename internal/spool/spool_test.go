package spool

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathUsesISOWeekFilename(t *testing.T) {
	dir := filepath.Join("/vault", "_Weekly", ".hackatrak")
	day := time.Date(2026, 1, 1, 18, 30, 0, 0, time.FixedZone("CST", -6*60*60))

	got, err := Path(dir, "i568439", day)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "spool-i568439-2026-W01.json")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestParseRoundTrip(t *testing.T) {
	const raw = `{
  "version": 1,
  "account": "i568439",
  "year": 2026,
  "week": 1,
  "days": [
    {
      "date": "2026-01-01",
      "bucket": "SAP Business Network & ANT",
      "slug": "hackatrak",
      "ticket": "NEXTGEN-1",
      "text": "- [[hackatrak]] (NEXTGEN-1): defined the spool"
    }
  ]
}
`
	file, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if file.Version != Version || file.Account != "i568439" || file.Year != 2026 || file.Week != 1 {
		t.Errorf("file = %#v", file)
	}
	if len(file.Days) != 1 || file.Days[0].Slug != "hackatrak" || file.Days[0].Ticket != "NEXTGEN-1" {
		t.Errorf("Days = %#v", file.Days)
	}
	if file.Days[0].Bucket != "SAP Business Network & ANT" {
		t.Errorf("Bucket = %q", file.Days[0].Bucket)
	}

	out, err := file.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != raw {
		t.Errorf("Bytes() =\n%s\nwant\n%s", out, raw)
	}
}

func TestParseRejectsSchemaErrors(t *testing.T) {
	cases := []string{
		`{"version":2,"account":"i568439","year":2026,"week":1,"days":[]}`,
		`{"version":1,"account":"a/b","year":2026,"week":1,"days":[]}`,
		`{"version":1,"account":"i568439","year":2026,"week":0,"days":[]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":[{"date":"2026-01-01","bucket":"","slug":"hackatrak","text":"- [[hackatrak]]: x"}]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":[{"date":"2026-01-05","bucket":"SAP","slug":"hackatrak","text":"- [[hackatrak]]: x"}]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":[{"date":"01-01-2026","bucket":"SAP","slug":"hackatrak","text":"- [[hackatrak]]: x"}]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":[{"date":"2026-01-01","bucket":"SAP","slug":"bad--slug","text":"- [[bad--slug]]: x"}]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":[{"date":"2026-01-01","bucket":"SAP","slug":"hackatrak","text":"- [[hackatrak]]:\nsecond"}]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":[{"date":"2026-01-01","bucket":"SAP","slug":"hackatrak","text":"- [[hackatrak]]: x <!-- ht:hackatrak -->"}]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":[{"date":"2026-01-01","bucket":"SAP","slug":"hackatrak","text":"- [[hackatrak]]: x"},{"date":"2026-01-01","bucket":"SAP","slug":"hackatrak","text":"- [[hackatrak]]: y"}]}`,
		`{"version":1,"account":"i568439","year":2026,"week":1,"days":null}`,
	}
	for _, raw := range cases {
		_, err := Parse([]byte(raw))
		if err == nil {
			t.Fatalf("Parse(%s) error = nil", raw)
		}
		if !strings.Contains(err.Error(), "spool:") {
			t.Errorf("error = %v, want spool:", err)
		}
	}
}

func TestParseAllowsEmptyDays(t *testing.T) {
	raw := []byte(`{"version":1,"account":"hackastak","year":2026,"week":1,"days":[]}`)
	file, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Days) != 0 {
		t.Errorf("Days = %#v", file.Days)
	}
}
