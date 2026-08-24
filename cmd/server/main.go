// The app: an HTTP signup service. This file is wiring only — it holds no
// logic. Flow: plan (core) -> gate (core) -> run (shell).
package main

import (
	"fmt"
	"net/http"
	"os"

	"agentic-sandbox/internal/core"
	"agentic-sandbox/internal/shell"
)

func main() {
	http.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		email := r.URL.Query().Get("email")

		effects, err := core.Signup(email) // 1. PLAN  (pure)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		vetted, err := shell.Gate(effects) // 2. GATE  — required to get a Vetted
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		shell.Run(vetted) // 3. RUN   — only accepts Vetted; can't skip the gate

		_, _ = fmt.Fprintf(w, "ok: %d effects\n", len(effects))
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	fmt.Println("listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "server stopped:", err)
		os.Exit(1)
	}
}
