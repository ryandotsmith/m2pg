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
	"time"
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

func getMetrics(d *sql.DB, name string, result chan []*metric) {
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
	var metrics []*metric
	for rows.Next() {
		m := &metric{}
		rows.Scan(&m.Id, &m.Name, &m.Count, &m.Mean)
		metrics = append(metrics, m)
	}
	result <- metrics
}

func composeMetrics(name string) (returnList []*metric) {
	result := make(chan []*metric)
	for _, db := range dbArray {
		go getMetrics(db, name, result)
	}
	//First result to come back wins.
	metrics := <-result
	uniqueMetrics := make(map[string]*metric)
	for _, metric := range metrics {
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
