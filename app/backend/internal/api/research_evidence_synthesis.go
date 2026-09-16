package api

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"trading-agents/internal/agents"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/scoring"
)

func synthesizeEvidenceOnlyDossier(facts []agents.EvidenceOnlyFact, byName map[string]dataflows.ToolInvocation, researchContext *agents.ResearchContext, ticker string) (*agents.EvidenceOnlyDossier, error) {
	fundamentals, err := dataflows.ParseFundamentalEvidence(byName["get_fundamentals"].RawPayload)
	if err != nil {
		return nil, fmt.Errorf("structured fundamentals unavailable: %w", err)
	}
	historical, err := dataflows.ParseHistoricalEvidence(byName["get_historical_evidence"].RawPayload)
	if err != nil {
		return nil, fmt.Errorf("structured historical evidence unavailable: %w", err)
	}
	news, err := dataflows.ParseNewsEvidenceReview(byName["get_news"].RawPayload)
	if err != nil {
		return nil, fmt.Errorf("structured news review unavailable: %w", err)
	}

	periods, err := evidenceFinancialPeriods(fundamentals.Periods)
	if err != nil {
		return nil, err
	}
	derivationInputs, err := evidenceFinancialDerivationInputs(fundamentals.DerivationInputs)
	if err != nil {
		return nil, err
	}
	var researchPrice *float64
	if len(historical.Observations) > 0 {
		latest := historical.Observations[len(historical.Observations)-1]
		value := latest.Close
		researchPrice = &value
	}
	calculations := financialEvidenceCalculations(periods, derivationInputs, researchPrice)
	calculations = append(calculations, scenarioEvidenceCalculations(researchContext, calculations, periods, researchPrice)...)
	if len(historical.Observations) > 0 {
		latest := historical.Observations[len(historical.Observations)-1]
		value := latest.Close
		calculations = append(calculations, agents.EvidenceOnlyCalculation{
			Name: "as_of_research_price:" + latest.Date, Formula: "latest completed dated Nasdaq historical close",
			Inputs: map[string]float64{"close": latest.Close}, Value: &value, Unit: "currency units",
			Status: "computed", EvidenceIDs: []string{"tool-historical"},
		})
	}
	for _, calculation := range dataflows.CalculateHistoricalEvidence(historical) {
		calculations = append(calculations, agents.EvidenceOnlyCalculation{
			Name: calculation.Name, Formula: calculation.Formula, Inputs: calculation.Inputs,
			Value: calculation.Value, Unit: calculation.Unit, Status: calculation.Status,
			Reason: calculation.Reason, EvidenceIDs: []string{"tool-historical"},
		})
	}
	historicalReview := &agents.EvidenceHistoricalReview{
		SampleCount: historical.SampleCount, MinimumSessions: historical.MinimumSessions,
		TimeGranularity: historical.TimeGranularity, Status: historical.Status,
		Observations: make([]agents.EvidenceHistoricalObservation, 0, len(historical.Observations)),
	}
	for _, row := range historical.Observations {
		historicalReview.Observations = append(historicalReview.Observations, agents.EvidenceHistoricalObservation{
			Date: row.Date, Open: row.Open, High: row.High, Low: row.Low, Close: row.Close, Volume: row.Volume,
		})
	}
	pricePoints := make([]scoring.PriceObservation, 0, len(historical.Observations))
	for _, row := range historical.Observations {
		pricePoints = append(pricePoints, scoring.PriceObservation{Date: row.Date, Close: row.Close})
	}
	modelReport := scoring.EvaluateOHLCVMomentum(pricePoints)
	marketSubmodel := &agents.EvidenceMarketSubmodel{
		Name: modelReport.Name, Scope: modelReport.Scope, Status: modelReport.Status, Result: modelReport.Result,
		DatasetHash: modelReport.DatasetHash, Feature: modelReport.Feature, Label: modelReport.Label,
		Benchmark: modelReport.Benchmark, Cutoff: modelReport.Cutoff, TrainSamples: modelReport.TrainSamples,
		TestSamples: modelReport.TestSamples, Baseline: modelReport.Baseline, Reasons: append([]string(nil), modelReport.Reasons...),
		FiveFactorUse:  modelReport.FiveFactorUse,
		Classification: "experimental_negative_control", ProductSignal: false,
		Metric: agents.EvidenceMarketModelMetric{Name: modelReport.Metric.Name, SampleCount: modelReport.Metric.SampleCount,
			IC: modelReport.Metric.IC, IC95: modelReport.Metric.IC95, DirectionAccuracy: modelReport.Metric.DirectionAccuracy, Accuracy95: modelReport.Metric.Accuracy95},
	}
	if len(historical.Observations) > 0 {
		historicalReview.StartDate = historical.Observations[0].Date
		historicalReview.EndDate = historical.Observations[len(historical.Observations)-1].Date
	}
	newsReview := &agents.EvidenceNewsReview{
		InputCount: news.InputCount, IncludedCount: news.IncludedCount,
		ExcludedLowRelevance: news.ExcludedLowRelevance, DuplicatesRemoved: news.DuplicatesRemoved,
		Rules: append([]string(nil), news.Rules...), Items: make([]agents.EvidenceNewsItem, 0, len(news.Items)),
	}
	for _, item := range news.Items {
		newsReview.Items = append(newsReview.Items, agents.EvidenceNewsItem{
			Title: item.Title, URL: item.URL, PublishedAt: item.PublishedAt, Source: item.Source,
			SourceTier: item.SourceTier, ContentType: item.ContentType, Theme: item.Theme, RelevanceRule: item.RelevanceRule,
		})
	}

	decision := agents.ResearchDecisionObserve
	gaps := []string{
		"No LLM debate, forecast, valuation target, trade instruction, or validation of the five-factor panel was performed.",
		evidenceOnlyQuoteCrossCheckGap(byName),
	}
	contextAssessment := assessResearchContext(researchContext, ticker, calculations, periods, researchPrice)
	gaps = append(gaps, researchContextGaps(researchContext, contextAssessment)...)
	quarterCount, annualCount := 0, 0
	for _, period := range periods {
		if !validEvidenceFinancialField(period, "totalRevenue") {
			continue
		}
		switch financialPeriodFrequency(period) {
		case "quarterly":
			quarterCount++
		case "annual":
			annualCount++
		}
	}
	if quarterCount < 5 {
		gaps = append(gaps, fmt.Sprintf("Comparable quarterly history is incomplete: available=%d target=5; missing periods are not synthesized.", quarterCount))
	}
	if annualCount < 3 {
		gaps = append(gaps, fmt.Sprintf("Comparable annual history is incomplete: available=%d target=3; missing periods are not synthesized.", annualCount))
	}
	if quarterCount+annualCount < len(periods) {
		gaps = append(gaps, fmt.Sprintf("Financial periods without complete field-level SEC duration provenance remain null/unknown: %d.", len(periods)-quarterCount-annualCount))
	}
	if !calculationIsComputed(calculations, "market_cap") {
		gaps = append(gaps, "Filing-bound shares or the PIT historical close was unavailable; market capitalization, P/S, and P/B remain unavailable.")
	} else {
		if !calculationIsComputed(calculations, "price_to_sales") {
			gaps = append(gaps, "TTM revenue was unavailable; P/S remains unavailable.")
		}
		if !calculationIsComputed(calculations, "price_to_book") {
			gaps = append(gaps, "Filing-bound stockholders' equity was unavailable; P/B remains unavailable.")
		}
	}
	if historical.Status != "complete" || historical.SampleCount < dataflows.HistoricalEvidenceMinimumSessions {
		decision = agents.ResearchDecisionAbstain
		gaps = append(gaps, fmt.Sprintf("Historical sample is incomplete: %d valid sessions; at least %d are required.", historical.SampleCount, dataflows.HistoricalEvidenceMinimumSessions))
	}
	return &agents.EvidenceOnlyDossier{
		MethodVersion: agents.EvidenceOnlyMethodVersion,
		Label:         "Evidence-only research dossier; no LLM and not a 10-Agent analysis",
		Disclaimer:    "For auditable research review only; not investment advice and not an executable trade instruction.",
		Facts:         facts, Calculations: calculations, FinancialPeriods: periods,
		FinancialDerivationInputs: derivationInputs,
		Historical:                historicalReview, NewsReview: newsReview, MarketSubmodel: marketSubmodel,
		ResearchContext:       cloneResearchContext(researchContext),
		ContextAssessment:     contextAssessment,
		FinancialTrendSummary: financialTrendSummary(periods, calculations),
		RiskDiagnostics:       riskDiagnostics(calculations),
		Risks: []string{
			"Market and news observations can become stale after the stated provider data times.",
			"The optional quote cross-check may share Nasdaq lineage with the primary historical source and is never represented as independent confirmation.",
			"Filing values may require issuer-specific accounting interpretation; missing fields remain null/unknown and are not estimated.",
			"News source tiers and topic labels are deterministic review aids, not truth or sentiment scores.",
			"The retained momentum experiment is an experimental negative control, not a product signal; a failed result is not hidden or converted into advice.",
		},
		Gaps: gaps,
		ConclusionBasis: []string{
			"The latest completed close in dated Nasdaq historical OHLCV is the sole primary research price and passed the trade-date PIT gate.",
			"A quote, when captured, is an optional disclosed cross-check only; it neither blocks the dossier nor establishes independent confirmation.",
			"All displayed transforms use the listed formulas, exact numeric inputs, and linked evidence IDs; unknown values are not extrapolated.",
			"Optional mandate, holdings, liquidity, tax, and risk-budget context is stored in the audit input hash; omitted context is reported as a gap and never inferred.",
			"User-supplied scenario assumptions are deterministic sensitivities, not forecasts or target prices; risk discussion uses observed volatility, drawdown, and ADV rather than the experimental momentum result.",
			"The conclusion is limited to evidence review and does not encode a buy, sell, hold, sizing, or execution recommendation.",
		},
		Conclusion: decision,
	}, nil
}

