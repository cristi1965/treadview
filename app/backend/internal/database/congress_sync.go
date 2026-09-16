package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"trading-agents/internal/models"

	"gorm.io/gorm"
)

type congressLiveFile struct {
	UpdatedAt     string `json:"updatedAt"`
	Source        string `json:"source"`
	FilingsParsed int    `json:"filingsParsed"`
	Count         int    `json:"count"`
	Trades        []struct {
		Politician string `json:"politician"`
		Title      string `json:"title"`
		Party      string `json:"party"`
		District   string `json:"district"`
		Symbol     string `json:"symbol"`
		Type       string `json:"type"`
		Amount     string `json:"amount"`
		Date       string `json:"date"`
		Source     string `json:"source"`
		FilingDate string `json:"filingDate"`
		SourceURL  string `json:"sourceURL"`
		FilingID   string `json:"filingId"`
	} `json:"trades"`
}

// RunCapitolTradesSync refreshes congress trades from:
//  1. House Clerk PTR PDFs (official)
//  2. Public secondary aggregators for Senate/House overlay (Quiver public page,
//     optional QUIVER_API_KEY / FMP_API_KEY, GitHub historical fallback)
//  3. members.json seed gap-fill
//
// Senate eFD is blocked (403) from many datacenter IPs — secondary sources cover it.
func RunCapitolTradesSync() {
	log.Println("[CapTrades] Syncing House PTR + secondary aggregators...")

	SyncMutex.Lock()
	WhalesSyncStatus = "House PTR / secondary congress disclosures..."
	SyncMutex.Unlock()

	livePath, err := runHousePTRScript()
	houseCount := 0
	if err != nil {
		log.Printf("[CapTrades] House PTR script failed: %v", err)
	} else {
		houseCount = loadCongressLiveJSON(livePath)
	}

	// Never replace a committed disclosure snapshot with seed data after a live
	// fetch failure. The bootstrap path already seeds an empty database.
	if houseCount == 0 {
		log.Println("[CapTrades] live PTR empty — retaining committed congress snapshot")
	} else {
		seedFill := fillMissingMembersFromSeed()
		log.Printf("[CapTrades] seed gap-fill rows=%d", seedFill)
	}

	secondaryCount := 0
	if secPath, err := runCongressSecondaryScript(); err != nil {
		log.Printf("[CapTrades] secondary aggregator failed: %v", err)
	} else {
		secondaryCount = mergeCongressSecondaryJSON(secPath)
	}

	senateSeed := refreshSenateTradesFromSeed()
	log.Printf("[CapTrades] Done: house=%d secondary=%d senate_seed=%d", houseCount, secondaryCount, senateSeed)
}

