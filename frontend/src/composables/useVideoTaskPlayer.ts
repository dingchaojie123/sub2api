import { onBeforeUnmount, ref } from 'vue'
import axios from 'axios'
import { VideoContentError } from '@/api/videoTasks'

export type VideoPlayerState = 'idle' | 'loading' | 'ready' | 'error'

type ContentFetcher = (id: string, options?: { signal?: AbortSignal }) => Promise<Blob>

/**
 * Loads a video task's result through an authenticated content endpoint and exposes it as an object URL.
 *
 * The bytes are fetched with the session (not a bare <video src>), so the endpoint can stay behind auth.
 * A 404 marks the video as expired upstream (retrying is pointless); a blob the player cannot decode
 * is kept so it can still be downloaded.
 */
export function useVideoTaskPlayer(fetcher: ContentFetcher) {
  const videoUrl = ref<string | null>(null)
  const state = ref<VideoPlayerState>('idle')
  const expired = ref(false)
  let blob: Blob | null = null
  let controller: AbortController | null = null
  let currentId: string | null = null

  function revokeUrl() {
    if (videoUrl.value) URL.revokeObjectURL(videoUrl.value)
    videoUrl.value = null
  }

  function reset() {
    controller?.abort()
    controller = null
    currentId = null
    revokeUrl()
    blob = null
    state.value = 'idle'
    expired.value = false
  }

  async function load(id: string) {
    if (state.value === 'loading' || state.value === 'ready') return
    controller?.abort()
    const own = new AbortController()
    controller = own
    currentId = id
    state.value = 'loading'
    try {
      const fetched = await fetcher(id, { signal: own.signal })
      if (own.signal.aborted || currentId !== id) return
      // Upstreams often answer application/octet-stream, which players handle unreliably for blobs.
      const type = fetched.type.toLowerCase().startsWith('video/') ? fetched.type : 'video/mp4'
      blob = new Blob([fetched], { type })
      videoUrl.value = URL.createObjectURL(blob)
      state.value = 'ready'
    } catch (error) {
      const canceled = axios.isCancel(error) || (error as { code?: string } | null)?.code === 'ERR_CANCELED'
      if (canceled || own.signal.aborted || currentId !== id) return
      expired.value = error instanceof VideoContentError && error.status === 404
      state.value = 'error'
    }
  }

  /** Bytes arrived but the <video> element cannot decode them: drop the URL, keep the blob for download. */
  function onVideoError() {
    revokeUrl()
    state.value = 'error'
  }

  function retry() {
    const id = currentId
    if (!id) return
    reset()
    void load(id)
  }

  /** The loaded blob, or a fresh fetch when the video was never loaded (e.g. download before preview). */
  async function getBlob(id: string): Promise<Blob> {
    return blob ?? fetcher(id)
  }

  onBeforeUnmount(reset)

  return { videoUrl, state, expired, load, reset, retry, onVideoError, getBlob }
}
