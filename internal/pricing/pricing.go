package pricing

// ModelPricing 模型定价信息（每百万 token 的价格，单位：美元）
type ModelPricing struct {
	InputPrice       float64 // 输入 token 价格
	OutputPrice      float64 // 输出 token 价格
	CacheWritePrice  float64 // 缓存写入价格（通常等于输入价格）
	CacheReadPrice   float64 // 缓存读取价格（通常比输入价格便宜）
}

// 默认定价表（基于 2025 年 1 月的公开定价）
// 价格单位：美元/百万 token
var defaultPricing = map[string]map[string]ModelPricing{
	// Claude 模型定价
	"claude": {
		// Claude 4 系列
		"claude-sonnet-4-20250514": {
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheWritePrice: 3.75,
			CacheReadPrice:  0.30,
		},
		"claude-sonnet-4-5-20250929": {
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheWritePrice: 3.75,
			CacheReadPrice:  0.30,
		},
		"claude-opus-4-20250514": {
			InputPrice:      15.0,
			OutputPrice:     75.0,
			CacheWritePrice: 18.75,
			CacheReadPrice:  1.50,
		},
		"claude-opus-4-5-20251101": {
			InputPrice:      15.0,
			OutputPrice:     75.0,
			CacheWritePrice: 18.75,
			CacheReadPrice:  1.50,
		},
		// Claude 3.5 系列
		"claude-3-5-sonnet-20241022": {
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheWritePrice: 3.75,
			CacheReadPrice:  0.30,
		},
		"claude-3-5-sonnet-20240620": {
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheWritePrice: 3.75,
			CacheReadPrice:  0.30,
		},
		"claude-3-5-haiku-20241022": {
			InputPrice:      0.80,
			OutputPrice:     4.0,
			CacheWritePrice: 1.0,
			CacheReadPrice:  0.08,
		},
		// Claude 3 系列
		"claude-3-opus-20240229": {
			InputPrice:      15.0,
			OutputPrice:     75.0,
			CacheWritePrice: 18.75,
			CacheReadPrice:  1.50,
		},
		"claude-3-sonnet-20240229": {
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheWritePrice: 3.75,
			CacheReadPrice:  0.30,
		},
		"claude-3-haiku-20240307": {
			InputPrice:      0.25,
			OutputPrice:     1.25,
			CacheWritePrice: 0.30,
			CacheReadPrice:  0.03,
		},
		// 默认 Claude 定价（用于未知模型）
		"default": {
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheWritePrice: 3.75,
			CacheReadPrice:  0.30,
		},
	},
	// OpenAI 模型定价
	"openai": {
		"gpt-4o": {
			InputPrice:      2.50,
			OutputPrice:     10.0,
			CacheWritePrice: 2.50,
			CacheReadPrice:  1.25,
		},
		"gpt-4o-2024-11-20": {
			InputPrice:      2.50,
			OutputPrice:     10.0,
			CacheWritePrice: 2.50,
			CacheReadPrice:  1.25,
		},
		"gpt-4o-2024-08-06": {
			InputPrice:      2.50,
			OutputPrice:     10.0,
			CacheWritePrice: 2.50,
			CacheReadPrice:  1.25,
		},
		"gpt-4o-mini": {
			InputPrice:      0.15,
			OutputPrice:     0.60,
			CacheWritePrice: 0.15,
			CacheReadPrice:  0.075,
		},
		"gpt-4o-mini-2024-07-18": {
			InputPrice:      0.15,
			OutputPrice:     0.60,
			CacheWritePrice: 0.15,
			CacheReadPrice:  0.075,
		},
		"gpt-4-turbo": {
			InputPrice:      10.0,
			OutputPrice:     30.0,
			CacheWritePrice: 10.0,
			CacheReadPrice:  10.0,
		},
		"gpt-4-turbo-2024-04-09": {
			InputPrice:      10.0,
			OutputPrice:     30.0,
			CacheWritePrice: 10.0,
			CacheReadPrice:  10.0,
		},
		"gpt-4": {
			InputPrice:      30.0,
			OutputPrice:     60.0,
			CacheWritePrice: 30.0,
			CacheReadPrice:  30.0,
		},
		"gpt-3.5-turbo": {
			InputPrice:      0.50,
			OutputPrice:     1.50,
			CacheWritePrice: 0.50,
			CacheReadPrice:  0.50,
		},
		"o1": {
			InputPrice:      15.0,
			OutputPrice:     60.0,
			CacheWritePrice: 15.0,
			CacheReadPrice:  7.50,
		},
		"o1-2024-12-17": {
			InputPrice:      15.0,
			OutputPrice:     60.0,
			CacheWritePrice: 15.0,
			CacheReadPrice:  7.50,
		},
		"o1-mini": {
			InputPrice:      3.0,
			OutputPrice:     12.0,
			CacheWritePrice: 3.0,
			CacheReadPrice:  1.50,
		},
		"o1-mini-2024-09-12": {
			InputPrice:      3.0,
			OutputPrice:     12.0,
			CacheWritePrice: 3.0,
			CacheReadPrice:  1.50,
		},
		"o3-mini": {
			InputPrice:      1.10,
			OutputPrice:     4.40,
			CacheWritePrice: 1.10,
			CacheReadPrice:  0.55,
		},
		"o3-mini-2025-01-31": {
			InputPrice:      1.10,
			OutputPrice:     4.40,
			CacheWritePrice: 1.10,
			CacheReadPrice:  0.55,
		},
		// 默认 OpenAI 定价
		"default": {
			InputPrice:      2.50,
			OutputPrice:     10.0,
			CacheWritePrice: 2.50,
			CacheReadPrice:  1.25,
		},
	},
	// OpenAI2 (Responses API) 使用相同定价
	"openai2": {
		"gpt-4o": {
			InputPrice:      2.50,
			OutputPrice:     10.0,
			CacheWritePrice: 2.50,
			CacheReadPrice:  1.25,
		},
		"gpt-4o-mini": {
			InputPrice:      0.15,
			OutputPrice:     0.60,
			CacheWritePrice: 0.15,
			CacheReadPrice:  0.075,
		},
		"o3-mini": {
			InputPrice:      1.10,
			OutputPrice:     4.40,
			CacheWritePrice: 1.10,
			CacheReadPrice:  0.55,
		},
		"default": {
			InputPrice:      2.50,
			OutputPrice:     10.0,
			CacheWritePrice: 2.50,
			CacheReadPrice:  1.25,
		},
	},
	// Gemini 模型定价
	"gemini": {
		"gemini-2.0-flash": {
			InputPrice:      0.10,
			OutputPrice:     0.40,
			CacheWritePrice: 0.10,
			CacheReadPrice:  0.025,
		},
		"gemini-2.0-flash-exp": {
			InputPrice:      0.0, // 免费
			OutputPrice:     0.0,
			CacheWritePrice: 0.0,
			CacheReadPrice:  0.0,
		},
		"gemini-1.5-pro": {
			InputPrice:      1.25,
			OutputPrice:     5.0,
			CacheWritePrice: 1.25,
			CacheReadPrice:  0.3125,
		},
		"gemini-1.5-pro-002": {
			InputPrice:      1.25,
			OutputPrice:     5.0,
			CacheWritePrice: 1.25,
			CacheReadPrice:  0.3125,
		},
		"gemini-1.5-flash": {
			InputPrice:      0.075,
			OutputPrice:     0.30,
			CacheWritePrice: 0.075,
			CacheReadPrice:  0.01875,
		},
		"gemini-1.5-flash-002": {
			InputPrice:      0.075,
			OutputPrice:     0.30,
			CacheWritePrice: 0.075,
			CacheReadPrice:  0.01875,
		},
		"gemini-1.5-flash-8b": {
			InputPrice:      0.0375,
			OutputPrice:     0.15,
			CacheWritePrice: 0.0375,
			CacheReadPrice:  0.01,
		},
		// 默认 Gemini 定价
		"default": {
			InputPrice:      0.075,
			OutputPrice:     0.30,
			CacheWritePrice: 0.075,
			CacheReadPrice:  0.01875,
		},
	},
}

