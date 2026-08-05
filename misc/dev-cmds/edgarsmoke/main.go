package main

import (
	"fmt"
	"os"
	"path/filepath"
	"trading-agents/internal/database"
)

func main() {
	_ = os.Setenv("EDGAR_ONLY_SLUGS", "warren-buffett,bill-ackman,seth-klarman")
	tmp, err := os.MkdirTemp("", "edgar-smoke-*")
	if err != nil {
		panic(err)
	}
	fmt.Println("temp_db_dir", tmp)
	database.InitDB(tmp)
	database.RunEDGARSync()
	var n int64
	database.DB.Raw("select count(*) from holdings h join gurus g on g.id=h.guru_id where g.slug in ('warren-buffett','bill-ackman','seth-klarman')").Scan(&n)
	fmt.Println("holdings_for_sample", n)
	type row struct {
		Symbol string
		Name   string
		Change string
		Weight float64
		Value  string
	}
	var rows []row
	database.DB.Raw(`select h.stock_symbol as symbol, h.stock_name as name, h.change, h.weight, h.value from holdings h join gurus g on g.id=h.guru_id where g.slug='warren-buffett' order by h.weight desc limit 8`).Scan(&rows)
	for _, r := range rows {
		fmt.Printf("%s\t%s\t%.2f\t%s\t%s\n", r.Symbol, r.Value, r.Weight, r.Change, r.Name)
	}
	_ = filepath.Join(tmp, "trades.db")
}