func cloneResearchContext(input *agents.ResearchContext) *agents.ResearchContext {
	if input == nil {
		return nil
	}
	copy := *input
	copy.Holdings = append([]agents.ResearchHolding(nil), input.Holdings...)
	if input.IssuerCapPct != nil {
		value := *input.IssuerCapPct
		copy.IssuerCapPct = &value
	}
	if input.Scenario != nil {
		scenario := *input.Scenario
		if input.Scenario.RevenueGrowthPct != nil {
			value := *input.Scenario.RevenueGrowthPct
			scenario.RevenueGrowthPct = &value
		}
		if input.Scenario.PSMultiple != nil {
			value := *input.Scenario.PSMultiple
			scenario.PSMultiple = &value
		}
		copy.Scenario = &scenario
	}
	return &copy
}

func researchContextGaps(input *agents.ResearchContext, assessment *agents.EvidenceContextAssessment) []string {
	gaps := make([]string, 0, 7)
	if input == nil || strings.TrimSpace(input.Mandate) == "" {
		gaps = append(gaps, "Research mandate was not supplied; no mandate is inferred.")
	}
	if input == nil || len(input.Holdings) == 0 {
		gaps = append(gaps, "Holdings were not supplied; no position exposure is inferred.")
	}
	if input == nil || strings.TrimSpace(input.Liquidity) == "" {
		gaps = append(gaps, "Liquidity requirements were not supplied; no liquidity constraint is inferred.")
	}
	if input == nil || strings.TrimSpace(input.Tax) == "" {
		gaps = append(gaps, "Tax context was not supplied; no tax assumption is inferred.")
	}
	if input == nil || strings.TrimSpace(input.RiskBudget) == "" {
		gaps = append(gaps, "Risk budget was not supplied; no risk capacity is inferred.")
	}
	if input == nil || input.IssuerCapPct == nil {
		gaps = append(gaps, "Structured issuer_cap_pct was not supplied; free-text risk budget is not parsed into a limit.")
	}
	if assessment != nil && assessment.IssuerCap.Status != "provided" {
		gaps = append(gaps, "Ticker holding headroom is unknown: "+assessment.IssuerCap.Reason)
	}
	return gaps
}

func assessResearchContext(input *agents.ResearchContext, ticker string, calculations []agents.EvidenceOnlyCalculation, periods []agents.EvidenceFinancialPeriod, researchPrice *float64) *agents.EvidenceContextAssessment {
	unknown := func(reason string) agents.EvidenceContextInput {
		return agents.EvidenceContextInput{Status: "unknown", Reason: reason}
	}
	provided := func(value string) agents.EvidenceContextInput {
		return agents.EvidenceContextInput{Status: "provided", Value: value}
	}
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	assessment := &agents.EvidenceContextAssessment{
		Inputs: agents.EvidenceContextInputs{
			Mandate: unknown("not supplied"), Holdings: unknown("not supplied"), Liquidity: unknown("not supplied"),
			Tax: unknown("not supplied"), RiskBudget: unknown("not supplied"),
		},
		Holding:   agents.EvidenceContextHolding{Status: "unknown", Symbol: ticker, Reason: "current ticker is absent from supplied holdings"},
		IssuerCap: agents.EvidenceContextIssuerCap{Status: "unknown", Reason: "issuer_cap_pct and ticker weight are required"},
		Scenario:  agents.EvidenceContextScenario{Status: "unknown", Reason: "optional scenario assumptions not supplied", EvidenceIDs: []string{}, Disclosure: "User-supplied sensitivity assumptions only; not a forecast, target price, or investment advice."},
	}
	if input == nil {
		assessment.Limitations = []string{"No research context was supplied; no portfolio constraint or scenario is inferred."}
		return assessment
	}
	if strings.TrimSpace(input.Mandate) != "" {
		assessment.Inputs.Mandate = provided(input.Mandate)
	}
	if len(input.Holdings) > 0 {
		assessment.Inputs.Holdings = provided(fmt.Sprintf("%d supplied holding entries", len(input.Holdings)))
	}
	if strings.TrimSpace(input.Liquidity) != "" {
		assessment.Inputs.Liquidity = provided(input.Liquidity)
	}
	if strings.TrimSpace(input.Tax) != "" {
		assessment.Inputs.Tax = provided(input.Tax)
	}
	if strings.TrimSpace(input.RiskBudget) != "" {
		assessment.Inputs.RiskBudget = provided(input.RiskBudget)
	}
	capValid := input.IssuerCapPct != nil && finiteRange(*input.IssuerCapPct, 0, 100)
	if capValid {
		assessment.IssuerCap.CapPct = floatPointer(*input.IssuerCapPct)
	} else if input.IssuerCapPct != nil {
		assessment.IssuerCap.Reason = "issuer_cap_pct must be a finite percentage between 0 and 100"
	}
	matches := make([]agents.ResearchHolding, 0, 1)
	for _, holding := range input.Holdings {
		if strings.EqualFold(strings.TrimSpace(holding.Symbol), ticker) {
			matches = append(matches, holding)
		}
	}
	if len(matches) != 1 {
		if len(matches) == 0 {
			assessment.Holding.Reason = "current ticker is absent from supplied holdings"
		} else {
			assessment.Holding.Reason = "multiple entries for current ticker make weight ambiguous"
		}
	} else {
		holding := matches[0]
		assessment.Holding.Quantity = holding.Quantity
		assessment.Holding.WeightPct = holding.WeightPct
		assessment.Holding.EvidenceIDs = []string{"input-research-context"}
		if holding.WeightPct != nil && finiteRange(*holding.WeightPct, 0, 100) {
			assessment.Holding.Status = "provided"
			assessment.Holding.Reason = ""
			if capValid {
				headroom := *input.IssuerCapPct - *holding.WeightPct
				within := headroom >= 0
				assessment.IssuerCap.Status = "provided"
				assessment.IssuerCap.Reason = ""
				assessment.IssuerCap.HeadroomPct = &headroom
				assessment.IssuerCap.WithinCap = &within
				assessment.IssuerCap.EvidenceIDs = []string{"input-research-context"}
			} else if input.IssuerCapPct == nil {
				assessment.IssuerCap.Reason = "issuer_cap_pct is missing; free-text risk budget is not parsed"
			}
		} else {
			assessment.Holding.Reason = "current ticker weight_pct is missing or invalid"
		}
	}
	assessment.Scenario = contextScenarioAssessment(input, calculations, periods, researchPrice)
	for _, limitation := range []struct{ status, message string }{
		{assessment.Holding.Status, assessment.Holding.Reason},
		{assessment.IssuerCap.Status, assessment.IssuerCap.Reason},
		{assessment.Scenario.Status, assessment.Scenario.Reason},
	} {
		if limitation.status != "provided" && limitation.message != "" {
			assessment.Limitations = append(assessment.Limitations, limitation.message)
		}
	}
	return assessment
}

