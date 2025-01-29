package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"context"
	"github.com/gorilla/mux"
	"github.com/segmentio/kafka-go"
)

var (logger *log.Logger
	kafkaWriter *kafka.Conn	
)

const cleanupWindow = 10 * time.Second
const deduplicationWindow = 1 * time.Minute
const maxCacheEvictions = 10000

type Cache interface {
	get(id string) bool
	set(id string)
	startCleanupRoutine()
}

type InMem struct {
	seen sync.Map
}

type RequestsHandler struct {
	Req           atomic.Uint32
	ticker        *time.Ticker
	cache         Cache
	cacheModifier int32
}

func (c *InMem) get(id string) bool {
	_, ok := c.seen.Load(id)
	return ok
}

func (c *InMem) set(id string) {
	c.seen.Store(id, time.Now())
}

func (c *InMem) startCleanupRoutine() {
	ticker := time.NewTicker(cleanupWindow)
	go func() {
		
		for range ticker.C {
			count := 0
			now := time.Now()
			c.seen.Range(func(key, value interface{}) bool {
				req:= value.(time.Time)
				if now.Sub(req) > deduplicationWindow {
					c.seen.Delete(key) // Remove expired ID if it's older than deduplicationWindow
					count++
				}
				return count <= maxCacheEvictions
			})
		}
	}()
}

func (u *RequestsHandler) accept(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	if ok := u.cache.get(fmt.Sprintf("%d:%s", u.cacheModifier, id)); ok {
		http.Error(w, "ratelimited.. try again", http.StatusBadRequest)
		return
	}

	u.Req.Add(1)

	endpoint := r.URL.Query().Get("endpoint")
	logger.Printf("id: %s, endpoint: %s", id, endpoint)

	if endpoint != "" {
		url := fmt.Sprintf("http://localhost:8000%s?visits=%d", endpoint, u.Req.Load())

		resp, _ := http.Post(url, "application/json", nil)
		log.Printf("response code: %d", resp.StatusCode)
		if resp.StatusCode != 200 {
			http.Error(w, "failed", http.StatusBadRequest)
			return
		}
	}

	u.cache.set(fmt.Sprintf("%d:%s", u.cacheModifier, id))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`ok`))

}

func (u *RequestsHandler) postEndpoint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (u *RequestsHandler) flushRequests() {
	for  range u.ticker.C {
		val := u.Req.Load()
		u.Req.Store(0)
		u.cacheModifier++

		logger.Printf("unique requests %d", val)

		_, err := kafkaWriter.WriteMessages(kafka.Message{Value: []byte(fmt.Sprintf("%d", val))})
		if err != nil {
			log.Printf("Failed to write message to Kafka: %v", err)
		}
	}
}

func main() {
	handler := &RequestsHandler{
		cache: &InMem{
			seen: sync.Map{},
		},
	}
	handler.ticker = time.NewTicker(deduplicationWindow)
	var err error
	kafkaWriter , err = kafka.DialLeader(context.Background(), "tcp", "localhost:9092", "requests", 0)
	if err != nil {
		log.Fatalf("Failed to connect to Kafka: %v", err)
	}
	go handler.flushRequests()

	handler.cache.startCleanupRoutine()

	

	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open log file")
	}
	logger = log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	router := mux.NewRouter()
	router.HandleFunc("/api/verve/accept", handler.accept).Methods("GET")
	router.HandleFunc("/test", handler.postEndpoint).Methods("POST")
	http.ListenAndServe(":8000", router)

}
