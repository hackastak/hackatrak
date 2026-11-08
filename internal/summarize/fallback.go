package summarize

import (
	"fmt"
	"strings"
)

// fileFallback is the presence line used when no commit subject survives.
// A non-positive count is not a file day. The sample is the comma-separated
// names the scan stored. An incomplete sample ends with an ellipsis inside
// the parentheses. An empty sample omits the parentheses.
func fileFallback(count int, sample string) string {
	if count <= 0 {
		return ""
	}
	noun := "files"
	if count == 1 {
		noun = "file"
	}
	names := sampleNames(sample)
	if len(names) > count {
		names = names[:count]
	}
	if len(names) == 0 {
		return fmt.Sprintf("touched %d %s", count, noun)
	}
	shown := strings.Join(names, ", ")
	if len(names) < count {
		shown += ", …"
	}
	return fmt.Sprintf("touched %d %s (%s)", count, noun, shown)
}

func sampleNames(sample string) []string {
	if strings.TrimSpace(sample) == "" {
		return nil
	}
	parts := strings.Split(sample, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" || name == "…" {
			continue
		}
		names = append(names, name)
	}
	return names
}
