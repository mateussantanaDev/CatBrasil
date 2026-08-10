# Documentação Técnica Completa do Módulo Cliente (Usuário) — CatBrasil v2.0

Esta documentação especifica **100% das regras de negócio, fluxos, algoritmos, dados consumidos e contratos de API (REST/JSON)** do Módulo do Cliente/Operador da plataforma CatBrasil. 

---

## 📋 1. Visão Geral da Arquitetura do Cliente

O Módulo Cliente é a interface web/mobile utilizada pelos operadores para gerenciar a frota de máquinas pesadas de mineração, coletar rendimentos diários, movimentar saldo via PIX, indicar novos operadores e evoluir de nível.

### Principais Entidades:
1. **User (Operador)**: Perfil do usuário, saldo, nível, XP, senha financeira, missões.
2. **Garage Machine (Máquina na Frota)**: Contrato ativo de máquina com timer individual de 24h a partir da data/hora exata do aluguel.
3. **Store Machine (Catálogo)**: Máquinas disponíveis para contratação.
4. **Transaction (Histórico Financeiro)**: Extrato de depósitos PIX, saques, rendimentos e custos de aluguel.
5. **Referral (Rede de Afiliados)**: Indicados diretos e bônus de 5% sobre produção.
6. **Roulette (Roleta Diária)**: Gamificação de login diário com premiação em saldo.
7. **Announcements/Events**: Comunicados e eventos lançados pela gestão.

---

## 🔐 2. Autenticação e Gestão de Sessão

### 2.1 Fluxo de Cadastro com Validação SMS
1. O usuário preenche: **Nome**, **Celular (DDD + 9 dígitos)**, **Senha (mín. 6 chars)**, **Confirmação de Senha** e **Código de Convite (opcional)**.
2. O frontend valida o número de celular (`/^\d{11}$/`) e a coincidência das senhas.
3. O sistema dispara a requisição de SMS contendo um código numérico de 6 dígitos.
4. O usuário insere o código no modal de verificação SMS.
5. Após validação bem-sucedida, o usuário é cadastrado com saldo inicial e recebe o token JWT de sessão.

### 2.2 Endpoints de Autenticação

#### `POST /api/auth/send-sms`
- **Requisição:**
  ```json
  {
    "phone": "11999999999"
  }
  ```
- **Resposta Sucesso (200):**
  ```json
  {
    "success": true,
    "message": "Código de verificação enviado via SMS."
  }
  ```

#### `POST /api/auth/register`
- **Requisição:**
  ```json
  {
    "username": "Mateus Silva",
    "phone": "11999999999",
    "password": "suasenhafechada",
    "verificationCode": "123456",
    "inviteCode": "CB-458F" // opcional
  }
  ```
- **Resposta Sucesso (201):**
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6...",
    "user": {
      "id": "CB-458F",
      "username": "Mateus Silva",
      "phone": "11999999999",
      "balance": 2450.00,
      "nivel": 8,
      "xp": 500,
      "securityPassword": null,
      "coletasCompleted": 4,
      "tratorRented": false,
      "alreadySpunToday": false,
      "referralBonusSuspended": false,
      "withdrawLocked": false
    }
  }
  ```

#### `POST /api/auth/login`
- **Requisição:**
  ```json
  {
    "login": "11999999999", // Telefone ou Usuário
    "password": "suasenhafechada"
  }
  ```
- **Resposta Sucesso (200):**
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6...",
    "user": { ... }
  }
  ```

---

## 🚜 3. Módulo Garagem & Minha Frota (Regra Crítica de Timers)

### 3.1 Algoritmo de Relógio Individual por Máquina
Cada máquina na frota possui um **ciclo de 24 horas exatas (86.400 segundos)** iniciado a partir da **data/hora exata (`lastCollectedAt`)** em que foi alugada ou coletada pela última vez.

#### Estados da Máquina:
- **`aguardando`**: `agora < (lastCollectedAt + 24h)`. A produção está em andamento.
- **`disponivel`**: `(lastCollectedAt + 24h) <= agora <= (lastCollectedAt + 24h + 10min)`. A coleta está liberada. Janela de tolerância de caixa = **10 minutos**.
- **`expirado`**: `agora > (lastCollectedAt + 24h + 10min)`. A coleta não foi realizada a tempo. O caixa retém o rendimento e a máquina inicia novo ciclo ao clicar.

