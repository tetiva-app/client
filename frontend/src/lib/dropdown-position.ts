export interface AnchorRect {
  top: number
  bottom: number
  left: number
  width: number
}

export interface DropdownPlacement {
  left: number
  width: number
  maxHeight: number
  top?: number
  bottom?: number
}

export interface DropdownOptions {
  gap?: number
  maxHeight?: number
}

const DEFAULT_GAP = 4
const DEFAULT_MAX_HEIGHT = 240

// A menu anchored inside the request editor is clipped by that pane, so it is
// placed in viewport coordinates instead: above the anchor when the room below
// is the smaller half, and always capped so the list scrolls on its own.
export function dropdownPlacement(
  anchor: AnchorRect,
  viewportHeight: number,
  options: DropdownOptions = {},
): DropdownPlacement {
  const gap = options.gap ?? DEFAULT_GAP
  const max = options.maxHeight ?? DEFAULT_MAX_HEIGHT

  const below = viewportHeight - anchor.bottom - gap
  const above = anchor.top - gap
  // Flipping on the smaller half also picks the larger side when neither can
  // hold the whole list; the height then follows that side, because a flipped
  // menu grows upward from its bottom edge and would run past the viewport.
  const flip = below < Math.min(max, above)
  const room = Math.max(Math.floor(flip ? above : below), 0)

  const placement: DropdownPlacement = {
    left: Math.round(anchor.left),
    width: Math.round(anchor.width),
    maxHeight: Math.min(max, room),
  }
  if (flip) {
    placement.bottom = Math.round(viewportHeight - anchor.top + gap)
  } else {
    placement.top = Math.round(anchor.bottom + gap)
  }

  return placement
}

// The style a fixed-position menu gets from a placement.
export function placementStyle(placement: DropdownPlacement): Record<string, string> {
  const style: Record<string, string> = {
    position: 'fixed',
    left: `${placement.left}px`,
    width: `${placement.width}px`,
    maxHeight: `${placement.maxHeight}px`,
  }
  if (placement.top !== undefined) style.top = `${placement.top}px`
  if (placement.bottom !== undefined) style.bottom = `${placement.bottom}px`

  return style
}
