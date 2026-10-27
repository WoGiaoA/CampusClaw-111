// Local same-origin development entry only. Compose uses Nginx instead.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func main() {
	target := os.Getenv("DEV_API_URL")
	if target == "" {
		target = "http://127.0.0.1:18081"
	}
	upstream, e := url.Parse(target)
	if e != nil {
		log.Fatal(e)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	static := http.FileServer(http.Dir("web"))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" {
			proxy.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/login" || r.URL.Path == "/materials" || r.URL.Path == "/register" || r.URL.Path == "/profile" {
			http.ServeFile(w, r, "web/index.html")
			return
		}
		static.ServeHTTP(w, r)
	})
	log.Println("Development entry http://127.0.0.1:8080 (not Nginx)")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", handler))
}
