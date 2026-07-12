import Danmaku from 'danmaku/dist/esm/danmaku.canvas.js'

export interface DanmakuLine {
  time: number
  text: string
  color: number
  type: number
}

export interface DanmakuConfig {
  fontSize: number
  opacity: number
  speed: number
}

const DEFAULT_CONFIG: DanmakuConfig = {
  fontSize: 24,
  opacity: 1.0,
  speed: 144,
}

function colorToHex(color: number): string {
  return `#${color.toString(16).padStart(6, '0')}`
}

function danmakuTypeToMode(type: number): 'rtl' | 'ltr' | 'top' | 'bottom' {
  if (type === 4) return 'top'
  if (type === 5) return 'bottom'
  return 'rtl'
}

export class DanmakuEngine {
  private danmaku: InstanceType<typeof Danmaku> | null = null
  private container: HTMLElement
  private dummyMedia: HTMLVideoElement
  private config: DanmakuConfig = { ...DEFAULT_CONFIG }
  private allComments: DanmakuLine[] = []
  private lastTime = 0

  constructor(container: HTMLElement) {
    this.container = container

    this.dummyMedia = document.createElement('video')
    this.dummyMedia.style.display = 'none'
    this.dummyMedia.muted = true
    this.dummyMedia.preload = 'auto'
    container.appendChild(this.dummyMedia)
  }

  init() {
    if (this.danmaku) {
      this.danmaku.destroy()
    }

    this.danmaku = new Danmaku({
      container: this.container,
      media: this.dummyMedia,
      engine: 'canvas',
      speed: this.config.speed,
      comments: [],
    })
  }

  loadComments(lines: DanmakuLine[]) {
    this.allComments = lines

    if (!this.danmaku) return

    const comments = lines.map((l) => ({
      text: l.text,
      mode: danmakuTypeToMode(l.type),
      time: l.time,
      style: {
        font: `${this.config.fontSize}px sans-serif`,
        fillStyle: colorToHex(l.color),
        strokeStyle: '#000000',
        lineWidth: 1.0,
      },
    }))

    this.danmaku.comments = comments
    this.dummyMedia.currentTime = 0
  }

  syncTime(position: number, isSeeking: boolean) {
    if (!this.danmaku) return

    const timeDiff = Math.abs(position - this.lastTime)

    if (isSeeking || timeDiff > 2.0) {
      this.seekTo(position)
    } else {
      this.dummyMedia.currentTime = position
    }

    this.lastTime = position
  }

  seekTo(position: number) {
    if (!this.danmaku) return

    this.danmaku.clear()

    const comments = this.allComments
      .filter((l) => l.time >= position)
      .map((l) => ({
        text: l.text,
        mode: danmakuTypeToMode(l.type),
        time: l.time,
        style: {
          font: `${this.config.fontSize}px sans-serif`,
          fillStyle: colorToHex(l.color),
          strokeStyle: '#000000',
          lineWidth: 1.0,
        },
      }))

    this.dummyMedia.currentTime = position
    this.danmaku.comments = comments
  }

  updateConfig(config: Partial<DanmakuConfig>) {
    this.config = { ...this.config, ...config }
    if (this.danmaku) {
      this.danmaku.speed = this.config.speed
    }
  }

  resize() {
    this.danmaku?.resize()
  }

  clear() {
    this.danmaku?.clear()
  }

  destroy() {
    this.danmaku?.destroy()
    this.danmaku = null
    this.dummyMedia.remove()
  }
}
