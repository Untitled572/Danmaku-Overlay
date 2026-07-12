import { ref } from 'vue'

const searchQuery = ref('')

export function useSearch() {
  const setSearchQuery = (query: string) => {
    searchQuery.value = query
  }

  return {
    searchQuery,
    setSearchQuery,
  }
}
