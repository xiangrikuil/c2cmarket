export function linuxDoProfileSummaryUrl(linuxDoId: string) {
  return `https://linux.do/u/${linuxDoId.replace(/^@/, '')}/summary`
}
