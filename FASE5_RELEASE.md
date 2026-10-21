# Fase 5 — validação de release self hosted

Estes comandos devem ser executados **em staging isolado**, com dados sintéticos e sem credenciais financeiras reais. O smoke padrão não altera dados. `--full-sandbox` cria um usuário com CPF matematicamente válido, nome e email sintéticos, registra identidade **pendente**, prova o bloqueio de PIX e ativa 2FA. `--pix-sandbox` só cria uma cobrança de R$ 1 no stub local depois de uma revisão explícita. Nenhum desses testes comprova identidade civil, liquidação ou saque na SuitPay real.

> **Estado da validação:** O ensaio histórico do staging ficou na migração 8. O baseline corrente deve ser lido dos arquivos `backend/migrations/*.up.sql` e comparado ao banco pelo gate `scripts/release/validate_staging.py`; não aceite uma contagem copiada deste documento como prova. A restauração cifrada em PostgreSQL descartável foi provada localmente, mas a cópia fora do host, RPO/RTO, entrega de alertas, identidade/SMS reais e CI remoto ainda precisam de evidência. O corte real continua **BLOQUEADO**; a homologação oficial da SuitPay permanece uma dependência separada.

---

## 1. Topologia de Rede e Portas do Host

> [!WARNING]
> **Host atual com portas ocupadas:** O contêiner legado `catbrasil_caddy` publica 80/443; `catbrasil_api`, `catbrasil_client_app` e `catbrasil_admin_app` publicam 8080/3000/3002. Há outro PostgreSQL publicado em 5432. O Compose de produção não foi iniciado neste host.

- **Ambiente de Staging:** O compose de staging (`compose.staging.yml`) contorna essa colisão publicando o Caddy e os stubs em portas locais alternativas (`127.0.0.1:8088`, `127.0.0.1:18081`, `127.0.0.1:18082`).
- **Ambiente de Produção:** O Caddy (`compose.prod.yml`) requer 80/443 para TLS. O Compose base também publica API/cliente/ERP em 127.0.0.1:8080/3000/3002; inventarie e libere ou remapeie essas portas no plano de corte. O overlay remove a publicação do PostgreSQL no host, preservando o serviço de terceiros em 5432. Planeje a migração do legado sem interromper staging ou outros projetos. O orquestrador agora testa o bind de **todas as portas publicadas no Compose renderizado** antes da primeira mutação de serviço; esse portão passou em testes locais, sem executar o corte neste host.

---

## 2. Cadeia de Dependências de Release (P0.4 / P0.5 / P0.6)

A prontidão para o corte segue uma ordem rigorosa de dependências técnicas:

1. **P0.4 — Identidade Civil e Entrega de SMS Operacionais (PENDENTE):**
   - Esteira humana de validação documental e KYC civil com evidência auditável, revisor humano distinto e retenção regulatória de registros.
   - Integração do gateway HTTP próprio a provedor/meio real de entrega de SMS (testes com telefone sintético e stubs locais não comprovam conformidade).
2. **P0.5 — Prova no Sandbox Oficial da SuitPay (PREPARAÇÃO LOCAL ACEITA / ENSAIO OFICIAL BLOQUEADO):**
   - Execução dos 13 cenários de teste documentados em `P0_5_SANDBOX.md` contra o ambiente oficial de sandbox da SuitPay.
   - Batimento triplo com divergência zero em centavos: razão contábil interno, banco PostgreSQL e extrato oficial da SuitPay.
   - Bloqueado até liberação formal de credenciais de sandbox, IP de saída autorizado e FQDN/webhook HTTPS público.
3. **P0.6 — Operação e Corte Greenfield (PREPARAÇÃO LOCAL VERIFICADA / CORTE BLOQUEADO):**
   - Orquestrador `scripts/release/cutover_orchestrator.sh` e validador `scripts/release/validate_compose_config.py` implementados e cobertos por testes locais: evidências assinadas antes de Docker, imagens por digest, configuração final renderizada, bind de todas as portas publicadas, espera de saúde da API/frontends e limpeza dos serviços de aplicação se a fase 8 falhar.
   - Portão de pré-voo `scripts/release/preflight_cutover.py` aprovando fail-closed o estado zero absoluto no banco `catbrasil_prod`; leia a sequência e o inventário exigidos no próprio validador.
   - Scripts `scripts/release/backup.sh` e `scripts/release/restore-verify.sh` com cifragem obrigatória via `age` (sem persistência em plaintext em nenhum ambiente, inclusive staging).
   - Checksum SHA-256 gerado e validado antes da publicação do artefato de backup, com descarte de arquivos parciais/finais em caso de falha.
   - Restauração isolada com conferência de tabelas e sequência de migrações definidas em `restore-verify.sh`; a prova local descartável foi concluída, sem comprovar cópia externa.
   - Regras locais do Prometheus validadas com `promtool`; entrega a destinatários externos ainda requer configuração e teste.
   - Sem cópia e restauração fora do host, RPO/RTO medidos, imagens publicadas por digest nem prova operacional de identidade/SMS reais. Nenhum `--execute` foi feito.

