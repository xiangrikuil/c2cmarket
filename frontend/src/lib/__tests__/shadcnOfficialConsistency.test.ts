import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const sourceRoot = fileURLToPath(new URL('../../', import.meta.url))
const frontendRoot = fileURLToPath(new URL('../../../', import.meta.url))
const uiRoot = join(sourceRoot, 'components/ui')

function source(path: string) {
  return readFileSync(join(sourceRoot, path), 'utf8')
}

function vueFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    return entry.isDirectory() ? vueFiles(path) : entry.name.endsWith('.vue') ? [path] : []
  })
}

describe('shadcn-vue official consistency contract', () => {
  it('keeps the complete installed primitive inventory explicit', () => {
    const groups = readdirSync(uiRoot, { withFileTypes: true })
      .filter(entry => entry.isDirectory())
      .map(entry => entry.name)
      .sort()

    expect(groups).toEqual([
      'alert',
      'badge',
      'button',
      'card',
      'chart',
      'checkbox',
      'collapsible',
      'dialog',
      'dropdown-menu',
      'input',
      'label',
      'popover',
      'radio-group',
      'select',
      'sonner',
      'stepper',
      'switch',
      'table',
      'tabs',
      'textarea',
      'tooltip',
    ])
  })

  it('preserves official floating-layer composition and defaults', () => {
    const tooltip = source('components/ui/tooltip/TooltipContent.vue')
    const popover = source('components/ui/popover/PopoverContent.vue')
    const popoverIndex = source('components/ui/popover/index.ts')
    const dialog = source('components/ui/dialog/DialogContent.vue')
    const dialogIndex = source('components/ui/dialog/index.ts')
    const dropdownItem = source('components/ui/dropdown-menu/DropdownMenuItem.vue')
    const dropdownSub = source('components/ui/dropdown-menu/DropdownMenuSubContent.vue')
    const select = source('components/ui/select/SelectContent.vue')

    expect(tooltip).toContain('<TooltipPortal>')
    expect(tooltip).toContain('<TooltipArrow')
    expect(tooltip).toContain('sideOffset: 4')
    expect(popover).toContain('<PopoverPortal>')
    expect(popover).toContain('align: "center"')
    expect(popover).toContain('max-w-(--reka-popover-content-available-width)')
    expect(popoverIndex).toContain('PopoverAnchor')
    expect(dialog).toContain('<DialogPortal>')
    expect(dialog).toContain('<DialogOverlay />')
    expect(dialog).toContain('showCloseButton?: boolean')
    expect(dialog).not.toContain('overflow-y-auto')
    expect(dialogIndex).toContain('DialogScrollContent')
    expect(dropdownItem).toContain("data-[variant=destructive]:*:[svg]:text-destructive!")
    expect(dropdownSub).toContain('max-w-(--reka-dropdown-menu-content-available-width)')
    expect(select).toContain('<SelectPortal>')
    expect(select).toContain('position: "popper"')
    expect(select).toContain('max-h-(--reka-select-content-available-height)')
  })

  it('keeps current official sizes while retaining documented product variants', () => {
    const button = source('components/ui/button/index.ts')
    const badge = source('components/ui/badge/index.ts')

    expect(button).toContain('"xs": "h-6')
    expect(button).toContain('"icon-xs": "size-6')
    for (const variant of ['identity', 'verified', 'trust', 'capability', 'model', 'status']) {
      expect(badge).toContain(`${variant}:`)
    }
  })

  it('does not restore legacy icon primitives or global interaction overrides', () => {
    const uiSources = vueFiles(uiRoot).map(path => readFileSync(path, 'utf8')).join('\n')
    const styles = source('styles.css')
    const packageJson = readFileSync(join(frontendRoot, 'package.json'), 'utf8')
    const lockfile = readFileSync(join(frontendRoot, 'pnpm-lock.yaml'), 'utf8')

    expect(uiSources).not.toContain('@radix-icons/vue')
    expect(packageJson).not.toContain('@radix-icons/vue')
    expect(lockfile).not.toContain('@radix-icons/vue')
    expect(styles).not.toContain('[data-slot="card"] {\n  border-radius: var(--radius);\n  gap: 1rem;')
    expect(styles).not.toContain('html[data-theme="minimal-modern"] [data-slot="card"]:hover')
    expect(styles).not.toContain('html[data-theme="minimal-modern"] [data-slot="button"]:active')
  })

  it('keeps tooltip alignment centered at business call sites', () => {
    const businessSources = vueFiles(sourceRoot)
      .filter(path => !path.startsWith(uiRoot))
      .map(path => readFileSync(path, 'utf8'))
      .join('\n')
    const tooltipTags = [...businessSources.matchAll(/<TooltipContent\b([^>]*)>/gs)]

    for (const [, attributes] of tooltipTags) {
      expect(attributes).not.toMatch(/\balign=["'](?:start|end)["']/)
      expect(attributes).not.toMatch(/\bside-offset=/)
    }
  })

  it('keeps representative business positioning exceptions local and intentional', () => {
    const productCombobox = source('components/carpool-publish/CarpoolProductCombobox.vue')
    const officialPrices = source('pages/OfficialPricesPage.vue')
    const ownerHeader = source('components/api-service-owner/ApiServiceOwnerHeader.vue')
    const adminUsers = source('pages/AdminUsersPage.vue')
    const businessSources = vueFiles(sourceRoot)
      .filter(path => !path.startsWith(uiRoot))
      .map(path => readFileSync(path, 'utf8'))
      .join('\n')
    const selectTags = [...businessSources.matchAll(/<SelectContent\b([^>]*)>/gs)]

    expect(productCombobox).toMatch(/<PopoverContent[^>]*align="start"/s)
    expect(officialPrices).toMatch(/<PopoverContent[^>]*align="end"/s)
    expect(ownerHeader).toMatch(/<DropdownMenuContent[^>]*align="end"/s)
    expect(adminUsers).toMatch(/<DialogContent[^>]*h-dvh[^>]*slide-in-from-right/s)
    expect(selectTags.length).toBeGreaterThan(0)
    for (const [, attributes] of selectTags) {
      expect(attributes).not.toMatch(/\b(?:align|side)=/)
    }
  })
})