func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func finiteRange(value, minimum, maximum float64) bool {
	return isFinite(value) && value >= minimum && value <= maximum
}

func floatPointer(value float64) *float64 { return &value }

func evidenceFinancialPeriods(input []dataflows.FinancialPeriod) ([]agents.EvidenceFinancialPeriod, error) {
	return convertEvidenceFinancialPeriods(input, 2)
}

func evidenceFinancialDerivationInputs(input []dataflows.FinancialPeriod) ([]agents.EvidenceFinancialPeriod, error) {
	return convertEvidenceFinancialPeriods(input, 0)
}

func convertEvidenceFinancialPeriods(input []dataflows.FinancialPeriod, minimum int) ([]agents.EvidenceFinancialPeriod, error) {
	periods := append([]dataflows.FinancialPeriod(nil), input...)
	sort.Slice(periods, func(i, j int) bool { return periods[i].PeriodEnd > periods[j].PeriodEnd })
	out := make([]agents.EvidenceFinancialPeriod, 0, len(periods))
	for _, period := range periods {
		if _, err := time.Parse("2006-01-02", period.PeriodEnd); err != nil || strings.TrimSpace(period.FilingDate) == "" || strings.TrimSpace(period.Accession) == "" || strings.TrimSpace(period.SourceURL) == "" {
			return nil, fmt.Errorf("financial period %q lacks verifiable period, filing, accession, or source URL metadata", period.FiscalPeriod)
		}
		converted := agents.EvidenceFinancialPeriod{
			FiscalPeriod: stableFinancialPeriodIdentity(period.Frequency, period.PeriodStart, period.PeriodEnd), PeriodEnd: period.PeriodEnd, FilingDate: period.FilingDate,
			Frequency: period.Frequency, Form: period.Form, PeriodStart: period.PeriodStart,
			Accession: period.Accession, SourceURL: period.SourceURL,
			FieldEvidence: make(map[string]agents.EvidenceFinancialField),
		}
		for _, field := range period.AvailableFields {
			evidence, ok := validFinancialFieldEvidence(period, field)
			if !ok {
				continue
			}
			converted.FieldEvidence[field] = agents.EvidenceFinancialField{
				Source: evidence.Source, SourceURL: evidence.URL, Unit: evidence.Unit,
				PeriodStart: evidence.PeriodStart, PeriodEnd: evidence.PeriodEnd, Form: evidence.Form,
				FilingDate: evidence.Filed, Accession: evidence.Accession, PeriodKind: evidence.PeriodKind,
			}
			converted.AvailableFields = append(converted.AvailableFields, field)
			switch field {
			case "totalRevenue":
				converted.TotalRevenue = financialValue(period.TotalRevenue)
			case "grossProfit":
				converted.GrossProfit = financialValue(period.GrossProfit)
			case "operatingIncome":
				converted.OperatingIncome = financialValue(period.OperatingIncome)
			case "netIncome":
				converted.NetIncome = financialValue(period.NetIncome)
			case "totalAssets":
				converted.TotalAssets = financialValue(period.TotalAssets)
			case "totalLiabilities":
				converted.TotalLiabilities = financialValue(period.TotalLiabilities)
			case "stockholdersEquity":
				converted.StockholdersEquity = financialValue(period.StockholdersEquity)
			case "cashAndEquivalents":
				converted.CashAndEquivalents = financialValue(period.CashAndEquivalents)
			case "sharesOutstanding":
				converted.SharesOutstanding = financialValue(period.SharesOutstanding)
			}
		}
		out = append(out, converted)
	}
	if len(out) < minimum {
		return nil, fmt.Errorf("at least %d complete filing-bound financial periods are required", minimum)
	}
	return out, nil
}

func stableFinancialPeriodIdentity(frequency, start, end string) string {
	frequency = strings.ToLower(strings.TrimSpace(frequency))
	if frequency == "" {
		frequency = "unknown"
	}
	if strings.TrimSpace(start) == "" {
		return frequency + ":" + end
	}
	return frequency + ":" + start + "/" + end
}

func financialValue(value float64) *float64 { copy := value; return &copy }

func validFinancialFieldEvidence(period dataflows.FinancialPeriod, field string) (dataflows.FinancialFieldEvidence, bool) {
	evidence, ok := period.FieldEvidence[field]
	if !ok || !isSECFieldSource(evidence.Source, evidence.URL) || evidence.Unit == "" || evidence.Form != period.Form || evidence.Filed != period.FilingDate || evidence.Accession != period.Accession {
		return dataflows.FinancialFieldEvidence{}, false
	}
	if _, err := time.Parse("2006-01-02", evidence.PeriodEnd); err != nil {
		return dataflows.FinancialFieldEvidence{}, false
	}
	if field == "sharesOutstanding" {
		filed, filedErr := time.Parse("2006-01-02", period.FilingDate)
		observed, observedErr := time.Parse("2006-01-02", evidence.PeriodEnd)
		periodEnd, periodEndErr := time.Parse("2006-01-02", period.PeriodEnd)
		days := int(observed.Sub(periodEnd).Hours() / 24)
		if evidence.Unit != "shares" || evidence.PeriodKind != "instant" || evidence.PeriodStart != evidence.PeriodEnd || filedErr != nil || observedErr != nil || periodEndErr != nil || observed.After(filed) || days < 0 || days > 60 {
			return dataflows.FinancialFieldEvidence{}, false
		}
	} else if evidence.PeriodEnd != period.PeriodEnd {
		return dataflows.FinancialFieldEvidence{}, false
	} else if financialFieldKind(field) == "duration" {
		if evidence.PeriodKind != "duration" || evidence.PeriodStart != period.PeriodStart || !validFinancialDuration(period.Frequency, evidence.PeriodStart, evidence.PeriodEnd) {
			return dataflows.FinancialFieldEvidence{}, false
		}
	} else if evidence.PeriodKind != "instant" || evidence.PeriodStart != evidence.PeriodEnd {
		return dataflows.FinancialFieldEvidence{}, false
	}
	return evidence, true
}

func isSECFieldSource(source, sourceURL string) bool {
	return (source == "sec-edgar-companyfacts" || source == "sec-edgar-filing-facts") &&
		(strings.HasPrefix(sourceURL, "https://www.sec.gov/Archives/") || strings.HasPrefix(sourceURL, "https://data.sec.gov/"))
}

func financialFieldKind(field string) string {
	switch field {
	case "totalRevenue", "grossProfit", "operatingIncome", "netIncome":
		return "duration"
	default:
		return "instant"
	}
}

func validFinancialDuration(frequency, startRaw, endRaw string) bool {
	start, startErr := time.Parse("2006-01-02", startRaw)
	end, endErr := time.Parse("2006-01-02", endRaw)
	if startErr != nil || endErr != nil || !start.Before(end) {
		return false
	}
	days := int(end.Sub(start).Hours()/24) + 1
	switch strings.ToLower(frequency) {
	case "quarterly":
		return days >= 70 && days <= 110
	case "annual":
		return days >= 330 && days <= 400
	case "year-to-date":
		return days >= 230 && days <= 300
	default:
		return false
	}
}