---

## 3. Preparar staging

1. Use um projeto Compose distinto (`-p catbrasil-staging`), `.staging.env` exclusivo, segredos novos e armazenamento separado. Execute `sh docker/staging/prepare-network.sh` e depois:
   ```sh
   docker compose -p catbrasil-staging --env-file .staging.env -f docker-compose.yml -f compose.staging.yml --profile observability up -d --build
   ```
2. Execute `python3 scripts/release/validate_staging.py`. O gate sem mutação exige `backend-migrate` concluído com exit 0, serviços de execução ativos, healthchecks configurados aprovados e **exatamente** a sequência de migrações SQL da fonte no banco de staging. Um serviço antigo com `/ready=200` não passa se o schema estiver defasado. O override publica Caddy e as inboxes sandbox só em `127.0.0.1`; não abra essas portas na Internet. Registre o commit e os IDs das imagens construídas no ensaio; o gate não consegue provar sozinho que elas vieram daquele commit.
3. Configure o gateway PIX stub de staging com `python3 scripts/release/bootstrap-pix-staging.py --confirm-staging`. O script lê as credenciais do stub no Compose, exige `APP_ENV=staging`, `PIX_MODE=suitpay`, URL interna do stub e rótulo Docker `catbrasil-staging`, atualiza apenas a linha `system_config` desse banco e confere a leitura de volta. Não imprime segredos nem os coloca na linha de comando. Nunca use credenciais reais nesse ambiente.
4. Execute o smoke sem mutação da seção 7 e a carga limitada da seção 8 somente depois que o gate passar. A versão exigida é determinada dinamicamente pelo diretório de migrações; confirme também `GET /ready=200`.

---

## 4. Backup e Prova de Restauração (P0.6 — Fail-Closed com age)

> [!IMPORTANT]
> **Requisitos Mandatórios de Segurança:**
> 1. **Cifragem Obrigatória com `age`:** O dump é pipeado diretamente do `pg_dump` para o utilitário `age`, gerando arquivos com extensão obrigatória `.dump.age`. Não há suporte a plaintext em nenhum ambiente, nem mesmo staging.
> 2. **Tratamento de Dependência `age` como Bloqueio:** Se o comando `age` ou a chave pública do destinatário não estiverem presentes, o backup aborta com código de erro 2.
> 3. **Nunca Persistir Plaintext no Host:** O stream nunca toca o filesystem desprotegido, nem durante o dump nem durante a restauração.
> 4. **Checksum SHA-256 Validado Pré-Publicação:** O arquivo `.sha256` é computado e validado antes da publicação do artefato final. Qualquer falha remove arquivos parciais e finais.
> 5. **Alvo Explícito e Obrigatório:** O script exige `--project` e `--env-file` explícitos na invocação (sem defaults do shell ou `.prod.env`), valida labels Compose, conexões seguras com `POSTGRES_USER` e `POSTGRES_DB` explícitos em `psql`/`pg_isready`, `current_database()` e inspeciona exclusivamente o mount em `Destination=/var/lib/postgresql/data` exigindo `Type=volume Name=catbrasil_prod_pgdata` para produção.
> 6. **Limpeza Total via Trap:** Qualquer falha ou interrupção (mv, checksum, chmod, SIGINT/TERM) antes da conclusão total aciona o trap para remover arquivos parciais e também o artefato final e seu checksum, garantindo que nenhum backup órfão/corrompido permaneça.
> 7. **Validação de Schema:** A restauração descartável exige as tabelas e a sequência exata declaradas em `restore-verify.sh`; atualize esse contrato no mesmo commit sempre que adicionar migrações.

### Dependência de Cifragem: `age`

O `age` (Actually Good Encryption) é uma ferramenta gratuita, open-source e self-hosted recomendada para cifragem de backups:

