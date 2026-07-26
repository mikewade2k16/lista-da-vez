export type CapabilityMode = 'unknown' | 'off' | 'shadow' | 'canary' | 'on'

export interface IntelligenceEvidenceRef {
  id: string
  sourceKey: string
  observedAt?: string | null
  excerpt?: string
  masked?: boolean
  hash?: string
}

export interface IntelligenceFactView {
  id: string
  factKey: string
  label?: string
  value: unknown
  state: 'candidate' | 'confirmed' | 'contested' | 'superseded' | string
  confidence?: number | null
  asOf?: string | null
  evidenceRefs?: IntelligenceEvidenceRef[]
}

export interface IntelligenceSummaryView {
  id: string
  summaryType: string
  text: string
  status: string
  asOf?: string | null
  promptVersionRef?: string
  contextVersionRef?: string
}

export interface IntelligenceTimelineItem {
  id: string
  kind: string
  title: string
  occurredAt: string
  channel?: string
  sourceKey?: string
  summary?: string
}

export interface IntelligenceFactsPage {
  items: IntelligenceFactView[]
  nextCursor: string
}

export interface IntelligenceTimelinePage {
  items: IntelligenceTimelineItem[]
  nextCursor: string
}

export interface CustomerIntelligenceProfileState {
  facts: IntelligenceFactView[]
  summaries: IntelligenceSummaryView[]
  timeline: IntelligenceTimelineItem[]
}

export interface CustomerClientOption {
  id: string
  name: string
  organizationName: string
}
