// m2pg.
// Read & Write metrics to a PostgreSQL database with some notion of HA.

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bmizerany/pq"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var dbArray []*sql.DB

type metric struct {
	Id    string  `json:"id"`
	Name  string  `json:"name"`
	Count float64 `json:"count"`
	Mean  float64 `json:"mean"`
}

// This is technically NOT universal. However,
// the cost of duplicates is not great for m2pg.
func genUUID() string {
	f, _ := os.Open("/dev/urandom")
	b := make([]byte, 16)
	f.Read(b)
	f.Close()
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Convenience
func writeJson(resp http.ResponseWriter, status int, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("at=error error=%s\n", err)
		writeJson(resp, 500, map[string]string{"error": "Internal Server Error"})
	}
	resp.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp.WriteHeader(status)
	resp.Write(b)
	resp.Write([]byte("\n"))
}

// Multiple databses for increased availability.
func initDb() {
	urls := strings.Split(os.Getenv("DATABASE_URLS"), "|")
	for _, url := range urls {
		conf, err := pq.ParseURL(url)
		if err != nil {
			fmt.Printf("Unable to parse DATABASE_URLS.\n")
			os.Exit(1)
		}
		db, err := sql.Open("postgres", conf)
		if err != nil {
			fmt.Printf("Unable to connect to postgres\n")
			os.Exit(1)
		}
		dbArray = append(dbArray, db)
	}
}

// InsertMetric guarantees to write the metric to 1 database.
// An error is returned if we were unable to write to at least 1 database.
// Writes are wrapped in a timeout so that network-partitioned  databases will
// not disrupt m2pg's write APIs.
func insertMetric(m *metric) (string, error) {
	id := genUUID()
	ch := make(chan int, 1)
	for _, db := range dbArray {
		go func(d *sql.DB) {
			_, err := d.Exec(`
					INSERT INTO metrics (id, name, count, mean)
					VALUES ($1, $2, $3, $4)`,
				id, m.Name, m.Count, m.Mean)
			if err != nil {
				fmt.Printf("measure=insert-error error=%s\n", err)
			} else {
				ch <- 1
			}
		}(db)
	}
	timeout := time.Tick(time.Second * 10)
	select {
	case <-ch:
		return id, nil
	case <-timeout:
		return "", errors.New("Unable to write metric")
	}
	return "", errors.New("Unhandled error.")
}

// GetMetrics will query the supplied database for metrics
// inside of a timeout. We wrap the query inside of a timeout
// for the case in which the database is offline. When the query
// has timedot, we signal that we are done via the WaitGroup so that
// the caller of getMetrics can sucessfully degrade.
func getMetrics(d *sql.DB, name string, metricsCh chan []*metric, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	result := make(chan []*metric, 1)
	go func() {
		rows, err := d.Query(`
			select
			  id,
			  name,
			  sum(count) as count,
			  avg(mean) as mean
			from
			  metrics
			group by id, name
		`)
		if err != nil {
			fmt.Printf("at=select-error err=%s\n", err)
			result <- make([]*metric, 0)
			return
		}
		defer rows.Close()
		var metrics []*metric
		for rows.Next() {
			m := &metric{}
			rows.Scan(&m.Id, &m.Name, &m.Count, &m.Mean)
			metrics = append(metrics, m)
		}
		result <- metrics
	}()
	timeout := time.Tick(time.Second * 10)
	select {
	case metrics := <-result:
		metricsCh <- metrics
	case <-timeout:
		fmt.Printf("at=query-timeout\n")
	}
}

// ComposeMetrics will query each database in parellel, remove duplicates
// then return a slice of metrics that were matched by the query.
func composeMetrics(name string) (returnList []*metric) {
	results := make(chan []*metric)
	var wg sync.WaitGroup
	for _, db := range dbArray {
		go getMetrics(db, name, results, &wg)
	}
	//When all of the goroutines are finished getting metrics,
	//we will close the chan to break or loop.
	go func(c chan []*metric, w *sync.WaitGroup) {
		w.Wait()
		close(c)
	}(results, &wg)
	// Since we are querying many databases, we will dedupe using our UUID.
	uniqueMetrics := make(map[string]*metric)
	for metrics := range results {
		for _, metric := range metrics {
			uuid := metric.Id
			if _, ok := uniqueMetrics[uuid]; !ok {
				uniqueMetrics[uuid] = metric
			}
		}
	}
	//We used a map to filter duplicate metrics. We will now convert
	//the map into a slice to handoff to the HTTP response.
	for _, metric := range uniqueMetrics {
		returnList = append(returnList, metric)
	}
	return returnList
}

// m2pg has two endpoints. POST /metrics and GET /metrics.
func routeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		m := &metric{}
		json.NewDecoder(r.Body).Decode(m)
		b, err := insertMetric(m)
		if err != nil {
			writeJson(w, 500, map[string]error{"error": err})
		} else {
			writeJson(w, 201, b)
		}
	case "GET":
		writeJson(w, 200, composeMetrics("hello"))
	}
}

func main() {
	initDb()
	http.HandleFunc("/metrics", routeHandler)
	http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