func financialEvidenceCalculations(periods, derivationInputs []agents.EvidenceFinancialPeriod, researchPrice *float64) []agents.EvidenceOnlyCalculation {
	calculations := make([]agents.EvidenceOnlyCalculation, 0, len(periods)*4+5)
	for _, period := range periods {
		grossProfit, grossProfitAvailable := periodMetric(period, "grossProfit")
		operatingIncome, operatingIncomeAvailable := periodMetric(period, "operatingIncome")
		netIncome, netIncomeAvailable := periodMetric(period, "netIncome")
		revenue, revenueAvailable := periodMetric(period, "totalRevenue")
		liabilities, liabilitiesAvailable := periodMetric(period, "totalLiabilities")
		assets, assetsAvailable := periodMetric(period, "totalAssets")
		calculations = append(calculations,
			ratioCalculation("gross_margin:"+period.PeriodEnd, "gross_profit / total_revenue * 100", grossProfit, revenue, "gross_profit", "total_revenue", grossProfitAvailable, revenueAvailable),
			ratioCalculation("operating_margin:"+period.PeriodEnd, "operating_income / total_revenue * 100", operatingIncome, revenue, "operating_income", "total_revenue", operatingIncomeAvailable, revenueAvailable),
			ratioCalculation("net_margin:"+period.PeriodEnd, "net_income / total_revenue * 100", netIncome, revenue, "net_income", "total_revenue", netIncomeAvailable, revenueAvailable),
			ratioCalculation("liability_ratio:"+period.PeriodEnd, "total_liabilities / total_assets * 100", liabilities, assets, "total_liabilities", "total_assets", liabilitiesAvailable, assetsAvailable),
		)
	}
	latest := periods[0]
	if prior := comparablePeriod(periods[1:], latest, 60, 120); prior != nil && financialPeriodFrequency(latest) == "quarterly" && financialPeriodFrequency(*prior) == "quarterly" {
		latestRevenue, latestRevenueOK := periodMetric(latest, "totalRevenue")
		priorRevenue, priorRevenueOK := periodMetric(*prior, "totalRevenue")
		latestNetIncome, latestNetIncomeOK := periodMetric(latest, "netIncome")
		priorNetIncome, priorNetIncomeOK := periodMetric(*prior, "netIncome")
		latestRevenueOK = latestRevenueOK && comparableMetricPeriods(latest, *prior, "totalRevenue")
		priorRevenueOK = priorRevenueOK && latestRevenueOK
		latestNetIncomeOK = latestNetIncomeOK && comparableMetricPeriods(latest, *prior, "netIncome")
		priorNetIncomeOK = priorNetIncomeOK && latestNetIncomeOK
		calculations = append(calculations,
			growthCalculation("revenue_qoq:"+latest.PeriodEnd, latestRevenue, priorRevenue, latest.PeriodEnd, prior.PeriodEnd, latestRevenueOK, priorRevenueOK),
			growthCalculation("net_income_qoq:"+latest.PeriodEnd, latestNetIncome, priorNetIncome, latest.PeriodEnd, prior.PeriodEnd, latestNetIncomeOK, priorNetIncomeOK),
			cashChangeCalculation(latest, *prior, "cash_change_qoq:"+latest.PeriodEnd),
			ratioTrendCalculation("gross_margin_change_qoq_pp:"+latest.PeriodEnd, latest, *prior, "grossProfit", "totalRevenue"),
			ratioTrendCalculation("operating_margin_change_qoq_pp:"+latest.PeriodEnd, latest, *prior, "operatingIncome", "totalRevenue"),
			ratioTrendCalculation("net_margin_change_qoq_pp:"+latest.PeriodEnd, latest, *prior, "netIncome", "totalRevenue"),
			ratioTrendCalculation("liability_ratio_change_qoq_pp:"+latest.PeriodEnd, latest, *prior, "totalLiabilities", "totalAssets"),
		)
	} else {
		calculations = append(calculations, unknownCalculation("revenue_qoq:"+latest.PeriodEnd, "(current / prior_comparable_quarter - 1) * 100", "no comparable prior quarter 60-120 days earlier"))
	}
	if prior := comparablePeriod(periods[1:], latest, 330, 400); prior != nil && financialPeriodFrequency(*prior) == financialPeriodFrequency(latest) {
		latestRevenue, latestRevenueOK := periodMetric(latest, "totalRevenue")
		priorRevenue, priorRevenueOK := periodMetric(*prior, "totalRevenue")
		latestNetIncome, latestNetIncomeOK := periodMetric(latest, "netIncome")
		priorNetIncome, priorNetIncomeOK := periodMetric(*prior, "netIncome")
		latestRevenueOK = latestRevenueOK && comparableMetricPeriods(latest, *prior, "totalRevenue")
		priorRevenueOK = priorRevenueOK && latestRevenueOK
		latestNetIncomeOK = latestNetIncomeOK && comparableMetricPeriods(latest, *prior, "netIncome")
		priorNetIncomeOK = priorNetIncomeOK && latestNetIncomeOK
		calculations = append(calculations,
			growthCalculation("revenue_yoy:"+latest.PeriodEnd, latestRevenue, priorRevenue, latest.PeriodEnd, prior.PeriodEnd, latestRevenueOK, priorRevenueOK),
			growthCalculation("net_income_yoy:"+latest.PeriodEnd, latestNetIncome, priorNetIncome, latest.PeriodEnd, prior.PeriodEnd, latestNetIncomeOK, priorNetIncomeOK),
		)
	} else {
		calculations = append(calculations,
			unknownCalculation("revenue_yoy:"+latest.PeriodEnd, "(current / prior_comparable_year - 1) * 100", "no same-frequency comparable period 330-400 days earlier"),
			unknownCalculation("net_income_yoy:"+latest.PeriodEnd, "(current / prior_comparable_year - 1) * 100", "no same-frequency comparable period 330-400 days earlier"),
		)
	}
	if latestAnnual := latestFinancialPeriodByFrequency(periods, "annual"); latestAnnual != nil {
		if priorAnnual := comparablePeriod(periods, *latestAnnual, 330, 400); priorAnnual != nil {
			currentRevenue, currentRevenueOK := periodMetric(*latestAnnual, "totalRevenue")
			priorRevenue, priorRevenueOK := periodMetric(*priorAnnual, "totalRevenue")
			currentIncome, currentIncomeOK := periodMetric(*latestAnnual, "netIncome")
			priorIncome, priorIncomeOK := periodMetric(*priorAnnual, "netIncome")
			currentRevenueOK = currentRevenueOK && comparableMetricPeriods(*latestAnnual, *priorAnnual, "totalRevenue")
			priorRevenueOK = priorRevenueOK && currentRevenueOK
			currentIncomeOK = currentIncomeOK && comparableMetricPeriods(*latestAnnual, *priorAnnual, "netIncome")
			priorIncomeOK = priorIncomeOK && currentIncomeOK
			calculations = append(calculations,
				growthCalculation("annual_revenue_yoy:"+latestAnnual.PeriodEnd, currentRevenue, priorRevenue, latestAnnual.PeriodEnd, priorAnnual.PeriodEnd, currentRevenueOK, priorRevenueOK),
				growthCalculation("annual_net_income_yoy:"+latestAnnual.PeriodEnd, currentIncome, priorIncome, latestAnnual.PeriodEnd, priorAnnual.PeriodEnd, currentIncomeOK, priorIncomeOK),
			)
		} else {
			calculations = append(calculations,
				unknownCalculation("annual_revenue_yoy:"+latestAnnual.PeriodEnd, "(current_annual / prior_comparable_annual - 1) * 100", "no comparable prior annual period 330-400 days earlier"),
				unknownCalculation("annual_net_income_yoy:"+latestAnnual.PeriodEnd, "(current_annual / prior_comparable_annual - 1) * 100", "no comparable prior annual period 330-400 days earlier"),
			)
		}
	}
	calculations = append(calculations, ttmCalculations(periods, derivationInputs)...)
	calculations = append(calculations, valuationCalculations(periods, calculations, researchPrice)...)
	return calculations
}

func latestFinancialPeriodByFrequency(periods []agents.EvidenceFinancialPeriod, frequency string) *agents.EvidenceFinancialPeriod {
	for index := range periods {
		if financialPeriodFrequency(periods[index]) == frequency {
			return &periods[index]
		}
	}
	return nil
}

func comparablePeriod(periods []agents.EvidenceFinancialPeriod, latestPeriod agents.EvidenceFinancialPeriod, minDays, maxDays int) *agents.EvidenceFinancialPeriod {
	latest, err := time.Parse("2006-01-02", latestPeriod.PeriodEnd)
	if err != nil {
		return nil
	}
	for index := range periods {
		if financialPeriodFrequency(periods[index]) != financialPeriodFrequency(latestPeriod) {
			continue
		}
		candidate, err := time.Parse("2006-01-02", periods[index].PeriodEnd)
		if err == nil {
			days := int(latest.Sub(candidate).Hours() / 24)
			if days >= minDays && days <= maxDays {
				return &periods[index]
			}
		}
	}
	return nil
}

