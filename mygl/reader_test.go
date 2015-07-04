package mygl

import (
	"io"
	"os"
	"path"
	"reflect"
	"testing"
	"time"
)

const (
	testFileDir = "test_files"
)

func assertFile(t *testing.T, filename string, expected []*Entry) {
	f, err := os.Open(path.Join(testFileDir, filename))
	if err != nil {
		t.Fatalf("unable to open test file %s for reading, %v", filename, err)
	}
	r := NewReader(f)
	for i, e := range expected {
		actual, err := r.ReadEntry()
		if err != nil {
			t.Fatalf("%s entry %d, unable to read entry, %v", filename, i, err)
		}
		if !reflect.DeepEqual(actual, e) {
			t.Errorf(
				"%s entry %d, entries do not match\nexpected: %s\nactual:   %s",
				filename,
				i,
				e,
				actual,
			)
		}
	}
	if _, err := r.ReadEntry(); err != io.EOF {
		t.Errorf("%s, expected io.EOF at end but got %v", filename, err)
	}
}

func parseTime(t *testing.T, input string) time.Time {
	ti, err := time.Parse(TimeFormat, input)
	if err != nil {
		t.Fatalf("time %s is in the wrong format", input)
	}
	return ti
}

func TestReader_ReadEntry_simple(t *testing.T) {
	assertFile(t, "simple", []*Entry{
		{
			Time:     parseTime(t, "150703 23:26:04"),
			ID:       15,
			Command:  CmdQuery,
			Argument: "SET GLOBAL query_cache_size=0",
		},
		{
			Time:    parseTime(t, "150703 23:26:13"),
			ID:      15,
			Command: CmdQuery,
			Argument: `SELECT
*
FROM
blah_core.users`,
		},
		{
			Time:     parseTime(t, "150703 23:30:07"),
			ID:       16,
			Command:  CmdConnect,
			Argument: "someuser@localhost on blah_core",
		},
		{
			Time:     parseTime(t, "150703 23:30:07"),
			ID:       16,
			Command:  CmdQuery,
			Argument: "select @@version_comment limit 1",
		},
		{
			Time:     parseTime(t, "150703 23:30:07"),
			ID:       16,
			Command:  CmdQuery,
			Argument: "SELECT COUNT(DISTINCT user_id), COUNT(DISTINCT organisation_id) FROM usersorganisations WHERE last_on >= UNIX_TIMESTAMP() - 600",
		},
		{
			Time:     parseTime(t, "150703 23:30:07"),
			ID:       16,
			Command:  CmdQuit,
			Argument: "",
		},
		{
			Time:     parseTime(t, "150703 23:30:07"),
			ID:       17,
			Command:  CmdConnect,
			Argument: "someuser@localhost on blah_core",
		},
		{
			Time:     parseTime(t, "150703 23:30:07"),
			ID:       17,
			Command:  CmdQuery,
			Argument: "select @@version_comment limit 1",
		},
	})
}
