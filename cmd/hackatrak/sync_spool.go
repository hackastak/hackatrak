package main

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/spool"
	"github.com/hackastak/hackatrak/internal/store"
)

// sapBucket is the weekly-note heading for work-account handoff.
// A spool day labeled "SAP" is the same heading.
const sapBucket = "SAP Business Network & ANT"

// publishSync is the role split after collect. The spooler writes a spool
// file and does not open the weekly note. The writer splices the note.
// The returned count is note bullets changed, never spool days.
func publishSync(cfg config.Config, db *store.DB, now time.Time, logw io.Writer) (int, error) {
	switch cfg.Role {
	case config.RoleSpooler:
		if err := writeSpool(cfg, db, now, logw); err != nil {
			return 0, err
		}
		return 0, nil
	case config.RoleWriter:
		return writeOwnBuckets(cfg, db, now, logw)
	default:
		return 0, fmt.Errorf("sync: role %q is not supported", cfg.Role)
	}
}

// writeSpool writes this account's ISO-week spool from derived and
// summarized day-presence. It does not read or create the weekly note.
// An idle week leaves any existing spool where it is.
func writeSpool(cfg config.Config, db *store.DB, now time.Time, logw io.Writer) error {
	bullets, err := weekBullets(cfg, db, now, log.New(logw, "", 0))
	if err != nil {
		return err
	}
	if len(bullets) == 0 {
		return nil
	}
	year, week := now.ISOWeek()
	days := make([]spool.Day, len(bullets))
	for i, bullet := range bullets {
		days[i] = spool.Day{
			Date:   bullet.Day,
			Bucket: bullet.Bucket,
			Slug:   bullet.Slug,
			Ticket: bullet.Ticket,
			Text:   bullet.Text,
		}
	}
	path, err := spool.Path(cfg.SpoolDir, cfg.Account, now)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	wrote, err := spool.Write(path, spool.File{
		Version: spool.Version,
		Account: cfg.Account,
		Year:    year,
		Week:    week,
		Days:    days,
	})
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	if !wrote || logw == nil {
		return nil
	}
	noun := "project-days"
	if len(days) == 1 {
		noun = "project-day"
	}
	_, err = fmt.Fprintf(logw, "sync: spooled %s (%d %s)\n", path, len(days), noun)
	return err
}

// spoolBullets reads every current-week spool file and keeps the days that
// belong in the SAP bucket. Other headings are logged and skipped so a
// spool cannot splice the writer's own journal. The marker is added later
// by the same splice path as an own-bucket bullet.
func spoolBullets(cfg config.Config, now time.Time, logger *log.Logger) ([]noteBullet, error) {
	year, week := now.ISOWeek()
	paths, err := spool.Discover(cfg.SpoolDir, year, week)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	var bullets []noteBullet
	for _, path := range paths {
		file, err := spool.Read(path)
		if err != nil {
			return nil, fmt.Errorf("sync: %w", err)
		}
		if file.Year != year || file.Week != week {
			return nil, fmt.Errorf("sync: %s is %04d-W%02d, not %04d-W%02d", path, file.Year, file.Week, year, week)
		}
		for _, day := range file.Days {
			bucket, ok := sapHeading(day.Bucket)
			if !ok {
				if logger != nil {
					logger.Printf("sync: skip spool %s %s: bucket %q is not %s", day.Date, day.Slug, day.Bucket, sapBucket)
				}
				continue
			}
			bullets = append(bullets, noteBullet{
				Day:    day.Date,
				Bucket: bucket,
				Slug:   day.Slug,
				Ticket: day.Ticket,
				Text:   day.Text,
			})
		}
	}
	return bullets, nil
}

func sapHeading(bucket string) (string, bool) {
	switch strings.TrimSpace(bucket) {
	case "SAP", sapBucket:
		return sapBucket, true
	default:
		return "", false
	}
}