#### Fórmula de Cálculo do Bônus de Nível sobre o Rendimento:
$$\text{Rendimento Total} = \text{yieldAmount} \times \left(1 + (\text{nivel} - 1) \times 0.05\right)$$

*Exemplo: Nível 8 com máquina de R$ 110,00/dia = $110 \times (1 + 7 \times 0.05) = 110 \times 1.35 = \text{R\$ } 148,50$.*

### 3.2 Endpoints da Garagem

#### `GET /api/garage`
- **Headers:** `Authorization: Bearer <token>`
- **Resposta Sucesso (200):**
  ```json
  [
    {
      "id": "mach-1722800000",
      "userId": "CB-458F",
      "name": "Escavadeira CAT 320",
      "cost": 2500.00,
      "yieldAmount": 110.00,
      "lastCollectedAt": "2026-08-04T08:00:00.000Z",
      "state": "aguardando", // "aguardando" | "disponivel" | "expirado"
      "secondsRemaining": 45000
    }
  ]
  ```

#### `POST /api/garage/collect`
- **Headers:** `Authorization: Bearer <token>`
- **Requisição:**
  ```json
  {
    "machineId": "mach-1722800000"
  }
  ```
- **Lógica Backend:**
  1. Verifica se a máquina pertence ao usuário.
  2. Checa o horário atual em relação a `lastCollectedAt + 24h` e a tolerância de 10 min.
  3. Se dentro do prazo: calcula `totalGained` com bônus do nível, adiciona ao `user.balance`, incrementa `user.coletasCompleted` em 1, atualiza `lastCollectedAt` para `NOW()`, cria registro de transação de Rendimento.
  4. Se expirado: não adiciona saldo (`isExpired: true`), atualiza `lastCollectedAt` para `NOW()`.
- **Resposta Sucesso (200):**
  ```json
  {
    "totalGained": 148.50,
    "isExpired": false,
    "newBalance": 2598.50,
    "coletasCompleted": 5
  }
  ```

---

## 🛒 4. Loja de Máquinas & Contratação

### 4.1 Regras de Contratação
- O usuário deve possuir saldo suficiente (`user.balance >= machine.cost`).
- Ao alugar:
  1. Deduz `machine.cost` do saldo do usuário.
  2. Cria uma nova máquina na garagem com `lastCollectedAt = NOW()`.
  3. Se o modelo for `"trator_d8t"`, marca `user.tratorRented = true` (requisito de missão de nível).
  4. Registra transação financeira negativa ("Aluguel [Nome da Máquina]").

### 4.2 Endpoints da Loja

#### `GET /api/store/machines`
- **Resposta Sucesso (200):**
  ```json
  [
    {
      "id": "trator_d8t",
      "name": "Trator de Esteira D8T",
      "price": 1500.00,
      "dailyYield": 75.00,
      "stockQuantity": 15,
      "imageUrl": "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=400&q=80",
      "active": true
    },
    {
      "id": "cat_320",
      "name": "Escavadeira CAT 320",
      "price": 2500.00,
      "dailyYield": 110.00,
      "stockQuantity": 8,
      "imageUrl": "https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=400&q=80",
      "active": true
    }
  ]
  ```

#### `POST /api/garage/rent`
- **Headers:** `Authorization: Bearer <token>`
- **Requisição:**
  ```json
  {
    "machineModel": "trator_d8t"
  }
  ```
- **Resposta Sucesso (200):**
  ```json
  {
    "newMachine": {
      "id": "mach-1722809999",
      "userId": "CB-458F",
      "name": "Trator de Esteira D8T",
      "cost": 1500.00,
      "yieldAmount": 75.00,
      "lastCollectedAt": "2026-08-04T13:24:00.000Z",
      "state": "aguardando",
      "secondsRemaining": 86400
    },
    "newBalance": 950.00
  }
  ```

---

## 💳 5. Operações Financeiras (PIX & Saque)

