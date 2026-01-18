import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(date: string | Date): string {
  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(date))
}

export function formatNumber(num: number): string {
  return new Intl.NumberFormat('en-US').format(num)
}

export function formatCurrency(amount: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(amount)
}

export function formatPercentage(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

export function truncate(str: string, length: number): string {
  if (str.length <= length) return str
  return str.slice(0, length) + '...'
}

export function getInitials(name: string): string {
  return name
    .split(' ')
    .map(n => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

export const providerColors: Record<string, string> = {
  OPENAI: '#10a37f',
  ANTHROPIC: '#d4a373',
  GEMINI: '#4285f4',
  BEDROCK: '#ff9900',
  AZURE_OPENAI: '#0078d4',
  OLLAMA: '#ffffff',
  GROQ: '#f55036',
  MISTRAL: '#ff7000',
  TOGETHER: '#6366f1',
  COHERE: '#39594d',
}

export const providerIcons: Record<string, string> = {
  OPENAI: '🤖',
  ANTHROPIC: '🧠',
  GEMINI: '💎',
  BEDROCK: '☁️',
  AZURE_OPENAI: '🔷',
  OLLAMA: '🦙',
  GROQ: '⚡',
  MISTRAL: '🌬️',
  TOGETHER: '🤝',
  COHERE: '🔮',
}

