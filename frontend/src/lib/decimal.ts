import Decimal from 'decimal.js'

Decimal.set({
  precision: 40,
  rounding: Decimal.ROUND_HALF_UP,
  toExpNeg: -30,
  toExpPos: 40,
})

export type DecimalInput = Decimal.Value

function decimal(value: DecimalInput) {
  return new Decimal(value)
}

export function normalizeDecimal(value: DecimalInput, places: number) {
  return decimal(value).toDecimalPlaces(places, Decimal.ROUND_HALF_UP).toFixed(places)
}

export function normalizeDecimalTrimmed(value: DecimalInput, places: number) {
  return decimal(value).toDecimalPlaces(places, Decimal.ROUND_HALF_UP).toFixed(places).replace(/\.?0+$/, '')
}

export function divideDecimal(dividend: DecimalInput, divisor: DecimalInput, places: number) {
  const right = decimal(divisor)
  if (!right.isPositive()) throw new Error('除数必须为正数。')
  return decimal(dividend).div(right).toDecimalPlaces(places, Decimal.ROUND_DOWN).toFixed(places)
}

export function multiplyDecimal(left: DecimalInput, right: DecimalInput, places: number) {
  return decimal(left).mul(decimal(right)).toDecimalPlaces(places, Decimal.ROUND_HALF_UP).toFixed(places)
}

export function compareDecimal(left: DecimalInput, right: DecimalInput) {
  return decimal(left).cmp(decimal(right))
}

export function addDecimal(left: DecimalInput, right: DecimalInput, places: number) {
  return decimal(left).add(decimal(right)).toDecimalPlaces(places, Decimal.ROUND_HALF_UP).toFixed(places)
}

export function formatDecimal(value: DecimalInput, minimumPlaces = 0, maximumPlaces = minimumPlaces) {
  const normalized = decimal(value).toDecimalPlaces(maximumPlaces, Decimal.ROUND_HALF_UP)
  const [whole, fraction = ''] = normalized.toFixed(maximumPlaces).split('.')
  const keptFraction = fraction.replace(/0+$/, '').padEnd(minimumPlaces, '0')
  const groupedWhole = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return keptFraction ? `${groupedWhole}.${keptFraction}` : groupedWhole
}

export function isPositiveDecimal(value: DecimalInput) {
  try {
    return decimal(value).isPositive()
  } catch {
    return false
  }
}
