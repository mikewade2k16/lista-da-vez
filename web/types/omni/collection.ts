export type OmniTableCellType =
  | 'text'
  | 'number'
  | 'money'
  | 'select'
  | 'multiselect'
  | 'switch'
  | 'image'
  | 'custom'

export interface OmniSelectOption {
  label: string
  value: string | number
  color?: string
}

export interface OmniTableAction {
  id: string
  icon: string
  label: string
  color?: 'primary' | 'neutral' | 'success' | 'warning' | 'error'
  variant?: 'ghost' | 'soft' | 'solid' | 'outline'
  visible?: (row: Record<string, unknown>) => boolean
  disabled?: (row: Record<string, unknown>) => boolean
}

export interface OmniTableColumn {
  key: string
  label: string
  adminOnly?: boolean
  type?: OmniTableCellType
  editable?: boolean
  editableWhen?: (row: Record<string, unknown>) => boolean
  switchOnValue?: unknown
  switchOffValue?: unknown
  minWidth?: number
  maxWidth?: number
  align?: 'left' | 'center' | 'right'
  placeholder?: string
  options?: OmniSelectOption[]
  creatable?:
    | boolean
    | 'always'
    | {
        position?: 'top' | 'bottom'
        when?: 'empty' | 'always'
      }
  immediate?: boolean
  focusOnCreate?: boolean
  formatter?: (value: unknown, row: Record<string, unknown>) => string
  actions?: OmniTableAction[]
  // C16: admin pode marcar colunas como "travadas" (sempre visíveis,
  // ninguém esconde via UI). Default opcional declarado no workspace;
  // override persistido por user em localStorage via useOmniVisibleColumns.
  locked?: boolean
  // C16: posição padrão sugerida; usuário admin pode reordenar via drag-n-drop
  // e a ordem final fica em localStorage.
  defaultOrder?: number
}

export interface OmniFocusCell {
  rowId: string | number
  columnKey: string
  token?: number
}

export interface OmniTableCellUpdate {
  rowId: string | number
  key: string
  value: unknown
  immediate?: boolean
}

export interface OmniTableImageUpload {
  rowId: string | number
  key: string
  file: File
}

export type OmniFilterType = 'text' | 'select' | 'switch'

export interface OmniFilterDefinition {
  key: string
  label: string
  adminOnly?: boolean
  type: OmniFilterType
  placeholder?: string
  options?: OmniSelectOption[]
  switchOnValue?: unknown
  switchOffValue?: unknown
  mode?: 'all' | 'columns'
  columns?: string[]
  customPredicate?: (row: Record<string, unknown>, filterValue: unknown) => boolean
  accessor?: (row: Record<string, unknown>) => unknown
}
