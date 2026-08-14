package apihealth

import (
	"math/big"
	"strings"
)

func DailyBaseCostUpperBoundUSD(price PriceSnapshot) string {
	input, inputOK := decimalRat(price.InputPricePerMillion)
	output, outputOK := decimalRat(price.OutputPricePerMillion)
	if !inputOK || !outputOK {
		return ""
	}
	input.Mul(input, big.NewRat(ProbeInputTokenUpperBound*SummaryTheoreticalSamples, 1_000_000))
	output.Mul(output, big.NewRat(ProbeOutputTokenLimit*SummaryTheoreticalSamples, 1_000_000))
	return decimalString(new(big.Rat).Add(input, output), 10)
}

func AttemptCostUSD(price PriceSnapshot, usage TokenUsage) string {
	if !usage.Complete() {
		return ""
	}
	inputPrice, inputOK := decimalRat(price.InputPricePerMillion)
	outputPrice, outputOK := decimalRat(price.OutputPricePerMillion)
	if !inputOK || !outputOK {
		return ""
	}
	inputTokens := *usage.InputTokens
	if usage.CachedInputTokens != nil && *usage.CachedInputTokens > 0 {
		cachedTokens := *usage.CachedInputTokens
		if cachedTokens > inputTokens {
			cachedTokens = inputTokens
		}
		inputTokens -= cachedTokens
		if cachedPrice, ok := decimalRat(price.CachedInputPricePerMillion); ok {
			cachedPrice.Mul(cachedPrice, big.NewRat(cachedTokens, 1_000_000))
			inputPrice.Mul(inputPrice, big.NewRat(inputTokens, 1_000_000))
			inputPrice.Add(inputPrice, cachedPrice)
		} else {
			inputTokens += cachedTokens
			inputPrice.Mul(inputPrice, big.NewRat(inputTokens, 1_000_000))
		}
	} else {
		inputPrice.Mul(inputPrice, big.NewRat(inputTokens, 1_000_000))
	}
	outputPrice.Mul(outputPrice, big.NewRat(*usage.OutputTokens, 1_000_000))
	return decimalString(new(big.Rat).Add(inputPrice, outputPrice), 10)
}

func decimalRat(value string) (*big.Rat, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	result, ok := new(big.Rat).SetString(value)
	return result, ok && result.Sign() >= 0
}

func decimalString(value *big.Rat, places int) string {
	if value == nil {
		return ""
	}
	return value.FloatString(places)
}
