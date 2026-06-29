import type { FieldErrors } from '@/lib/formValidation'

export type OfficialPriceSubmitField =
  | 'productPlanId'
  | 'product'
  | 'plan'
  | 'region'
  | 'channel'
  | 'originalPriceCurrency'
  | 'originalPriceAmount'
  | 'originalPrice'
  | 'openingMethod'
  | 'sourceUrl'
  | 'note'

export type OfficialPriceSubmitForm = Record<OfficialPriceSubmitField, string>

export type SourceLinkState = 'idle' | 'success' | 'error'

export type CompletenessItem = {
  label: string
  status: 'done' | 'warning' | 'pending'
  hint: string
}

export type SubmitterPreview = {
  name: string
  trustLevel: number | null
  verified: boolean
  avatarText: string
}

export type OfficialPriceSubmitErrors = FieldErrors<OfficialPriceSubmitField>
