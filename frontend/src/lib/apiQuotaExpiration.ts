const beijingOffsetMinutes = 8 * 60

function pad(value: number) {
  return String(value).padStart(2, '0')
}

function beijingPartsFromUTC(date: Date) {
  const shifted = new Date(date.getTime() + beijingOffsetMinutes * 60 * 1000)
  return {
    year: shifted.getUTCFullYear(),
    month: shifted.getUTCMonth() + 1,
    day: shifted.getUTCDate(),
    hour: shifted.getUTCHours(),
    minute: shifted.getUTCMinutes(),
  }
}

export function formatBeijingDateTimeInput(date: Date) {
  const parts = beijingPartsFromUTC(date)
  return `${parts.year}-${pad(parts.month)}-${pad(parts.day)}T${pad(parts.hour)}:${pad(parts.minute)}`
}

export function defaultQuotaExpiresAtInput(now = new Date()) {
  return formatBeijingDateTimeInput(new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000))
}

export function beijingDateTimeInputToISOString(value: string) {
  const trimmed = value.trim()
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(trimmed)
  if (!match) return ''

  const [, yearText, monthText, dayText, hourText, minuteText] = match
  const year = Number(yearText)
  const month = Number(monthText)
  const day = Number(dayText)
  const hour = Number(hourText)
  const minute = Number(minuteText)
  const utc = new Date(Date.UTC(year, month - 1, day, hour, minute) - beijingOffsetMinutes * 60 * 1000)
  const parts = beijingPartsFromUTC(utc)
  if (parts.year !== year || parts.month !== month || parts.day !== day || parts.hour !== hour || parts.minute !== minute) {
    return ''
  }
  return utc.toISOString()
}

export function formatQuotaExpiresAtLabel(value?: string | null) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const parts = beijingPartsFromUTC(date)
  return `${parts.year}-${pad(parts.month)}-${pad(parts.day)} ${pad(parts.hour)}:${pad(parts.minute)}`
}
