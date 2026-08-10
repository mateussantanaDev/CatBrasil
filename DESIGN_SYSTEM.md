# UNISISM — Design System

Guia de replicação da linguagem visual e componentes do UNISISM para ser aplicado na **Face 2 (Secretaria / Moderação)** e em quaisquer Faces futuras (ex.: aplicativo do cidadão).

> **Objetivo:** qualquer equipe, a partir deste documento, consegue montar uma nova UI do ecossistema UNISISM com **identidade visual idêntica** à Face 1 (UBS), usando os mesmos tokens, componentes e padrões.

---

## Índice

1. [Filosofia B2G Brutalista](#1-filosofia-b2g-brutalista)
2. [Tokens](#2-tokens)
3. [Setup técnico (Tailwind + Fontes + CSS global)](#3-setup-técnico)
4. [Layout shell](#4-layout-shell)
5. [Componentes](#5-componentes)
6. [Padrões de tela](#6-padrões-de-tela)
7. [Estados (loading / empty / erro / sucesso)](#7-estados)
8. [Navegação e RBAC (row-levels no UI)](#8-navegação-e-rbac)
9. [Acessibilidade](#9-acessibilidade)
10. [Anti-patterns (o que NÃO fazer)](#10-anti-patterns)
11. [Checklist de replicação para Face 2](#11-checklist-de-replicação-para-face-2)

---

## 1. Filosofia B2G Brutalista

**B2G = Business to Government.** O UNISISM é software institucional que passa por auditoria, é operado sob jornada longa, exibe dados sensíveis, e seu usuário é um servidor público treinado — não um consumidor.

**Mental model** (referências visuais): terminais da Bloomberg, painéis da Palantir, consoles da AWS, interfaces da NASA Mission Control.

### Princípios

| # | Princípio | Tradução prática |
|---|---|---|
| P1 | **Autoridade institucional** | Fonte sem serifa · alto contraste · azul marinho como cor de ação. Nada que pareça "startup fofa". |
| P2 | **Densidade informacional** | Uma tela ≈ um Bloomberg panel. Evitar rolagem longa. Prefira `text-xs` e `text-[11px]` com `leading-tight`. |
| P3 | **Cantos retos** | Proibido `rounded-*` além de `rounded-sm`. O reset global zera `border-radius` em `button/input/select/textarea`. |
| P4 | **Bordas finas, não sombras** | Hierarquia por bordas `border-slate-200` e por cor de fundo, nunca por `shadow-md` difuso. |
| P5 | **Cores sóbrias** | Azul marinho `blue-900` = ação. Cinzas `slate-*` = neutro. Semântico só em estados (verde ok / âmbar pendência / vermelho crítico). |
| P6 | **Monoespaçada para dados** | `font-mono` em protocolos, CPF, datas, IDs, timestamps e badges. `font-sans` (Inter) em nomes e textos corridos. |
| P7 | **Atalhos por teclado** | Todo botão primário e toda aba ganha um `<kbd>` com letra/número. O usuário é operador — navega com teclado. |
| P8 | **Sub-rotas sobre modais** | Informação persistente/detalhada = rota. Ação pontual (resolver pendência, abrir perfil) = sub-rota ou modal focado. |
| P9 | **Zero gradiente colorido** | Única exceção: gradientes muito sutis `from-white to-slate-50` em cabeçalhos/rodapés pra criar profundidade mínima. |
| P10 | **RBAC visível** | UI esconde o que o backend já proíbe via 403. Não mostra botão que o usuário não pode usar. |

---

## 2. Tokens

### 2.1. Paleta de cores

Todos derivados do Tailwind v4 stock — **não** criar paleta customizada.

| Classe | Uso |
|---|---|
| `bg-slate-50` | Fundo da página |
| `bg-white` | Fundo de painéis e cards |
| `bg-slate-100` | Hover de linhas / botões secundários ativos |
| `border-slate-200` | Borda padrão de containers (1px) |
| `border-slate-300` | Borda de botões secundários e inputs |
| `text-slate-900` | Texto primário |
| `text-slate-700` | Texto de tabelas e corpo |
| `text-slate-600` | Texto secundário |
| `text-slate-500` | Labels (`uppercase tracking-widest`) |
| `text-slate-400` | Placeholders e ícones neutros |

**Cor institucional (ação)**:

| Classe | Uso |
|---|---|
| `bg-blue-900` | Botão primário, barra de acento, avatar, badge ativa |
| `text-blue-900` | Links e números de protocolo |
| `bg-blue-50` / `border-blue-700` | Tabs ativas, informativo |

**Semânticas (estados apenas)**:

| Tom | Classes | Uso |
|---|---|---|
| Sucesso | `border-emerald-700 bg-emerald-50 text-emerald-800` | APROVADO, sessão ativa, exame normal, confirmação |
| Atenção | `border-amber-600 bg-amber-50 text-amber-800` | PENDÊNCIA, PRIORITÁRIA, exame alterado, aguardando |
| Crítico | `border-red-700 bg-red-50 text-red-800` | URGENTE, alergia grave, rejeitado, abandono >90d |
| Neutro escuro | `border-slate-600 bg-slate-50 text-slate-700` | Rascunho, encerrada |

**Regra de ouro**: nenhum uso estético/decorativo de cor. Verde/âmbar/vermelho são reservados pra comunicar **estado** — nunca pra "bonito".

### 2.2. Tipografia

**Famílias**:
- `font-sans` → Inter (nomes, textos corridos, cabeçalhos)
- `font-mono` → `ui-monospace` / SF Mono / JetBrains Mono (IDs, protocolos, datas, badges, labels uppercase, tabelas de dados)

**Escala em uso** (apenas estes tamanhos):

| Tailwind | px | Uso |
|---|---|---|
| `text-[9px]` | 9 | Atalhos de teclado (`<kbd>`) |
| `text-[10px]` | 10 | Labels `uppercase tracking-widest`, badges |
| `text-[11px]` | 11 | Texto auxiliar em painéis densos |
| `text-xs` | 12 | Corpo de tabelas, formulários |
| `text-sm` | 14 | Texto principal, inputs |
| `text-base` | 16 | Títulos de painéis destacados |
| `text-lg` | 18 | Cabeçalho de modal / login |
| `text-xl`–`text-3xl` | 20–30 | Métricas numéricas (`MetricCard`) |

**Features**: Inter com `font-feature-settings: 'cv02','cv03','cv04','cv11'` (melhora legibilidade de `0/O` e `1/I/l`). Já configurado globalmente.

**Spacing de texto**:
- Labels sempre: `text-[10px] font-semibold tracking-widest uppercase`
- Títulos de painéis: `font-mono text-xs font-bold tracking-widest uppercase`
- Dados tabulares: `font-mono text-xs`
- Números grandes: `font-mono text-3xl font-bold tracking-tight`

### 2.3. Espaçamento

Baseado em múltiplos de 4px (padrão Tailwind). Em uso:

- `gap-px` → divisores 1px em grids
- `gap-1` / `gap-1.5` / `gap-2` → grupos de chips/kbd
- `gap-3` → separação entre elementos dentro de painel
- `gap-4` → entre painéis
- `px-4 py-3` → padding padrão de cabeçalho de painel
- `px-4 py-4` → padding de conteúdo de painel
- `px-6 py-10` → padding de página de login/standalone

### 2.4. Bordas

| Classe | Uso |
|---|---|
| `border border-slate-200` | Container padrão |
| `border border-slate-100` | Divisores de lista (`divide-y divide-slate-100`) |
| `border-2 border-red-700` | Destaque crítico (alergias, pendências urgentes) |
| `border-2 border-emerald-700` | Confirmação de sucesso (protocolo criado) |
| `border-2 border-slate-900` | Modal (brutalismo pesado) |
| `border-l-4 border-blue-900` | Blockquote de justificativa |
| `border-l-2 border-blue-900` | Item de navegação ativo |

### 2.5. Sombras

**Praticamente banidas.** Exceções documentadas:

- Modal: `shadow-[8px_8px_0_rgba(15,23,42,0.12)]` → sombra offset BRUTA, não difusa
- Itens ativos da sidebar: `shadow-sm` sutil como única exceção

Nunca usar `shadow-md`, `shadow-lg`, `shadow-xl`.

### 2.6. Cantos arredondados

**Zero**. O reset global força `border-radius: 0` em `button/input/textarea/select`. Único arredondamento permitido é `rounded-sm` (2px) em casos raríssimos.

### 2.7. Ícones

Usar **Heroicons** inline (SVG) com `stroke-width="1.5"` ou `"2"`, `class="h-3 w-3"` a `h-10 w-10`. Não instalar biblioteca de ícones — copia-cola direto do [heroicons.com](https://heroicons.com) quando necessário.

---

## 3. Setup técnico

### 3.1. Dependências mínimas

```json
{
  "@tailwindcss/forms": "^0.5.11",
  "@tailwindcss/typography": "^0.5.19",
  "@tailwindcss/vite": "^4.2.2",
  "tailwindcss": "^4.2.4",
  "svelte": "^5.55.2",
  "@sveltejs/kit": "^2.57.0",
  "typescript": "^6.0.2"
}
```

### 3.2. `src/routes/layout.css`

Arquivo **idêntico** em todas as Faces:

```css
@import 'tailwindcss';
@plugin '@tailwindcss/forms';
@plugin '@tailwindcss/typography';

@theme {
  --font-sans:
    'Inter', system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
    'Helvetica Neue', Arial, sans-serif;
  --font-mono:
    ui-monospace, 'SF Mono', 'JetBrains Mono', Menlo, Consolas, 'Liberation Mono',
    monospace;
}

html, body {
  font-family: var(--font-sans);
  font-feature-settings: 'cv02', 'cv03', 'cv04', 'cv11';
  -webkit-font-smoothing: antialiased;
  background-color: rgb(248 250 252);
  color: rgb(15 23 42);
}

/* B2G Brutalist: squared corners by default, subtle outlines over shadows. */
button, input, textarea, select { border-radius: 0; }

::selection {
  background-color: rgb(30 58 138);
  color: white;
}
```

### 3.3. `src/app.html` (fonte Inter via CDN)

```html
<link rel="preconnect" href="https://rsms.me/" />
<link rel="stylesheet" href="https://rsms.me/inter/inter.css" />
```

Ou empacotar localmente para air-gapped. Nunca usar Google Fonts (LGPD).

---

## 4. Layout shell

Toda tela autenticada do UNISISM segue **este shell**:

```
┌──────────┬──────────────────────────────────────┐
│          │  Header (breadcrumb + título + clock)│
│          ├──────────────────────────────────────┤
│ Sidebar  │                                      │
│ (w-60)   │  Main (max-w-[1600px], px-6 py-5)    │
│          │                                      │
│          │                                      │
│          ├──────────────────────────────────────┤
│          │  Footer (versão + sessão)            │
└──────────┴──────────────────────────────────────┘
```

Implementação: [src/routes/ubs/+layout.svelte](src/routes/ubs/+layout.svelte).

### 4.1. Sidebar (w-60, fixa)

- Topo: logo UNISISM (quadrado azul `bg-blue-900` com letra) + nome do sistema + "FACE N / ÁREA"
- Bloco de contexto: **UNIDADE OPERACIONAL** / **PREFEITURA** / **ACESSO GLOBAL** (conforme escopo)
- Nav vertical com border-left ativo (`border-blue-900`), atalho `<kbd>` à direita
- Rodapé com avatar (iniciais), nome, role — **clicável**, abre `/face-N/perfil`

Componente: [Sidebar.svelte](src/lib/presentation/components/Sidebar.svelte).

### 4.2. Header

- Esquerda: breadcrumb + título da página (`pageInfo`)
- Direita: badges de status da API, `role` do usuário, relógio

### 4.3. Footer

- Esquerda: `UNISISM v0.1.0 · FACE N · BUILD YYYY.MM`
- Direita: `ATENDENTE: matricula · UNIDADE: X · ESCOPO: Y` (de `auth.me`)

### 4.4. Standalone layout (login)

Duas colunas — `w-[420px]` institucional azul à esquerda, formulário à direita.
Sem sidebar/header/footer. Implementação: [src/routes/login/+layout.svelte](src/routes/login/+layout.svelte).

---

## 5. Componentes

Todos em `src/lib/presentation/components/`. Svelte 5 puro (runes + snippets), zero dependência externa.

### 5.1. `PanelHeader`

Cabeçalho padronizado de todo painel. Traz índice numerado opcional.

```svelte
<PanelHeader title="Paciente" subtitle="Identificação resumida" index="01">
  <StatusBadge prioridade="URGENTE" />
</PanelHeader>
```

**Props**: `title` (string) · `subtitle?` · `index?` (string, ex "01") · `children?` (snippet).
**Visual**: borda inferior `slate-200`, fundo gradiente suave `from-slate-50 to-white`, chip azul quadrado com o índice, título em `font-mono text-xs font-bold tracking-widest uppercase`.

→ [PanelHeader.svelte](src/lib/presentation/components/PanelHeader.svelte)

### 5.2. `MetricCard`

Card numérico com barra de acento lateral. **Principal elemento de dashboard.**

```svelte
<MetricCard
  label="Encaminhamentos Hoje"
  value={47}
  sublabel="Ingestões consolidadas em 24h"
  trend="+12%"
  trendDirection="up"
  accent="default"
/>
```

**Props**:
- `label` · `value` (string|number) · `sublabel?`
- `trend?` (string curta, ex "+12%") · `trendDirection?`: `'up' | 'down' | 'neutral'`
- `accent?`: `'default' | 'warning' | 'critical' | 'success'`

**Visual**: borda `slate-200`, barra lateral 1px de 4 cores conforme `accent`, número grande monoespaçado, trend colorido (verde/vermelho/cinza).

→ [MetricCard.svelte](src/lib/presentation/components/MetricCard.svelte)

### 5.3. `StatusBadge`

Badge com borda fina para estados. Aceita ou `status` ou `prioridade`.

```svelte
<StatusBadge status="PENDENCIA_DOCUMENTO" />
<StatusBadge prioridade="URGENTE" />
```

**Estados mapeados**: RASCUNHO, AGUARDANDO_REGULACAO, PENDENCIA_DOCUMENTO, APROVADO, REJEITADO.
**Prioridades**: ELETIVA, PRIORITARIA, URGENTE, EMERGENCIA (esta última com fundo vermelho escuro).

**Visual**: `border px-1.5 py-0.5 font-mono text-[10px] font-bold tracking-wider`.

Na Face 2 adicione novos estados (ex.: `EM_ANALISE`, `AGUARDANDO_AGENDAMENTO`) no mesmo padrão de tones semânticos.

→ [StatusBadge.svelte](src/lib/presentation/components/StatusBadge.svelte)

### 5.4. `PrimaryButton`

Botão brutalista quadrado com 3 variantes + suporte a atalho.

```svelte
<PrimaryButton
  label="Consolidar e Enviar"
  onclick={...}
  loading={enviando}
  disabled={!preenchido}
  variant="primary"   {/* 'primary' | 'secondary' | 'danger' */}
  shortcut="↵"
  fullWidth
/>
```

**Variantes**:
- `primary` → fundo `blue-900`, texto branco
- `secondary` → fundo branco, borda `slate-300`, hover `blue-900`
- `danger` → fundo `red-800`, texto branco

**Regras**: sempre `font-bold tracking-widest uppercase`. Ícone de atalho renderizado como `<kbd>` com borda semitransparente.

→ [PrimaryButton.svelte](src/lib/presentation/components/PrimaryButton.svelte)

### 5.5. `FormField`

Input controlado com label uppercase e grid-span built-in.

```svelte
<FormField
  label="Nova Senha"
  name="novaSenha"
  type="password"   {/* 'text' | 'password' | 'email' | 'number' | 'date' */}
  span={6}          {/* 1 | 2 | 3 | 4 | 6 | 12 → grid col-span-N */}
  mono
  loading={extraindo}
  bind:value={senha}
/>
```

**Regras**:
- Sempre usar dentro de um grid `grid-cols-12 gap-3`
- `mono` quando o valor é numérico/padrão (CPF, CRM, SUS, CID-10)
- `readonly` → fundo `slate-50`
- `loading` → overlay animado

→ [FormField.svelte](src/lib/presentation/components/FormField.svelte)

### 5.6. `Dropzone`

Área drag & drop grande e óbvia (WCAG acessível via teclado).

```svelte
<Dropzone
  label="ARRASTE O PDF DA SOLICITAÇÃO MÉDICA"
  sublabel="PDF nativo ou escaneado. O sistema executará OCR + extração."
  acceptTypes="application/pdf"
  mode="ocr"          {/* 'ocr' | 'simple' */}
  loading={extraindo}
  loadingLabel="LENDO DOCUMENTO ORIGINAL..."
  multiple
  variant="primary"   {/* 'primary' (240px) | 'secondary' (140px) */}
  files={solicitacaoFile}
  onFiles={handleSolicitacao}
/>
```

**Estados**: idle (mostra ícone + labels) · loading (spinner + texto OCR) · filled (lista de arquivos com tamanho).

**Regras de negócio**:
- Upload da Solicitação Médica → `mode="ocr"` (mostra "OCR + EXTRAÇÃO ESTRUTURADA EM ANDAMENTO")
- Anexos complementares → `mode="simple"` + `variant="secondary"` (sem OCR, apenas upload)

→ [Dropzone.svelte](src/lib/presentation/components/Dropzone.svelte)

### 5.7. `Modal`

Modal focado para ações pontuais (resolver pendência, confirmar alteração).

```svelte
<Modal
  isOpen={aberto}
  onClose={() => (aberto = false)}
  title="Resolver Pendência"
  subtitle="Readequação e reenvio à Regulação"
  maxWidth="lg"   {/* sm | md | lg | xl */}
>
  {/* children */}
</Modal>
```

**Visual**: backdrop `slate-900/60`, caixa `border-2 border-slate-900` com sombra offset brutalista `shadow-[8px_8px_0_rgba(15,23,42,0.12)]`. Fecha por backdrop, Esc ou botão X.

**Quando usar**: ações que **não** fazem sentido como rota (confirmar, editar pontualmente, confirmar destruição). Para fluxos maiores, prefira sub-rotas.

→ [Modal.svelte](src/lib/presentation/components/Modal.svelte)

### 5.8. `SubNav`

Barra de abas brutalista para sub-rotas.

```svelte
<SubNav tabs={[
  { label: 'Visão Geral', href: '/face-N/dashboard', shortcut: '1' },
  { label: 'Fila', href: '/face-N/dashboard/fila', shortcut: '3',
    badge: 9, badgeTone: 'critical' },
]} />
```

**Props por tab**: `label` · `href` (exact match para ativo) · `shortcut?` · `badge?` (string|number) · `badgeTone?`: `'default' | 'warning' | 'critical' | 'success'`.

**Visual**: tab ativa com `border-t` azul-marinho 2px + fundo branco (vs. `slate-50` das inativas).

→ [SubNav.svelte](src/lib/presentation/components/SubNav.svelte)

### 5.9. `TimelineStep`

Item de timeline vertical para histórico de eventos.

```svelte
<ol>
  {#each eventos as ev, i}
    <TimelineStep
      tipo={ev.tipo}
      titulo={ev.titulo}
      descricao={ev.descricao}
      autor={ev.autor}
      autorPapel={ev.autorPapel}
      em={ev.em}
      isLast={i === eventos.length - 1}
    />
  {/each}
</ol>
```

**Visual**: bolinha colorida por tipo (azul=criação, cinza=anexo, âmbar=pendência, verde=aprovação/agendamento, vermelho=rejeição) + linha vertical conectando. Cada evento é um mini-painel branco.

→ [TimelineStep.svelte](src/lib/presentation/components/TimelineStep.svelte)

### 5.10. `HistoricoTable`

Tabela especializada para listagens de encaminhamento com busca + clique na linha.

```svelte
<HistoricoTable
  titulo="Aguardando Regulação"
  subtitulo="Encaminhamentos em fila na SMS"
  lista={aguardando}
  carregando={...}
  mostrarStatus={false}
/>
```

Reutilizável em qualquer listagem de encaminhamentos. Para outras entidades (usuários, UBSs etc.), crie variantes no mesmo padrão: `<DataTable titulo colunas linhas>`.

→ [HistoricoTable.svelte](src/lib/presentation/components/HistoricoTable.svelte)

### 5.11. Padrão de tabela (para replicar)

Quando precisar criar uma tabela nova (ex.: lista de UBSs na Face 2):

```svelte
<div class="overflow-x-auto">
  <table class="w-full border-collapse text-xs">
    <thead>
      <tr class="border-b border-slate-200 bg-slate-50 text-left
                 font-mono text-[10px] tracking-widest text-slate-600 uppercase">
        <th class="border-r border-slate-200 px-3 py-2">Coluna 1</th>
        <th class="border-r border-slate-200 px-3 py-2">Coluna 2</th>
        <th class="px-3 py-2">Ação</th>
      </tr>
    </thead>
    <tbody class="font-mono">
      {#each linhas as l (l.id)}
        <tr class="cursor-pointer border-b border-slate-100 hover:bg-slate-50"
            onclick={() => goto(`/detalhe/${l.id}`)}>
          <td class="border-r border-slate-100 px-3 py-2 font-bold text-blue-900
                     underline decoration-blue-900/30 underline-offset-2">
            {l.protocolo}
          </td>
          <td class="border-r border-slate-100 px-3 py-2 font-sans text-slate-900">
            {l.nome}
          </td>
          <td class="px-3 py-2" onclick={(e) => e.stopPropagation()}>
            <PrimaryButton label="Abrir" variant="secondary" onclick={...} />
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
```

**Regras**:
- `border-collapse` + `border-r` entre colunas (não `border` em cada célula)
- Head: `bg-slate-50` + label uppercase `font-mono text-[10px]`
- Body: `font-mono` em toda tbody; sobrescreve com `font-sans` nas células de nome
- Hover: `bg-slate-50` (ou `bg-red-50/40` em listas críticas)
- Linha clicável sempre tem `cursor-pointer` + `onclick` com `goto` + coluna de ação usa `onclick={(e) => e.stopPropagation()}`

---

## 6. Padrões de tela

### 6.1. Dashboard

```
┌──────────────────────────────────────────────────┐
│ SubNav (tabs do dashboard)                       │
├──────────────────────────────────────────────────┤
│ Grid 2/3/6 de MetricCards                        │
├──────────────────────────────────────────────────┤
│ Grid 12                                          │
│ ┌── col-span-9 ──┐ ┌── col-span-3 ──┐            │
│ │  Tabela        │ │  Ações rápidas │            │
│ │  (últimos)     │ │  + Atalhos     │            │
│ └────────────────┘ └────────────────┘            │
└──────────────────────────────────────────────────┘
```

Exemplo: [src/routes/ubs/dashboard/+page.svelte](src/routes/ubs/dashboard/+page.svelte).

### 6.2. Lista + filtros + tabela

```
┌──────────────────────────────────────────────────┐
│ SubNav (tabs de filtro por status)               │
├──────────────────────────────────────────────────┤
│ Grid 2/4 de MetricCards (contagens por filtro)   │
├──────────────────────────────────────────────────┤
│ Painel único:                                    │
│  ├ PanelHeader com badge "N / total REGISTROS"   │
│  ├ Barra de busca (input monoespaçado)           │
│  └ Tabela densa com hover                        │
└──────────────────────────────────────────────────┘
```

Exemplo: [src/routes/ubs/historico/+page.svelte](src/routes/ubs/historico/+page.svelte).

### 6.3. Detalhe com sub-tabs (rota dinâmica)

Para telas com várias facetas (encaminhamento, paciente, perfil):

```
/face-N/entidade/[id]/+layout.svelte  ← action bar + carrega dados + SubNav
                      +page.svelte     → Resumo
                      sub-a/+page      → Sub-tela A
                      sub-b/+page      → Sub-tela B
                      ...
```

**Padrão do `+layout.svelte`**:
1. Recebe `id` via `page.params.id`
2. Usa `$effect` para refetch quando `id` muda
3. Cria um Svelte context com getter `encaminhamento`/`paciente`/etc.
4. Renderiza action bar (voltar + breadcrumb + protocolo + badges + ações)
5. Renderiza `<nav>` com as abas
6. Gate: `{#if carregando}...{:else if erro}...{:else}{@render children()}{/if}`

Exemplo: [src/routes/ubs/encaminhamento/[id]/+layout.svelte](src/routes/ubs/encaminhamento/[id]/+layout.svelte).

### 6.4. Wizard multi-step

Para fluxos lineares (Novo Encaminhamento, Onboarding de paciente, Nova UBS…):

```
/face-N/acao/+layout.svelte        ← state compartilhado (context) + stepper
             +page.svelte           → Passo 1
             passo-b/+page.svelte   → Passo 2
             passo-c/+page.svelte   → Passo 3
```

**Padrão do stepper**: círculos numerados, `✓` quando concluído, preenchido em `blue-900` quando ativo, cinza quando bloqueado. Gate `podeIr(n)` — passo 2 só aberto quando passo 1 validado.

Exemplo: [src/routes/ubs/novo-encaminhamento/+layout.svelte](src/routes/ubs/novo-encaminhamento/+layout.svelte).

### 6.5. Tela de perfil/configuração

Header fixo com identidade (avatar + nome + ação "Encerrar Sessão") + SubNav com seções (Visão Geral, Conta, Segurança, Produção, Relatórios). Cada seção é sub-rota.

Exemplo: [src/routes/ubs/perfil/+layout.svelte](src/routes/ubs/perfil/+layout.svelte).

### 6.6. Login / standalone

Layout dedicado `/login/+layout.svelte` com painel institucional azul à esquerda + formulário à direita. Sem sidebar. Ver [src/routes/login/+layout.svelte](src/routes/login/+layout.svelte).

### 6.7. Fluxo de recuperação de senha

3 passos em uma **única rota** (`/login/esqueci-senha`) com estado local do passo atual (`$state<1|2|3|4>`). Stepper horizontal indica progresso. Último passo (`4`) é confirmação de sucesso.

Exemplo: [src/routes/login/esqueci-senha/+page.svelte](src/routes/login/esqueci-senha/+page.svelte).

---

## 7. Estados

### 7.1. Loading

**Spinner padrão brutalista**:
```svelte
<div class="h-6 w-6 animate-spin border-[3px] border-blue-900 border-t-transparent"></div>
```

Quando dentro de tabela, usar skeleton row:
```svelte
{#if carregando}
  {#each Array(6) as _, i (i)}
    <tr class="border-b border-slate-100">
      <td colspan="N" class="px-3 py-3">
        <div class="h-3 w-full animate-pulse bg-slate-100"></div>
      </td>
    </tr>
  {/each}
{/if}
```

### 7.2. Empty state

Mensagem centralizada em `font-sans text-sm text-slate-500`:
```svelte
<td colspan="N" class="px-3 py-12 text-center font-sans text-sm text-slate-500">
  Nenhum encaminhamento encontrado com os filtros aplicados.
</td>
```

Quando for toda uma tela, adicionar CTA de ação primária embaixo.

### 7.3. Erro

Container com borda vermelha dupla:
```svelte
<div class="border-2 border-red-700 bg-red-50 p-6 text-center">
  <div class="font-mono text-sm font-bold tracking-widest text-red-900 uppercase">
    Encaminhamento não encontrado
  </div>
  <p class="mt-2 text-xs text-red-800">O ID informado não foi localizado.</p>
</div>
```

**Erros de validação inline** (em forms):
```svelte
<div class="border border-red-700 bg-red-50 px-3 py-2
            font-mono text-[11px] font-bold tracking-wider text-red-800 uppercase">
  ⚠ {mensagem}
</div>
```

### 7.4. Sucesso

Confirmação de ação crítica:
```svelte
<div class="border-2 border-emerald-700 bg-emerald-50 p-6">
  <div class="font-mono text-[11px] font-bold tracking-widest text-emerald-800 uppercase">
    ✓ ENCAMINHAMENTO CONSOLIDADO COM SUCESSO
  </div>
  <div class="mt-1 font-mono text-3xl font-bold text-emerald-900">
    UBS-2026-100137
  </div>
</div>
```

### 7.5. Info / alerta contextual

Faixa com borda lateral:
```svelte
<div class="border-l-4 border-amber-600 bg-amber-50 px-3 py-2">
  <div class="font-mono text-[10px] font-bold tracking-widest text-amber-800 uppercase">
    Observações da Regulação
  </div>
  <div class="mt-0.5 text-xs text-amber-900">{texto}</div>
</div>
```

Paleta de borda lateral: `emerald-700` (sucesso) / `amber-600` (atenção) / `red-700` (crítico) / `blue-900` (informativo/justificativa).

---

## 8. Navegação e RBAC

### 8.1. Sub-rotas sobre modais

Critério de decisão:

| Situação | Use |
|---|---|
| Múltiplas facetas de um recurso (detalhe, abas) | Sub-rotas |
| Fluxo linear em passos (wizard) | Sub-rotas com stepper |
| Busca/filtro por status | Sub-rotas (URL-shareable) |
| Ação pontual sobre um recurso (editar, resolver, confirmar) | Modal focado |
| Seleção rápida em contexto | Dropdown/popover (não temos ainda) |

### 8.2. Row-level no UI (RBAC)

**Regra**: UI esconde o que o backend já proíbe via 403 — defesa em profundidade.

Pattern central: contexto de auth com helpers puros.

```ts
// src/lib/presentation/contexts/authContext.ts
export const rbac = {
  podeConsolidarEncaminhamento(role: Role | undefined): boolean {
    return role === 'ATENDENTE_UBS' || role === 'COORDENADOR_UBS' || role === 'DESENVOLVEDOR';
  },
  podeCriarUsuario(role: Role | undefined): boolean {
    return role === 'DESENVOLVEDOR' || role === 'ADMIN';
  },
  // …
};
```

E uso em componentes:

```svelte
{#if auth.podeConsolidarEncaminhamento}
  <PrimaryButton label="Novo Encaminhamento" ... />
{/if}
```

**Na Face 2 (Regulação)**, defina os helpers equivalentes:
- `podeAprovarEncaminhamento(role)` → `REGULADOR_SMS | DESENVOLVEDOR`
- `podeRegistrarPendencia(role)` → mesmo
- `podeVerFilaCompleta(role)` → todos autenticados

### 8.3. Auth guard no layout raiz

Todo `+layout` raiz da área autenticada faz:

```ts
onMount(async () => {
  if (!api.tokens.get()) {
    goto('/login', { replaceState: true });
    return;
  }
  try {
    me = await api.auth.me();
  } catch (e) {
    if (e instanceof ApiError && e.status !== 401) {
      api.tokens.set(null);
      goto('/login', { replaceState: true });
    }
  } finally {
    autenticando = false;
  }
});
```

Erros 401 são tratados pelo callback global `setOnUnauthorized` em `src/lib/api/index.ts` — **não** precisa tratar em cada chamada.

### 8.4. Breadcrumbs

Tabela `pageTitles: Record<string, { label, crumb }>` no `+layout.svelte` raiz + regex pra rotas dinâmicas. Título em `pageInfo.label`, breadcrumb em `pageInfo.crumb`. Exemplo: `UBS / HISTÓRICO / DETALHE / PACIENTE`.

---

## 9. Acessibilidade

- **Contraste**: toda combinação já atende WCAG AA (textos `slate-900` em `slate-50`, `slate-700` em `white`).
- **Atalhos de teclado**: todo botão primário e toda aba têm `<kbd>` visível. Implementação de `keydown` opcional mas recomendada.
- **Navegação por Tab**: todos os elementos interativos (incluindo linhas clicáveis de tabela) devem ter `tabindex`, `role="button"` e handler `onkeydown` (Enter/Space) — ver [Dropzone.svelte](src/lib/presentation/components/Dropzone.svelte).
- **ARIA**: `aria-label` em botões icônicos, `aria-busy={loading}` em containers de carregamento, `role="presentation"` em backdrops de modal.
- **Foco visível**: manter `focus:ring-1 focus:ring-blue-900` + `focus:border-blue-900` em inputs.
- **Anúncio de estado**: usar `aria-live="polite"` em toasts/feedback quando implementados.

---

## 10. Anti-patterns

| ❌ NÃO FAZER | ✅ FAZER |
|---|---|
| `rounded-lg`, `rounded-xl`, `rounded-full` | `rounded-sm` no máximo, quase sempre retos |
| `shadow-md` ou `shadow-lg` difusos | Bordas finas `border-slate-200` |
| Gradientes coloridos (`from-blue-500 to-purple-500`) | `from-white to-slate-50` sutil (apenas headers/footers) |
| Paleta custom (`#3E8EDE`…) | Apenas `slate-*` e `blue-900` + semânticas |
| Cor decorativa (verde "porque é bonito") | Verde/âmbar/vermelho apenas para ESTADO |
| Ícones coloridos decorativos | Heroicons em `currentColor` |
| Tipografia com serifa, scripts, ou fontes divertidas | Inter (sans) + ui-monospace |
| Tooltips excessivos | Labels uppercase explícitas |
| Uma tela com 10 responsabilidades | Sub-rotas — uma tela por responsabilidade |
| Botão fantasma (texto-only sem borda) | `<PrimaryButton variant="secondary">` |
| Ícones grandes + pouca densidade | Alta densidade de dados, ícones pequenos `h-3 w-3` a `h-5 w-5` |
| Animações elaboradas | Apenas `transition-colors` e `animate-spin`/`animate-pulse` |
| Mocks no código de produção | Camada `$lib/api/` tipada com contrato do backend |
| Hardcode de `me.nome` | `useAuth().me` de contexto |

---

## 11. Checklist de replicação para Face 2

Passo a passo pra criar a Face 2 (Secretaria / Moderação) com identidade visual idêntica.

### Setup inicial

- [ ] Novo projeto SvelteKit 2 + Svelte 5 + TypeScript + Tailwind v4
- [ ] Instalar as deps listadas em [§3.1](#31-dependências-mínimas)
- [ ] Copiar `src/routes/layout.css` integral
- [ ] Copiar bloco do Inter em `src/app.html`
- [ ] Criar `.env` com `VITE_API_BASE_URL` apontando pro backend comum

### Camada de API

- [ ] Copiar `src/lib/api/types.ts` (mesmo contrato — fonte da verdade é o backend)
- [ ] Copiar `src/lib/api/client.ts` + modificação `setOnUnauthorized`
- [ ] Criar `src/lib/api/index.ts` com singleton + redirect global ao expirar
- [ ] Manter `src/lib/domain/models/*.ts` como re-exports dos tipos

### Design system (copiar integral)

- [ ] `src/lib/presentation/components/PanelHeader.svelte`
- [ ] `src/lib/presentation/components/MetricCard.svelte`
- [ ] `src/lib/presentation/components/StatusBadge.svelte` (adicionar novos estados específicos de Regulação)
- [ ] `src/lib/presentation/components/PrimaryButton.svelte`
- [ ] `src/lib/presentation/components/FormField.svelte`
- [ ] `src/lib/presentation/components/Dropzone.svelte`
- [ ] `src/lib/presentation/components/Modal.svelte`
- [ ] `src/lib/presentation/components/SubNav.svelte`
- [ ] `src/lib/presentation/components/TimelineStep.svelte`
- [ ] `src/lib/presentation/components/Sidebar.svelte` (adaptar menu e label "FACE 2 / SMS")

> **Sugestão**: extrair estes componentes para um pacote npm `@unisism/ui` consumido pelas duas Faces. Evita drift de design ao longo do tempo.

### Auth + guard

- [ ] Copiar `src/lib/presentation/contexts/authContext.ts` + adaptar helpers RBAC para roles da Face 2 (`REGULADOR_SMS` como cidadão de primeira classe)
- [ ] Copiar estrutura de `src/routes/login/*` (layout standalone + login + esqueci-senha)
- [ ] Replicar padrão de guard no `+layout.svelte` da área autenticada

### Estrutura de telas da Face 2

Proposta (ajustar ao produto):

```
src/routes/sms/
├── +layout.svelte              ← shell + auth guard
├── dashboard/
│   ├── +layout.svelte           ← SubNav (Visão / Pendentes / Aprovados / Rejeitados)
│   ├── +page.svelte              → Visão Geral
│   └── pendentes/+page.svelte   → Fila de análise
├── encaminhamento/[id]/
│   ├── +layout.svelte           ← action bar com "Aprovar" / "Solicitar Correção" / "Rejeitar"
│   ├── +page.svelte              → Resumo
│   ├── paciente/+page.svelte
│   ├── clinico/+page.svelte
│   ├── anexos/+page.svelte
│   └── historico/+page.svelte
├── pacientes/                   ← mesma estrutura da Face 1
├── perfil/                      ← mesma estrutura da Face 1
└── relatorios/                  ← relatórios específicos (SLA, ranking UBS…)
```

### Ações específicas da Face 2 (Regulação)

Criar componentes análogos ao `ResolverPendencia.svelte`:

- `AprovarEncaminhamento.svelte` — modal com nota de aprovação + agendamento previsto
- `SolicitarCorrecao.svelte` — modal com texto da pendência (vira `observacoesRegulacao` no encaminhamento)
- `RejeitarEncaminhamento.svelte` — modal com motivo obrigatório

Cada um chama endpoints que o backend **ainda precisa expor** (hoje o contrato só tem `resolve-pendencia` pro lado UBS). Alinhar com backend antes de construir.

### Testes de consistência visual

Antes do go-live, conferir:

- [ ] Nenhum `rounded-lg`/`rounded-xl`/`rounded-full` no codebase (`grep -rn "rounded-\(lg\|xl\|full\|2xl\|3xl\)" src/`)
- [ ] Nenhuma `shadow-lg`/`shadow-xl` (`grep -rn "shadow-\(lg\|xl\|2xl\)" src/`)
- [ ] Nenhuma cor fora de `slate-*`, `blue-900`/950`, `emerald-*`, `amber-*`, `red-*`
- [ ] Todo botão primário tem `<PrimaryButton>`, nunca `<button>` solto
- [ ] `svelte-check` com 0 erros 0 warnings
- [ ] Comparar paleta lado a lado com a Face 1 em duas telas equivalentes (dashboard, detalhe)

---

## 12. Referências rápidas

| Tópico | Arquivo |
|---|---|
| CSS global + reset brutalista | [src/routes/layout.css](src/routes/layout.css) |
| Componentes do design system | [src/lib/presentation/components/](src/lib/presentation/components/) |
| Contextos de sessão e dados | [src/lib/presentation/contexts/](src/lib/presentation/contexts/) |
| Cliente HTTP tipado | [src/lib/api/](src/lib/api/) |
| Modelos de domínio (re-exports) | [src/lib/domain/models/](src/lib/domain/models/) |
| Contrato de backend | [BACKEND_API.md](BACKEND_API.md) |
| Rotas de exemplo (UBS) | [src/routes/ubs/](src/routes/ubs/) |

---

## 13. Governança do design system

**Quem altera o quê**:

- Tokens (cores / fontes / tipografia) → alteração raríssima, requer aprovação do Diretor de UX/UI
- Componentes do core (`<PanelHeader>`, `<PrimaryButton>` etc.) → PR revisado por qualquer engenheiro sênior
- Novos componentes (`<DataTable>`, `<Chart>`) → podem ser criados livremente desde que sigam os princípios de [§1](#1-filosofia-b2g-brutalista)
- Paleta semântica (verde/âmbar/vermelho) → **imutável**

**Divergências Face 1 ↔ Face 2** (ou futuras):
- Permitidas: menu items, atalhos, rótulos, dados mock iniciais
- **Proibidas**: tokens, componentes visuais, padrões de layout, anti-patterns

> Qualquer divergência visual entre Faces significa quebra do design system. Abrir uma issue no repo e corrigir antes do merge.
