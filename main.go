package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

//go:embed static/*
var content embed.FS

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := flag.Uint("port", 3000, "Listen port")
	addr := flag.String("addr", "", "Bind address")
	certFile := flag.String("cert", "", "Server certificate for HTTPS")
	keyFile := flag.String("key", "", "Server key for HTTPS")
	flag.Parse()
	bindAddr := fmt.Sprintf("%s:%d", *addr, *port)

	subFS, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatalf("failed to create sub FS: %v", err)
	}
	fileServer := http.FileServer(http.FS(subFS))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	})
	loggedMux := loggingMiddleware(mux)

	server := &http.Server{
		Addr:    bindAddr,
		Handler: loggedMux,
	}

	go func() {
		tlsEnabled := *certFile != "" && *keyFile != ""
		log.Printf("listening on %s (TLS=%v)\n", bindAddr, tlsEnabled)
		var err error
		if tlsEnabled {
			err = server.ListenAndServeTLS(*certFile, *keyFile)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %v", err)
	}
	log.Println("server stopped")
}
