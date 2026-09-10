import { describe, expect, it } from 'vitest'
import { dropdownPlacement, placementStyle } from './dropdown-position'

const anchor = { top: 200, bottom: 232, left: 340, width: 240 }

describe('dropdownPlacement', () => {
  it('opens below when the menu fits there', () => {
    const placement = dropdownPlacement(anchor, 900)

    expect(placement.top).toBe(236)
    expect(placement.bottom).toBeUndefined()
    expect(placement.maxHeight).toBe(240)
    expect(placement.left).toBe(340)
    expect(placement.width).toBe(240)
  })

  it('flips above when the room below is the smaller half', () => {
    const placement = dropdownPlacement(anchor, 300)

    expect(placement.top).toBeUndefined()
    expect(placement.bottom).toBe(104)
    expect(placement.maxHeight).toBe(196)
  })

  it('caps the height to the room below when it cannot flip', () => {
    const placement = dropdownPlacement({ top: 40, bottom: 72, left: 0, width: 200 }, 200)

    expect(placement.top).toBe(76)
    expect(placement.maxHeight).toBe(124)
  })

  it('never grows past the edge it is anchored to', () => {
    const cramped = { top: 30, bottom: 62, left: 0, width: 200 }
    const placement = dropdownPlacement(cramped, 90)

    // Flipped onto the larger half and capped there, so the menu stays on screen.
    expect(placement.bottom).toBe(64)
    expect(placement.maxHeight).toBe(26)
  })

  it('never grows past the cap', () => {
    const placement = dropdownPlacement(anchor, 4000, { maxHeight: 120 })

    expect(placement.maxHeight).toBe(120)
  })
})

describe('placementStyle', () => {
  it('pins the menu in viewport coordinates', () => {
    expect(placementStyle(dropdownPlacement(anchor, 900))).toEqual({
      position: 'fixed',
      left: '340px',
      width: '240px',
      maxHeight: '240px',
      top: '236px',
    })
  })

  it('anchors a flipped menu by its bottom edge', () => {
    const style = placementStyle(dropdownPlacement(anchor, 300))

    expect(style.bottom).toBe('104px')
    expect(style.top).toBeUndefined()
  })
})
