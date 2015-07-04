package mygl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"
)

var prefixRegexp = regexp.MustCompile(
	`^(\d{6} \d{2}:\d{2}:\d{2}|\t)\t\s*([\d]+) ([^\t]+)\t`)

// Common commands from the query log.
const (
	CmdBinlogDump    = "Binlog Dump"
	CmdChangeUser    = "Change user"
	CmdCloseStmt     = "Close stmt"
	CmdConnect       = "Connect"
	CmdConnectOut    = "Connect Out"
	CmdCreateDB      = "Create DB"
	CmdDaemon        = "Daemon"
	CmdDebug         = "Debug"
	CmdDelayedInsert = "Delayed insert"
	CmdDropDB        = "Drop DB"
	CmdError         = "Error"
	CmdExecute       = "Execute"
	CmdFetch         = "Fetch"
	CmdFieldList     = "Field List"
	CmdInitDB        = "Init DB"
	CmdKill          = "Kill"
	CmdLongData      = "Long Data"
	CmdPing          = "Ping"
	CmdPrepare       = "Prepare"
	CmdProcesslist   = "Processlist"
	CmdQuery         = "Query"
	CmdQuit          = "Quit"
	CmdRefresh       = "Refresh"
	CmdRegisterSlave = "Register Slave"
	CmdResetStmt     = "Reset stmt"
	CmdSetOption     = "Set option"
	CmdShutdown      = "Shutdown"
	CmdSleep         = "Sleep"
	CmdStatistics    = "Statistics"
	CmdTableDump     = "Table Dump"
	CmdTime          = "Time"
)

// AtFormat is the format of the time column
const TimeFormat = "060102 15:04:05"

const peekLen = 64

const (
	readStateFind = iota
	readStateConsume
)

// Entry is an entry in the MySQL general log.
type Entry struct {
	Time              time.Time
	ID                int
	Command, Argument string
}

func (c Entry) String() string {
	return fmt.Sprintf(
		"%s	%d %s	%s",
		c.Time.Format(TimeFormat),
		c.ID,
		c.Command,
		c.Argument,
	)
}

type peekReader interface {
	io.Reader
	ReadString(delim byte) (line string, err error)
	Peek(n int) ([]byte, error)
}

// Reader reads Command instances from a reader.
type Reader struct {
	inner    peekReader
	lastTime time.Time
}

// NewReader returns a new Reader, wrapping the passed reader in a bufio.Reader
// if the existing one doesn't provide the Peek function.
func NewReader(r io.Reader) *Reader {
	var pr peekReader
	if ppr, ok := r.(peekReader); ok {
		pr = ppr
	} else {
		pr = bufio.NewReader(r)
	}
	return &Reader{
		inner: pr,
	}
}

// ReadEntry reads an entry from a general log, skipping headers if required.
func (r *Reader) ReadEntry() (*Entry, error) {
	entry := &Entry{
		Time: r.lastTime,
	}
	state := readStateFind
	arg := bytes.Buffer{}
	defer func() {
		entry.Argument = arg.String()
	}()

	running := true
	for running {
		switch state {
		case readStateFind:
			line, err := r.inner.ReadString('\n')
			if err != nil && err != io.EOF {
				return entry, err
			}
			if matches := prefixRegexp.FindStringSubmatch(line); matches != nil {
				state = readStateConsume
				// Time
				if matches[1][0] != ' ' {
					if t, err := time.Parse(TimeFormat, matches[1]); err == nil {
						entry.Time = t
						r.lastTime = t
					}
				}
				// ID
				if id, err := strconv.Atoi(matches[2]); err == nil {
					entry.ID = id
				}
				entry.Command = matches[3]
				arg.WriteString(line[len(matches[0]):])
			} else if err == io.EOF {
				return entry, err
			}
		case readStateConsume:
			p, err := r.inner.Peek(peekLen)
			if err != nil && err != io.EOF {
				return entry, err
			}
			if len(p) == 0 || prefixRegexp.Match(p) {
				// This is a new query or the end of the reader, don't consume.
				arg.Truncate(arg.Len() - 1)
				running = false
			} else {
				line, err := r.inner.ReadString('\n')
				if err != nil && err != io.EOF {
					return entry, err
				}
				arg.WriteString(line)
			}
		}
	}
	return entry, nil
}
