export interface HistoricalReadiness {
  profile: 'historical-research'
  status: string
  ready: boolean
  httpStatus: number
  checks: Array<{ id: string; status: string; detail?: string; remedy?: string }>
  remedies?: string[]
  disclaimer: string
}

export function normalizeHistoricalReadiness(
  value: unknown,
  httpStatus: number,
): HistoricalReadiness | null {
  if (!value || typeof value !== 'object') return null

  const candidate = value as Partial<HistoricalReadiness>
  if (
    candidate.profile !== 'historical-research' ||
    typeof candidate.status !== 'string' ||
    typeof candidate.ready !== 'boolean' ||
    !Array.isArray(candidate.checks) ||
    typeof candidate.disclaimer !== 'string'
  ) {
    return null
  }

  return { ...candidate, httpStatus } as HistoricalReadiness
}
