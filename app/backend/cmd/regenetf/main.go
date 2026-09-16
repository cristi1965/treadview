// regenetf refreshes a bounded, auditable ETF scope and promotes it only when complete.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"trading-agents/internal/etfrefresh"
)

func main() {
	limit := flag.Int("limit", etfrefresh.DefaultScopeLimit, "maximum number of top-AUM ETFs to refresh")
	root := flag.String("root", "", "backend root containing data/")
	flag.Parse()
	if *root == "" {
		*root = findBackendRoot()
	}
	service := etfrefresh.NewService(etfrefresh.Config{BackendRoot: *root, ScopeLimit: *limit})
	if err := service.RunOnce(context.Background()); err != nil {
		log.Fatal(err)
	}
	log.Printf("ETF refresh complete; current last-good written under %s", filepath.Join(*root, "data"))
}

func findBackendRoot() string {
	cwd, _ := os.Getwd()
	for _, candidate := range []string{cwd, filepath.Join(cwd, "app", "backend")} {
		if _, err := os.Stat(filepath.Join(candidate, "go.mod")); err == nil {
			return candidate
		}
	}
	return cwd
}
