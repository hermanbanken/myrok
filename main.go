package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type request struct {
	UUID       string              `json:"uuid"`
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"body_base64"`
}
type response struct {
	UUID       string              `json:"uuid"`
	Status     int16               `json:"status"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"body_base64"`
}

type endpoint struct {
	requests  chan request
	responses map[string]chan response
	context   context.Context
}

var (
	upgrader  = websocket.Upgrader{} // use default options
	endpoints sync.Map
)

func main() {
	server := &http.Server{
		Addr: fmt.Sprintf(":%s", os.Getenv("PORT")),
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/proxy" {
				proxy(rw, r)
				return
			}

			endpointID, _ := splitPath(strings.TrimPrefix(r.URL.Path, "/"))
			if iface, ok := endpoints.Load(endpointID); ok {
				e := iface.(endpoint)
				processRequest(e, rw, r)
				return
			}
			http.Error(rw, "Not Found", http.StatusNotFound)
		}),
	}

	err := server.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func proxy(rw http.ResponseWriter, r *http.Request) {
	endpointID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	e := endpoint{
		requests:  make(chan request),
		responses: make(map[string]chan response),
		context:   ctx,
	}
	defer cancel()
	endpoints.Store(endpointID, e)
	defer endpoints.Delete(endpointID)

	// upgrade websocket
	c, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	defer c.Close()

	// announce endpoint url
	err = c.WriteJSON(map[string]interface{}{"endpoint": endpointID})
	if err != nil {
		log.Println("write:", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	// read pump
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var resp response
				err := c.ReadJSON(&resp)
				if err != nil {
					log.Println("recv:", err)
					if errors.Is(err, net.ErrClosed) {
						return
					}
					close := websocket.FormatCloseMessage(websocket.CloseAbnormalClosure, "error receiving")
					c.WriteControl(websocket.CloseMessage, close, time.Now().Add(3*time.Second))
					c.Close()
					return
				}
				log.Printf("response: %+v", resp)
				inbox := e.responses[resp.UUID]
				if inbox != nil {
					inbox <- resp
				}
			}
		}
	}()

	// write pump
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-e.requests:
			log.Printf("request: %+v", r)
			err = c.WriteJSON(r)
			if err != nil {
				log.Println("write:", err)
				if errors.Is(err, net.ErrClosed) {
					return
				}
				close := websocket.FormatCloseMessage(websocket.CloseAbnormalClosure, "error writing")
				c.WriteControl(websocket.CloseMessage, close, time.Now().Add(3*time.Second))
				c.Close()
				return
			}
		}
	}
}

func processRequest(e endpoint, rw http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(rw, "Invalid body", http.StatusBadRequest)
		return
	}
	// write 1 request
	req := request{
		UUID:       uuid.NewString(),
		Method:     r.Method,
		Path:       r.URL.Path,
		Headers:    r.Header,
		BodyBase64: base64.StdEncoding.EncodeToString(body),
	}
	responseChannel := make(chan response)
	e.responses[req.UUID] = responseChannel
	defer delete(e.responses, req.UUID)
	e.requests <- req

	// read 1 response
	tick := time.NewTimer(10 * time.Second)
	select {
	case <-tick.C:
		http.Error(rw, "Gateway Timeout", http.StatusGatewayTimeout)
	case resp := <-responseChannel:
		body, err = base64.StdEncoding.DecodeString(resp.BodyBase64)
		if err != nil {
			http.Error(rw, "Invalid proxy response", http.StatusInternalServerError)
			return
		}
		for k, v := range resp.Headers {
			for _, val := range v {
				rw.Header().Add(k, val)
			}
		}
		rw.WriteHeader(int(resp.Status))
		rw.Write([]byte(body))
	}
}

func splitPath(p string) (string, string) {
	i := strings.Index(p, "/")
	if i == -1 {
		return p, ""
	}
	return p[0:i], p[i+1:]
}
