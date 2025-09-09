import { useQuery } from "@tanstack/react-query"
import { queriesService } from "./queries"
import { QueryResponse } from "./chat";

interface UseListQueriesOptions {
  namespace: string;
  getStatus: (query: QueryResponse) => string
}
  
export const useListQueries = ({ namespace, getStatus }: UseListQueriesOptions) => {
  return useQuery({
    queryKey: ['list-all-queries', namespace],
    queryFn: () => queriesService.list(namespace),
    enabled: !!namespace,
    refetchInterval: (query) => {
      const data = query.state.data;

      if (!data) return false;
      return data.items?.some(query => {
        const status = getStatus(query);
        console.log(status)
        return status === "running" || status === "evaluating";
      }) ? 5000 : false;
    }
  })
}