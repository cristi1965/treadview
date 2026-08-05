package main

import (
	"fmt"
	"os"
	"trading-agents/internal/database"
)

func main() {
	_ = os.Setenv("EDGAR_ONLY_SLUGS", "warren-buffett")
	tmp, _ := os.MkdirTemp("", "sec-smoke-*")
	fmt.Println("tmp", tmp)
	database.InitDB(tmp)
	database.RunCapitolTradesSync()
	var total, senate int64
	database.DB.Table("congress_trades").Count(&total)
	database.DB.Table("congress_trades").Where("title = ?", "Senator").Count(&senate)
	fmt.Printf("congress_total=%d senate=%d\n", total, senate)
	type crow struct{ Politician, Symbol, Type, Date, Title string }
	var rows []crow
	database.DB.Raw(`select politician, symbol, type, date, title from congress_trades where title='Senator' order by date desc limit 8`).Scan(&rows)
	for _, r := range rows {
		fmt.Printf("SEN %s %s %s %s\n", r.Date, r.Politician, r.Symbol, r.Type)
	}
}
