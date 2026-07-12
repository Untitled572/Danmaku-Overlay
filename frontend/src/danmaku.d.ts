declare module 'danmaku/dist/esm/danmaku.canvas.js' {
  interface DanmakuComment {
    text: string
    mode?: 'rtl' | 'ltr' | 'top' | 'bottom'
    time?: number
    style?: Record<string, string | number>
    render?: () => HTMLCanvasElement
  }

  interface DanmakuOptions {
    container: HTMLElement
    media?: HTMLMediaElement
    comments?: DanmakuComment[]
    engine?: 'dom' | 'canvas'
    speed?: number
  }

  class Danmaku {
    constructor(options: DanmakuOptions)
    comments: DanmakuComment[]
    speed: number
    emit(comment: DanmakuComment): void
    resize(): void
    show(): void
    hide(): void
    clear(): void
    destroy(): void
  }

  export default Danmaku
}
