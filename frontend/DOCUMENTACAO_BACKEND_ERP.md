# 📘 Documentação Completa de Integração Backend — Módulo ERP CatBrasil

> **Versão da Documentação:** 3.0.0  
> **Escopo:** Módulo de Gestão ERP (DRE, Liquidez, Gestão de Operadores, Fila de Saques PIX, Roleta & Probabilidades, Comunicados Rich Media, Afiliados & Frota)  
> **Padrão de Comunicação:** REST API (JSON) · Autenticação Bearer (JWT Admin)  

---

## 📑 Sumário

1. [Visão Geral & Arquitetura do Módulo ERP](#1-visão-geral--arquitetura-do-módulo-erp)
2. [Modelos de Dados do ERP (Schemas)](#2-modelos-de-dados-do-erp-schemas)
3. [Módulo 1: Autenticação & Sessão do Gestor](#3-módulo-1-autenticação--sessão-do-gestor)
4. [Módulo 2: DRE Automático & Regras do Gateway PIX](#4-módulo-2-dre-automático--regras-do-gateway-pix)
5. [Módulo 3: Controle Individual & Auditoria de Operadores](#5-módulo-3-controle-individual--auditoria-de-operadores)
6. [Módulo 4: Fila de Saques PIX & Aprovação Automática](#6-módulo-4-fila-de-saques-pix--aprovação-automática)
7. [Módulo 5: Roleta & Matriz de Probabilidades Matemáticas](#7-módulo-5-roleta--matriz-de-probabilidades-matemáticas)
8. [Módulo 6: Publicação de Comunicados Rich Media](#8-módulo-6-publicação-de-comunicados-rich-media)
9. [Módulo 7: Rede de Afiliados & Regras de Comissão](#9-módulo-7-rede-de-afiliados--regras-de-comissão)
10. [Módulo 8: Gestão de Frota & Ofertas Customizadas](#10-módulo-8-gestão-de-frota--ofertas-customizadas)
11. [Formulário de Cálculo DRE & Métricas Financeiras](#11-formulário-de-cálculo-dre--métricas-financeiras)
12. [Matriz de Erros HTTP & Códigos de Retorno](#12-matriz-de-erros-http--códigos-de-retorno)

---

## 1. Visão Geral & Arquitetura do Módulo ERP

O **Módulo ERP CatBrasil** é uma aplicação desktop/web standalone voltada à auditoria financeira, controle de liquidez da banca, moderação de saques e configuração de regras de negócio em tempo real.

### Base URL dos Endpoints de Gestão
- **Desenvolvimento:** `http://localhost:3000/api/v1/admin`
- **Produção:** `https://api.catbrasil.app/v1/admin`

### Autenticação & Autorização
Todas as requisições enviadas pelo módulo ERP exigem obrigatoriamente a presença do token JWT no cabeçalho HTTP:

```http
Authorization: Bearer <jwt_admin_token>
Content-Type: application/json
```

---

## 2. Modelos de Dados do ERP (Schemas)

### 2.1 Schema: `GestaoConfig` (Configuração Global do ERP)
```json
{
  "globalWithdrawalsLocked": false,
  "autoApproveEnabled": false,
  "autoApproveWithdrawalsUnder": 200.00,
  "referralCommissionPercent": 5.0,
  "requiredReferralsCount": 3,
  "requireReferralsForWithdrawal": true,
  "antiSelfReferralLock": true,
  "bonusPerMutiraoAmount": 50.00,
  "collectionGraceMinutes": 10,
  "gateway": {
    "name": "Gateway PIX Padrão",
    "depositFeePercent": 1.5,
    "withdrawFeeFixed": 1.00,
    "pixExpirationMinutes": 15
  },
  "roulette": {
    "strictModalMode": true,
    "requireDepositToSpin": false
  }
}
```

### 2.2 Schema: `RouletteSlice` (Fatia da Roleta)
```json
{
  "id": 1,
  "label": "R$ 10",
  "amount": 10.00,
  "probability": 5.0,
  "color": "#ffb200",
  "textColor": "#111111"
}
```

### 2.3 Schema: `Announcement` (Comunicado Rich Media)
```json
{
  "id": "evt-1770000000000",
  "title": "Verão CAT: +20% Rendimento",
  "category": "CAMPANHA DA GESTÃO",
  "content": "Todas as coletas realizadas na Garagem garantem bônus automático.",
  "imageUrl": "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80",
  "ctaLink": "https://catbrasil.app/loja",
  "ctaText": "VER MÁQUINAS EM OFERTA",
  "showInWelcomeModal": true,
  "badgeColor": "#ffb200",
  "active": true,
  "date": "28/07/2026",
  "viewsCount": 1420
}
```

### 2.4 Schema: `FleetMachine` (Modelo do Catálogo de Frota)
```json
{
  "id": "cat_320",
  "name": "Escavadeira CAT 320",
  "category": "ESCAVAÇÃO",
  "price": 150.00,
  "returnPercentPerDay": 16.0,
  "durationDays": 30,
  "dailyYield": 24.00,
  "totalContractReturn": 720.00,
  "maxPurchasesPerUser": 2,
  "active": true,
  "imageUrl": "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80"
}
```

### 2.5 Schema: `UserPromo` (Promoção Customizada por Usuário/Rede)
```json
{
  "id": "promo-1770000000000",
  "title": "Promoção VIP Usuário #104",
  "targetType": "user",
  "targetId": "usr-104",
  "machineId": "trator_d8t",
  "customPrice": 40.00,
  "customReturnPercentPerDay": 20.0,
  "customMaxPurchases": 3,
  "active": true,
  "date": "28/07/2026"
}
```

---

## 3. Módulo 1: Autenticação & Sessão do Gestor

### 3.1 Login no ERP de Gestão
- **`POST /admin/auth/login`**
- **Requisição:**
```json
{
  "username": "admin",
  "password": "senhaModoGestor",
  "pin": "2026"
}
```
- **Resposta `200 OK`:**
```json
{
  "success": true,
  "message": "Autenticação aprovada",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "gestor": {
      "id": "gst-001",
      "username": "admin",
      "role": "DIRETORIA"
    }
  }
}
```

### 3.2 Logout no ERP
- **`POST /admin/auth/logout`**
- **Cabeçalho:** `Authorization: Bearer <jwt_token>`

---

## 4. Módulo 2: DRE Automático & Regras do Gateway PIX

### 4.1 Obter Métricas Consolidadas do DRE
- **`GET /admin/dre/metrics`**
- **Resposta `200 OK`:**
```json
{
  "success": true,
  "data": {
    "totalDeposits": 150000.00,
    "gatewayFeePercent": 1.5,
    "gatewayFees": 2250.00,
    "netDeposits": 147750.00,
    "totalWithdrawals": 45000.00,
    "netBankProfit": 102750.00,
    "profitMargin": 68.5,
    "totalUserBalances": 32000.00,
    "liquidityCoverageRatio": 321.1,
    "retainedExpiredYields": 8400.00,
    "retainedUnspunPrizes": 2150.00,
    "empiricalForgetRate": 33.3
  }
}
```

### 4.2 Configurar Gateway PIX e Tolerância de Coleta
- **`PUT /admin/config/gateway`**
- **Requisição:**
```json
{
  "name": "Gateway PIX Padrão",
  "depositFeePercent": 1.5,
  "withdrawFeeFixed": 1.00,
  "pixExpirationMinutes": 15,
  "collectionGraceMinutes": 10
}
```

### 4.3 Comutador de Trava de Emergência de Saques Globais
- **`POST /admin/config/toggle-global-lock`**
- **Requisição:** `{ "locked": true }`

---

## 5. Módulo 3: Controle Individual & Auditoria de Operadores

### 5.1 Listar Operadores com Métricas Financeiras
- **`GET /admin/users?query=mateus&status=todos`**
- **Resposta `200 OK`:**
```json
{
  "success": true,
  "data": [
    {
      "id": "CB-458F",
      "username": "Mateus",
      "phone": "11999999999",
      "balance": 2450.00,
      "nivel": 8,
      "withdrawLocked": false,
      "referralBonusSuspended": false,
      "financialStats": {
        "totalDeposited": 3000.00,
        "totalWithdrawn": 550.00,
        "netLtvProfit": 2450.00
      }
    }
  ]
}
```

### 5.2 Ajustar Saldo Manualmente (Crédito / Débito)
- **`POST /admin/users/:userId/adjust-balance`**
- **Requisição:**
```json
{
  "type": "add",
  "amount": 500.00,
  "reason": "Crédito promocional concedido pela gestão"
}
```

### 5.3 Alterar Nível de Carreira Direto
- **`PUT /admin/users/:userId/level`**
- **Requisição:** `{ "nivel": 10 }`

### 5.4 Alternar Trava Individual de Saque
- **`PATCH /admin/users/:userId/toggle-withdraw-lock`**

### 5.5 Injetar Máquina Bônus na Garagem do Cliente
- **`POST /admin/users/:userId/inject-machine`**
- **Requisição:** `{ "machineModel": "cat_320" }`

### 5.6 Resetar Giro Diário da Roleta
- **`POST /admin/users/:userId/reset-spin`**

---

## 6. Módulo 4: Fila de Saques PIX & Aprovação Automática

### 6.1 Listar Fila de Saques
- **`GET /admin/withdrawals?status=pendentes`**
- **Status suportados:** `pendentes`, `aprovados`, `rejeitados`, `todos`.

### 6.2 Atualizar Regras de Aprovação Automática
- **`PUT /admin/config/auto-approve`**
- **Requisição:**
```json
{
  "autoApproveEnabled": true,
  "autoApproveWithdrawalsUnder": 200.00
}
```

### 6.3 Aprovar Saque Manualmente
- **`POST /admin/withdrawals/:txId/approve`**

### 6.4 Rejeitar Saque Manualmente (com Estorno e Trava Opcional)
- **`POST /admin/withdrawals/:txId/reject`**
- **Requisição:**
```json
{
  "lockAccount": true,
  "reason": "Requisitos de indicação não preenchidos"
}
```

---

## 7. Módulo 5: Roleta & Matriz de Probabilidades Matemáticas

### 7.1 Obter Matriz de Probabilidades e Métricas EV
- **`GET /admin/roulette/matrix`**
- **Resposta `200 OK`:**
```json
{
  "success": true,
  "data": {
    "slices": [...],
    "metrics": {
      "totalProb": 100.0,
      "expectedValueEV": 6.20,
      "isProbValid": true
    }
  }
}
```

### 7.2 Atualizar Fatias e Regras da Roleta
- **`PUT /admin/roulette/matrix`**
- **Requisição:**
```json
{
  "slices": [
    { "id": 1, "label": "R$ 10", "amount": 10.00, "probability": 5.0, "color": "#ffb200", "textColor": "#111" },
    { "id": 6, "label": "R$ 5", "amount": 5.00, "probability": 87.9, "color": "#111", "textColor": "#fff" }
  ],
  "rules": {
    "strictModalMode": true,
    "requireDepositToSpin": false
  }
}
```

---

## 8. Módulo 6: Publicação de Comunicados Rich Media

### 8.1 Listar Todos os Comunicados Lançados
- **`GET /admin/announcements`**

### 8.2 Criar Comunicado Rich Media
- **`POST /admin/announcements`**
- **Requisição:**
```json
{
  "title": "Nova Frota Disponível",
  "category": "CAMPANHA DA GESTÃO",
  "content": "Confira os novos modelos adicionados ao catálogo.",
  "imageUrl": "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80",
  "ctaLink": "https://catbrasil.app/loja",
  "ctaText": "ACESSAR LOJA",
  "showInWelcomeModal": true
}
```

### 8.3 Alternar Status do Comunicado (Ativo/Inativo)
- **`PATCH /admin/announcements/:id/toggle`**

### 8.4 Remover Comunicado
- **`DELETE /admin/announcements/:id`**

---

## 9. Módulo 7: Rede de Afiliados & Regras de Comissão

### 9.1 Salvar Regras Globais de Afiliados
- **`PUT /admin/config/referrals`**
- **Requisição:**
```json
{
  "referralCommissionPercent": 5.0,
  "requiredReferralsCount": 3,
  "bonusPerMutiraoAmount": 50.00,
  "requireReferralsForWithdrawal": true,
  "antiSelfReferralLock": true
}
```

### 9.2 Alternar Suspensão de Comissão Individual de Usuário
- **`PATCH /admin/users/:userId/toggle-commission-lock`**

---

## 10. Módulo 8: Gestão de Frota & Ofertas Customizadas

### 10.1 Salvar Catálogo Global de Frota
- **`PUT /admin/fleet/catalog`**
- **Requisição:**
```json
{
  "catalog": [
    {
      "id": "trator_d8t",
      "name": "Trator de Esteira CAT D8T",
      "category": "PESADA",
      "price": 50.00,
      "returnPercentPerDay": 15.0,
      "durationDays": 30,
      "maxPurchasesPerUser": 1,
      "active": true,
      "imageUrl": "..."
    }
  ]
}
```

### 10.2 Adicionar Oferta Promocional Customizada
- **`POST /admin/fleet/promos`**
- **Requisição:**
```json
{
  "title": "Promoção VIP Usuário #104",
  "targetType": "user",
  "targetId": "usr-104",
  "machineId": "trator_d8t",
  "customPrice": 40.00,
  "customReturnPercentPerDay": 20.0,
  "customMaxPurchases": 3
}
```

### 10.3 Remover Oferta Promocional
- **`DELETE /admin/fleet/promos/:id`**

---

## 11. Formulário de Cálculo DRE & Métricas Financeiras

O backend deve implementar as seguintes regras de cálculo no serviço DRE:

1. **Taxa do Gateway PIX**: $\text{gatewayFees} = \text{totalDeposits} \times \left(\frac{\text{depositFeePercent}}{100}\right)$
2. **Depósito Líquido Entrado**: $\text{netDeposits} = \text{totalDeposits} - \text{gatewayFees}$
3. **Lucro Líquido da Banca**: $\text{netBankProfit} = \text{netDeposits} - \text{totalWithdrawals}$
4. **Margem de Lucro (%)**: $\text{profitMargin} = \left(\frac{\text{netBankProfit}}{\text{totalDeposits}}\right) \times 100$
5. **Índice de Cobertura de Liquidez (%)**: $\text{liquidityCoverageRatio} = \left(\frac{\text{netBankProfit}}{\text{totalUserBalances}}\right) \times 100$
6. **Valor Esperado da Roleta (EV em R$)**: $EV = \sum_{i=1}^{n} \left( \text{amount}_i \times \frac{\text{probability}_i}{100} \right)$

---

## 12. Matriz de Erros HTTP & Códigos de Retorno

| HTTP Code | Error Code | Descrição |
| :--- | :--- | :--- |
| `401` | `ERP_AUTH_REQUIRED` | Acesso negado. Token de gestor ausente ou inválido. |
| `403` | `GLOBAL_WITHDRAWALS_LOCKED` | Sistema com trava global de saques ativada pela diretoria. |
| `403` | `USER_WITHDRAW_LOCKED` | Saque travado individualmente para a conta do usuário. |
| `400` | `INVALID_ROULETTE_PROBABILITY` | A soma das probabilidades da roleta não fecha em 100.0%. |
| `404` | `OPERATOR_NOT_FOUND` | Operador/Cliente não encontrado na base de dados. |

---
*Fim da Documentação Oficial do Módulo ERP CatBrasil v3.0.*
