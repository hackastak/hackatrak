package vault

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLocateSectionTargetsBucketJournalWeekday(t *testing.T) {
	note := string(readFixture(t, "2026-W45.md"))
	chicago := mustLocation(t, "America/Chicago")

	tests := []struct {
		name   string
		bucket string
		day    time.Time
		want   string
		absent []string
	}{
		{
			name:   "sap monday is not the hackastak monday",
			bucket: "SAP Business Network & ANT",
			day:    time.Date(2026, 11, 2, 18, 30, 0, 0, chicago),
			want: strings.Join([]string{
				"- [[supply-chain-monitor]] (NEXTGEN-60297): root-caused the shared batch failure <!-- ht:supply-chain-monitor -->",
				"	- (NEXTGEN-60297): reviewed the unentitled read path",
				"	- mentioned Monday in a nested note, not a heading",
				"",
			}, "\n"),
			absent: []string{"Hackatrak: added git repo discovery", "# Hackastak", "Tuesday"},
		},
		{
			name:   "sap friday stops before the bucket separator",
			bucket: "SAP Business Network & ANT",
			day:    time.Date(2026, 11, 6, 9, 0, 0, 0, chicago),
			want:   "- OOO - advisory board meeting\n",
			absent: []string{"# __", "# Hackastak", "Start Hackastak for Business"},
		},
		{
			name:   "hackastak monday ignores the checklist above the journal",
			bucket: "Hackastak",
			day:    time.Date(2026, 11, 2, 8, 0, 0, 0, chicago),
			want: strings.Join([]string{
				"- Staksmith: restored what-next and backlog-review",
				"	- ran the reflect skill baseline",
				"- Hackatrak: added git repo discovery and current branch lookup",
				"",
			}, "\n"),
			absent: []string{
				"Finish the Hackatrak work tracker",
				"<!-- ht:hackatrak -->",
				"SAP Business Network",
				"Tuesday",
			},
		},
		{
			name:   "sunday stays sunday in central time",
			bucket: "Hackastak",
			day:    time.Date(2026, 11, 8, 22, 0, 0, 0, chicago),
			want: strings.Join([]string{
				"- Hackatrak: condense commit subjects into one line <!-- ht:hackatrak -->",
				"	- add file fallback and ticket extraction",
				"- Hackatrak: set up production repo infra",
				"",
				"",
				"",
			}, "\n"),
			absent: []string{"General Thoughts", "End of Week Tasks", "Saturday", "collapse events into day presence"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LocateSection(note, tt.bucket, tt.day)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(note, "\n")
			if lines[got.Heading] != got.Weekday {
				t.Fatalf("heading line = %q, want %q", lines[got.Heading], got.Weekday)
			}
			if got.Start != got.Heading+1 {
				t.Fatalf("start = %d, heading = %d", got.Start, got.Heading)
			}
			body := strings.Join(lines[got.Start:got.End], "\n")
			if body != tt.want {
				t.Fatalf("section =\n%q\nwant\n%q", body, tt.want)
			}
			for _, bad := range tt.absent {
				if strings.Contains(body, bad) {
					t.Fatalf("section contains %q\n%s", bad, body)
				}
			}
		})
	}
}

func TestLocateSectionDoesNotCrossBuckets(t *testing.T) {
	note := string(readFixture(t, "2026-W45.md"))
	day := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	sap, err := LocateSection(note, "SAP Business Network & ANT", day)
	if err != nil {
		t.Fatal(err)
	}
	hack, err := LocateSection(note, "Hackastak", day)
	if err != nil {
		t.Fatal(err)
	}
	if sap.Heading >= hack.Heading {
		t.Fatalf("SAP heading %d is not before Hackastak heading %d", sap.Heading, hack.Heading)
	}
	if sap.End > hack.Heading {
		t.Fatalf("SAP section end %d overlaps Hackastak heading %d", sap.End, hack.Heading)
	}
	if hack.Bucket != "Hackastak" || sap.Weekday != "Monday" || hack.Weekday != "Monday" {
		t.Fatalf("sections = %#v %#v", sap, hack)
	}
}

