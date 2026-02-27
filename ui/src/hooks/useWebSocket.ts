import { useEffect, useRef } from 'react'
import type { WsMessage } from '../api/types'
import { useStore } from '../store/useStore'

const WS_URL = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/stream`
const RECONNECT_DELAY_MS = 3_000

export function useWebSocket() {
  const setLiveSnapshot = useStore((s) => s.setLiveSnapshot)
  const setWsConnected = useStore((s) => s.setWsConnected)
  const wsRef = useRef<WebSocket | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    let cancelled = false

    function connect() {
      if (cancelled) return
      const ws = new WebSocket(WS_URL)
      wsRef.current = ws

      ws.onopen = () => {
        if (!cancelled) setWsConnected(true)
      }

      ws.onmessage = (ev) => {
        if (cancelled) return
        try {
          const msg = JSON.parse(ev.data as string) as WsMessage
          if (msg.event === 'snapshot') setLiveSnapshot(msg)
        } catch {
          // malformed message — ignore
        }
      }

      ws.onclose = () => {
        if (cancelled) return
        setWsConnected(false)
        timerRef.current = setTimeout(connect, RECONNECT_DELAY_MS)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      cancelled = true
      if (timerRef.current) clearTimeout(timerRef.current)
      wsRef.current?.close()
      setWsConnected(false)
    }
  }, [setLiveSnapshot, setWsConnected])
}
