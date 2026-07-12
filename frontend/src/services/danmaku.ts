import { api } from './api'

export interface Series {
  ID: string
  BangumiID?: number
  Title: string
  NameCN?: string
  CoverPath?: string
  TotalEps?: number
  CurrentEp?: number
  AirDate?: string
  Rating?: number
  Tags?: string
  Summary?: string
  LastPlayedAt?: string
}

export interface Episode {
  ID: string
  SeriesID: string
  LibraryID: number
  DandanEpisodeID: number
  RelativePath: string
  FileMD5: string
  FileHash: string
  DanmakuPath?: string
  EpIndex?: number
  MatchStatus: string
  ScrapeStatus: string
  WatchProgress: number
  LastPlayedAt?: string
}

export interface History {
  ID: number
  UserID: number
  EpisodeID: string
  Position: number
  UpdatedAt?: string
}

export interface Library {
  ID: number
  RootPath: string
}

export interface DanmakuLine {
  time: number
  text: string
  color: number
  type: number
}

export interface Settings {
  [key: string]: unknown
}

export const danmakuApi = {
  getSeries(search?: string): Promise<Series[]> {
    const params: Record<string, string> = {}
    if (search) params.q = search
    return api.get<{ series: Series[]; total: number }>('/search', params).then(res => res.series || [])
  },

  getEpisodes(seriesId?: string): Promise<Episode[]> {
    const params: Record<string, string> = {}
    if (seriesId !== undefined) params.series_id = seriesId
    return api.get<Episode[]>('/episodes', params)
  },

  getDanmaku(episodeId: string): Promise<DanmakuLine[]> {
    return api.get<DanmakuLine[]>(`/episodes/${episodeId}/danmaku`)
  },

  matchEpisode(episodeId: string): Promise<{
    episode_id: string
    dandan_episode_id: number
    danmaku_path?: string
  }> {
    return api.post(`/episodes/${episodeId}/match`)
  },

  getProgress(episodeId?: string): Promise<History[]> {
    const params: Record<string, string> = {}
    if (episodeId !== undefined) params.episode_id = episodeId
    return api.get<History[]>('/progress', params)
  },

  updateProgress(episodeId: string, position: number, duration?: number): Promise<{ ok: boolean }> {
    return api.post('/progress', { episode_id: episodeId, position, duration })
  },

  triggerScan(): Promise<{ message: string }> {
    return api.post('/scan')
  },

  triggerScrape(): Promise<{ message: string }> {
    return api.post('/scrape')
  },

  getSettings(): Promise<Settings> {
    return api.get<Settings>('/settings')
  },

  updateSettings(settings: Settings): Promise<{ ok: boolean }> {
    return api.put('/settings', settings)
  },

  getLibraries(): Promise<Library[]> {
    return api.get<Library[]>('/library')
  },

  createLibrary(rootPath: string): Promise<Library> {
    return api.post('/library', { root_path: rootPath })
  },

  deleteLibrary(id: number | string): Promise<{ ok: boolean }> {
    return api.delete<{ ok: boolean }>('/library', { id: String(id) })
  },

  getTaskStatus(): Promise<any> {
    return api.get<any>('/status')
  },

  checkHealth(): Promise<{ status: string; database: string }> {
    return api.get<{ status: string; database: string }>('/health')
  },

  searchBangumi(keyword: string): Promise<any[]> {
    return api.get<any[]>('/bangumi/search', { keyword })
  },

  bindBangumi(seriesId: string | number, bangumiId: number): Promise<{ series_id: string | number; bangumi_id: number }> {
    return api.post(`/series/${seriesId}/bind`, { bangumi_id: bangumiId })
  },

  getLibraryFiles(libraryId: number | string): Promise<any[]> {
    return api.get<any[]>('/library/files', { library_id: String(libraryId) })
  },

  playEpisode(episodeId: string | number): Promise<any> {
    return api.get<any>('/play', { episode_id: String(episodeId) })
  },
}