```sh
# Instalação no host (se ainda não instalado)
# Debian / Ubuntu:
apt-get install -y age

# macOS:
brew install age

# Gerar par de chaves dedicado para backups (em local seguro ou host de gestão de chaves):
age-keygen -o /srv/keys/catbrasil-backup-identity.key
# A saída exibe a chave pública (recipient) no formato:
# Public key: age1...
# Guarde a chave privada com permissão estrita: chmod 400 /srv/keys/catbrasil-backup-identity.key
```

### Executar Backup de Produção

O comando confere o projeto `-p catbrasil-prod`, o arquivo de ambiente `--env-file .prod.env`, o volume `catbrasil_prod_pgdata` e o banco `catbrasil_prod`:

```sh
scripts/release/backup.sh \
  --project catbrasil-prod \
  --env-file .prod.env \
  --recipient "$AGE_RECIPIENT" \
  /srv/backups/catbrasil/catbrasil-$(date +%Y%m%d_%H%M%S).dump.age
```

*(Ou utilize `--recipient-file /srv/keys/catbrasil-backup.pub` caso a chave pública esteja salva em arquivo).*

Para staging, especifique `--project catbrasil-staging` e `--env-file .staging.env`:

```sh
scripts/release/backup.sh \
  --project catbrasil-staging \
  --env-file .staging.env \
  --recipient "$AGE_RECIPIENT" \
  /srv/backups/catbrasil/staging-$(date +%Y%m%d_%H%M%S).dump.age
```

### Executar Prova de Restauração em Contêiner Descartável

A prova cria um PostgreSQL 16 isolado (`--network none`, sem portas ou volumes persistentes), confere o SHA-256 obrigatório, decifra o backup via streaming diretamente para o `pg_restore`, valida tabelas e migrações exigidas pelo script e destrói o contêiner ao final:

```sh
scripts/release/restore-verify.sh \
  --identity /srv/keys/catbrasil-backup-identity.key \
  /srv/backups/catbrasil/catbrasil-20261017_120000.dump.age
```

### Evidência exigida para liberar o corte

Antes de `cutover_orchestrator.sh --execute`, prepare fora do repositório um manifesto JSON com `approved_by`, `approved_at` e os seis itens `p0_4_identity_sms`, `p0_5_sandbox_official`, `backup_restored_offhost`, `monitoring_alerts`, `images_immutable` e `release_checklist_signed`. Cada item deve trazer `status: "approved"`, `evidence_ref`, `verified_by` e `artifact` com caminho absoluto de um arquivo de evidência real e seu SHA-256. O revisor de identidade/SMS deve ser distinto do operador do corte. Assine os bytes exatos do manifesto com `ssh-keygen -Y sign` no namespace `catbrasil:release:evidence-manifest:v1`, mantenha `allowed_signers` fora do repositório e confira que a identidade de `CUTOVER_OPERATOR` é a autorizada nesse arquivo.

O orquestrador chama essa verificação **antes de qualquer comando Docker** em `--execute`. A assinatura registra quem aprovou o pacote; ela e o SHA-256 não provam que o conteúdo da evidência é verdadeiro. O responsável pelo release precisa revisar materialmente os artefatos: SMS entregue em canal real, ensaio oficial da SuitPay, backup `age` copiado e restaurado fora do host, entrega efetiva de alertas, digests publicados e checklist assinado. Sem essas provas, o manifesto não deve ser marcado como aprovado.

Publique as imagens da aplicação em um registry sob referências `repo@sha256:<64 hex>` e obtenha os digests reais de PostgreSQL, Caddy e Prometheus; preencha as variáveis em `.prod.env` a partir de `.prod.env.example`, incluindo `BOOTSTRAP_ADMIN_USERNAME` exclusivo. A imagem da API deve conter `/app/bootstrap-admin`. `--validate-only` confere o formato e a configuração renderizada, sem consultar o registry. No `--execute`, antes da primeira mutação de serviço, o orquestrador baixa cada imagem pinada e confere seu `RepoDigest` local. O operador também deve vincular cada digest ao commit aprovado e revisar o material de release. Sem digests reais, a validação deve falhar fechada.

