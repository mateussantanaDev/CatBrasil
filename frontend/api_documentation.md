# 📄 Documentação Completa de Integração Backend - CatBrasil

> **Versão do Documento:** 2.0.0  
> **Arquitetura Base:** REST API (JSON) + Autenticação Bearer (JWT) + Configurações Dinâmicas de Gestão  
> **Status:** Especificação Oficial de Comunicação Frontend-Backend  

---

## 📑 Sumário

1. [Visão Geral & Arquitetura](#1-visão-geral--arquitetura)
2. [Modelos de Dados (Schemas)](#2-modelos-de-dados-schemas)
3. [Módulo 1: Autenticação & Cadastro com SMS](#3-módulo-1-autenticação--cadastro-com-sms)
4. [Módulo 2: Perfil do Operador & Segurança](#4-módulo-2-perfil-do-operador--segurança)
5. [Módulo 3: Garagem & Ciclos de 24h por Máquina](#5-módulo-3-garagem--ciclos-de-24h-por-máquina)
6. [Módulo 4: Loja de Contratos & Limites de Frota](#6-módulo-4-loja-de-contratos--limites-de-frota)
7. [Módulo 5: Roleta Diária Premiada (Configurada pela Gestão)](#7-módulo-5-roleta-diária-premiada-configurada-pela-gestão)
8. [Módulo 6: Rede de Afiliados & Comissões](#8-módulo-6-rede-de-afiliados--comissões)
9. [Módulo 7: Financeiro (PIX, Saques & Extrato)](#9-módulo-7-financeiro-pix-saques--extrato)
10. [Módulo 8: Comunicados & Eventos da Gestão](#10-módulo-8-comunicados--eventos-da-gestão)
11. [Painel de Controle Admin (Gestão)](#11-painel-de-controle-admin-gestão)
12. [Códigos de Erro Padronizados](#12-códigos-de-erro-padronizados)

---

## 1. Visão Geral & Arquitetura

### Base URL
- **Desenvolvimento:** `http://localhost:3000/api/v1`
- **Produção:** `https://api.catbrasil.app/v1`

### Cabeçalhos Padrão (Headers)
Todas as requisições autenticadas exigem o envio do token JWT no cabeçalho:
```http
Authorization: Bearer <token_jwt>
Content-Type: application/json
```

### Formato de Resposta Padronizado

#### Sucesso (`200 OK` / `201 Created`)
```json
{
  "success": true,
  "message": "Operação realizada com sucesso",
  "data": { ... }
}
```

#### Erro (`400 Bad Request`, `401 Unauthorized`, `403 Forbidden`, `409 Conflict`)
```json
{
  "success": false,
  "code": "SECURITY_PASSWORD_INVALID",
  "message": "A senha financeira de 6 dígitos informada é inválida."
}
```

---

## 2. Modelos de Dados (Schemas)

### User Schema (`user`)
```json
{
  "id": "CB-458F",
  "username": "Mateus Silva",
  "phone": "11999999999",
  "balance": 2450.00,
  "nivel": 8,
  "xp": 500,
  "coletasCompleted": 4,
  "tratorRented": false,
  "alreadySpunToday": false,
  "securityPassword": true,
  "referralBonusSuspended": false
}
```

### Machine Schema (`machine`)
```json
{
  "id": "mach-102",
  "userId": "CB-458F",
  "name": "Escavadeira CAT 320",
  "cost": 2500.00,
  "yieldAmount": 110.00,
  "lastCollectedAt": "2026-07-28T19:34:00.000Z",
  "state": "aguardando",
  "secondsRemaining": 86400
}
```

---

## 3. Módulo 1: Autenticação & Cadastro com SMS

### 3.1 Solicitar Código de Verificação SMS
- **`POST /auth/register-request`**
- **Payload Request:**
```json
{ "phone": "11999999999" }
```
- **Response `200 OK`:**
```json
{
  "success": true,
  "message": "Código de verificação enviado via SMS.",
  "data": { "expiresInSeconds": 300 }
}
```

### 3.2 Confirmar Código & Entrar / Registrar
- **`POST /auth/verify-code`**
- **Payload Request:**
```json
{
  "phone": "11999999999",
  "code": "123456",
  "name": "Mateus Silva",
  "password": "senhaSegura123",
  "inviteCode": "CB-458F"
}
```
- **Response `200 OK`:**
```json
{
  "success": true,
  "message": "Autenticado com sucesso.",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": { "id": "CB-458F", "username": "Mateus Silva", "balance": 2450.00 }
  }
}
```

---

## 4. Módulo 2: Perfil do Operador & Segurança

### 4.1 Obter Dados do Operador Logado
- **`GET /user/profile`** (Bearer Token)
- **Response `200 OK`:** Retorna o objeto `user` completo.

### 4.2 Cadastrar / Alterar Senha Financeira (6 Dígitos)
- **`POST /user/security-password`** (Bearer Token)
- **Payload Request:**
```json
{ "securityPassword": "123456" }
```

### 4.3 Solicitar Promoção de Nível de Carreira
- **`POST /user/level-up`** (Bearer Token)
- **Regras do Backend:** O backend valida se `coletasCompleted >= 5`, `tratorRented === true` e se possui `3 indicados ativos`.

---

## 5. Módulo 3: Garagem & Ciclos de 24h por Máquina

### 5.1 Listar Máquinas da Garagem
- **`GET /garage`** (Bearer Token)
- **Regra dos Relógios Individuais:** Cada máquina tem seu ciclo de 24 horas contado a partir da hora exata do seu aluguel ou última coleta (`lastCollectedAt`).

### 5.2 Coletar Rendimento da Máquina
- **`POST /garage/collect`** (Bearer Token)
- **Payload Request:**
```json
{ "machineId": "mach-102" }
```
- **Regra de Proteção do Caixa:** Se a coleta for tentada após a janela de tolerância de 10 minutos pós-ciclo, o rendimento é zerado e o ciclo é reiniciado sem pagar o prêmio ao operador.

---

## 6. Módulo 4: Loja de Contratos & Limites de Frota

### 6.1 Alugar Máquina na Loja
- **`POST /garage/rent`** (Bearer Token)
- **Payload Request:**
```json
{ "machineModel": "trator_d8t" }
```
- **Response `200 OK`:** Deduz o valor do saldo, cria a máquina com `lastCollectedAt = Date.now()` (iniciando relógio individual de 24h a partir do momento exato da compra).

---

## 7. Módulo 5: Roleta Diária Premiada (Configurada pela Gestão)

### 7.1 Obter Configuração Ativa da Roleta (Definida pelo Painel Admin)
- **`GET /roulette/config`** (Bearer Token ou Público)
- **Response `200 OK`:**
```json
{
  "success": true,
  "data": {
    "slices": [
      { "id": 1, "label": "R$ 10", "type": "balance", "amount": 10.00, "color": "#ffb200", "textColor": "#111", "probability": 0.05 },
      { "id": 2, "label": "R$ 50", "type": "balance", "amount": 50.00, "color": "#111111", "textColor": "#fff", "probability": 0.02 },
      { "id": 3, "label": "R$ 100", "type": "balance", "amount": 100.00, "color": "#2e7d32", "textColor": "#fff", "probability": 0.01 },
      { "id": 4, "label": "R$ 20", "type": "balance", "amount": 20.00, "color": "#d32f2f", "textColor": "#fff", "probability": 0.04 },
      { "id": 5, "label": "R$ 500", "type": "balance", "amount": 500.00, "color": "#ffb200", "textColor": "#111", "probability": 0.001 },
      { "id": 6, "label": "R$ 5", "type": "balance", "amount": 5.00, "color": "#111111", "textColor": "#fff", "probability": 0.879 }
    ]
  }
}
```

### 7.2 Executar Giro Diário Único
- **`POST /roulette/spin`** (Bearer Token)
- **Regra de Boas-Vindas:** O giro só pode ser feito uma vez por dia (`alreadySpunToday === false`).
- **Regra de Fechamento:** Se o usuário fechar o modal de Boas-Vindas sem girar, o endpoint de fechar registra `alreadySpunToday = true`, cancelando a roleta daquele dia.

---

## 8. Módulo 6: Rede de Afiliados & Comissões

### 8.1 Listar Convidados Diretos
- **`GET /referrals`** (Bearer Token)
- Retorna os usuários indicados e a flag `referralBonusSuspended`. Se houver algum convidado inativo, a comissão de 5% fica temporariamente suspensa.

---

## 9. Módulo 7: Financeiro (PIX, Saques & Extrato)

### 9.1 Gerar Depósito via PIX
- **`POST /financial/deposit/pix`** (Bearer Token)
- **Payload Request:**
```json
{ "amount": 500.00 }
```
- **Response `200 OK`:** Retorna o código PIX Copia e Cola, QR Code e expiração de 15 minutos.

### 9.2 Solicitar Saque / Retirada
- **`POST /financial/withdraw`** (Bearer Token)
- **Payload Request:**
```json
{
  "amount": 250.00,
  "pixKey": "11999999999",
  "securityPassword": "123456"
}
```
- **Validações:** Verifica se a `securityPassword` de 6 dígitos bate com a cadastrada, se o saldo é suficiente e se a conta atende aos requisitos de indicados ativos.

### 9.3 Obter Extrato / Histórico Financeiro
- **`GET /financial/transactions?filter=todos|depositos|saques|rendimentos`** (Bearer Token)

---

## 10. Módulo 8: Comunicados & Eventos da Gestão

### 10.1 Listar Comunicados Ativos da Gestão
- **`GET /announcements/active`** (Bearer Token ou Público)
- Retorna os comunicados e eventos lançados pela gestão para serem exibidos no modal acionado pelo botão flutuante 📢 na Garagem.
- **Response `200 OK`:**
```json
{
  "success": true,
  "data": {
    "announcements": [
      {
        "id": "ann-1",
        "title": "Verão CAT: +20% Rendimento",
        "category": "CAMPANHA DA GESTÃO",
        "content": "Todas as coletas realizadas na Garagem garantem +20% de bônus automático depositado diretamente no seu saldo livre.",
        "badgeColor": "#ffb200"
      },
      {
        "id": "ann-2",
        "title": "Mutirão de Operadores",
        "category": "BÔNUS DE REDE",
        "content": "Convide 3 novos operadores qualificados e ative o prêmio instantâneo de R$ 500,00.",
        "badgeColor": "#2e7d32"
      }
    ]
  }
}
```

---

## 11. Painel de Controle Admin (Gestão)

### 11.1 Atualizar Configuração da Roleta (Definir Fatias & Probabilidades)
- **`POST /admin/roulette/config`** (Admin Token)
- **Payload Request:**
```json
{
  "slices": [
    { "id": 1, "label": "R$ 10", "type": "balance", "amount": 10.00, "probability": 0.05 },
    { "id": 2, "label": "R$ 50", "type": "balance", "amount": 50.00, "probability": 0.02 },
    { "id": 3, "label": "R$ 100", "type": "balance", "amount": 100.00, "probability": 0.01 },
    { "id": 4, "label": "R$ 20", "type": "balance", "amount": 20.00, "probability": 0.04 },
    { "id": 5, "label": "R$ 500", "type": "balance", "amount": 500.00, "probability": 0.001 },
    { "id": 6, "label": "R$ 5", "type": "balance", "amount": 5.00, "probability": 0.879 }
  ]
}
```

---

## 12. Códigos de Erro Padronizados

| HTTP Code | Error Code | Descrição |
| :--- | :--- | :--- |
| `400` | `INVALID_PHONE` | Celular deve conter 11 dígitos com DDD e 9º dígito. |
| `400` | `INSUFFICIENT_FUNDS` | Saldo em carteira insuficiente. |
| `401` | `UNAUTHORIZED` | Token JWT ausente ou expirado. |
| `401` | `INVALID_SECURITY_PASSWORD` | Senha financeira de 6 dígitos incorreta. |
| `403` | `COLLECTION_EXPIRED` | Tolerância de 10 minutos expirada. Rendimento retido pelo caixa. |
| `403` | `ROULETTE_ALREADY_SPUN` | Giro diário já utilizado ou cancelado ao fechar modal. |
| `409` | `PHONE_ALREADY_EXISTS` | Este número de celular já possui cadastro no sistema. |

---
*Fim da Documentação da API Backend CatBrasil v2.0.*