func pythonBin() string {
	if v := strings.TrimSpace(os.Getenv("PYTHON_BIN")); v != "" {
		return v
	}
	candidates := []string{
		filepath.Join(".venv", "bin", "python3"),
		filepath.Join("app", "backend", ".venv", "bin", "python3"),
		"python3",
		"/usr/local/bin/python3",
		"/usr/bin/python3",
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append([]string{
			filepath.Join(cwd, ".venv", "bin", "python3"),
			filepath.Join(cwd, "app", "backend", ".venv", "bin", "python3"),
		}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err != nil || st.IsDir() {
			// may be bare "python3" on PATH
			if !strings.Contains(c, string(os.PathSeparator)) && c != "python3" {
				continue
			}
		}
		cmd := exec.Command(c, "-c", "import pypdf")
		if err := cmd.Run(); err == nil {
			return c
		}
	}
	return "python3"
}

func runHousePTRScript() (string, error) {
	script := findHousePTRScript()
	if script == "" {
		return "", fmt.Errorf("house_ptr_sync.py not found")
	}
	outPath := filepath.Join("data", "congress-live.json")
	if _, err := os.Stat("data"); err != nil {
		outPath = filepath.Join("app", "backend", "data", "congress-live.json")
	}
	_ = os.MkdirAll(filepath.Dir(outPath), 0o755)

	py := pythonBin()
	cmd := exec.Command(py, script, "--years", "2025,2026", "--limit", "100", "--days", "200", "--out", outPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", py, err)
	}
	return outPath, nil
}

func findHousePTRScript() string {
	return findScript("house_ptr_sync.py")
}

func findScript(name string) string {
	candidates := []string{
		filepath.Join("scripts", name),
		filepath.Join("app", "backend", "scripts", name),
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "scripts", name),
			filepath.Join(cwd, "app", "backend", "scripts", name),
		)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func runCongressSecondaryScript() (string, error) {
	script := findScript("congress_secondary_sync.py")
	if script == "" {
		return "", fmt.Errorf("congress_secondary_sync.py not found")
	}
	outPath := filepath.Join("data", "congress-secondary.json")
	if _, err := os.Stat("data"); err != nil {
		outPath = filepath.Join("app", "backend", "data", "congress-secondary.json")
	}
	_ = os.MkdirAll(filepath.Dir(outPath), 0o755)
	py := pythonBin()
	cmd := exec.Command(py, script, "--out", outPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", py, err)
	}
	return outPath, nil
}

func mergeCongressSecondaryJSON(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[CapTrades] read secondary %s: %v", path, err)
		return 0
	}
	var file congressLiveFile
	if err := json.Unmarshal(raw, &file); err != nil {
		log.Printf("[CapTrades] parse secondary %s: %v", path, err)
		return 0
	}

	// Build dedupe set from existing rows
	type key struct{ p, s, t, d, a string }
	seen := map[key]bool{}
	var existing []models.CongressTrade
	DB.Find(&existing)
	for _, row := range existing {
		seen[key{strings.ToLower(row.Politician), row.Symbol, row.Type, row.Date, row.Amount}] = true
	}

	n := 0
	for _, t := range file.Trades {
		sym := strings.ToUpper(strings.TrimSpace(t.Symbol))
		if sym == "" || len(sym) > 12 {
			continue
		}
		side := strings.ToUpper(t.Type)
		if side != "BUY" && side != "SELL" {
			if strings.HasPrefix(side, "S") {
				side = "SELL"
			} else {
				side = "BUY"
			}
		}
		title := t.Title
		if title == "" {
			title = "Representative"
		}
		party := t.Party
		if party == "" || party == "Unknown" {
			party = lookupParty(memberPartyIndex(), t.Politician)
		}
		amount := strings.ReplaceAll(t.Amount, "\n", " ")
		k := key{strings.ToLower(t.Politician), sym, side, t.Date, amount}
		if seen[k] {
			continue
		}
		seen[k] = true
		row := models.CongressTrade{
			Politician: t.Politician,
			Title:      title,
			Party:      party,
			District:   t.District,
			Symbol:     sym,
			Type:       side,
			Amount:     amount,
			Date:       t.Date,
			Source:     congressSourceOrUnknown(t.Source, file.Source),
			FilingDate: congressFieldOrUnknown(t.FilingDate),
			SourceURL:  congressFieldOrUnknown(t.SourceURL),
			FilingID:   congressFieldOrUnknown(t.FilingID),
		}
		if err := DB.Create(&row).Error; err == nil {
			n++
		}
	}
	log.Printf("[CapTrades] merged %d secondary trades from %s (%s)", n, path, file.UpdatedAt)
	return n
}

func loadCongressLiveJSON(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[CapTrades] read %s: %v", path, err)
		return 0
	}
	var file congressLiveFile
	if err := json.Unmarshal(raw, &file); err != nil {
		log.Printf("[CapTrades] parse %s: %v", path, err)
		return 0
	}

	partyByName := memberPartyIndex()
	rows := make([]models.CongressTrade, 0, len(file.Trades))
	for _, t := range file.Trades {
		sym := strings.ToUpper(strings.TrimSpace(t.Symbol))
		if sym == "" || len(sym) > 12 || strings.TrimSpace(t.Politician) == "" || strings.TrimSpace(t.Date) == "" {
			continue
		}
		side := strings.ToUpper(t.Type)
		if side != "BUY" && side != "SELL" {
			if strings.HasPrefix(side, "S") {
				side = "SELL"
			} else {
				side = "BUY"
			}
		}
		party := t.Party
		if party == "" {
			party = lookupParty(partyByName, t.Politician)
		}
		rows = append(rows, models.CongressTrade{
			Politician: t.Politician,
			Title:      "Representative",
			Party:      party,
			District:   t.District,
			Symbol:     sym,
			Type:       side,
			Amount:     strings.ReplaceAll(t.Amount, "\n", " "),
			Date:       t.Date,
			Source:     congressSourceOrUnknown(t.Source, file.Source),
			FilingDate: congressFieldOrUnknown(t.FilingDate),
			SourceURL:  congressFieldOrUnknown(t.SourceURL),
			FilingID:   congressFieldOrUnknown(t.FilingID),
		})
	}
	if len(rows) == 0 {
		log.Printf("[CapTrades] refusing to replace congress snapshot with zero valid rows from %s", path)
		return 0
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM congress_trades").Error; err != nil {
			return err
		}
		return tx.Create(&rows).Error
	}); err != nil {
		log.Printf("[CapTrades] atomic congress replacement failed for %s: %v", path, err)
		return 0
	}
	log.Printf("[CapTrades] loaded %d house PTR trades from %s (%s)", len(rows), path, file.UpdatedAt)
	return len(rows)
}

