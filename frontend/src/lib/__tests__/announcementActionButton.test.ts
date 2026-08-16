import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { validateAnnouncementFormInput } from '@/lib/announcementUtils'
import type { AnnouncementFormInput } from '@/types/announcement'

const editor = readFileSync(new URL('../../components/announcements/AnnouncementEditor.vue', import.meta.url), 'utf8')
const editorPage = readFileSync(new URL('../../pages/AdminAnnouncementEditorPage.vue', import.meta.url), 'utf8')

function validInput(overrides: Partial<AnnouncementFormInput> = {}): AnnouncementFormInput {
  return {
    title: '平台服务调整公告',
    summary: '这是一条用于验证可选跳转按钮的公告摘要。',
    contentMarkdown: '## 调整内容\n\n公告正文包含足够的有效内容。',
    category: 'platform',
    level: 'normal',
    channels: ['message_center'],
    audience: { type: 'all' },
    isPinned: false,
    isDismissible: true,
    requiresAck: false,
    publishAt: '2026-08-10T04:00:00.000Z',
    ...overrides,
  }
}

describe('公告可选跳转按钮', () => {
  it('创建时默认关闭，开启后才展示填写项', () => {
    expect(editor).toContain('const hasActionButton = ref(false)')
    expect(editor).toContain('v-if="hasActionButton"')
    expect(editor).toContain('添加跳转按钮')
    expect(editor).toContain('按钮文案')
    expect(editor).toContain('跳转链接')
    expect(editor).not.toContain('CTA 文案')
    expect(editorPage).not.toContain('CTA')
  })

  it('编辑已有按钮时自动开启，关闭后清空且不再提交旧值', () => {
    expect(editor).toContain('hasActionButton.value = Boolean(input.ctaLabel || input.ctaUrl)')
    expect(editor).toContain("form.ctaLabel = ''")
    expect(editor).toContain("form.ctaUrl = ''")
    expect(editor).toContain('ctaLabel: hasActionButton.value ? form.ctaLabel.trim() || undefined : undefined')
    expect(editor).toContain('ctaUrl: hasActionButton.value ? form.ctaUrl.trim() || undefined : undefined')
  })

  it('不添加按钮时允许两个字段同时为空', () => {
    expect(validateAnnouncementFormInput(validInput())).toEqual({ valid: true, errors: {} })
  })

  it('添加按钮时要求文案和链接成对填写', () => {
    const labelOnly = validateAnnouncementFormInput(validInput({ ctaLabel: '查看详情' }))
    const urlOnly = validateAnnouncementFormInput(validInput({ ctaUrl: '/announcements' }))
    const complete = validateAnnouncementFormInput(validInput({ ctaLabel: '查看详情', ctaUrl: '/announcements' }))

    expect(labelOnly.errors.ctaUrl).toBe('填写按钮文案后，也需要填写跳转链接。')
    expect(urlOnly.errors.ctaLabel).toBe('填写跳转链接后，也需要填写按钮文案。')
    expect(complete).toEqual({ valid: true, errors: {} })
  })

  it('继续拒绝不允许的外部跳转地址', () => {
    const result = validateAnnouncementFormInput(validInput({
      ctaLabel: '查看详情',
      ctaUrl: 'https://example.com/announcement',
    }))

    expect(result.errors.ctaUrl).toBe('跳转地址只允许站内相对路径或白名单 HTTPS 地址。')
  })
})
