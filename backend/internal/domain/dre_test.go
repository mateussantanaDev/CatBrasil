package domain_test

import (
	"testing"

	"catbrasil/backend/internal/domain"
)

func TestDRE_CalculateDREMetrics(t *testing.T) {
	// Exemplo baseado na Seção 4.1 / 11 da documentação oficial do ERP CatBrasil v3.0:
	// totalDeposits = 150000.00
	// gatewayFeePercent = 1.50 -> gatewayFees = 2250.00
	// netDeposits = 150000 - 2250 = 147750.00
	// totalWithdrawals = 45000.00
	// netBankProfit = 147750 - 45000 = 102750.00
	// profitMargin = (102750 / 150000) * 100 = 68.5%
	// totalUserBalances = 32000.00
	// liquidityCoverageRatio = (102750 / 32000) * 100 = 321.1%
	// retainedExpiredYields = 8400.00
	// retainedUnspunPrizes = 2150.00
	// expiredCollectCount = 10, successCollectCount = 20 -> total = 30 -> forgetRate = (10/30)*100 = 33.3%

	metrics := domain.CalculateDREMetrics(
		150000.00,
		1.50,
		45000.00,
		32000.00,
		8400.00,
		2150.00,
		500.00,
		10,
		20,
	)

	if metrics.GatewayFees != 2250.00 {
		t.Errorf("GatewayFees = %v, want 2250.00", metrics.GatewayFees)
	}

	if metrics.NetDeposits != 147750.00 {
		t.Errorf("NetDeposits = %v, want 147750.00", metrics.NetDeposits)
	}

	if metrics.NetBankProfit != 102750.00 {
		t.Errorf("NetBankProfit = %v, want 102750.00", metrics.NetBankProfit)
	}

	if metrics.ProfitMargin != 68.5 {
		t.Errorf("ProfitMargin = %v, want 68.5", metrics.ProfitMargin)
	}

	if metrics.LiquidityCoverageRatio != 321.1 {
		t.Errorf("LiquidityCoverageRatio = %v, want 321.1", metrics.LiquidityCoverageRatio)
	}

	if metrics.EmpiricalForgetRate != 33.3 {
		t.Errorf("EmpiricalForgetRate = %v, want 33.3", metrics.EmpiricalForgetRate)
	}
}
