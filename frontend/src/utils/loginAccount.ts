// This deployment provisions short accounts under sub2api.local. Keep the
// backend's email authentication, captcha and two-factor checks unchanged.
export function resolveLoginEmail(value: string): string | null {
  const account = value.trim()
  if (!account) return null
  if (account.includes('@')) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(account) ? account : null
  }
  if (!/^[a-zA-Z0-9][a-zA-Z0-9._-]*$/.test(account)) return null
  return `${account}@sub2api.local`
}
