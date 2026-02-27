import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'

export function usePipelines() {
  return useQuery({
    queryKey: ['pipelines'],
    queryFn: api.pipelines,
    staleTime: 10_000,
    refetchInterval: 15_000,
  })
}

export function usePipeline(id: string) {
  return useQuery({
    queryKey: ['pipelines', id],
    queryFn: () => api.pipeline(id),
    staleTime: 10_000,
    refetchInterval: 15_000,
    enabled: Boolean(id),
  })
}
