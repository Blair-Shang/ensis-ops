import { ref, type Ref } from 'vue'
import type { RdpSessionInfo } from '@/api/types/rdp'

/**
 * 把 rdp-service 的本机 WebSocket 画到 canvas。
 * 二进制帧：type=2 桌面尺寸，type=1 脏矩形（RGBA）。
 *
 * @param canvas - 承载桌面的画布
 */
export function useRdpSurface(canvas: Ref<HTMLCanvasElement | null>) {
  const frameConnected = ref(false)
  let socket: WebSocket | null = null
  let image: ImageData | null = null
  let paintFrame = 0
  let dirty = false

  function paint(): void {
    paintFrame = 0
    const element = canvas.value
    if (!element || !image || !dirty) {
      return
    }
    const context = element.getContext('2d')
    if (!context) {
      return
    }
    if (element.width !== image.width || element.height !== image.height) {
      element.width = image.width
      element.height = image.height
    }
    context.putImageData(image, 0, 0)
    dirty = false
  }

  function schedulePaint(): void {
    dirty = true
    if (paintFrame === 0) {
      paintFrame = requestAnimationFrame(paint)
    }
  }

  function ensureImage(width: number, height: number): void {
    if (width <= 0 || height <= 0) {
      return
    }
    if (image && image.width === width && image.height === height) {
      return
    }
    image = new ImageData(width, height)
    schedulePaint()
  }

  function applyRect(buffer: ArrayBuffer): void {
    if (!image || buffer.byteLength < 9) {
      return
    }
    const view = new DataView(buffer)
    const x = view.getUint16(1, true)
    const y = view.getUint16(3, true)
    const width = view.getUint16(5, true)
    const height = view.getUint16(7, true)
    const pixels = new Uint8ClampedArray(buffer, 9)
    if (width === 0 || height === 0 || pixels.length < width * height * 4) {
      return
    }
    for (let row = 0; row < height; row += 1) {
      const destY = y + row
      if (destY < 0 || destY >= image.height || x >= image.width) {
        continue
      }
      const copyWidth = Math.min(width, image.width - Math.max(0, x))
      if (copyWidth <= 0) {
        continue
      }
      const srcOffset = row * width * 4
      const destOffset = (destY * image.width + Math.max(0, x)) * 4
      image.data.set(pixels.subarray(srcOffset, srcOffset + copyWidth * 4), destOffset)
    }
    schedulePaint()
  }

  function onMessage(event: MessageEvent): void {
    if (!(event.data instanceof ArrayBuffer) || event.data.byteLength < 1) {
      return
    }
    const view = new DataView(event.data)
    const type = view.getUint8(0)
    if (type === 2 && event.data.byteLength >= 5) {
      ensureImage(view.getUint16(1, true), view.getUint16(3, true))
      return
    }
    if (type === 1) {
      applyRect(event.data)
    }
  }

  /**
   * 连上画面通道。重复调用会先关掉上一条。
   *
   * @param info - session.info 返回的本机地址和令牌
   */
  function connectFrame(info: RdpSessionInfo): void {
    disconnectFrame()
    const url = `ws://${info.frameHost}:${info.framePort}/?token=${encodeURIComponent(info.frameToken)}`
    const ws = new WebSocket(url)
    ws.binaryType = 'arraybuffer'
    socket = ws
    ws.onopen = () => {
      if (socket === ws) {
        frameConnected.value = true
      }
    }
    ws.onclose = () => {
      if (socket === ws) {
        frameConnected.value = false
      }
    }
    ws.onmessage = onMessage
  }

  /** 关闭画面通道，不关闭 RDP 会话。 */
  function disconnectFrame(): void {
    frameConnected.value = false
    socket?.close()
    socket = null
    if (paintFrame !== 0) {
      cancelAnimationFrame(paintFrame)
      paintFrame = 0
    }
  }

  /**
   * 把鼠标事件换算成远端像素。画面铺满画布，按比例映射。
   *
   * @param event - 画布上的鼠标事件
   */
  function remotePoint(event: MouseEvent): { x: number; y: number } | null {
    const element = canvas.value
    if (!element || !image) {
      return null
    }
    const rect = element.getBoundingClientRect()
    if (rect.width <= 0 || rect.height <= 0) {
      return null
    }
    const x = ((event.clientX - rect.left) / rect.width) * image.width
    const y = ((event.clientY - rect.top) / rect.height) * image.height
    if (x < 0 || y < 0 || x >= image.width || y >= image.height) {
      return null
    }
    return { x: Math.floor(x), y: Math.floor(y) }
  }

  return { frameConnected, connectFrame, disconnectFrame, remotePoint }
}
