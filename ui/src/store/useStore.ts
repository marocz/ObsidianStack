import { create } from 'zustand'
import type { SnapshotResponse, WsMessage } from '../api/types'

interface AppState {
  /** Latest snapshot received via WebSocket. Null until first message arrives. */
  liveSnapshot: SnapshotResponse | null

  /** True while the WebSocket connection is open. */
  wsConnected: boolean

  setLiveSnapshot: (msg: WsMessage) => void
  setWsConnected: (connected: boolean) => void
}

export const useStore = create<AppState>((set) => ({
  liveSnapshot: null,
  wsConnected: false,
  setLiveSnapshot: (msg) => set({ liveSnapshot: msg.data }),
  setWsConnected: (wsConnected) => set({ wsConnected }),
}))