func memberPartyIndex() map[string]string {
	out := map[string]string{}
	var members []RawMember
	_ = json.Unmarshal(membersJSON, &members)
	for _, m := range members {
		party := "Democratic"
		if m.Party == "R" {
			party = "Republican"
		}
		out[strings.ToLower(m.Name)] = party
		parts := strings.Fields(m.Name)
		if len(parts) > 0 {
			out[strings.ToLower(parts[len(parts)-1])] = party
		}
	}
	return out
}

func lookupParty(idx map[string]string, name string) string {
	if p, ok := idx[strings.ToLower(name)]; ok && p != "" {
		return p
	}
	parts := strings.Fields(name)
	if len(parts) > 0 {
		if p := idx[strings.ToLower(parts[len(parts)-1])]; p != "" {
			return p
		}
	}
	return "Unknown"
}

func reloadAllMembersSeed() int {
	DB.Exec("DELETE FROM congress_trades")
	return insertMembersSeed(false)
}

// fillMissingMembersFromSeed adds seed trades for politicians with no live PTR rows.
func fillMissingMembersFromSeed() int {
	return insertMembersSeed(true)
}

func insertMembersSeed(onlyMissing bool) int {
	var members []RawMember
	if err := json.Unmarshal(membersJSON, &members); err != nil {
		return 0
	}
	covered := map[string]bool{}
	if onlyMissing {
		var names []string
		DB.Model(&models.CongressTrade{}).Distinct("politician").Pluck("politician", &names)
		for _, n := range names {
			covered[strings.ToLower(n)] = true
			parts := strings.Fields(n)
			if len(parts) > 0 {
				covered[strings.ToLower(parts[len(parts)-1])] = true
			}
		}
	}

	n := 0
	for _, member := range members {
		if onlyMissing {
			last := ""
			parts := strings.Fields(member.Name)
			if len(parts) > 0 {
				last = strings.ToLower(parts[len(parts)-1])
			}
			if covered[strings.ToLower(member.Name)] || (last != "" && covered[last]) {
				continue
			}
		}
		title := "Representative"
		if looksLikeSenator(member) {
			title = "Senator"
		}
		party := "Democratic"
		if member.Party == "R" {
			party = "Republican"
		}
		for _, trade := range member.Trades {
			side := "BUY"
			if strings.ToLower(trade.Side) == "sell" {
				side = "SELL"
			}
			amount := trade.Size
			if amount == "" {
				amount = "$1,000 - $15,000"
			}
			amount = strings.ReplaceAll(amount, "$$", "$")
			row := models.CongressTrade{
				Politician: member.Name,
				Title:      title,
				Party:      party,
				District:   member.State + " " + member.District,
				Symbol:     trade.Ticker,
				Type:       side,
				Amount:     amount,
				Date:       trade.Date,
				Source:     congressFieldOrUnknown(trade.Source),
				FilingDate: congressFieldOrUnknown(trade.FilingDate),
				SourceURL:  congressFieldOrUnknown(trade.SourceURL),
				FilingID:   congressFieldOrUnknown(trade.FilingID),
			}
			if err := DB.Create(&row).Error; err == nil {
				n++
			}
		}
	}
	return n
}

func refreshSenateTradesFromSeed() int {
	// Current members.json is House-only; keep helper for future Senate rows.
	var members []RawMember
	if err := json.Unmarshal(membersJSON, &members); err != nil {
		return 0
	}
	n := 0
	for _, member := range members {
		if !looksLikeSenator(member) {
			continue
		}
		party := "Democratic"
		if member.Party == "R" {
			party = "Republican"
		}
		for _, trade := range member.Trades {
			side := "BUY"
			if strings.ToLower(trade.Side) == "sell" {
				side = "SELL"
			}
			amount := strings.ReplaceAll(trade.Size, "$$", "$")
			if amount == "" {
				amount = "$1,000 - $15,000"
			}
			row := models.CongressTrade{
				Politician: member.Name,
				Title:      "Senator",
				Party:      party,
				District:   member.State + " " + member.District,
				Symbol:     trade.Ticker,
				Type:       side,
				Amount:     amount,
				Date:       trade.Date,
				Source:     congressFieldOrUnknown(trade.Source),
				FilingDate: congressFieldOrUnknown(trade.FilingDate),
				SourceURL:  congressFieldOrUnknown(trade.SourceURL),
				FilingID:   congressFieldOrUnknown(trade.FilingID),
			}
			if err := DB.Create(&row).Error; err == nil {
				n++
			}
		}
	}
	return n
}

func looksLikeSenator(m RawMember) bool {
	d := strings.ToUpper(strings.TrimSpace(m.District))
	if strings.Contains(strings.ToLower(m.Name), "sen.") {
		return true
	}
	hasDigit := false
	for _, r := range d {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}
	return d != "" && !hasDigit
}
