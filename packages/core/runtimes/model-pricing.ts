import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export interface ModelPricing {
  model_code: string;
  input_price: number;
  output_price: number;
  currency: string;
}

export const modelPricingKeys = {
  all: (wsId: string) => ["model-pricing", wsId] as const,
};

export function modelPricingOptions(wsId: string) {
  return queryOptions({
    queryKey: modelPricingKeys.all(wsId),
    queryFn: (): Promise<ModelPricing[]> => api.listLLMModelPricing(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

// Build a lookup map from model_code to pricing info
export function buildPricingMap(pricing: ModelPricing[]): Map<string, ModelPricing> {
  const map = new Map<string, ModelPricing>();
  for (const p of pricing) {
    if (p.input_price > 0 || p.output_price > 0) {
      map.set(p.model_code, p);
    }
  }
  return map;
}
