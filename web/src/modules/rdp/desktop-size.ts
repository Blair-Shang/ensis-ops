/** 与 rdp-service 的桌面尺寸约束一致。 */
const MIN_EDGE = 200
const MAX_WIDTH = 4096
const MAX_HEIGHT = 2048

function align4(value: number): number {
  if (value < MIN_EDGE) {
    return value
  }
  return (value + 3) & ~3
}

function clampEdge(value: number, max: number): number {
  if (value < MIN_EDGE) {
    return align4(MIN_EDGE)
  }
  if (value > max) {
    return align4(max)
  }
  return align4(value)
}

/**
 * 把画面区域换成远端桌面像素。按 devicePixelRatio 取整，并在超出上限时等比缩小，避免拉变形。
 * 区域还没布局出来时返回 undefined，会话沿用站点里保存的宽高。
 */
export function fitRdpDesktop(
  cssWidth: number,
  cssHeight: number,
  devicePixelRatio: number,
): { width: number; height: number } | undefined {
  if (cssWidth < 32 || cssHeight < 32) {
    return undefined
  }
  const ratio = devicePixelRatio > 0 ? devicePixelRatio : 1
  let width = Math.round(cssWidth * ratio)
  let height = Math.round(cssHeight * ratio)
  const scale = Math.min(1, MAX_WIDTH / width, MAX_HEIGHT / height)
  if (scale < 1) {
    width = Math.round(width * scale)
    height = Math.round(height * scale)
  }
  return {
    width: clampEdge(width, MAX_WIDTH),
    height: clampEdge(height, MAX_HEIGHT),
  }
}
