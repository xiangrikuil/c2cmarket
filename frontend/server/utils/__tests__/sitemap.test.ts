import { describe, expect, it } from 'vitest'
import { collectPublicPages, safeSitemapSegment, sitemapLastmod } from '../sitemap'

describe('dynamic sitemap helpers', () => {
  it('follows every opaque cursor without parsing it', async () => {
    const cursors: string[] = []
    const items = await collectPublicPages(async (cursor) => {
      cursors.push(cursor)
      return cursor
        ? { items: [{ id: 'second' }], nextCursor: null }
        : { items: [{ id: 'first' }], nextCursor: 'opaque-cursor' }
    })

    expect(cursors).toEqual(['', 'opaque-cursor'])
    expect(items.map(item => item.id)).toEqual(['first', 'second'])
  })

  it('rejects malformed or looping pagination responses', async () => {
    await expect(collectPublicPages(async () => ({ nextCursor: null }))).rejects.toThrow('invalid list response')
    await expect(collectPublicPages(async () => ({ items: [], nextCursor: 'same' }), 3)).rejects.toThrow('repeated cursor')
  })

  it('normalizes lastmod and rejects unsafe path segments', () => {
    expect(sitemapLastmod({ updatedAt: '2026-07-18T00:00:00+08:00' })).toBe('2026-07-17T16:00:00.000Z')
    expect(safeSitemapSegment('public-id_1')).toBe('public-id_1')
    expect(safeSitemapSegment('../private')).toBe('')
  })
})
