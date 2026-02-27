import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'

export function useCerts() {
  return useQuery({
    queryKey: ['certs'],
    queryFn: api.certs,
    staleTime: 30_000,
    refetchInterval: 60_000,
  })
}
