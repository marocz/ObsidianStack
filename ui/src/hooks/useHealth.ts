import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: api.health,
    staleTime: 10_000,
    refetchInterval: 15_000,
  })
}
