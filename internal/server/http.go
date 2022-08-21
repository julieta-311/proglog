package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// NewHTTPServer takes in an address for the server to run on and returns
// a server handling POST and GET requests to root.
func NewHTTPServer(addr string) *http.Server {
	httpsrv := newHTTPServer()

	r := mux.NewRouter()
	r.HandleFunc("/", httpsrv.handleProduce).Methods(http.MethodPost)
	r.HandleFunc("/", httpsrv.handleConsume).Methods(http.MethodGet)

	return &http.Server{
		Addr:         addr,
		Handler:      r,
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

}

type httpServer struct {
	Log *Log
}

func newHTTPServer() *httpServer {
	return &httpServer{
		Log: NewLog(),
	}
}

// ProduceRequest contains the record that the requester wants to log.
type ProduceRequest struct {
	Record Record `json:"record"`
}

// ProduceResponse holds the offset an record was logged at.
type ProduceResponse struct {
	Offset uint64 `json:"offset"`
}

// ConsumeRequest holds the offset a requester wants to read logs from.
type ConsumeRequest struct {
	Offset uint64 `json:"offset"`
}

// ConsumeResponse holds a record read from a Log.
type ConsumeResponse struct {
	Record Record `json:"record"`
}

func (s *httpServer) handleProduce(w http.ResponseWriter, r *http.Request) {
	var req ProduceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	off, err := s.Log.Append(req.Record)
	if err != nil {
		fmt.Printf("failed to append record: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	res := ProduceResponse{Offset: off}
	if err = json.NewEncoder(w).Encode(res); err != nil {
		fmt.Printf("failed to encode produce response: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func (s *httpServer) handleConsume(w http.ResponseWriter, r *http.Request) {
	var req ConsumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	record, err := s.Log.Read(req.Offset)
	if err != nil {
		fmt.Printf("failed to read record at %v: %v", req.Offset, err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	res := ConsumeResponse{Record: record}
	if err = json.NewEncoder(w).Encode(res); err != nil {
		fmt.Printf("failed to encode consume response: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}
