import { describe, it, expect } from 'vitest'
import {
  PREVIEW_CSP,
  base64ToBytes,
  imageFormatLabel,
  mayBeImage,
  textPreviewKind,
} from './response-preview'

describe('mayBeImage', () => {
  it('asks the backend for image types and for bodies it has to sniff', () => {
    expect(mayBeImage('image/png')).toBe(true)
    expect(mayBeImage('IMAGE/JPEG; q=1')).toBe(true)
    expect(mayBeImage('')).toBe(true)
    expect(mayBeImage('application/octet-stream')).toBe(true)
  })

  it('leaves other binaries alone', () => {
    expect(mayBeImage('application/pdf')).toBe(false)
    expect(mayBeImage('application/zip')).toBe(false)
    expect(mayBeImage('video/mp4')).toBe(false)
  })
})

describe('textPreviewKind', () => {
  it('trusts the content type', () => {
    expect(textPreviewKind('text/html; charset=utf-8', '{}')).toBe('html')
    expect(textPreviewKind('image/svg+xml', '')).toBe('svg')
    expect(textPreviewKind('application/json', '<html></html>')).toBeNull()
    expect(textPreviewKind('text/plain', '<svg></svg>')).toBeNull()
  })

  it('sniffs when the content type is missing', () => {
    expect(textPreviewKind('', '\n  <!DOCTYPE html><html></html>')).toBe('html')
    expect(textPreviewKind('', '<html lang="en">')).toBe('html')
    expect(textPreviewKind('', '<svg xmlns="http://www.w3.org/2000/svg"/>')).toBe('svg')
    expect(textPreviewKind('', '<?xml version="1.0"?>\n<!-- logo -->\n<svg viewBox="0 0 1 1"></svg>')).toBe('svg')
    expect(textPreviewKind('', '<?xml version="1.0"?>\n<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "x.dtd">\n<svg></svg>')).toBe('svg')
    expect(textPreviewKind('', '<?xml version="1.0"?><feed></feed>')).toBeNull()
    expect(textPreviewKind('', '{"html": "<html>"}')).toBeNull()
    expect(textPreviewKind('', '')).toBeNull()
  })
})

describe('PREVIEW_CSP', () => {
  it('blocks every network fetch and keeps inline styles', () => {
    expect(PREVIEW_CSP).toBe("default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:")
  })
})

describe('imageFormatLabel', () => {
  it('names the format the way people know it', () => {
    expect(imageFormatLabel('image/png')).toBe('PNG')
    expect(imageFormatLabel('image/jpeg')).toBe('JPEG')
    expect(imageFormatLabel('image/svg+xml')).toBe('SVG')
    expect(imageFormatLabel('image/vnd.microsoft.icon')).toBe('ICO')
    expect(imageFormatLabel('image/x-icon')).toBe('ICO')
  })
})

describe('base64ToBytes', () => {
  it('decodes binary data', () => {
    expect(Array.from(base64ToBytes('iVBORw0KGgo='))).toEqual([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])
    expect(base64ToBytes('').length).toBe(0)
  })
})