Na janela de staging atualizada, passe `validate_staging.py`, execute smoke e carga limitados, backup `age`, cópia e restauração isolada fora do host; registre duração, hora do último backup íntegro, commit e resultados. O ensaio de callbacks e saques no sandbox oficial fica para quando o acesso existir. Na janela de produção, primeiro resolva os conflitos de portas e DNS/TLS, prove as dependências P0.4/P0.5, revise e assine o pacote, faça `--validate-only` e só então considere `--execute` com plantão e terminal interativo sem gravação de sessão. O preflight do banco `catbrasil_prod` deve aprovar estado operacional zero e o schema exigido pelo código antes de iniciar a API. Em seguida o orquestrador invoca `/app/bootstrap-admin` em serviço sem logging Docker: o operador fornece senha exclusiva e confirma o código TOTP no próprio terminal. Se o bootstrap falhar, nenhum serviço público inicia. Se ele concluir e uma fase posterior falhar, não repita o corte às cegas: o preflight passará a bloquear a base com administrador e exigirá revisão humana. Após subir os serviços, confirme saúde da API/frontends, TLS, smoke sem mutação e telemetria; acompanhe a primeira operação autorizada no novo ledger.

---

## 5. Parâmetros Operacionais: RPO e RTO (Alvos Provisórios)

> [!NOTE]
> Os parâmetros de RPO e RTO estabelecidos abaixo representam **metas de engenharia provisionais**. Nenhuma alegação de RPO zero ou estimativa arbitrária de recuperação em segundos é sustentável sem medição cronometrada sob carga real e estratégia de WAL contínuo. Os valores definitivos serão estabelecidos após aferição formal no ensaio de corte.

| Parâmetro | Meta Operacional Provisória | Estratégia e Procedimento de Mitigação |
|---|---|---|
| **RPO (Recovery Point Objective)** | **<= 1 hora** (operação regular)<br>Provisório na janela de corte | - Rotina periódica de backup lógico criptografado agendada no host.<br>- No corte greenfield: snapshot lógico obrigatório imediatamente antes da abertura de tráfego para minimizar a janela de perda.<br>- Transferência/cópia off-site imediata dos arquivos `.dump.age` e `.sha256`.<br>- *Nota:* RPO zero exigiria replicação contínua/streaming de WAL (não configurado na arquitetura lógica simples atual). |
| **RTO (Recovery Time Objective)** | **<= 15 minutos** (meta provisória) | - Inicialização do contêiner isolado de restauração descartável.<br>- Descriptografia e restauração via streaming direto do dump lógico.<br>- Validação de integridade relacional conforme `restore-verify.sh`.<br>- Subida dos contêineres de produção.<br>- *Nota:* O tempo total efetivo depende diretamente da CPU do host, I/O do armazenamento e volume do banco. |

### Medição Obrigatória de RTO/RPO

Durante a simulação de contingência e em cada ensaio pré-corte:
1. Registre o timestamp exato do início da rotina de contingência/restauração (`T0`).
2. Registre o timestamp em que o script `restore-verify.sh` retorna código 0 (`T1`).
3. Registre o timestamp em que a API atinge `GET /ready = 200` (`T2`).
4. Calcule `RTO = T2 - T0`. Registre os valores reais medidos na ata de corte para substituir as metas provisionais.

---

## 6. Plano de Reversão Auditável (Rollback Procedure)

Caso ocorra qualquer anomalia durante o corte (falha no portão de pré-voo `preflight_cutover.py`, erro 503 contínuo em `/ready`, divergência relacional ou falha no provedor de pagamentos):

### Gatilhos de Aborto Imediato:
- `preflight_cutover.py` retorna código 1 ou 2.
- `restore-verify.sh` falha na validação do schema exigido ou da integridade do backup.
- `GET /ready` falha por mais de 60 segundos após a inicialização.
- Falha na validação do sandbox oficial da SuitPay.

### Passos de Execução do Rollback:

1. **Preservação Prévia de Evidências e Logs (OBRIGATÓRIO ANTES DO DOWN):**
   Capture os logs completos dos contêineres e do orquestrador antes de qualquer parada ou destruição de contêineres, aplicando umask e permissões restritas antes da captura:
   ```sh
   umask 077
   TIMESTAMP=$(date +%Y%m%d_%H%M%S)
   REPORT_DIR="/srv/reports/cutover_aborted_${TIMESTAMP}"
   mkdir -p "$REPORT_DIR"
   chmod 700 "$REPORT_DIR"

   # Coleta logs de todos os serviços do stack greenfield sem interpolação defeituosa de glob
   docker compose -p catbrasil-prod --env-file .prod.env -f docker-compose.yml -f compose.prod.yml logs --no-color > "${REPORT_DIR}/compose.log" 2>&1
   ```