func financialPeriodFrequency(period agents.EvidenceFinancialPeriod) string {
	if period.Frequency != "" {
		return strings.ToLower(period.Frequency)
	}
	if strings.HasPrefix(strings.ToLower(period.FiscalPeriod), "quarter") || strings.Contains(strings.ToUpper(period.FiscalPeriod), "-Q") {
		return "quarterly"
	}
	if strings.HasPrefix(strings.ToLower(period.FiscalPeriod), "annual") || strings.Contains(strings.ToUpper(period.FiscalPeriod), "-FY") {
		return "annual"
	}
	return "unknown"
}

func ttmCalculations(periods, derivationInputs []agents.EvidenceFinancialPeriod) []agents.EvidenceOnlyCalculation {
	quarters := make([]agents.EvidenceFinancialPeriod, 0, 4)
	for _, period := range periods {
		if financialPeriodFrequency(period) == "quarterly" {
			quarters = append(quarters, period)
			if len(quarters) == 4 {
				break
			}
		}
	}
	makeDirectTTM := func(name, field string) agents.EvidenceOnlyCalculation {
		formula := "sum(latest 4 filing-bound quarterly values)"
		if len(quarters) < 4 {
			return unknownCalculation(name, formula, fmt.Sprintf("requires 4 quarterly periods; available=%d", len(quarters)))
		}
		for index := 1; index < len(quarters); index++ {
			newer, newerErr := time.Parse("2006-01-02", quarters[index-1].PeriodEnd)
			older, olderErr := time.Parse("2006-01-02", quarters[index].PeriodEnd)
			days := int(newer.Sub(older).Hours() / 24)
			if newerErr != nil || olderErr != nil || days < 60 || days > 120 {
				return unknownCalculation(name, formula, "latest quarterly periods are not four consecutive 60-120 day intervals")
			}
		}
		inputs := map[string]float64{}
		total := 0.0
		for _, period := range quarters {
			if !validEvidenceFinancialField(period, field) || !validFinancialDuration(period.Frequency, period.FieldEvidence[field].PeriodStart, period.FieldEvidence[field].PeriodEnd) {
				return unknownCalculation(name, formula, "a quarterly value lacks proven discrete-period field evidence")
			}
			value, ok := periodMetric(period, field)
			if !ok {
				return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: inputs, Status: "unknown", Reason: "a required quarterly value is unavailable", EvidenceIDs: []string{"tool-fundamentals"}}
			}
			inputs[period.PeriodEnd] = value
			total += value
		}
		return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: inputs, Value: &total, Unit: "currency units", Status: "computed", EvidenceIDs: []string{"tool-fundamentals"}}
	}
	directRevenue := makeDirectTTM("ttm_revenue", "totalRevenue")
	directNetIncome := makeDirectTTM("ttm_net_income", "netIncome")
	if directRevenue.Status == "computed" && directNetIncome.Status == "computed" {
		return []agents.EvidenceOnlyCalculation{directRevenue, directNetIncome}
	}
	derived := derivedQ4TTMCalculations(periods, derivationInputs)
	out := make([]agents.EvidenceOnlyCalculation, 0, 4)
	if directRevenue.Status == "computed" {
		out = append(out, directRevenue)
	} else {
		for _, name := range []string{"derived_fiscal_q4_revenue", "ttm_revenue"} {
			if calculation := findCalculation(derived, name); calculation != nil {
				out = append(out, *calculation)
			}
		}
	}
	if directNetIncome.Status == "computed" {
		out = append(out, directNetIncome)
	} else {
		for _, name := range []string{"derived_fiscal_q4_net_income", "ttm_net_income"} {
			if calculation := findCalculation(derived, name); calculation != nil {
				out = append(out, *calculation)
			}
		}
	}
	return out
}

func derivedQ4TTMCalculations(periods, derivationInputs []agents.EvidenceFinancialPeriod) []agents.EvidenceOnlyCalculation {
	unknown := func(field, label string) []agents.EvidenceOnlyCalculation {
		reason := "requires a filing-bound annual value, a reported Q1-Q3 YTD value for the same fiscal span, and three consecutive post-year-end quarters"
		return []agents.EvidenceOnlyCalculation{
			unknownCalculation("derived_fiscal_q4_"+field, "annual - reported_q1_to_q3_ytd", reason),
			unknownCalculation("ttm_"+label, "derived_fiscal_q4 + latest 3 filing-bound quarters", reason),
		}
	}
	if len(periods) == 0 {
		out := unknown("revenue", "revenue")
		return append(out, unknown("net_income", "net_income")...)
	}
	latest := periods[0]
	var annual *agents.EvidenceFinancialPeriod
	for index := range periods {
		if financialPeriodFrequency(periods[index]) == "annual" && periods[index].PeriodEnd < latest.PeriodEnd {
			annual = &periods[index]
			break
		}
	}
	if annual == nil {
		out := unknown("revenue", "revenue")
		return append(out, unknown("net_income", "net_income")...)
	}
	currentQuarters := make([]agents.EvidenceFinancialPeriod, 0, 3)
	for _, period := range periods {
		if financialPeriodFrequency(period) == "quarterly" && period.PeriodEnd > annual.PeriodEnd {
			currentQuarters = append(currentQuarters, period)
		}
	}
	sort.Slice(currentQuarters, func(i, j int) bool { return currentQuarters[i].PeriodEnd < currentQuarters[j].PeriodEnd })
	if len(currentQuarters) != 3 || !consecutiveQuartersAfterAnnual(*annual, currentQuarters) {
		out := unknown("revenue", "revenue")
		return append(out, unknown("net_income", "net_income")...)
	}
	var ytd *agents.EvidenceFinancialPeriod
	for index := range derivationInputs {
		candidate := &derivationInputs[index]
		if financialPeriodFrequency(*candidate) == "year-to-date" && candidate.PeriodStart == annual.PeriodStart && candidate.PeriodEnd < annual.PeriodEnd {
			end, endErr := time.Parse("2006-01-02", candidate.PeriodEnd)
			annualEnd, annualErr := time.Parse("2006-01-02", annual.PeriodEnd)
			if endErr == nil && annualErr == nil {
				days := int(annualEnd.Sub(end).Hours() / 24)
				if days >= 60 && days <= 120 {
					ytd = candidate
					break
				}
			}
		}
	}
	if ytd == nil {
		out := unknown("revenue", "revenue")
		return append(out, unknown("net_income", "net_income")...)
	}
	build := func(field, label string) []agents.EvidenceOnlyCalculation {
		annualValue, annualOK := periodMetric(*annual, field)
		ytdValue, ytdOK := periodMetric(*ytd, field)
		annualOK = annualOK && validEvidenceFinancialField(*annual, field)
		ytdOK = ytdOK && validEvidenceFinancialField(*ytd, field)
		inputs := map[string]float64{}
		if annualOK {
			inputs["annual_"+annual.PeriodEnd] = annualValue
		}
		if ytdOK {
			inputs["reported_q1_to_q3_ytd_"+ytd.PeriodEnd] = ytdValue
		}
		if !annualOK || !ytdOK {
			return unknown(label, label)
		}
		q4 := annualValue - ytdValue
		q4Calculation := agents.EvidenceOnlyCalculation{
			Name: "derived_fiscal_q4_" + label, Formula: "annual - reported_q1_to_q3_ytd", Inputs: inputs,
			Value: &q4, Unit: "currency units", Status: "computed", EvidenceIDs: []string{"tool-fundamentals"},
		}
		ttmInputs := map[string]float64{"derived_fiscal_q4": q4}
		total := q4
		for _, quarter := range currentQuarters {
			value, ok := periodMetric(quarter, field)
			if !ok || !validEvidenceFinancialField(quarter, field) {
				return []agents.EvidenceOnlyCalculation{q4Calculation, unknownCalculation("ttm_"+label, "derived_fiscal_q4 + latest 3 filing-bound quarters", "a post-year-end quarter lacks field-level evidence")}
			}
			ttmInputs["quarter_"+quarter.PeriodEnd] = value
			total += value
		}
		ttm := agents.EvidenceOnlyCalculation{
			Name: "ttm_" + label, Formula: "derived_fiscal_q4 + latest 3 filing-bound quarters", Inputs: ttmInputs,
			Value: &total, Unit: "currency units", Status: "computed", EvidenceIDs: []string{"tool-fundamentals"},
		}
		return []agents.EvidenceOnlyCalculation{q4Calculation, ttm}
	}
	out := build("totalRevenue", "revenue")
	return append(out, build("netIncome", "net_income")...)
}

