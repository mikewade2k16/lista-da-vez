import type { OmniFilterDefinition } from '~/types/omni/collection'

function normalizeText(value: unknown) {
  return String(value ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
}

function defaultAccessor(row: Record<string, unknown>, key: string) {
  return row[key]
}

function rowContainsText(
  row: Record<string, unknown>,
  value: string,
  filter: OmniFilterDefinition,
  rowKeys: string[],
) {
  if (!value) {
    return true
  }

  const sourceKeys =
    filter.mode === 'columns' && Array.isArray(filter.columns) && filter.columns.length
      ? filter.columns
      : rowKeys

  const haystack = sourceKeys.map((key) => normalizeText(defaultAccessor(row, key))).join(' ')

  return haystack.includes(value)
}

export function applyOmniFilters<T extends Record<string, unknown>>(
  rows: T[],
  filtersValue: Record<string, unknown>,
  definitions: OmniFilterDefinition[],
) {
  const list = Array.isArray(rows) ? rows : []
  const rowKeys = list.length > 0 ? Object.keys(list[0] ?? {}) : []

  return list.filter((row) => {
    for (const definition of definitions) {
      const rawValue = filtersValue[definition.key]
      const normalized = normalizeText(rawValue)

      if (!normalized) {
        continue
      }

      if (definition.customPredicate) {
        if (!definition.customPredicate(row, rawValue)) {
          return false
        }
        continue
      }

      if (definition.type === 'text') {
        if (!rowContainsText(row, normalized, definition, rowKeys)) {
          return false
        }
        continue
      }

      if (definition.type === 'select') {
        const currentValue = definition.accessor
          ? definition.accessor(row)
          : defaultAccessor(row, definition.key)

        if (normalizeText(currentValue) !== normalized) {
          return false
        }
      }
    }

    return true
  })
}
