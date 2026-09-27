// Node's global navigator reports the machine locale; tests must not depend on it.
if (typeof globalThis.navigator !== 'undefined') {
  Object.defineProperty(globalThis.navigator, 'language', { value: 'en-US', configurable: true })
}

export {}
