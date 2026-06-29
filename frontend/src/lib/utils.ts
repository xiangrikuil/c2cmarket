import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function money(value: number | string, prefix = '¥') {
  if (typeof value === 'string') return value
  return `${prefix}${value}`
}

export function wait<T>(data: T, ms = 160): Promise<T> {
  return new Promise(resolve => window.setTimeout(() => resolve(data), ms))
}