### 5.1 Depósito PIX Instantâneo
- Gera código PIX Copia e Cola (`00020126...`) e QR Code correspondente.
- Define tempo de expiração padrão (ex: 15 minutos / 900 segundos).
- Registra transação com status `"pendente"`.

#### `POST /api/financial/deposit/pix`
- **Requisição:**
  ```json
  {
    "amount": 500.00
  }
  ```
- **Resposta Sucesso (200):**
  ```json
  {
    "txId": "PIX-982341",
    "amount": 500.00,
    "pixCode": "00020126580014br.gov.bcb.pix0136catbrasil-deposit-500-uuid5204000053039865405500.005802BR5916CatBrasil Mining6009SAO PAULO620705031236304E8A2",
    "expirationMinutes": 15
  }
  ```

### 5.2 Solicitação de Saque PIX
- Requer validação estrita da **Senha Financeira de 6 Dígitos** (`securityPassword`).
- Valida se `user.withdrawLocked === false`.
- Valida se `user.balance >= amount`.
- Deduz o valor do saldo instantaneamente e registra transação do tipo `"Saque"` com valor negativo.

#### `POST /api/financial/withdraw`
- **Requisição:**
  ```json
  {
    "amount": 250.00,
    "pixKey": "11999999999",
    "securityPassword": "123456"
  }
  ```
- **Resposta Sucesso (200):**
  ```json
  {
    "success": true,
    "newBalance": 2200.00,
    "txId": "WDR-481920"
  }
  ```

#### `GET /api/financial/transactions`
- **Query Params:** `?filter=todos` | `depositos` | `saques` | `rendimentos`
- **Resposta Sucesso (200):**
  ```json
  [
    {
      "id": "tx-1",
      "userId": "CB-458F",
      "data": "20/07/2026 14:30",
      "tipo": "Depósito PIX",
      "valor": 300.00,
      "status": "sucesso"
    },
    {
      "id": "tx-2",
      "userId": "CB-458F",
      "data": "18/07/2026 09:15",
      "tipo": "Saque",
      "valor": -150.00,
      "status": "sucesso"
    }
  ]
  ```

---

## 👥 6. Rede de Afiliados (Bônus de Indicação de 5%)

### 6.1 Regras de Negócio
- Cada operador possui um link único: `https://catbrasil.app/ref/{user.id}`.
- O operador recebe **5% de comissão direta** sobre a produção diária de seus indicados ativos.
- Caso a gestão marque `referralBonusSuspended: true` na conta do operador (ex: suspeita de fraude/auto-indicação), a comissão é zerada (0%).

#### `GET /api/referrals`
- **Resposta Sucesso (200):**
  ```json
  {
    "referralLink": "https://catbrasil.app/ref/CB-458F",
    "commissionRate": 5.0,
    "isSuspended": false,
    "referrals": [
      { "id": "ref-1", "nome": "João Silva", "status": "ativo", "deposito": 500.00, "data": "18/07/2026" },
      { "id": "ref-2", "nome": "Maria Oliveira", "status": "ativo", "deposito": 350.00, "data": "19/07/2026" },
      { "id": "ref-3", "nome": "Carlos Pereira", "status": "ativo", "deposito": 1200.00, "data": "20/07/2026" }
    ]
  }
  ```

---

## 🎰 7. Roleta Diária Premiada & Comunicados

### 7.1 Giro Diário
- Permitido **apenas 1 giro por dia** (`user.alreadySpunToday === false`).
- Premiação adicionada diretamente ao saldo do usuário.
- Prêmios possíveis na roleta: `R$ 5`, `R$ 10`, `R$ 20`, `R$ 50`, `R$ 100`, `R$ 500`.
- Se o usuário fechar o modal de avisos sem girar a roleta, o giro do dia é marcado como concluído (`alreadySpunToday = true`).

#### `POST /api/gamification/roulette/spin`
- **Resposta Sucesso (200):**
  ```json
  {
    "prizeAmount": 50.00,
    "prizeMessage": "VOCÊ GANHOU R$ 50,00 EM CRÉDITOS!",
    "newBalance": 2500.00,
    "alreadySpunToday": true
  }
  ```

