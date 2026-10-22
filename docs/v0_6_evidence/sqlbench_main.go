// Benchmark modernc SQLite under ACE-like concurrent load: W writer goroutines + R reader goroutines.
// Modes: "one" = SetMaxOpenConns(1) for everything (current ACE); "split" = 1 writer conn + read-only pool.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

func open(path string, n int) *sql.DB {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(n)
	return db
}

func pct(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return d[int(float64(len(d)-1)*p)]
}

func run(mode string, writers, readers int, rate float64, dur time.Duration) {
	path := fmt.Sprintf("%s/b_%s_%d.db", os.TempDir(), mode, time.Now().UnixNano())
	defer os.Remove(path)
	w := open(path, 1)
	r := w
	if mode == "split" {
		r = open(path, readers)
	}
	w.Exec(`create table t(id integer primary key, cycle text, layer int, body text, created_at text)`)
	w.Exec(`create index t_cycle on t(cycle)`)
	body := string(make([]byte, 4000)) // ~ a thought trace row
	var mu sync.Mutex
	var wl, rl []time.Duration
	stop := time.Now().Add(dur)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tick := time.Duration(float64(time.Second) * float64(writers) / rate)
			for time.Now().Before(stop) {
				t := time.Now()
				if _, err := w.Exec(`insert into t(cycle, layer, body, created_at) values(?,?,?,?)`, fmt.Sprintf("c%d", t.UnixNano()%500), i%7, body, t.String()); err != nil {
					panic(err)
				}
				d := time.Since(t)
				mu.Lock(); wl = append(wl, d); mu.Unlock()
				time.Sleep(tick - d)
			}
		}(i)
	}
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				t := time.Now()
				var n int
				r.QueryRow(`select count(*) from (select id from t where cycle = ? order by id desc limit 50)`, fmt.Sprintf("c%d", t.UnixNano()%500)).Scan(&n)
				mu.Lock(); rl = append(rl, time.Since(t)); mu.Unlock()
				time.Sleep(20 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
	fmt.Printf("%-5s target=%5.0f writes/s writers=%2d readers=%2d | writes=%6d (%.0f/s) p50=%v p99=%v max=%v | reads=%6d p50=%v p99=%v\n",
		mode, rate, writers, readers, len(wl), float64(len(wl))/dur.Seconds(), pct(wl, .5), pct(wl, .99), pct(wl, 1), len(rl), pct(rl, .5), pct(rl, .99))
}

func main() {
	for _, rate := range []float64{25, 250, 2500} {
		for _, mode := range []string{"one", "split"} {
			run(mode, 16, 8, rate, 8*time.Second)
		}
	}
}