2. **Interrupção Segura dos Serviços de Produção:**
   Interrompa os serviços do stack greenfield preservando estritamente os volumes de dados:
   ```sh
   docker compose -p catbrasil-prod --env-file .prod.env -f docker-compose.yml -f compose.prod.yml down
   ```
   > [!CAUTION]
   > NUNCA execute `docker compose down -v`. A remoção de volumes destrói evidências periciais e a integridade do banco `catbrasil_prod_pgdata`.

3. **Custódia de Segredos e Chaves:**
   - Preserve o arquivo `.prod.env` e a chave `IDENTITY_SECRET` intactos. Sem o segredo correto, dados pessoais criptografados não poderão ser recuperados.
   - Guarde as chaves privadas age (`AGE_IDENTITY_FILE`) sob controle de acesso estrito (permissões 0400).

4. **Bifurcação Decisória de Retomada: Pré vs. Pós-Primeira Operação Financeira:**

   > [!WARNING]
   > A estratégia de contingência depende criticamente de terem ocorrido ou não transações financeiras reais no stack greenfield:

   - **Cenário A: Rollback Pré-Primeira Operação Financeira (Aborto antes de qualquer transação real):**
     Se o aborto ocorreu antes da primeira operação financeira (nenhum PIX real gerado/liquidado, nenhum saque, nenhuma aposta ou movimentação com dinheiro real):
     - O estado contábil pré-existente não foi alterado.
     - É seguro restabelecer o serviço que anteriormente ocupava as portas 80/443:
       ```sh
       # O comando exato DEVE ser checado e documentado pelo operador antes da janela de corte.
       # Exemplo com systemd (se aplicável ao ambiente verificado):
       # sudo systemctl start <servico-web-verificado-no-host>
       # Exemplo com contêiner independente pré-existente:
       # docker start <conteiner-legado-verificado>
       ```
     - Confirme a volta da disponibilidade com `curl -fsS https://<dominio-do-host>/health` ou comando de checagem equivalente.

   - **Cenário B: Incidente Pós-Primeira Operação Financeira (Incidente após qualquer transação real):**
     > [!CAUTION]
     > **PROIBIÇÃO ABSOLUTA DE REINICIAR O STACK DE BANCO FICTÍCIO / LEGADO ANTIGO.**
     > Após qualquer transação financeira real (depósito PIX creditado, saque, aposta ou movimentação de saldo), **NUNCA** reinicie o banco legado fictício. Fazer isso corromperia a custódia dos fundos dos clientes, sobrescreveria estados financeiros irreversíveis e geraria passivo legal grave.
     - **Congelar Tráfego Externo (Traffic Freeze):** Interrompa imediatamente o tráfego externo roteando o Caddy ou firewall para uma página estática de manutenção (`503 Service Unavailable`), cessando a recepção de novas requisições sem desligar a persistência.
     - **Isolamento e Preservação do Ledger:** Mantenha o volume de dados `catbrasil_prod_pgdata` intacto e isolado.
     - **Reconciliação no Novo Ledger:** Conduza a reconciliação e batimento pericial exclusivamente sobre o novo ledger contábil no PostgreSQL greenfield (`transactions`, `financial_events`, `deposit_gateway_events`, `cashin_deposit_intents`, `payout_submissions`). Qualquer retomada operacional será feita obrigatoriamente a partir do novo ledger consolidado e auditado, jamais a partir da base legada antiga.

5. **Assinatura e Registro de Auditoria:**
   Preencha o registro formal de aborto/contingência com:
   - Identificador do Operador / Tech Lead.
   - Horário exato de início e fim da janela de corte.
   - Commit Git testado.
   - Arquivo de logs preservado em `$REPORT_DIR/compose.log`.
   - Código de erro e saída do comando que disparou o aborto.
   - Registro se houve ou não qualquer transação financeira prévia e hash do ledger reconciliado.

---

## 7. Smoke de ponta a ponta

Primeiro verifique vida, prontidão, frontends, bloqueio de rotas anônimas, simulação desativada e rejeição de webhook sem assinatura:

**Pré-condição:** o `validate_staging.py` deve sair com código 0 na mesma stack, depois de `up -d --build`. Registre `git rev-parse HEAD` e os IDs das imagens da aplicação no relatório do ensaio; uma stack antiga pode responder `/ready` sem executar o código novo.

```sh
python3 scripts/release/smoke.py --base-url http://localhost:8088 \
  --client-url http://localhost:8088 --admin-url http://admin.localhost:8088 \
  --confirm-staging
```

