import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = (relativePath) => readFileSync(new URL(relativePath, import.meta.url), 'utf8')

test('stock detail isolates quote failure from the other detail requests', () => {
  const detail = source('../pages/StockDetail.tsx')

  assert.match(detail, /fetchQuoteResult\(\[normalizedSymbol\]\)\s*\.then\(/)
  assert.match(detail, /catch\(\(error\) => \(\{ result: null, error:/)
  assert.match(detail, /if \(!quoteCall\.result\) \{[\s\S]*setQuoteStatus\(\{ state: 'error'/)
})

test('history exposes loading and error without replacing records with a false empty state', () => {
  const store = source('../stores/analysisStore.ts')
  const page = source('../pages/History.tsx')

  assert.match(store, /historyLoading: boolean/)
  assert.match(store, /historyError: string/)
  assert.doesNotMatch(store, /fetchHistory:[\s\S]*catch[\s\S]*set\(\{ history: \[\] \}\)/)
  assert.match(page, /historyError &&[\s\S]*role="alert"[\s\S]*fetchHistory/)
  assert.match(page, /!historyLoading && !historyError && history\.length === 0/)
})

test('notes ignores article responses that are no longer current', () => {
  const notes = source('../pages/Notes.tsx')

  assert.match(notes, /articleRequestSequence = useRef\(0\)/)
  assert.match(notes, /const requestSequence = \+\+articleRequestSequence\.current/)
  assert.match(notes, /if \(requestSequence !== articleRequestSequence\.current\) return;/)
  assert.match(notes, /articleRequestSequence\.current \+= 1;[\s\S]*setActiveId\(null\)/)
})

test('scan store keeps only the latest list and quote responses and has one fetch owner', () => {
  const store = source('../stores/stocksStore.ts')
  const scan = source('../pages/Scan.tsx')

  assert.match(store, /let stocksRequestSequence = 0/)
  assert.match(store, /let quoteRequestSequence = 0/)
  assert.match(store, /requestSequence !== stocksRequestSequence/g)
  assert.match(store, /requestSequence !== quoteRequestSequence/g)
  assert.doesNotMatch(store, /setMarket:[\s\S]*?getState\(\)\.fetchStocks\(\)/)
  assert.match(scan, /useEffect\(\(\) => \{\s*fetchStocks\(\);\s*\}, \[debouncedSearch, market, sortBy, order, page\]\)/)
})

test('settings retains successful readiness results when one endpoint fails', () => {
  const settings = source('../pages/Settings.tsx')

  assert.match(settings, /Promise\.allSettled\(/)
  assert.match(settings, /setStatusLoadError\(/)
  assert.match(settings, /失败项保留上次成功结果/)
  assert.doesNotMatch(settings, /catch \(err\)[\s\S]*setSystemStatus\(null\)[\s\S]*setReadinessProfiles\(\[\]\)/)
  assert.match(settings, /statusLoadError &&[\s\S]*role="alert"/)
})

test('ETF holdings expose loading, error and retry while stale responses are ignored', () => {
  const etf = source('../pages/ETF.tsx')

  assert.match(etf, /holdingsRequestSequence = useRef\(0\)/)
  assert.match(etf, /const requestSequence = \+\+holdingsRequestSequence\.current/)
  assert.match(etf, /requestSequence !== holdingsRequestSequence\.current/g)
  assert.match(etf, /setHoldingsLoading\(true\)/)
  assert.match(etf, /setHoldingsError\(/)
  assert.match(etf, /holdingsError[\s\S]*role="alert"[\s\S]*loadHoldings\(holdingsSym\)/)
})
