export const BRAND_INFO = {
  name: 'AlphaFlowX',
  shortName: 'AFX',
  tagline: 'AI Trading Platform',
  domain: 'alphaflowx.com',
  iconPath: '/icons/alphaflowx-mark-v2.svg',
  version: '1.0.0',
} as const

export const OFFICIAL_LINKS = {
  website: `https://${BRAND_INFO.domain}`,
  twitter: 'https://x.com/alphaflowx',
  x: 'https://x.com/alphaflowx',
  telegram: 'https://t.me/alphaflowx',
  github: 'https://github.com/NoFxAiOS/nofx',
  legacyGithub: 'https://github.com/NoFxAiOS/nofx',
} as const

const BRAND_REPLACEMENTS: Array<[RegExp, string]> = [
  [/\bNOFX\b/g, BRAND_INFO.name],
  [/\bNoFx\b/g, BRAND_INFO.name],
  [/\bnofxai\.com\b/g, BRAND_INFO.domain],
  [/\bnofxos\.ai\b/g, BRAND_INFO.domain],
  [/NOFXENG/g, 'AFXENG'],
]

export function replaceBranding(text: string): string {
  return BRAND_REPLACEMENTS.reduce(
    (current, [pattern, replacement]) => current.replace(pattern, replacement),
    text
  )
}
