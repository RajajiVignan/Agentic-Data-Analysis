
import React, { createContext, useContext, useState, useCallback, useMemo } from "react";

export type FilterState = {
  column: string;
  value: string;
  operator: string;
};

type DashboardFilterContextType = {
  filters: FilterState[];
  addFilter: (filter: FilterState) => void;
  removeFilter: (column: string) => void;
  clearFilters: () => void;
  isActive: (column: string, value: string) => boolean;
};

const DashboardFilterContext = createContext<DashboardFilterContextType>({
  filters: [],
  addFilter: () => {},
  removeFilter: () => {},
  clearFilters: () => {},
  isActive: () => false,
});

export function DashboardFilterProvider({ children }: { children: React.ReactNode }) {
  const [filters, setFilters] = useState<FilterState[]>([]);

  const addFilter = useCallback((filter: FilterState) => {
    setFilters((prev) => {
      const exists = prev.some((f) => f.column === filter.column && f.value === filter.value);
      if (exists) {
        return prev.filter((f) => !(f.column === filter.column && f.value === filter.value));
      }
      const otherCols = prev.filter((f) => f.column !== filter.column);
      return [...otherCols, filter];
    });
  }, []);

  const removeFilter = useCallback((column: string) => {
    setFilters((prev) => prev.filter((f) => f.column !== column));
  }, []);

  const clearFilters = useCallback(() => {
    setFilters([]);
  }, []);

  const isActive = useCallback((column: string, value: string) => {
    return filters.some((f) => f.column === column && f.value === value);
  }, [filters]);

  const value = useMemo(() => ({ filters, addFilter, removeFilter, clearFilters, isActive }), [filters, addFilter, removeFilter, clearFilters, isActive]);

  return (
    <DashboardFilterContext.Provider value={value}>
      {children}
    </DashboardFilterContext.Provider>
  );
}

export function useDashboardFilter() {
  return useContext(DashboardFilterContext);
}
