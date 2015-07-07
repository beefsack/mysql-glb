package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/beefsack/mysql-glb/mygl"
	_ "github.com/go-sql-driver/mysql"
)

const flagPassDefault = "((read from STDIN))"

var roRegexp = regexp.MustCompile(`(?i)^\s*select`)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		// Flags
		user, pass, addr string
		conc             int
		readOnly         bool
		// Stats
		start, end time.Time
	)

	flag.StringVar(&user, "user", "", "MySQL username")
	flag.StringVar(&pass, "pass", flagPassDefault, "MySQL password")
	flag.StringVar(&addr, "addr", "", "MySQL address including protocol with address in brackets.  Eg. tcp(localhost:3306) or unix(/tmp/mysql.sock)")
	flag.IntVar(&conc, "conc", 1, "number of queries to run concurrently")
	flag.BoolVar(&readOnly, "ro", true, "perform read only queries")
	flag.Parse()
	if conc <= 0 {
		log.Fatalf("query concurrency must be positive")
	}

	st := newStats()
	start = time.Now()

	// Make sure we output if SIGINT is raised.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		finalOutput(st, time.Now().Sub(start))
		os.Exit(0)
	}()

	// Build a base DSL string for connecting.
	baseDSLBuf := bytes.Buffer{}
	if user != "" {
		baseDSLBuf.WriteString(user)
		if pass != "" {
			baseDSLBuf.WriteRune(':')
			baseDSLBuf.WriteString(pass)
		}
		baseDSLBuf.WriteRune('@')
	}
	if addr != "" {
		baseDSLBuf.WriteString(addr)
	}
	baseDSLBuf.WriteRune('/')
	baseDSL := baseDSLBuf.String()

	// Worker pool based on conc param.
	connQueue := make(chan []*mygl.Entry, 1024)
	wg := sync.WaitGroup{}
	wg.Add(conc)
	for i := 0; i < conc; i++ {
		go func() {
			defer wg.Done()
			for {
				var (
					entry *mygl.Entry
					conn  *sql.DB
					err   error
				)
				entries, ok := <-connQueue
				if !ok {
					return
				}
				for len(entries) > 0 {
					entry, entries = entries[0], entries[1:]
					switch entry.Command {
					case mygl.CmdConnect:
						parts := strings.Split(entry.Argument, " ")
						dbName := parts[len(parts)-1]
						st.inc(statConnCount)
						conn, err = sql.Open("mysql", baseDSL+dbName)
						if err != nil {
							st.inc(statConnFailed)
						} else if err := conn.Ping(); err != nil {
							st.inc(statConnFailed)
							conn = nil
						} else {
							st.inc(statConnOpen)
						}
					case mygl.CmdQuery:
						st.inc(statQueryCount)
						if conn != nil &&
							(!readOnly || roRegexp.MatchString(entry.Argument)) {
							if _, err := conn.Exec(entry.Argument); err != nil {
								st.inc(statQueryFailed)
							}
						} else {
							st.inc(statQuerySkipped)
						}
					}
				}
				if conn != nil {
					conn.Close()
					st.dec(statConnOpen)
				}
			}
		}()
	}

	// Update every second
	rendering := true
	go func() {
		lastQueryCount := int32(0)
		for rendering {
			queryCount := st.get(statQueryCount) - st.get(statQuerySkipped)
			log.Printf(
				"Conns: %d\tQueries: %d\tQPS: %d",
				st.get(statConnOpen),
				queryCount,
				queryCount-lastQueryCount,
			)
			lastQueryCount = queryCount
			time.Sleep(time.Second)
		}
	}()

	r := mygl.NewReader(os.Stdin)
	connBuf := map[int][]*mygl.Entry{}
	for {
		entry, err := r.ReadEntry()
		if err != nil {
			if err != io.EOF {
				log.Fatalf("error reading entry from general log, %v", err)
			}
			break
		}
		connBuf[entry.ID] = append(connBuf[entry.ID], entry)
		if entry.Command == mygl.CmdQuit {
			connQueue <- connBuf[entry.ID]
			delete(connBuf, entry.ID)
		}
	}
	close(connQueue)
	wg.Wait()
	rendering = false
	end = time.Now()
	finalOutput(st, end.Sub(start))
}

func finalOutput(st stats, d time.Duration) {
	fmt.Printf(`
Total time:         %fs
Queries per second: %f
Total queries:      %d
Failed queries:     %d
Skipped queries:    %d
Total connections:  %d
Failed connections: %d
`,
		d.Seconds(),
		float64(st.get(statQueryCount)-st.get(statQuerySkipped))/d.Seconds(),
		st.get(statQueryCount),
		st.get(statQueryFailed),
		st.get(statQuerySkipped),
		st.get(statConnCount),
		st.get(statConnFailed),
	)
}
