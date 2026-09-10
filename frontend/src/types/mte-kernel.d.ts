// The package ships no types; only the subset the description editor uses is declared here.
declare module '@susisu/mte-kernel' {
  export class Point {
    constructor(row: number, column: number)
    readonly row: number
    readonly column: number
    equals(other: Point): boolean
  }
  export class Range {
    constructor(start: Point, end: Point)
    readonly start: Point
    readonly end: Point
  }
  export class Focus {
    constructor(row: number, column: number, offset: number)
    readonly row: number
    readonly column: number
    readonly offset: number
    setRow(row: number): Focus
  }
  export type AlignmentValue = 'none' | 'left' | 'right' | 'center'
  export const Alignment: { readonly NONE: 'none'; readonly LEFT: 'left'; readonly RIGHT: 'right'; readonly CENTER: 'center' }
  export const DefaultAlignment: { readonly LEFT: 'left'; readonly RIGHT: 'right'; readonly CENTER: 'center' }
  export const HeaderAlignment: { readonly FOLLOW: 'follow'; readonly LEFT: 'left'; readonly RIGHT: 'right'; readonly CENTER: 'center' }
  export const FormatType: { readonly NORMAL: 'normal'; readonly WEAK: 'weak' }
  export interface TextWidthOptions {
    normalize: boolean
    wideChars: Set<string>
    narrowChars: Set<string>
    ambiguousAsWide: boolean
  }
  export interface Options {
    leftMarginChars: Set<string>
    formatType: 'normal' | 'weak'
    minDelimiterWidth: number
    defaultAlignment: 'left' | 'right' | 'center'
    headerAlignment: 'follow' | 'left' | 'right' | 'center'
    smartCursor: boolean
    textWidthOptions: TextWidthOptions
  }
  export function options(obj?: Partial<Omit<Options, 'textWidthOptions'>> & { textWidthOptions?: Partial<TextWidthOptions> }): Options

  export class TableCell {
    constructor(rawContent: string)
    readonly content: string
    toText(): string
  }
  export class TableRow {
    constructor(cells: TableCell[], marginLeft: string, marginRight: string)
    getWidth(): number
    getCells(): TableCell[]
    toText(): string
    isDelimiter(): boolean
  }
  export class Table {
    constructor(rows: TableRow[])
    getHeight(): number
    getWidth(): number
    getHeaderWidth(): number
    getRows(): TableRow[]
    toLines(): string[]
    focusOfPosition(pos: Point, rowOffset: number): Focus | undefined
  }
  export function readTable(lines: string[], options: Options): Table
  export function completeTable(table: Table, options: Options): { table: Table; delimiterInserted: boolean }
  export function formatTable(table: Table, options: Options): { table: Table; marginLeft: string }

  export abstract class ITextEditor {
    abstract getCursorPosition(): Point
    abstract setCursorPosition(pos: Point): void
    abstract setSelectionRange(range: Range): void
    abstract getLastRow(): number
    abstract acceptsTableEdit(row: number): boolean
    abstract getLine(row: number): string
    abstract insertLine(row: number, line: string): void
    abstract deleteLine(row: number): void
    abstract replaceLines(startRow: number, endRow: number, lines: string[]): void
    abstract transact(func: () => void): void
  }

  export class TableEditor {
    constructor(textEditor: ITextEditor)
    resetSmartCursor(): void
    cursorIsInTable(options: Options): boolean
    format(options: Options): void
    escape(options: Options): void
    alignColumn(alignment: AlignmentValue, options: Options): void
    selectCell(options: Options): void
    moveFocus(rowOffset: number, columnOffset: number, options: Options): void
    nextCell(options: Options): void
    previousCell(options: Options): void
    nextRow(options: Options): void
    insertRow(options: Options): void
    deleteRow(options: Options): void
    moveRow(offset: number, options: Options): void
    insertColumn(options: Options): void
    deleteColumn(options: Options): void
    moveColumn(offset: number, options: Options): void
    formatAll(options: Options): void
  }
}
