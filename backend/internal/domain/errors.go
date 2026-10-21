package domain

import "errors"

// Erros de Domínio Padronizados
var (
	// Erros de Usuário / Autenticação
	ErrUserNotFound               = errors.New("operador não encontrado")
	ErrUserAlreadyExists          = errors.New("este nome de usuário ou telefone já está cadastrado")
	ErrInvalidCredentials         = errors.New("credenciais de acesso incorretas")
	ErrInvalidSecurityPassword    = errors.New("senha financeira incorreta")
	ErrSecurityPasswordNotSet     = errors.New("cadastre sua senha financeira de 6 dígitos antes de solicitar saques")
	ErrSecurityPasswordInvalidLen = errors.New("a senha de segurança deve conter exatamente 6 dígitos numéricos")
	ErrInvalid2FACode             = errors.New("código de autenticação em duas etapas (2FA) inválido")
	ErrTwoFactorRequired          = errors.New("informe o código de autenticação em duas etapas (2FA)")
	ErrTwoFactorNotConfigured     = errors.New("2FA não configurado para esta conta administrativa")
	ErrFinancialAuthLocked        = errors.New("muitas tentativas de autorização financeira; aguarde 15 minutos")
	ErrUnauthorized               = errors.New("acesso não autorizado: token ausente ou inválido")
	ErrForbidden                  = errors.New("acesso proibido para este nível de permissão")
	ErrMultiAccountDetected       = errors.New("múltiplas contas detectadas no mesmo dispositivo ou endereço IP")

	// Erros Financeiros & Saques
	ErrDepositUnavailable          = errors.New("depósitos PIX temporariamente indisponíveis")
	ErrInsufficientBalance         = errors.New("saldo em carteira insuficiente para realizar esta operação")
	ErrInsufficientYieldBalance    = errors.New("saldo de rendimentos acumulados de coletas insuficiente para a linha Reinvest")
	ErrWithdrawLocked              = errors.New("saques temporariamente travados para esta conta pela moderação")
	ErrGlobalWithdrawLocked        = errors.New("sistema em manutenção: saques globais temporariamente pausados pela diretoria")
	ErrMinimumWithdrawalAmount     = errors.New("o valor mínimo para saque PIX é de R$ 50,00")
	ErrRolloverNotMet              = errors.New("requisito de giro mínimo em contratação de frotas (Rollover) ainda não atingido")
	ErrTransactionNotFound         = errors.New("transação financeira não encontrada")
	ErrTransactionAlreadyProcessed = errors.New("transação já foi processada anteriormente (idempotente)")
	ErrInvalidTransactionStatus    = errors.New("status de transação inválido para esta operação")
	ErrConcurrentModification      = errors.New("dados alterados por outra operação; atualize e tente novamente")

	// Erros de Garagem & Mineração
	ErrMachineNotFound           = errors.New("máquina não encontrada na garagem")
	ErrYieldNotReady             = errors.New("ciclo de produção de 24h ainda em andamento: coleta indisponível")
	ErrYieldExpired              = errors.New("janela de coleta de 10 minutos expirada: rendimento retido para o caixa da banca")
	ErrMachineCatalogNotFound    = errors.New("modelo de máquina não encontrado no catálogo oficial")
	ErrCatalogItemInactive       = errors.New("modelo de máquina temporariamente indisponível para aluguel")
	ErrMaxPurchasesExceeded      = errors.New("limite máximo de contratos para este modelo de máquina já atingido")
	ErrLevelUpRequirementsNotMet = errors.New("requisitos de promoção de nível pendentes (coletas, contrato D8T ou indicados ativos)")

	// Erros de Peças & Oficina Caterpillar
	ErrIncompletePartsAssembly = errors.New("é necessário possuir as 4 peças completas (Motor C15, Esteira, Filtro e Turbo) para montar a Escavadeira")

	// Erros de Roleta
	ErrSpinAlreadyUsedToday      = errors.New("você já realizou seu giro premiado diário hoje")
	ErrProbabilitiesMustEqual100 = errors.New("a soma das probabilidades de todas as fatias da roleta deve totalizar exatamente 100.0%")
	ErrInvalidRouletteSlice      = errors.New("fatia da roleta inválida ou incompleta")

	// Erros de Comunicados & Ofertas
	ErrAnnouncementNotFound = errors.New("comunicado não encontrado")
	ErrPromoNotFound        = errors.New("oferta promocional não encontrada")

	// Erros de SMS
	ErrInvalidSMSCode    = errors.New("código de verificação SMS inválido ou expirado")
	ErrInvalidPhone      = errors.New("número de celular inválido")
	ErrSMSCooldownActive = errors.New("aguarde 60 segundos antes de solicitar um novo código SMS")
)
