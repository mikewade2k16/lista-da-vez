import type { PlanningStaffMember } from './types'

type StaffIdentity = Pick<PlanningStaffMember, 'name' | 'nick'>

export function planningStaffDisplayName(member?: StaffIdentity): string {
  return member?.nick?.trim() || member?.name.trim() || 'Funcionário'
}