func consecutiveQuartersAfterAnnual(annual agents.EvidenceFinancialPeriod, quarters []agents.EvidenceFinancialPeriod) bool {
	if len(quarters) != 3 {
		return false
	}
	annualEnd, annualErr := time.Parse("2006-01-02", annual.PeriodEnd)
	firstStart, firstErr := time.Parse("2006-01-02", quarters[0].PeriodStart)
	if annualErr != nil || firstErr != nil || daysBetween(annualEnd, firstStart) < 1 || daysBetween(annualEnd, firstStart) > 7 {
		return false
	}
	for index := 1; index < len(quarters); index++ {
		older, olderErr := time.Parse("2006-01-02", quarters[index-1].PeriodEnd)
		newer, newerErr := time.Parse("2006-01-02", quarters[index].PeriodEnd)
		if olderErr != nil || newerErr != nil || daysBetween(older, newer) < 60 || daysBetween(older, newer) > 120 {
			return false
		}
	}
	return true
}

func daysBetween(older, newer time.Time) int { return int(newer.Sub(older).Hours() / 24) }

func valuationCalculations(periods []agents.EvidenceFinancialPeriod, existing []agents.EvidenceOnlyCalculation, researchPrice *float64) []agents.EvidenceOnlyCalculation {
	unknownMarketCap := unknownCalculation("market_cap", "filing_bound_shares_outstanding * PIT_historical_close", "filing-bound shares or PIT historical close is unavailable")
	unknownPS := unknownCalculation("price_to_sales", "market_cap / ttm_revenue", "market capitalization or TTM revenue is unavailable")
	unknownPB := unknownCalculation("price_to_book", "market_cap / latest_filing_bound_stockholders_equity", "market capitalization or filing-bound equity is unavailable")
	if len(periods) == 0 || researchPrice == nil || *researchPrice <= 0 {
		return []agents.EvidenceOnlyCalculation{unknownMarketCap, unknownPS, unknownPB}
	}
	latest := periods[0]
	shares, sharesOK := periodMetric(latest, "sharesOutstanding")
	if !sharesOK || shares <= 0 || !validEvidenceFinancialField(latest, "sharesOutstanding") {
		return []agents.EvidenceOnlyCalculation{unknownMarketCap, unknownPS, unknownPB}
	}
	marketCap := shares * *researchPrice
	marketCapCalculation := agents.EvidenceOnlyCalculation{
		Name: "market_cap", Formula: "filing_bound_shares_outstanding * PIT_historical_close",
		Inputs: map[string]float64{"shares_outstanding_" + latest.FieldEvidence["sharesOutstanding"].PeriodEnd: shares, "pit_historical_close": *researchPrice},
		Value:  &marketCap, Unit: "currency units", Status: "computed", EvidenceIDs: []string{"tool-fundamentals", "tool-historical"},
	}
	ps := unknownPS
	if ttm := findCalculation(existing, "ttm_revenue"); ttm != nil && ttm.Status == "computed" && ttm.Value != nil && *ttm.Value != 0 {
		value := marketCap / *ttm.Value
		ps = agents.EvidenceOnlyCalculation{Name: "price_to_sales", Formula: "market_cap / ttm_revenue", Inputs: map[string]float64{"market_cap": marketCap, "ttm_revenue": *ttm.Value}, Value: &value, Unit: "ratio", Status: "computed", EvidenceIDs: []string{"tool-fundamentals", "tool-historical"}}
	}
	pb := unknownPB
	if equity, ok := periodMetric(latest, "stockholdersEquity"); ok && equity != 0 && validEvidenceFinancialField(latest, "stockholdersEquity") {
		value := marketCap / equity
		pb = agents.EvidenceOnlyCalculation{Name: "price_to_book", Formula: "market_cap / latest_filing_bound_stockholders_equity", Inputs: map[string]float64{"market_cap": marketCap, "stockholders_equity": equity}, Value: &value, Unit: "ratio", Status: "computed", EvidenceIDs: []string{"tool-fundamentals", "tool-historical"}}
	}
	return []agents.EvidenceOnlyCalculation{marketCapCalculation, ps, pb}
}

func scenarioEvidenceCalculations(context *agents.ResearchContext, existing []agents.EvidenceOnlyCalculation, periods []agents.EvidenceFinancialPeriod, researchPrice *float64) []agents.EvidenceOnlyCalculation {
	if context == nil || context.Scenario == nil {
		return nil
	}
	const disclosure = "user-supplied research_context.scenario assumption; not a forecast or target price"
	ids := []string{"input-research-context", "tool-fundamentals", "tool-historical"}
	inputs := map[string]float64{}
	sources := map[string]string{}
	if context.Scenario.RevenueGrowthPct != nil {
		inputs["user_revenue_growth_pct"] = *context.Scenario.RevenueGrowthPct
		sources["user_revenue_growth_pct"] = disclosure
	}
	if context.Scenario.PSMultiple != nil {
		inputs["user_ps_multiple"] = *context.Scenario.PSMultiple
		sources["user_ps_multiple"] = disclosure
	}
	unknown := func(name, formula, reason string, used map[string]float64) agents.EvidenceOnlyCalculation {
		return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: used, Status: "unknown", Reason: reason, EvidenceIDs: ids, InputSources: sources}
	}
	ttm := findCalculation(existing, "ttm_revenue")
	growthValid := context.Scenario.RevenueGrowthPct != nil && isFinite(*context.Scenario.RevenueGrowthPct) && *context.Scenario.RevenueGrowthPct > -100
	var scenarioRevenue agents.EvidenceOnlyCalculation
	if !growthValid || ttm == nil || ttm.Status != "computed" || ttm.Value == nil || *ttm.Value <= 0 {
		scenarioRevenue = unknown("scenario_revenue", "ttm_revenue * (1 + user_revenue_growth_pct / 100)", "valid user revenue growth and computed TTM revenue are required", cloneFloatInputs(inputs))
	} else {
		used := cloneFloatInputs(inputs)
		used["ttm_revenue"] = *ttm.Value
		value := *ttm.Value * (1 + *context.Scenario.RevenueGrowthPct/100)
		scenarioRevenue = agents.EvidenceOnlyCalculation{Name: "scenario_revenue", Formula: "ttm_revenue * (1 + user_revenue_growth_pct / 100)", Inputs: used, Value: &value, Unit: "currency units", Status: "computed", EvidenceIDs: ids, InputSources: sources}
	}
	multipleValid := context.Scenario.PSMultiple != nil && isFinite(*context.Scenario.PSMultiple) && *context.Scenario.PSMultiple > 0
	var impliedMarketCap agents.EvidenceOnlyCalculation
	if !multipleValid || scenarioRevenue.Status != "computed" || scenarioRevenue.Value == nil {
		impliedMarketCap = unknown("scenario_implied_market_cap", "scenario_revenue * user_ps_multiple", "computed scenario revenue and a positive user P/S multiple are required", cloneFloatInputs(inputs))
	} else {
		used := map[string]float64{"scenario_revenue": *scenarioRevenue.Value, "user_ps_multiple": *context.Scenario.PSMultiple}
		value := *scenarioRevenue.Value * *context.Scenario.PSMultiple
		impliedMarketCap = agents.EvidenceOnlyCalculation{Name: "scenario_implied_market_cap", Formula: "scenario_revenue * user_ps_multiple", Inputs: used, Value: &value, Unit: "currency units", Status: "computed", EvidenceIDs: ids, InputSources: sources}
	}
	shares, sharesOK := latestFilingBoundShares(periods)
	var impliedPrice agents.EvidenceOnlyCalculation
	if impliedMarketCap.Status != "computed" || impliedMarketCap.Value == nil || !sharesOK {
		impliedPrice = unknown("scenario_implied_price", "scenario_implied_market_cap / filing_bound_shares_outstanding", "computed scenario market cap and filing-bound shares are required", cloneFloatInputs(inputs))
	} else {
		used := map[string]float64{"scenario_implied_market_cap": *impliedMarketCap.Value, "filing_bound_shares_outstanding": shares}
		value := *impliedMarketCap.Value / shares
		impliedPrice = agents.EvidenceOnlyCalculation{Name: "scenario_implied_price", Formula: "scenario_implied_market_cap / filing_bound_shares_outstanding", Inputs: used, Value: &value, Unit: "currency units per share", Status: "computed", EvidenceIDs: ids, InputSources: sources}
	}
	var change agents.EvidenceOnlyCalculation
	if impliedPrice.Status != "computed" || impliedPrice.Value == nil || researchPrice == nil || *researchPrice <= 0 {
		change = unknown("scenario_change_pct", "(scenario_implied_price / PIT_historical_close - 1) * 100", "computed scenario price and PIT historical close are required", cloneFloatInputs(inputs))
	} else {
		used := map[string]float64{"scenario_implied_price": *impliedPrice.Value, "pit_historical_close": *researchPrice}
		value := (*impliedPrice.Value / *researchPrice - 1) * 100
		change = agents.EvidenceOnlyCalculation{Name: "scenario_change_pct", Formula: "(scenario_implied_price / PIT_historical_close - 1) * 100", Inputs: used, Value: &value, Unit: "percent", Status: "computed", EvidenceIDs: ids, InputSources: sources}
	}
	return []agents.EvidenceOnlyCalculation{scenarioRevenue, impliedMarketCap, impliedPrice, change}
}

