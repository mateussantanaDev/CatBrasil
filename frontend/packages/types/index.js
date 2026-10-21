/**
 * @typedef {Object} User
 * @property {string} id
 * @property {string} username
 * @property {string} phone
 * @property {string} role
 * @property {number} balance
 * @property {number} yieldBalanceAvailable
 * @property {number} requiredRollover
 * @property {number} currentRollover
 * @property {number} xp
 * @property {number} nivel
 * @property {number} coletasCompleted
 * @property {boolean} tratorRented
 * @property {boolean} withdrawLocked
 * @property {boolean} freeWithdrawActive
 * @property {boolean} alreadySpunToday
 * @property {number} motorC15Count
 * @property {number} esteiraReforcadaCount
 * @property {number} filtroOleoCount
 * @property {number} turboGarrettCount
 * @property {boolean} twoFactorEnabled
 * @property {boolean} multiAccountFlag
 * @property {boolean} referralBonusSuspended
 * @property {string} [securityPassword]
 * @property {string} createdAt
 */

/**
 * @typedef {'aguardando' | 'disponivel' | 'expirado'} MachineState
 */

/**
 * @typedef {Object} Machine
 * @property {string} id
 * @property {string} userId
 * @property {string} name
 * @property {number} cost
 * @property {number} yieldAmount
 * @property {string} lastCollectedAt
 * @property {MachineState} state
 * @property {string} [overrideState]
 * @property {number} [overrideSecondsRemaining]
 * @property {number} secondsRemaining
 * @property {string} createdAt
 */

/**
 * @typedef {Object} FleetCatalogItem
 * @property {string} id
 * @property {string} name
 * @property {string} category
 * @property {number} price
 * @property {number} dailyYield
 * @property {number} returnPercentPerDay
 * @property {number} durationDays
 * @property {number} totalContractReturn
 * @property {number} stockQuantity
 * @property {number} maxPurchasesPerUser
 * @property {boolean} reinvestOnly
 * @property {string} imageUrl
 * @property {boolean} active
 */

/**
 * @typedef {Object} Transaction
 * @property {string} id
 * @property {string} userId
 * @property {string} tipo
 * @property {number} valor
 * @property {'pendente' | 'sucesso' | 'rejeitado' | 'cancelado'} status
 * @property {string} [externalId]
 * @property {string} [pixKey]
 * @property {string} [pixCode]
 * @property {string} [qrCodeUrl]
 * @property {number} [netAmount]
 * @property {number} [feeAmount]
 * @property {number} [valorEfetivoPago]
 * @property {string} [paidAt]
 * @property {string} [rejectionReason]
 * @property {string} createdAt
 */

/**
 * @typedef {Object} RouletteSlice
 * @property {number} id
 * @property {string} label
 * @property {number} amount
 * @property {number} probability
 * @property {string} color
 * @property {string} textColor
 */

/**
 * @typedef {Object} LeaderboardEntry
 * @property {number} rank
 * @property {string} userId
 * @property {string} username
 * @property {number} weeklyTonnage
 * @property {string} prize
 * @property {string} badge
 */

/**
 * @typedef {Object} Announcement
 * @property {string} id
 * @property {string} title
 * @property {string} category
 * @property {string} content
 * @property {string} imageUrl
 * @property {string} ctaLink
 * @property {string} ctaText
 * @property {boolean} showInWelcomeModal
 * @property {string} badgeColor
 * @property {boolean} active
 * @property {number} viewsCount
 * @property {string} createdAt
 */

/**
 * @typedef {Object} DREMetrics
 * @property {number} totalDeposits
 * @property {number} gatewayFeePercent
 * @property {number} gatewayFees
 * @property {number} netDeposits
 * @property {number} totalWithdrawals
 * @property {number} netBankProfit
 * @property {number} profitMargin
 * @property {number} totalUserBalances
 * @property {number} liquidityCoverageRatio
 * @property {number} retainedExpiredYields
 * @property {number} retainedUnspunPrizes
 * @property {number} empiricalForgetRate
 * @property {number} totalRoulettePrizes
 * @property {number} expiredCollectCount
 * @property {number} successCollectCount
 */

export {};