// GetPricing 获取指定转换器和模型的定价
// transformer: claude, openai, openai2, gemini
// model: 模型名称
func GetPricing(transformer, model string) ModelPricing {
	// 获取转换器的定价表
	transformerPricing, ok := defaultPricing[transformer]
	if !ok {
		// 未知转换器，使用 Claude 默认定价
		return defaultPricing["claude"]["default"]
	}

	// 查找具体模型的定价
	if pricing, ok := transformerPricing[model]; ok {
		return pricing
	}

	// 尝试模糊匹配（处理带日期后缀的模型名）
	for modelPrefix, pricing := range transformerPricing {
		if modelPrefix != "default" && len(model) >= len(modelPrefix) {
			if model[:len(modelPrefix)] == modelPrefix {
				return pricing
			}
		}
	}

	// 返回该转换器的默认定价
	if defaultPrice, ok := transformerPricing["default"]; ok {
		return defaultPrice
	}

	// 最终回退到 Claude 默认定价
	return defaultPricing["claude"]["default"]
}

// CalculateCost 计算成本（单位：美元）
// inputTokens: 输入 token 数
// outputTokens: 输出 token 数
// cacheWriteTokens: 缓存写入 token 数
// cacheReadTokens: 缓存读取 token 数
// pricing: 定价信息
func CalculateCost(inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens int, pricing ModelPricing) float64 {
	// 价格是每百万 token，所以需要除以 1,000,000
	inputCost := float64(inputTokens) * pricing.InputPrice / 1_000_000
	outputCost := float64(outputTokens) * pricing.OutputPrice / 1_000_000
	cacheWriteCost := float64(cacheWriteTokens) * pricing.CacheWritePrice / 1_000_000
	cacheReadCost := float64(cacheReadTokens) * pricing.CacheReadPrice / 1_000_000

	return inputCost + outputCost + cacheWriteCost + cacheReadCost
}