func latestFilingBoundShares(periods []agents.EvidenceFinancialPeriod) (float64, bool) {
	for _, period := range periods {
		if shares, ok := periodMetric(period, "sharesOutstanding"); ok && shares > 0 && validEvidenceFinancialField(period, "sharesOutstanding") {
			return shares, true
		}
	}
	return 0, false
}

func cloneFloatInputs(input map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func contextScenarioAssessment(input *agents.ResearchContext, calculations []agents.EvidenceOnlyCalculation, periods []agents.EvidenceFinancialPeriod, researchPrice *float64) agents.EvidenceContextScenario {
	out := agents.EvidenceContextScenario{
		Status: "unknown", EvidenceIDs: []string{},
		Disclosure: "User-supplied sensitivity assumptions only; not a forecast, target price, or investment advice.",
		Reason:     "optional scenario assumptions not supplied",
	}
	if input == nil {
		return out
	}
	out.EvidenceIDs = []string{"input-research-context"}
	if input.Scenario == nil {
		return out
	}
	out.EvidenceIDs = []string{"input-research-context", "tool-fundamentals", "tool-historical"}
	out.RevenueGrowthPct = input.Scenario.RevenueGrowthPct
	out.PSMultiple = input.Scenario.PSMultiple
	if ttm := findCalculation(calculations, "ttm_revenue"); ttm != nil && ttm.Status == "computed" {
		out.BaseTTMRevenue = ttm.Value
	}
	if shares, ok := latestFilingBoundShares(periods); ok {
		out.SharesOutstanding = floatPointer(shares)
	}
	if researchPrice != nil && *researchPrice > 0 {
		out.CurrentPrice = floatPointer(*researchPrice)
	}
	bindings := []struct {
		name   string
		target **float64
	}{
		{"scenario_revenue", &out.ImpliedRevenue},
		{"scenario_implied_market_cap", &out.ImpliedMarketCap},
		{"scenario_implied_price", &out.ImpliedPrice},
		{"scenario_change_pct", &out.ChangePct},
	}
	reasons := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		calculation := findCalculation(calculations, binding.name)
		if calculation == nil || calculation.Status != "computed" || calculation.Value == nil {
			if calculation != nil && calculation.Reason != "" {
				reasons = append(reasons, calculation.Reason)
			}
			continue
		}
		*binding.target = floatPointer(*calculation.Value)
	}
	if len(reasons) == 0 && out.ImpliedRevenue != nil && out.ImpliedMarketCap != nil && out.ImpliedPrice != nil && out.ChangePct != nil {
		out.Status = "provided"
		out.Reason = ""
	} else {
		out.Reason = strings.Join(uniqueStrings(reasons), "; ")
		if out.Reason == "" {
			out.Reason = "scenario evidence is incomplete"
		}
	}
	return out
}

func uniqueStrings(input []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(input))
	for _, value := range input {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func financialTrendSummary(periods []agents.EvidenceFinancialPeriod, calculations []agents.EvidenceOnlyCalculation) *agents.EvidenceFinancialTrendSummary {
	wanted := make([]struct{ label, name string }, 0, 6)
	if annual := latestFinancialPeriodByFrequency(periods, "annual"); annual != nil {
		wanted = append(wanted,
			struct{ label, name string }{"annual revenue growth", "annual_revenue_yoy:" + annual.PeriodEnd},
			struct{ label, name string }{"annual net income growth", "annual_net_income_yoy:" + annual.PeriodEnd},
		)
	}
	if quarter := latestFinancialPeriodByFrequency(periods, "quarterly"); quarter != nil {
		wanted = append(wanted,
			struct{ label, name string }{"latest quarter revenue QoQ", "revenue_qoq:" + quarter.PeriodEnd},
			struct{ label, name string }{"latest quarter net income QoQ", "net_income_qoq:" + quarter.PeriodEnd},
			struct{ label, name string }{"latest quarter revenue YoY", "revenue_yoy:" + quarter.PeriodEnd},
			struct{ label, name string }{"latest quarter net income YoY", "net_income_yoy:" + quarter.PeriodEnd},
		)
	}
	out := &agents.EvidenceFinancialTrendSummary{Status: "unknown", Metrics: make([]agents.EvidenceFinancialTrendMetric, 0, len(wanted))}
	computed := 0
	for _, item := range wanted {
		metric := agents.EvidenceFinancialTrendMetric{Label: item.label, CalculationName: item.name, Status: "unknown", Reason: "calculation unavailable"}
		if calculation := findCalculation(calculations, item.name); calculation != nil {
			metric.Status, metric.Value, metric.Unit, metric.Reason = calculation.Status, calculation.Value, calculation.Unit, calculation.Reason
			if calculation.Status == "computed" && calculation.Value != nil {
				computed++
			}
		}
		out.Metrics = append(out.Metrics, metric)
	}
	if computed == len(wanted) && computed > 0 {
		out.Status = "complete"
	} else if computed > 0 {
		out.Status = "partial"
	}
	return out
}

func riskDiagnostics(calculations []agents.EvidenceOnlyCalculation) *agents.EvidenceRiskDiagnostics {
	out := &agents.EvidenceRiskDiagnostics{
		Status: "unknown", Disclosure: "Descriptive historical diagnostics only; not a forecast, trading signal, or risk limit.",
		Metrics: make([]agents.EvidenceFinancialTrendMetric, 0, 3),
	}
	computed := 0
	for _, item := range []struct{ label, name string }{
		{"annualized volatility", "annualized_volatility"},
		{"maximum drawdown", "maximum_drawdown"},
		{"20-session average daily volume", "average_daily_volume_20d"},
	} {
		metric := agents.EvidenceFinancialTrendMetric{Label: item.label, CalculationName: item.name, Status: "unknown", Reason: "calculation unavailable"}
		if calculation := findCalculation(calculations, item.name); calculation != nil {
			metric.Status, metric.Value, metric.Unit, metric.Reason = calculation.Status, calculation.Value, calculation.Unit, calculation.Reason
			if calculation.Status == "computed" && calculation.Value != nil {
				computed++
			}
		}
		out.Metrics = append(out.Metrics, metric)
	}
	if computed == len(out.Metrics) {
		out.Status = "complete"
	} else if computed > 0 {
		out.Status = "partial"
	}
	return out
}

func findCalculation(calculations []agents.EvidenceOnlyCalculation, name string) *agents.EvidenceOnlyCalculation {
	for index := range calculations {
		if calculations[index].Name == name {
			return &calculations[index]
		}
	}
	return nil
}

func calculationIsComputed(calculations []agents.EvidenceOnlyCalculation, name string) bool {
	calculation := findCalculation(calculations, name)
	return calculation != nil && calculation.Status == "computed" && calculation.Value != nil
}

func ratioCalculation(name, formula string, numerator, denominator float64, numeratorName, denominatorName string, numeratorAvailable, denominatorAvailable bool) agents.EvidenceOnlyCalculation {
	inputs := map[string]float64{}
	if numeratorAvailable {
		inputs[numeratorName] = numerator
	}
	if denominatorAvailable {
		inputs[denominatorName] = denominator
	}
	if !numeratorAvailable || !denominatorAvailable || denominator == 0 {
		return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: inputs, Status: "unknown", Reason: "required field is missing or denominator is zero", Unit: "percent", EvidenceIDs: []string{"tool-fundamentals"}}
	}
	value := numerator / denominator * 100
	return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: inputs, Value: &value, Status: "computed", Unit: "percent", EvidenceIDs: []string{"tool-fundamentals"}}
}

