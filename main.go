package main

import (
	"flag"
	"net/http"
	"time"
	"forum/function"
	
)

func main() {
	addr := flag.String("addr", ":8080", "http network address")
	flag.Parse()
	logInf, logErr := function.Logger()
	srv := &http.Server{
		Addr:         *addr,
		Handler:      function.RegisterForumRoutes(),
		ErrorLog:     logErr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go function.DeleteExpiredSessions()
	logInf.Printf("The server is running on port %s", *addr)
	logErr.Fatal(srv.ListenAndServe())
}