// CalculateCostSimple 简化的成本计算（不区分缓存类型）
func CalculateCostSimple(inputTokens, outputTokens int, pricing ModelPricing) float64 {
	return CalculateCost(inputTokens, outputTokens, 0, 0, pricing)
}

// CostBreakdown 成本明细
type CostBreakdown struct {
	InputCost      float64 `json:"inputCost"`
	OutputCost     float64 `json:"outputCost"`
	CacheWriteCost float64 `json:"cacheWriteCost"`
	CacheReadCost  float64 `json:"cacheReadCost"`
	TotalCost      float64 `json:"totalCost"`
}

// CalculateCostBreakdown 计算成本明细
func CalculateCostBreakdown(inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens int, pricing ModelPricing) CostBreakdown {
	inputCost := float64(inputTokens) * pricing.InputPrice / 1_000_000
	outputCost := float64(outputTokens) * pricing.OutputPrice / 1_000_000
	cacheWriteCost := float64(cacheWriteTokens) * pricing.CacheWritePrice / 1_000_000
	cacheReadCost := float64(cacheReadTokens) * pricing.CacheReadPrice / 1_000_000

	return CostBreakdown{
		InputCost:      inputCost,
		OutputCost:     outputCost,
		CacheWriteCost: cacheWriteCost,
		CacheReadCost:  cacheReadCost,
		TotalCost:      inputCost + outputCost + cacheWriteCost + cacheReadCost,
	}
}

// GetAllPricing 获取所有定价信息（用于前端展示）
func GetAllPricing() map[string]map[string]ModelPricing {
	return defaultPricing
}

// GetTransformerPricing 获取指定转换器的所有定价
func GetTransformerPricing(transformer string) map[string]ModelPricing {
	if pricing, ok := defaultPricing[transformer]; ok {
		return pricing
	}
	return nil
}
