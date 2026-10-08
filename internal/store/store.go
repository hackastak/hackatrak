// Package store opens the local SQLite database and persists events.
package store

import (
	_ "modernc.org/sqlite" // pinned until the DB opener lands
)
