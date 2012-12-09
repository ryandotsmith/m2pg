package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bmizerany/pq"
	"net/http"
	"os"
	"strings"
	"sync"
)

var dbArray []*sql.DB

type metric struct {
	Id    string  `json:"id"`
	Name  string  `json:"name"`
	Count float64 `json:"count"`
	Mean  float64 `json:"mean"`
}

func genUUID() string {
	f, _ := os.Open("/dev/urandom")
	b := make([]byte, 16)
	f.Read(b)
	f.Close()
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

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

func insertMetric(m *metric) string {
	id := genUUID()
	ch := make(chan int, 1)
	for _, db := range dbArray {
		go func(d *sql.DB) {
			_, err := d.Exec(`
					INSERT INTO metrics (id, name, count, mean)
					VALUES ($1, $2, $3, $4)`,
				id, m.Name, m.Count, m.Mean)
			if err != nil {
				fmt.Printf("at=insert-error error=%s\n", err)
			}
			ch <- 1
		}(db)
	}
	// We can return when the first db insert succeeds.
	<-ch
	return id
}

func getMetrics(d *sql.DB, name string, results chan *metric, wg *sync.WaitGroup) {
	defer wg.Done()
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
		return
	}
	defer rows.Close()
	for rows.Next() {
		m := &metric{}
		rows.Scan(&m.Id, &m.Name, &m.Count, &m.Mean)
		results <- m
	}
}

func composeMetrics(name string) (returnList []*metric) {
	results := make(chan *metric)
	var wg sync.WaitGroup
	for _, db := range dbArray {
		wg.Add(1)
		go getMetrics(db, name, results, &wg)
	}
	// When all of the getMetrics funcs are complete,
	// we need to close the results chan to break our loop.
	go func(w *sync.WaitGroup, c chan *metric) {
		w.Wait()
		close(c)
	}(&wg, results)

	uniqueMetrics := make(map[string]*metric)
	for metric := range results {
		uuid := metric.Id
		if _, ok := uniqueMetrics[uuid]; !ok {
			uniqueMetrics[uuid] = metric
		}
	}
	for _, metric := range uniqueMetrics {
		returnList = append(returnList, metric)
	}
	return returnList
}

func routeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		m := &metric{}
		json.NewDecoder(r.Body).Decode(m)
		writeJson(w, 201, insertMetric(m))
	case "GET":
		writeJson(w, 200, composeMetrics("hello"))
	}
}

func main() {
	initDb()
	http.HandleFunc("/metrics", routeHandler)
	http.ListenAndServe(":8080", nil)
}