func growthCalculation(name string, current, prior float64, currentDate, priorDate string, currentAvailable, priorAvailable bool) agents.EvidenceOnlyCalculation {
	inputs := map[string]float64{}
	if currentAvailable {
		inputs["current_"+currentDate] = current
	}
	if priorAvailable {
		inputs["prior_"+priorDate] = prior
	}
	if !currentAvailable || !priorAvailable || prior == 0 {
		return agents.EvidenceOnlyCalculation{Name: name, Formula: "(current / prior - 1) * 100", Inputs: inputs, Status: "unknown", Reason: "prior comparable value is missing or zero", Unit: "percent", EvidenceIDs: []string{"tool-fundamentals"}}
	}
	value := (current/prior - 1) * 100
	return agents.EvidenceOnlyCalculation{Name: name, Formula: "(current / prior - 1) * 100", Inputs: inputs, Value: &value, Status: "computed", Unit: "percent", EvidenceIDs: []string{"tool-fundamentals"}}
}

func cashChangeCalculation(current, prior agents.EvidenceFinancialPeriod, name string) agents.EvidenceOnlyCalculation {
	currentCash, currentAvailable := periodMetric(current, "cashAndEquivalents")
	priorCash, priorAvailable := periodMetric(prior, "cashAndEquivalents")
	inputs := map[string]float64{}
	if currentAvailable {
		inputs["current_"+current.PeriodEnd] = currentCash
	}
	if priorAvailable {
		inputs["prior_"+prior.PeriodEnd] = priorCash
	}
	if !currentAvailable || !priorAvailable {
		return agents.EvidenceOnlyCalculation{Name: name, Formula: "current_cash - prior_cash", Inputs: inputs, Status: "unknown", Reason: "cash values are missing", Unit: "currency units", EvidenceIDs: []string{"tool-fundamentals"}}
	}
	value := currentCash - priorCash
	return agents.EvidenceOnlyCalculation{Name: name, Formula: "current_cash - prior_cash", Inputs: inputs, Value: &value, Status: "computed", Unit: "currency units", EvidenceIDs: []string{"tool-fundamentals"}}
}

func ratioTrendCalculation(name string, current, prior agents.EvidenceFinancialPeriod, numeratorField, denominatorField string) agents.EvidenceOnlyCalculation {
	formula := "(current_numerator / current_denominator - prior_numerator / prior_denominator) * 100"
	currentNumerator, currentNumeratorOK := periodMetric(current, numeratorField)
	currentDenominator, currentDenominatorOK := periodMetric(current, denominatorField)
	priorNumerator, priorNumeratorOK := periodMetric(prior, numeratorField)
	priorDenominator, priorDenominatorOK := periodMetric(prior, denominatorField)
	inputs := map[string]float64{}
	if currentNumeratorOK {
		inputs["current_numerator"] = currentNumerator
	}
	if currentDenominatorOK {
		inputs["current_denominator"] = currentDenominator
	}
	if priorNumeratorOK {
		inputs["prior_numerator"] = priorNumerator
	}
	if priorDenominatorOK {
		inputs["prior_denominator"] = priorDenominator
	}
	if !currentNumeratorOK || !currentDenominatorOK || !priorNumeratorOK || !priorDenominatorOK || currentDenominator == 0 || priorDenominator == 0 {
		return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: inputs, Status: "unknown", Reason: "required current or prior ratio field is unavailable", Unit: "percentage points", EvidenceIDs: []string{"tool-fundamentals"}}
	}
	value := (currentNumerator/currentDenominator - priorNumerator/priorDenominator) * 100
	return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: inputs, Value: &value, Status: "computed", Unit: "percentage points", EvidenceIDs: []string{"tool-fundamentals"}}
}

func periodMetric(period agents.EvidenceFinancialPeriod, field string) (float64, bool) {
	values := map[string]*float64{
		"totalRevenue": period.TotalRevenue, "grossProfit": period.GrossProfit,
		"operatingIncome": period.OperatingIncome, "netIncome": period.NetIncome,
		"totalAssets": period.TotalAssets, "totalLiabilities": period.TotalLiabilities,
		"stockholdersEquity": period.StockholdersEquity, "cashAndEquivalents": period.CashAndEquivalents,
		"sharesOutstanding": period.SharesOutstanding,
	}
	value := values[field]
	if value == nil || !validEvidenceFinancialField(period, field) {
		return 0, false
	}
	return *value, true
}

func validEvidenceFinancialField(period agents.EvidenceFinancialPeriod, field string) bool {
	evidence, ok := period.FieldEvidence[field]
	if !ok || !isSECFieldSource(evidence.Source, evidence.SourceURL) || evidence.Unit == "" || evidence.Form != period.Form || evidence.FilingDate != period.FilingDate || evidence.Accession != period.Accession {
		return false
	}
	if field == "sharesOutstanding" {
		filed, filedErr := time.Parse("2006-01-02", period.FilingDate)
		observed, observedErr := time.Parse("2006-01-02", evidence.PeriodEnd)
		periodEnd, periodEndErr := time.Parse("2006-01-02", period.PeriodEnd)
		days := int(observed.Sub(periodEnd).Hours() / 24)
		return evidence.Unit == "shares" && evidence.PeriodKind == "instant" && evidence.PeriodStart == evidence.PeriodEnd && filedErr == nil && observedErr == nil && periodEndErr == nil && !observed.After(filed) && days >= 0 && days <= 60
	}
	if evidence.PeriodEnd != period.PeriodEnd {
		return false
	}
	if financialFieldKind(field) == "duration" {
		return evidence.PeriodKind == "duration" && evidence.PeriodStart == period.PeriodStart && validFinancialDuration(period.Frequency, evidence.PeriodStart, evidence.PeriodEnd)
	}
	return evidence.PeriodKind == "instant" && evidence.PeriodStart == evidence.PeriodEnd
}

func comparableMetricPeriods(current, prior agents.EvidenceFinancialPeriod, field string) bool {
	if !validEvidenceFinancialField(current, field) || !validEvidenceFinancialField(prior, field) || financialPeriodFrequency(current) != financialPeriodFrequency(prior) {
		return false
	}
	if financialFieldKind(field) != "duration" {
		return true
	}
	currentStart, currentStartErr := time.Parse("2006-01-02", current.FieldEvidence[field].PeriodStart)
	currentEnd, currentEndErr := time.Parse("2006-01-02", current.FieldEvidence[field].PeriodEnd)
	priorStart, priorStartErr := time.Parse("2006-01-02", prior.FieldEvidence[field].PeriodStart)
	priorEnd, priorEndErr := time.Parse("2006-01-02", prior.FieldEvidence[field].PeriodEnd)
	if currentStartErr != nil || currentEndErr != nil || priorStartErr != nil || priorEndErr != nil {
		return false
	}
	currentDays := int(currentEnd.Sub(currentStart).Hours()/24) + 1
	priorDays := int(priorEnd.Sub(priorStart).Hours()/24) + 1
	difference := currentDays - priorDays
	if difference < 0 {
		difference = -difference
	}
	return difference <= 14
}

func unknownCalculation(name, formula, reason string) agents.EvidenceOnlyCalculation {
	return agents.EvidenceOnlyCalculation{Name: name, Formula: formula, Inputs: map[string]float64{}, Status: "unknown", Reason: reason, EvidenceIDs: []string{"tool-fundamentals"}}
}