Para cadastro, login, PIN, 2FA e extrato, configure `SMS_SANDBOX_INBOX_URL=http://127.0.0.1:18081/messages`, `SMS_SANDBOX_INSPECT_TOKEN` (em ambiente privado) e execute o mesmo comando com `--full-sandbox`. O script usa telefone brasileiro sintético aleatório, CPF válido apenas para teste e email `example.invalid`; lê o código somente na inbox local e não imprime CPF, SMS, senha, PIN ou JWT. Ele exige `identityStatus=pending` em `GET /api/v1/user/profile` e HTTP 403 `IDENTITY_NOT_VERIFIED` ao tentar gerar PIX. Se `STAGING_TEST_PHONE` estiver definido, use um número **de teste**, ainda sem conta no banco; cada execução registra nova conta. O resultado só vale quando o smoke termina com exit code 0.

Para incluir criação de cobrança no stub, configure `PIX_SANDBOX_INSPECT_URL=http://127.0.0.1:18082` e `PIX_SANDBOX_INSPECT_TOKEN`; após executar o bootstrap acima, defina `STAGING_GATEWAY_CONFIG_VERIFIED=1`.

Há duas formas de liberar **somente a identidade sintética de staging**:

1. **Revisão humana:** execute o smoke com `--full-sandbox --pix-sandbox --manual-review-timeout 600`. O script mostra apenas o `userId` e espera `identityStatus=verified`. Um usuário distinto com cargo `diretoria` e 2FA ativo entra por `POST /api/v1/admin/login` (usuário, senha, `pin`, `twoFactorCode`), confere `GET /api/v1/admin/identity/pending`, lê os dados via `POST /api/v1/admin/users/{userId}/identity/inspect` com `twoFactorCode` (acesso auditado), examina a evidência de teste e envia `PATCH /api/v1/admin/users/{userId}/identity` com `{"status":"verified","evidenceRef":"referência-de-evidência-com-5+caracteres","twoFactorCode":"TOTP"}` e seu JWT ERP. A rota recusa outro cargo, 2FA ausente, autorrevisão e estado inválido. Não envie CPF, documento nem código SMS no `evidenceRef`. Registre o revisor e a referência fora do repositório.
2. **Prova automatizada da rota, sem KYC independente:** defina `STAGING_IDENTITY_EVIDENCE_REF` com referência explícita a dados **sintéticos**, execute `--full-sandbox --pix-sandbox --bootstrap-erp`. O script confere projeto/env/URL do stub, cria **uma segunda conta sintética distinta** por SMS e cadastro normais, ativa seu 2FA, promove **somente essa revisora** temporariamente a `diretoria` no staging, autentica com PIN/TOTP, inspeciona e aprova a identidade da primeira conta pela API, configura o gateway e remove o cargo de diretoria ao final. O JWT ERP emitido antes da remoção precisa ser rejeitado. A própria conta alvo nunca revisa sua identidade. Isso testa controles técnicos; **não constitui revisão humana nem prova de identidade real**.

Após a revisão, o smoke exige `identityStatus=verified`, `pixCode` com marcador `STAGING-ONLY-` e a cobrança registrada no stub. **Callback confirmado, conciliação, saque manual e automático não são validados por esse stub**; registre evidência separada dos testes de integração e do provedor sandbox quando disponíveis.

---

## 8. Carga limitada e observabilidade

```sh
python3 scripts/release/load.py --base-url http://localhost:8088 \
  --confirm-staging --rps 2 --duration 30 --output /tmp/catbrasil-load.json
```

O teste só faz `GET /ready`, até 4 req/s por 90 segundos (abaixo do limitador global de 300/min), e falha se houver mais de 1% de respostas não 200 ou p95 acima de 1000 ms. Guarde o JSON, horário, commit, tamanho do banco e CPU/RAM durante a execução. Este teste prova disponibilidade sob carga leve; capacidade máxima de transações, latência de webhook e concorrência financeira dependem de um ensaio separado em staging e não devem ser inferidas do `/ready`.

No Prometheus local, confirme `up{job="catbrasil-api"}=1`, taxa de respostas 5xx, p95 de duração HTTP e crescimento de memória/conexões. Confira `docker compose ... logs --since=15m backend-api backend-cron postgres` sem tokens ou códigos. Faça ao menos um teste de API sem PostgreSQL: `/health` continua vivo, `/ready` vira 503, e após reconexão retorna 200. Refaça smoke e carga depois da recuperação. Monitore disco do volume PostgreSQL e espaço para backups, validade do TLS, fila/erros de SMS e PIX, e falhas de migração. Configure alertas externos ao host para indisponibilidade, erro 5xx, disco, backup atrasado e certificado.

