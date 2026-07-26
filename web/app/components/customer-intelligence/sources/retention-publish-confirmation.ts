interface RetentionPublishConfirmation {
  confirmed: true
}

export function isRetentionPublishConfirmed(value: unknown): value is RetentionPublishConfirmation {
  if (!value || typeof value !== 'object' || !('confirmed' in value)) return false
  return value.confirmed === true
}
