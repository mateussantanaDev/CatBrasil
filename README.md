# 🚜 CatBrasil - Plataforma ERP & Gamificação Caterpillar

CatBrasil é uma aplicação web full-stack de alta performance focada em gestão ERP gamificada e mineração virtual inspirada no universo Caterpillar. O sistema combina uma interface reativa moderna em **Svelte 5 + Vite** com um backend robusto em **Node.js + Express**, suporte a **WebSockets** em tempo real e integração com pagamentos via **PIX**.

---

## ⚡ Principais Funcionalidades

- 🔒 **Autenticação & Segurança:** Autenticação JWT, criptografia de senhas com bcrypt, sanitização e proteção contra DDoS/Brute-force.
- 🚜 **Gamificação & Mineração:** Gestão de máquinas virtuais, rendimentos diários, ranking semanal de operadores e sistema de peças (lootboxes).
- 💳 **Integração PIX:** Gateway integrado para depósitos e solicitações de saques automatizados.
- 📡 **Comunicação em Tempo Real:** Atualizações instantâneas de eventos, saldo e notificações via Socket.IO.
- 🎨 **Design System Exclusivo:** Interface mobile-first, visual industrial e componentes otimizados para alta performance em dados móveis.

---

## 🛠️ Tecnologias Utilizadas

### **Frontend**
- **Framework:** [Svelte 5](https://svelte.dev/)
- **Bundler:** [Vite](https://vitejs.dev/)
- **Comunicação:** Socket.IO Client, Fetch API
- **Estilização:** CSS3 Vanilla (Design System customizado)

### **Backend**
- **Runtime:** [Node.js](https://nodejs.org/)
- **Framework:** [Express](https://expressjs.com/)
- **Banco de Dados:** SQLite (`better-sqlite3`)
- **Real-time:** Socket.IO
- **Segurança:** JSON Web Tokens (JWT), `bcryptjs`, Rate-Limiting e Headers HTTP seguros

---

## 🚀 Como Executar o Projeto

### **Pré-requisitos**
- Node.js (v18 ou superior)
- npm ou yarn

### **1. Clonar o Repositório**
```bash
git clone https://github.com/mateussantanaDev/CatBrasil.git
cd CatBrasil
```

### **2. Executar o Backend**
```bash
cd backend
npm install
npm run dev
```
> O servidor backend estará rodando em `http://localhost:5001`.

### **3. Executar o Frontend**
```bash
cd ../frontend
npm install
npm run dev
```
> A aplicação estará disponível na porta padrão do Vite (ex: `http://localhost:5173`).

---

## 📂 Estrutura do Projeto

```text
CatBrasil/
├── backend/            # Servidor API Express, SQLite, Socket.IO e rotas PIX
├── frontend/           # Aplicação SPA em Svelte 5 + Vite
├── DESIGN_SYSTEM.md    # Especificações do design system da aplicação
└── README.md           # Documentação principal
```

---

## 📄 Licença

Projeto desenvolvido para fins comerciais / proprietários. Todos os direitos reservados.