As regras em `docker/prometheus/alerts.yml` são **locais** e incluem disponibilidade, erros HTTP/gateway/webhook e atraso/falha de cron. Elas usam as métricas existentes da API e aparecem no Prometheus quando o perfil `observability` é iniciado. Sem receptor externo configurado e testado, uma regra disparada permanece apenas no Prometheus e não avisa o plantão. Antes do corte, valide as regras no Prometheus de staging e prove entrega fora deste host; acrescente sinais de disco, atraso de backup e expiração TLS com seus respectivos exporters.

---

## 9. Checklist de corte

Registre evidência com data, commit, ambiente, responsável e resultado para cada item:

- [ ] Código e imagens imutáveis identificados; digests reais conferidos no registry e vinculados ao commit aprovado; Go tests, `go vet`, builds frontend e scan de dependências sem falhas.
- [ ] `.env` de produção exclusivo, segredos longos e distintos; portas PostgreSQL/API/Prometheus/inboxes fechadas para Internet; domínio e TLS válidos.
- [ ] Inventário e resolução de conflitos de portas 80/443, 8080, 3000 e 3002 no host atual; manter staging e outros projetos isolados.
- [ ] `APP_ENV=production`, `PIX_MODE=suitpay`, simulação e stubs ausentes; `SUITPAY_LIVE_ENABLED=false` literal no overlay atual. Qualquer mudança para `true` exige revisão separada após P0.4/P0.5 e aprovação do corte; credenciais reais de SMS e SuitPay fornecidas por canais seguros.
- [ ] Identidade civil verificada por processo real: CPF, nome e email do cadastro conferidos, evidência auditável e revisão por `diretoria` com 2FA; teste sintético de staging não substitui esta etapa.
- [ ] Cobrança real usa somente identidade `verified`; conta `pending` ou `rejected` recebe `IDENTITY_NOT_VERIFIED`; provedor oficial valida os dados do pagador.
- [ ] Todas as migrações da fonte aplicadas e `/ready=200` com banco real greenfield (`catbrasil_prod`); `IDENTITY_SECRET` preservado com controle de acesso; rollback de código documentado sem desfazer schema/dados.
- [ ] Dependências P0.4 (identidade civil e entrega SMS reais aprovadas por diretoria) e P0.5 (ensaio e batimento triplo aprovados no sandbox oficial da SuitPay) concluídas.
- [ ] Dependência P0.6 pronta: pacote com seis evidências reais externas ao repositório e SHA-256 conferido, assinado por operador autorizado e revisado materialmente antes de `--execute`; orquestrador greenfield validado, pré-voo no banco real aprovado em estado zero e schema corrente, backup lógico com `age` íntegro e restauração fora do host aprovada via `restore-verify.sh`; metas RPO/RTO medidas.
- [ ] Smoke completo em staging (SMS sandbox, identidade pendente e revisão explícita, auth, 2FA, PIN, extrato, PIX stub); fluxo real de callback, conciliação e saque validado no sandbox oficial do provedor.
- [ ] Timeouts ambíguos do cash-in (intenção pendente sem QR) conciliados por procedimento operacional; titularidade da chave de saque conferida antes de transferir.
- [ ] WS com JWT válido e expirado; evento financeiro só ao usuário correto; ERP role revogado bloqueado.
- [ ] Carga de leitura e ensaio de concorrência financeira aprovados, com limites de p95/erro definidos pela operação.
- [ ] Métricas e alertas ativos; plantão para primeiras 24 h; plano de comunicação e janela de corte definidos.
- [ ] Após corte, smoke sem mutação, `/ready`, frontends, métrica `up`, logs e primeira operação real autorizada acompanhados; plano de reversão pronto.

**Não ativar `SUITPAY_LIVE_ENABLED=true` nem autorizar o corte financeiro** enquanto a revisão real de identidade, credenciais reais, domínio/TLS e evidência do sandbox oficial permanecerem pendentes.

---

## 10. Evidência Histórica do Release v8 — staging local, 16/10/2026

> [!NOTE]
> Os registros abaixo representam a validação histórica do release na migração **8/8**, antes da implementação das migrações posteriores e dos scripts fail-closed P0.6. Eles são mantidos para fins de rastreabilidade de engenharia e **não comprovam o release atual**; use o gate de staging para conhecer o baseline da fonte.

