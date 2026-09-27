import { describe, expect, it } from 'vitest'
import { buttonVariants } from '.'

const VARIANTS = ['default', 'destructive', 'outline', 'secondary', 'ghost', 'link'] as const

describe('button cursor', () => {
  it.each(VARIANTS)('%s shows a pointer, and not-allowed without a hover change when disabled', (variant) => {
    const classes = buttonVariants({ variant }).split(/\s+/)

    expect(classes).toContain('cursor-pointer')
    expect(classes).toContain('disabled:cursor-not-allowed')
    expect(classes).not.toContain('disabled:pointer-events-none')
    if (classes.some((c) => c.startsWith('hover:bg-'))) expect(classes.some((c) => c.startsWith('disabled:hover:bg-'))).toBe(true)
    if (classes.includes('hover:underline')) expect(classes).toContain('disabled:hover:no-underline')
  })
})
