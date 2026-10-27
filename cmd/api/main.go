package main

import (
	"campusclaw/internal/campus"
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 3 * time.Second}
		response, e := client.Get("http://127.0.0.1:8080/health")
		if e != nil {
			os.Exit(1)
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	c, e := campus.ConfigFromEnv()
	if e != nil {
		log.Fatal(e)
	}
	reset := len(os.Args) > 1 && os.Args[1] == "reset-demo-passwords"
	if reset {
		if e = campus.ValidateDemoReset(c, os.Getenv("MYSQL_ADDR")); e != nil {
			log.Fatal(e)
		}
	}
	db, e := campus.OpenDB()
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for db.PingContext(ctx) != nil {
		select {
		case <-ctx.Done():
			log.Fatal("database unavailable")
		case <-time.After(time.Second):
		}
	}
	if e = campus.Migrate(ctx, db); e != nil {
		log.Fatal("schema initialization failed")
	}
	s, e := campus.New(db, c)
	if e != nil {
		log.Fatal(e)
	}
	if reset {
		if e = s.ResetDemoPasswords(ctx, os.Getenv("MYSQL_ADDR")); e != nil {
			log.Fatal(e)
		}
		log.Print("Four preset credentials reset; all their sessions revoked. First-login password change required.")
		return
	}
	if e = s.Seed(ctx); e != nil {
		log.Fatal(e)
	}
	if e = s.Reconcile(ctx); e != nil {
		log.Fatal("storage reconciliation failed")
	}
	address := os.Getenv("API_ADDR")
	if address == "" {
		address = ":8080"
	}
	log.Printf("CampusClaw API listening on %s", address)
	server := http.Server{Addr: address, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(server.ListenAndServe())
}
