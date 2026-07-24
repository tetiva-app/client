import { HighlightStyle } from '@codemirror/language'
import { tags } from '@lezer/highlight'

// One Dark Pro inspired — for the dark theme.
export const darkHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: '#c678dd' },
  { tag: tags.operator, color: '#56b6c2' },
  { tag: tags.string, color: '#98c379' },
  { tag: tags.number, color: '#d19a66' },
  { tag: tags.bool, color: '#d19a66' },
  { tag: tags.null, color: '#d19a66' },
  { tag: tags.propertyName, color: '#e06c75' },
  { tag: tags.punctuation, color: '#abb2bf' },
  { tag: tags.bracket, color: '#abb2bf' },
  { tag: tags.typeName, color: '#e5c07b' },
  { tag: tags.className, color: '#e5c07b' },
  { tag: tags.definition(tags.variableName), color: '#61afef' },
  { tag: tags.tagName, color: '#e06c75' },
  { tag: tags.attributeName, color: '#d19a66' },
  { tag: tags.attributeValue, color: '#98c379' },
  { tag: tags.comment, color: '#5c6370', fontStyle: 'italic' },
  { tag: tags.content, color: '#abb2bf' },
])

// GitHub light syntax palette — high contrast on the white light-theme background.
export const lightHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: '#CF222E' },
  { tag: tags.operator, color: '#0550AE' },
  { tag: tags.string, color: '#0A3069' },
  { tag: tags.number, color: '#0550AE' },
  { tag: tags.bool, color: '#0550AE' },
  { tag: tags.null, color: '#0550AE' },
  { tag: tags.propertyName, color: '#953800' },
  { tag: tags.punctuation, color: '#1F2328' },
  { tag: tags.bracket, color: '#1F2328' },
  { tag: tags.typeName, color: '#6F42C1' },
  { tag: tags.className, color: '#6F42C1' },
  { tag: tags.definition(tags.variableName), color: '#0550AE' },
  { tag: tags.tagName, color: '#116329' },
  { tag: tags.attributeName, color: '#953800' },
  { tag: tags.attributeValue, color: '#0A3069' },
  { tag: tags.comment, color: '#6E7781', fontStyle: 'italic' },
  { tag: tags.content, color: '#1F2328' },
])
