import { normalizeSqlDataType } from '@/modules/database/utils/column-value-type'

export { parseEditValue } from '@/modules/database/utils/sql-literal'

/**
 * 列类型是否要按 CLOB / NCLOB 赋值。
 * Vastbase Oracle 兼容走扩展协议时，无类型字符串赋给 CLOB 会执行成功但值不变，SET/INSERT 必须显式转型。
 */
function clobAssignCast(dataType?: string): 'clob' | 'nclob' | null {
  const t = normalizeSqlDataType(dataType)
  if (t === 'nclob') return 'nclob'
  if (t === 'clob' || t === 'character large object') return 'clob'
  return null
}

/**
 * SQL 字面量拼装（浏览行编辑 / INSERT / WHERE）。
 * PG / VastBase 布尔用 TRUE/FALSE。传入 CLOB/NCLOB 的 dataType 时，非 NULL 字面量追加 `::clob` / `::nclob`。
 *
 * @param value - 单元格原始值
 * @param dataType - 列类型；仅赋值语句需要，WHERE 不要传
 * @returns 可嵌入 SQL 的字面量
 */
export function toSqlLiteral(value: unknown, dataType?: string): string {
  const literal = sqlLiteral(value)
  const cast = literal === 'NULL' ? null : clobAssignCast(dataType)
  return cast ? `${literal}::${cast}` : literal
}

function sqlLiteral(value: unknown): string {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'boolean') return value ? 'TRUE' : 'FALSE'
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  if (typeof value === 'bigint') return value.toString()
  if (typeof value === 'object') {
    return `'${JSON.stringify(value).replace(/'/g, "''")}'`
  }
  const s = String(value)
  if (s.toUpperCase() === 'NULL') return 'NULL'
  return `'${s.replace(/'/g, "''")}'`
}