Staging isolado com migrações **8/8**: API, cliente, ERP, PostgreSQL, cron, Caddy, sandboxes e Prometheus saudáveis. O smoke completo passou com cadastro de CPF sintético, identidade pendente bloqueando PIX, consulta de dados protegida por TOTP e registrada em auditoria, revisão explícita por uma segunda conta sintética da diretoria, remoção do cargo, emissão de cobrança no stub e registro da cobrança. O stub recusa CPF com dígitos inválidos e não armazena CPF, nome ou email. O teste automatizado de diretoria não equivale a KYC humano.

`go test ./...` passou com PostgreSQL 16 descartável, incluindo duplicidade concorrente de CPF, trilha de inspeção/revisão e callback de cash-in antes da associação do QR. `go vet ./...` e testes `-race` de autenticação, pagamento, identidade e gateway passaram. Cliente e ERP compilaram; ainda há avisos antigos de acessibilidade em outras telas.

Backup do staging v8 teve SHA-256 conferido e foi restaurado com `pg_restore --exit-on-error` em PostgreSQL isolado: schema 8, 10 usuários, 7 transações, 6 identidades e 13 eventos. O dump temporário foi apagado após a prova. Carga leve antes da migração 8: 60 chamadas a `/ready` em 30 s, 0% de erros, p95 17,47 ms. Ela não mede throughput financeiro nem comprova o gateway oficial.

---

## 11. Evidência Histórica antes da migração 7 — 16/10/2026

Os resultados abaixo pertencem ao release anterior (migração 6), antes do cadastro obrigatório de identidade e da revisão por diretoria. Eles **não validam** o fluxo da migração 7 nem o baseline corrente; execute novamente smoke, backup/restore, carga e revisão no staging atualizado.

O staging isolado foi executado neste host com PostgreSQL 16, migrações **6/6** e API, cliente, ERP, Caddy, sandboxes e Prometheus saudáveis. O smoke completo passou com SMS capturado no sandbox, cadastro, login, PIN, TOTP, login ERP com PIN e TOTP, configuração PIX pela API ERP, remoção do cargo admin e rejeição do JWT antigo, além da criação de cobrança R$ 1 no stub PIX. O stub registrou a cobrança; nenhuma liquidação foi simulada como pagamento real.

O backup posterior à cobrança teve SHA-256 conferido e foi restaurado com `pg_restore --exit-on-error` em PostgreSQL descartável sem rede; a restauração mostrou migração 6, **2 usuários e 1 transação**. Ao desligar apenas o PostgreSQL de staging, `/health` ficou em 200 e `/ready` em 503; após religá-lo, `/ready` voltou a 200. Um `down`/`up` do projeto sem remover volumes preservou o usuário de teste. O Prometheus retornou `up{job="catbrasil-api"}=1`.

O ensaio de leitura fez **60 requisições a `/ready` em 30 segundos**, com **0% de erros** e **p95 de 13,82 ms** neste host. `go test ./...` e `go test -race` nos pacotes de HTTP, configuração e pagamento passaram. Estes números medem apenas este ambiente local e esta carga de leitura; não representam capacidade financeira em produção.

---

## 12. Status Atual das Pendências de Produção

- **P0.4:** Identidade civil e entrega SMS em canais reais (pendente de esteira humana de validação e provedor SMS com entrega real).
- **P0.5:** Prova no sandbox oficial SuitPay (preparação técnica e validador local concluídos; ensaio oficial e autenticação de evidências bloqueados por credenciais, IP de saída autorizado e webhook público).
- **P0.6:** Portões de evidências assinadas, digests, bind de portas publicadas, sequência de saúde e limpeza da fase 8 implementados. Houve prova local de backup cifrado e restauração descartável. Corte real pendente: sem `--execute`, preflight no banco real `catbrasil_prod`, cópia e restauração fora do host, RPO/RTO medidos ou carga financeira atual comprovada. Reexecute a suíte no commit candidato; os números de testes históricos acima não se aplicam automaticamente.
- **Infraestrutura:** Portas 80/443 ocupadas pelo Caddy legado; 8080/3000/3002 também publicadas por serviços legados; digests reais das imagens e domínio/TLS públicos de produção não provisionados.
- **Operacional:** Gateway SMS real, custódia de chaves `IDENTITY_SECRET` e `age`, rotina off-site de backups, entrega externa de alertas, plantão e testes de carga transacional sob concorrência pendentes.
- `SUITPAY_LIVE_ENABLED` permanece estritamente `false`.
