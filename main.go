package main

import (
	"bytes"
	"database/sql"
	"flag"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/beefsack/mysql-glb/mygl"
	_ "github.com/go-sql-driver/mysql"
)

const flagPassDefault = "((read from STDIN))"

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		// Flags
		user, pass, addr string
		conc             int
		// Stats
		queries, skipped, connCount, connErrors, queryErrors int
		start, end                                           time.Time
	)

	flag.StringVar(&user, "user", "", "MySQL username")
	flag.StringVar(&pass, "pass", flagPassDefault, "MySQL password")
	flag.StringVar(&addr, "addr", "", "MySQL address including protocol with address in brackets.  Eg. tcp(localhost:3306) or unix(/tmp/mysql.sock)")
	flag.IntVar(&conc, "conc", 1, "number of queries to run concurrently")
	flag.Parse()
	if conc <= 0 {
		log.Fatalf("query concurrency must be positive")
	}
	log.Printf("query concurrency is %d", conc)

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

	conns := map[int]*sql.DB{}
	connM := newIntLocker()
	entryQueue := make(chan *mygl.Entry, 1024)
	wg := sync.WaitGroup{}
	wg.Add(conc)
	start = time.Now()

	for i := 0; i < conc; i++ {
		go func() {
			defer wg.Done()
			for {
				entry, ok := <-entryQueue
				if !ok {
					return
				}
				connM.lock(entry.ID)
				switch entry.Command {
				case mygl.CmdConnect:
					parts := strings.Split(entry.Argument, " ")
					dbName := parts[len(parts)-1]
					connCount++
					db, err := sql.Open("mysql", baseDSL+dbName)
					if err != nil {
						connErrors++
					} else {
						if err := db.Ping(); err != nil {
							connErrors++
						} else {
							conns[entry.ID] = db
						}
					}
				case mygl.CmdQuit:
					db, ok := conns[entry.ID]
					if ok {
						db.Close()
						delete(conns, entry.ID)
					}
				case mygl.CmdQuery:
					db, ok := conns[entry.ID]
					if ok {
						queries++
						if _, err := db.Exec(entry.Argument); err != nil {
							queryErrors++
						}
					} else {
						skipped++
					}
				}
				connM.unlock(entry.ID)
			}
		}()
	}

	r := mygl.NewReader(os.Stdin)
	for {
		entry, err := r.ReadEntry()
		if err != nil {
			if err != io.EOF {
				log.Fatalf("error reading entry from general log, %v", err)
			}
			break
		}
		entryQueue <- entry
	}
	close(entryQueue)
	wg.Wait()
	end = time.Now()
	seconds := end.Sub(start).Seconds()

	log.Printf(`summary as follows:
Total time:         %fs
Queries per second: %f
Total queries:      %d
Failed queries:     %d
Skipped queries:    %d
Total connections:  %d
Failed connections: %d`,
		seconds,
		float64(queries)/seconds,
		queries,
		queryErrors,
		skipped,
		connCount,
		connErrors,
	)
}
