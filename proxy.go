package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

var cache = make(map[string][]byte)
var URLs []string

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}
func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	//caching the response
	reqURI := req.URL.String()
	var resp *http.Response
	var err error
	//_, exists := cache[reqURI]
	fmt.Println("Before map access")
	if _, exists := cache[reqURI]; exists {
		if err != nil {
			fmt.Println("Cache is not working")
		} else {
			fmt.Println("After map access")
			tmp := bytes.NewReader(cache[reqURI])
			reader := bufio.NewReader(tmp)
			resp, err = http.ReadResponse(reader, req)
		}
	} else {
		resp, err = http.DefaultTransport.RoundTrip(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			log.Fatal(err)
		}
		cache[req.URL.String()] = dump
		fmt.Println("Request is stored in cache")
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
func handlePost(w http.ResponseWriter, req *http.Request) {

	//fmt.Println("Enter URLs to block seperated by commas:")
	// reader := bufio.NewReader(os.Stdin)
	// input, _ := reader.ReadString('\n')
	req.ParseForm()

	input := req.Form["website"][0]

	tmp := strings.Split(input, ",")
	for i := range tmp {
		URLs = append(URLs, tmp[i])
	}
}
func main() {

	//URLs := input(emptyURLs)
	end := ":443"
	start := "//"

	//input1 := input + end
	//fmt.Print(strings.TrimSpace(input1))
	var proto string
	flag.StringVar(&proto, "proto", "https", "Proxy protocol (http or https)")
	flag.Parse()
	if proto != "http" && proto != "https" {
		log.Fatal("Protocol must be either http or https")
	}
	server := &http.Server{
		Addr: ":4000",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			reqURI := r.URL.String()

			//fmt.Println("Requested URL: " + r.URL.String())
			blocked := false
			for _, url := range URLs {

				fmt.Println("Requested URL: " + reqURI)
				fmt.Println("Blocked: " + start + strings.TrimSpace(url) + end)
				if ("//" + strings.TrimSpace(url) + ":443") == strings.TrimSpace(reqURI) {
					fmt.Println("This site is blocked")
					blocked = true
				}
			}
			if r.Method == http.MethodConnect && !blocked {
				handleTunneling(w, r)

			} else if r.Method == http.MethodPost && (strings.TrimSpace(r.Host) == "localhost:4000") {
				//fmt.Println(r.Host)
				fmt.Println("post req")
				handlePost(w, r)
			} else if !blocked {
				start := time.Now()
				handleHTTP(w, r)
				t := time.Now()
				elapsed := t.Sub(start)
				fmt.Println("Time for HTTP request: " + elapsed.String())
			}
			//fmt.Println(r)
			// URLs = input(URLs)
		}),
	}
	//URLs = input(URLs)
	log.Fatal(server.ListenAndServe())

}
