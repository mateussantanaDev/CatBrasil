package domain

import (
	"context"
	"math"
)

// DREMetrics consolida as métricas do Demonstrativo do Resultado do Exercício em tempo real
type DREMetrics struct {
	TotalDeposits          float64 `json:"totalDeposits"`
	GatewayFeePercent      float64 `json:"gatewayFeePercent"`
	GatewayFees            float64 `json:"gatewayFees"`
	NetDeposits            float64 `json:"netDeposits"`
	TotalWithdrawals       float64 `json:"totalWithdrawals"`
	NetBankProfit          float64 `json:"netBankProfit"`
	ProfitMargin           float64 `json:"profitMargin"`
	TotalUserBalances      float64 `json:"totalUserBalances"`
	LiquidityCoverageRatio float64 `json:"liquidityCoverageRatio"`
	RetainedExpiredYields  float64 `json:"retainedExpiredYields"`
	RetainedUnspunPrizes   float64 `json:"retainedUnspunPrizes"`
	EmpiricalForgetRate    float64 `json:"empiricalForgetRate"`
	TotalRoulettePrizes    float64 `json:"totalRoulettePrizes"`
	ExpiredCollectCount    int     `json:"expiredCollectCount"`
	SuccessCollectCount    int     `json:"successCollectCount"`
}

// CalculateDREMetrics executa a formulação matemática oficial do DRE CatBrasil v3.0:
// 1. gatewayFees = totalDeposits * (depositFeePercent / 100)
// 2. netDeposits = totalDeposits - gatewayFees
// 3. netBankProfit = netDeposits - totalWithdrawals
// 4. profitMargin = (netBankProfit / totalDeposits) * 100
// 5. liquidityCoverageRatio = (netBankProfit / totalUserBalances) * 100
// 6. empiricalForgetRate = (expiredCollectCount / (expiredCollectCount + successCollectCount)) * 100
func CalculateDREMetrics(
	totalDeposits, depositFeePercent, totalWithdrawals, totalUserBalances,
	retainedExpiredYields, retainedUnspunPrizes, totalRoulettePrizes float64,
	expiredCollectCount, successCollectCount int,
) *DREMetrics {
	gatewayFees := math.Round((totalDeposits*(depositFeePercent/100.0))*100) / 100
	netDeposits := math.Round((totalDeposits-gatewayFees)*100) / 100
	netBankProfit := math.Round((netDeposits-totalWithdrawals)*100) / 100

	profitMargin := 100.0
	if totalDeposits > 0 {
		profitMargin = math.Round(((netBankProfit/totalDeposits)*100.0)*10) / 10
	}

	liquidityCoverageRatio := 100.0
	if totalUserBalances > 0 {
		liquidityCoverageRatio = math.Round(((netBankProfit/totalUserBalances)*100.0)*10) / 10
	}

	totalCollects := expiredCollectCount + successCollectCount
	empiricalForgetRate := 33.3
	if totalCollects > 0 {
		empiricalForgetRate = math.Round(((float64(expiredCollectCount)/float64(totalCollects))*100.0)*10) / 10
	}

	return &DREMetrics{
		TotalDeposits:          math.Round(totalDeposits*100) / 100,
		GatewayFeePercent:      depositFeePercent,
		GatewayFees:            gatewayFees,
		NetDeposits:            netDeposits,
		TotalWithdrawals:       math.Round(totalWithdrawals*100) / 100,
		NetBankProfit:          netBankProfit,
		ProfitMargin:           profitMargin,
		TotalUserBalances:      math.Round(totalUserBalances*100) / 100,
		LiquidityCoverageRatio: liquidityCoverageRatio,
		RetainedExpiredYields:  math.Round(retainedExpiredYields*100) / 100,
		RetainedUnspunPrizes:   math.Round(retainedUnspunPrizes*100) / 100,
		EmpiricalForgetRate:    empiricalForgetRate,
		TotalRoulettePrizes:    math.Round(totalRoulettePrizes*100) / 100,
		ExpiredCollectCount:    expiredCollectCount,
		SuccessCollectCount:    successCollectCount,
	}
}

// DRERepository define o contrato para extração das métricas consolidadas do DRE
type DRERepository interface {
	GetMetrics(ctx context.Context) (*DREMetrics, error)
}