func TestLocateSectionTemplateWeekdays(t *testing.T) {
	raw := string(readFixture(t, "Weekly_note.md"))
	note, err := fillDateLine(raw, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	monday := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	sapFriday, err := LocateSection(note, "SAP Business Network & ANT", monday.AddDate(0, 0, 4))
	if err != nil {
		t.Fatal(err)
	}
	if body := spanText(note, sapFriday); strings.TrimSpace(body) != "" || strings.Contains(body, "#") {
		t.Fatalf("empty SAP Friday = %q", body)
	}
	_, err = LocateSection(note, "SAP Business Network & ANT", monday.AddDate(0, 0, 6))
	if !errors.Is(err, ErrWeekdayNotFound) {
		t.Fatalf("SAP Sunday err = %v, want ErrWeekdayNotFound", err)
	}

	for i := 0; i < 7; i++ {
		day := monday.AddDate(0, 0, i)
		got, err := LocateSection(note, "Hackastak", day)
		if err != nil {
			t.Fatalf("Hackastak %s: %v", day.Weekday(), err)
		}
		if got.Weekday != day.Weekday().String() {
			t.Fatalf("weekday = %s, want %s", got.Weekday, day.Weekday())
		}
		body := spanText(note, got)
		if strings.TrimSpace(body) != "" || strings.Contains(body, "#") {
			t.Fatalf("empty %s = %q", got.Weekday, body)
		}
	}
}

func TestLocateSectionMissingPieces(t *testing.T) {
	day := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		note   string
		bucket string
		want   error
	}{
		{name: "empty note", note: "", bucket: "Hackastak", want: ErrBucketNotFound},
		{name: "wrong bucket", note: "# Other\n\n#### Daily Journal\nWednesday\n", bucket: "Hackastak", want: ErrBucketNotFound},
		{name: "heading prefix is not the bucket", note: "# Hackastak Extra\n\n#### Daily Journal\nWednesday\n", bucket: "Hackastak", want: ErrBucketNotFound},
		{name: "h2 is not a bucket", note: "## Hackastak\n\n#### Daily Journal\nWednesday\n", bucket: "Hackastak", want: ErrBucketNotFound},
		{name: "journal outside the bucket", note: "# Other\n\n#### Daily Journal\nWednesday\n\n# Hackastak\n\n- checklist\n", bucket: "Hackastak", want: ErrJournalNotFound},
		{name: "h3 journal is not the journal", note: "# Hackastak\n\n### Daily Journal\nWednesday\n", bucket: "Hackastak", want: ErrJournalNotFound},
		{name: "weekday missing", note: "# Hackastak\n\n#### Daily Journal\nMonday\n", bucket: "Hackastak", want: ErrWeekdayNotFound},
		{name: "weekday word inside a bullet is not a heading", note: "# Hackastak\n\n#### Daily Journal\n- Monday standup\n", bucket: "Hackastak", want: ErrWeekdayNotFound},
		{name: "hashed weekday is not a heading", note: "# Hackastak\n\n#### Daily Journal\n## Wednesday\n", bucket: "Hackastak", want: ErrWeekdayNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LocateSection(tt.note, tt.bucket, day)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}

	t.Run("empty bucket", func(t *testing.T) {
		_, err := LocateSection("# Hackastak\n", "  ", day)
		if err == nil {
			t.Fatal("expected error for empty bucket")
		}
	})
}

func TestLocateSectionToleratesWhitespaceAndCRLF(t *testing.T) {
	note := "# Hackastak  \r\n\r\n#### Daily Journal\r\nWednesday  \r\n- kept\r\nThursday\r\n"
	day := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	got, err := LocateSection(note, "Hackastak", day)
	if err != nil {
		t.Fatal(err)
	}
	if got.Weekday != "Wednesday" {
		t.Fatalf("weekday = %s", got.Weekday)
	}
	lines := strings.Split(note, "\n")
	if strings.TrimRight(lines[got.Heading], " \t\r") != "Wednesday" {
		t.Fatalf("heading = %q", lines[got.Heading])
	}
	body := strings.Join(lines[got.Start:got.End], "\n")
	if body != "- kept\r" {
		t.Fatalf("body = %q", body)
	}
}

func spanText(note string, section Section) string {
	lines := strings.Split(note, "\n")
	return strings.Join(lines[section.Start:section.End], "\n")
}
