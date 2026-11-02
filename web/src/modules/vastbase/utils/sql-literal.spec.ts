import { describe, expect, it } from 'vitest'
import { toSqlLiteral } from './sql-literal'

describe('toSqlLiteral', () => {
  it('quotes text and booleans without a cast', () => {
    expect(toSqlLiteral(null)).toBe('NULL')
    expect(toSqlLiteral(true)).toBe('TRUE')
    expect(toSqlLiteral("a'b")).toBe("'a''b'")
    expect(toSqlLiteral('hello', 'varchar2(4000)')).toBe("'hello'")
    expect(toSqlLiteral('hello', 'text')).toBe("'hello'")
  })

  it('casts CLOB and NCLOB assignments so Vastbase stores the value', () => {
    expect(toSqlLiteral('正文', 'clob')).toBe("'正文'::clob")
    expect(toSqlLiteral('正文', 'CLOB')).toBe("'正文'::clob")
    expect(toSqlLiteral('national', 'nclob')).toBe("'national'::nclob")
    expect(toSqlLiteral(null, 'clob')).toBe('NULL')
  })
})