#### `GET /api/announcements/active`
- **Resposta Sucesso (200):**
  ```json
  [
    {
      "id": "evt-1",
      "title": "Campanha de Verão",
      "category": "BÔNUS DE DEPÓSITO",
      "content": "Receba 20% de bônus acumulado para coletas diárias.",
      "imageUrl": "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=400&q=80",
      "ctaLink": "https://catbrasil.app/ref/CB-458F",
      "ctaText": "VER DETALHES",
      "active": true
    }
  ]
  ```

---

## 🎖️ 8. Progressão de Carreira e Promoção de Nível

### 8.1 Missões Obrigatórias para Subir de Nível
Para subir do nível $N$ para o nível $N + 1$, o operador precisa completar simultaneamente:
1. **5 ou mais coletas concluídas** (`user.coletasCompleted >= 5`).
2. **Possuir pelo menos 1 contrato do Trator D8T** (`user.tratorRented === true`).
3. **Possuir 3 indicados diretos no status 'ativo'** (`referrals.filter(r => r.status === 'ativo').length >= 3`).

Ao promover o nível:
- `user.nivel += 1`
- `user.xp = 0`
- `user.coletasCompleted = 0`
- Incrementa o bônus de rendimento de todas as máquinas em +5%.

#### `POST /api/user/levelup`
- **Resposta Sucesso (200):**
  ```json
  {
    "success": true,
    "newLevel": 9,
    "xp": 0,
    "coletasCompleted": 0,
    "newLevelBonusPercent": 40.0 // (9-1)*5%
  }
  ```

---

## 🔒 9. Senha Financeira & Segurança da Conta

### 9.1 Definir ou Atualizar Senha Financeira

#### `POST /api/user/security-password`
- **Requisição:**
  ```json
  {
    "password": "654321" // Exatamente 6 dígitos
  }
  ```
- **Resposta Sucesso (200):**
  ```json
  {
    "success": true,
    "message": "Senha financeira atualizada com sucesso."
  }
  ```

---

## 📊 10. Matriz Resumida de Esquemas de Dados (DTOs)

```typescript
interface User {
  id: string;                      // ex: "CB-458F"
  username: string;                // ex: "Mateus"
  phone: string;                   // ex: "11999999999"
  balance: number;                 // ex: 2450.00
  nivel: number;                   // ex: 8
  xp: number;                      // ex: 500
  securityPassword?: string | null;// ex: "123456"
  coletasCompleted: number;        // ex: 4
  tratorRented: boolean;           // ex: false
  alreadySpunToday: boolean;       // ex: false
  referralBonusSuspended: boolean; // ex: false
  withdrawLocked: boolean;         // ex: false
}

interface GarageMachine {
  id: string;                      // ex: "mach-1"
  userId: string;                  // ex: "CB-458F"
  name: string;                    // ex: "Escavadeira CAT 320"
  cost: number;                    // ex: 2500.00
  yieldAmount: number;             // ex: 110.00
  lastCollectedAt: string;         // ISO Date String
  state: "aguardando" | "disponivel" | "expirado";
  secondsRemaining: number;
}

interface FinancialTransaction {
  id: string;                      // ex: "tx-1" / "PIX-123456" / "WDR-987654"
  userId: string;
  data: string;                    // ex: "20/07/2026 14:30"
  tipo: string;                    // "Depósito PIX" | "Saque" | "Rendimento [Nome]" | "Aluguel [Nome]"
  valor: number;                   // Positivo para depósitos/rendimentos, negativo para saques/aluguéis
  status: "sucesso" | "pendente" | "cancelado";
}

interface ReferralUser {
  id: string;                      // ex: "ref-1"
  referrerId: string;              // ex: "CB-458F"
  nome: string;                    // ex: "João Silva"
  status: "ativo" | "inativo";
  deposito: number;
  data: string;
}
```

---

## 🎯 Conclusão & Checklist do Backend
Com estes esquemas e contratos de API, o desenvolvedor backend pode implementar todas as rotas e regras de banco de dados (PostgreSQL/MySQL/MongoDB) com **100% de compatibilidade** com a versão cliente do CatBrasil.
