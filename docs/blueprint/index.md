# Gospa — Blueprint de Produto

> Deep dive: mercado, concorrentes, features, arquitetura e roadmap para o
> Gospa, um PSA moderno, open source, AI-native.
> Stack: **Go · React + TanStack · Zitadel · Restate.dev · Connect RPC +
> protobuf · shadcn/ui · Postgres · single binary**.

---

## Sumário executivo

O mercado de PSA (Professional Services Automation) está quebrado de um jeito raro: **nenhum dos grandes players é amado pelos seus usuários**. ConnectWise é lento e datado, Autotask é "dated UI" carregando baggage pós-aquisição da Kaseya, HaloPSA é poderoso mas exige um "Halo admin" dedicado, SuperOps é moderno mas ainda imaturo, e os open source existentes (ITFlow, Alga PSA) cobrem o básico mas carecem de polish, extensibilidade e AI real.

Existe uma janela clara para um PSA **open source, moderno, rápido como Linear, extensível como VS Code, e AI-native desde o primeiro commit** — e a stack escolhida ataca diretamente as dores estruturais dos concorrentes.

Esse documento apresenta:

1. Análise dos concorrentes (features, elogios, reclamações, gaps)
2. As 10 "regras" que devem guiar o produto para evitar os mesmos erros
3. Escopo de MVP que um MSP de 1–20 técnicos consegue usar em produção
4. Roadmap evolutivo até V2
5. Blueprint de arquitetura mapeado na stack
6. Estratégia open source + modelo de negócio

---

## 1. O mercado — panorama 2026

### Tamanho e crescimento

- Mercado global PSA em ~US$ 15,42 bi (2025), projeção de US$ 59,71 bi até 2030, **CAGR ~31%**.
- Segmento MSP é o motor: ticket-volume cresceu >40% ao ano enquanto headcount de técnicos ficou estagnado — pressão enorme por automação e AI.

### Segmentação dos players

| Categoria | Players | O que oferecem |
|---|---|---|
| **Legacy enterprise** | ConnectWise PSA, Autotask (Kaseya/Datto) | Feature-complete, integrações extensas, mas UX datada e pricing opaco |
| **Modern mid-market** | HaloPSA, SuperOps, Rocketlane | UI moderna, pricing transparente, ainda amadurecendo |
| **All-in-one (PSA+RMM)** | Atera, Syncro, Pulseway, N-able | Preço por técnico, endpoints ilimitados, PSA mais raso |
| **Chat-first / AI-native** | DeskDay, Thread | Tese nova: chat > formulários; AI-executando, não só sugerindo |
| **Open source** | ITFlow, Alga PSA, ERPNext, Odoo | Grátis, self-hosted, mas faltam polish, mobile e integrações |

### Mudanças estruturais ocorrendo agora

- **Era Email → Era Chat → Era AI**: a linha evolutiva do PSA. Em 2026 os MSPs estão cansados de "AI theater" (sugestões que não executam) e querem agentes que fecham tickets sozinhos.
- **Rage-quit silencioso**: MSPs não trocam de PSA de repente. Vão empilhando workarounds, planilhas sombra, canais mutados de notificação, até a migração virar prioridade. Esse é o momento ideal para oferecer algo melhor.
- **Consolidação vs. best-of-breed**: ecossistemas fechados (Kaseya) vs. stacks abertos (HaloPSA + Ninja + Hudu). Open source se encaixa melhor no modelo best-of-breed.

---

## 2. Análise profunda dos concorrentes

Organizado por: **o que fazem bem · o que elogiam · o que reclamam · o que pedem**.

### 2.1 ConnectWise PSA (Manage)

**O que fazem bem**
- Profundidade funcional: ticketing, procurement, CRM, projetos, billing, quoting com integração Ingram/Synnex. Nenhum outro cobre tanto chão.
- Ecossistema: centenas de integrações, mercado de consultores e add-ons (BrightGauge, TopLeft, etc.).
- ITIL-aligned, SLA tracking robusto.

**Elogios**
- "Tudo num lugar só" — tickets, tempo, projetos, billing.
- Integração com quase qualquer outra coisa do universo MSP.

**Reclamações (muito bem documentadas)**
- **Lentidão patológica**: cliques entre 6–40 segundos; o suporte considera "normal" até 7s. Impossível usar em tempo real com cliente no telefone.
- **Sem single-screen workflow**: criar ticket + registrar tempo + fechar exige várias telas/modais.
- **UI "anos 90"**: ícones cinza que parecem desabilitados, botões que não fazem nada, cart shopping button no home.
- **Busca só em resumos, não no conteúdo dos tickets**. Mata a ideia de knowledge base.
- **Client portal crippled**, o antigo ainda existe como botão desativado.
- **Produtos da suíte (Manage, Automate, Control, Sell) não se integram como deveriam** — parecem times diferentes.
- **Treinamento de semanas** para novos técnicos fazerem coisas básicas.
- **Pricing opaco**, contratos anuais, lock-in via switching cost.
- Suporte lento, técnicos que "respondem perguntas que ninguém fez" e fecham tickets.

**O que pedem**
- Velocidade (acima de tudo).
- Fluxos single-screen.
- Busca dentro do conteúdo dos tickets.
- Modo acessibilidade (fonte ajustável).
- Macros / dialog boxes ("continuous process improvement").
- Chat interno para colaborar em tickets.

---

### 2.2 Autotask PSA (Kaseya/Datto)

**O que fazem bem**
- Gestão de contratos madura, billing por uso, SLA tracking.
- Integração estreita com Datto RMM e IT Complete.
- 170+ integrações via open platform, SOAP + REST API.

**Elogios**
- ITIL-aligned, processos padronizados.
- Reporting decente (relativo aos pares).

**Reclamações**
- UI "dated", refresh de 2025 melhorou pouco.
- Baggage da Kaseya (ataque de supply chain de 2021 ainda surge no r/msp).
- Suporte piorou pós-aquisição da Datto.
- Pricing opaco, custo via bundle IT Complete.
- Vendedores agressivos ("caught in lies").
- Migração punitiva.

**O que pedem**
- UI moderna de verdade.
- Separar produto da reputação corporativa.
- Transparência de preço.

---

### 2.3 HaloPSA

**O que fazem bem**
- **Actions engine**: gatilhos configuráveis em qualquer record (ticket, ativo, contrato, projeto) que executam ações, scripts, APIs, updates, notificações. Mais flexível que automação rule-based do Autotask.
- UI browser-based rápida, drag & drop, corrige erros bem.
- Single-tenant — cada cliente tem instância e DB isolados.
- 4 produtos (PSA, ITSM, CRM, Service) no mesmo core engine.
- Pricing transparente e tiered (mais usuários = menos por seat).

**Elogios**
- Customização profunda.
- Responde a feedback rápido.
- Integração com muita coisa (Sage, Xero, Ninja, Datto).
- Support time excelente (quando casa com o time zone).

**Reclamações**
- **Curva de aprendizado íngreme**: MSPs com 20–50 técnicos geralmente designam "Halo admin" full-time. Virou título de cargo.
- Implementação em 3–6 semanas + US$ 8–12k de parceiro (Cliqsupport, Automation Theory).
- Minimum de 5 agents — fora do alcance de solo / 1–4 techs.
- Support "polarizing" — ótimo para quem é bem atendido, péssimo fora do time zone UK.
- Reviews em MSPGeek: MSPs sem Halo admin dedicado acham "inadequado", MSPs com acham o melhor PSA existente.
- Pessoas compram errado no trial (confundem PSA vs ITSM vs Service).

**O que pedem**
- Onboarding mais rápido.
- Self-service admin (remover a necessidade do "Halo admin").
- 24/7 follow-the-sun support.

---

### 2.4 SuperOps

**O que fazem bem**
- PSA + RMM num só lugar, cloud-native, construídos juntos desde o começo.
- UI moderna, AI-driven ticket routing (Monica AI).
- Pricing transparente (sem contrato).

**Elogios**
- "Limpo, rápido, como deveria ser".
- Monica AI ajuda em classificação e resposta.
- Feature velocity alta.

**Reclamações**
- Ecossistema de integrações ainda menor que legados.
- Customização de reports ainda inferior a HaloPSA/ConnectWise.
- Billing avançado ainda maturando.
- Linux RMM limitado.
- "Aposta num vendor novo".

**O que pedem**
- Paridade em reports e billing.
- Mais integrações nativas (QuickBooks workflows específicos).

---

### 2.5 Syncro

**O que fazem bem**
- **Pricing por técnico** (não por endpoint) — mudança de jogo para MSP break-fix/híbrido.
- Setup instantâneo, PSA + RMM + remote access num bundle.
- QuickBooks integration sólida.

**Elogios**
- "Faz tudo que eu preciso".
- Custo previsível.
- Sem contrato, sem minimum.

**Reclamações**
- PSA side raso: ticketing, time, invoice, portal — mas project management fraco, reports exigem export pro Excel.
- Não escala para MSPs grandes (50+ techs).

**O que pedem**
- Reports nativos decentes sem export.
- Project management real.

---

### 2.6 Atera

**O que fazem bem**
- Per-technician pricing, endpoints ilimitados.
- AI Copilot + IT Autopilot (tentativa de agentic AI real).
- Onboarding rápido.

**Elogios**
- Preço imbatível pra quem gerencia muitos endpoints.
- AI features evoluindo rápido.

**Reclamações**
- PSA "fica em muitas telas", cliques demais pra coisas simples.
- Customização limitada.
- Mobile app básico.
- Onboarding requer professional services.

---

### 2.7 DeskDay (chat-first)

**O que fazem bem**
- Tese **chat-first**: tickets nascem como conversas (Teams, mobile, web, email, desktop), não formulários.
- Helena AI como copiloto nativo, não add-on.
- Sentiment analysis no thread do ticket.
- Arquitetura microservices.

**Elogios**
- "Feels como WhatsApp, não como Jira."
- Integração nativa com Teams — cliente abre ticket sem sair do Teams.
- Time tracking captura atividade em todos os canais automaticamente.

**Reclamações / questões em aberto**
- Empresa muito jovem, tese não totalmente validada em MSPs grandes.
- Ecossistema de integrações pequeno.
- Aposta em vendor arriscada.

---

### 2.8 ITFlow (open source)

**O que fazem bem**
- GPLv3, 100% gratuito.
- Cobre o fundamental: tickets, invoicing, assets, docs, password manager, domain/SSL tracking, client portal.
- Instalação Ubuntu/Debian via script em minutos.
- AI suporta Ollama/LocalAI (self-hosted) ou ChatGPT.

**Elogios**
- "Massive upgrade sobre planilhas+duct tape pra MSPs solo/1–5 techs."
- Sem taxa por seat, ilimitado.

**Reclamações**
- **UI não vai ganhar prêmio** (palavras deles mesmos).
- **Sem app mobile nativo**.
- Não escala para MSPs médios/grandes.
- Sem integrações pré-built com accounting/PSA majors.
- Stack PHP+MySQL tradicional — difícil para devs modernos contribuírem.
- Comunidade pequena, PRs pausados.

---

### 2.9 Alga PSA (open source)

**O que fazem bem**
- Stack moderna: TypeScript, Next.js, PostgreSQL com row-level security, Redis event bus, event-sourced workflow engine.
- Community Edition (AGPL-3.0) + Enterprise Edition.
- Automation Hub com workflows TypeScript event-driven.
- Interval tracking automático (captura tempo de visualização de ticket no browser).
- Backed por Bellini Capital / Nine Minds.

**Elogios**
- Arquitetura séria: Docker Compose, Helm charts, Hocuspocus (realtime), Radix UI.
- International tax support (composite, threshold, holidays, reverse charge).

**Reclamações / riscos**
- Projeto novo (92 stars, 32 forks quando pesquisado).
- Next.js+Node para quem busca single-binary é pesado (exige Redis, DB, HocusPocus, PgBouncer, múltiplos serviços).
- Ainda faltam features maduras e ecossistema.

---

### 2.10 O que o r/msp está dizendo (síntese)

Sinais consistentes dos canais da comunidade:

- **"AI na maioria dos PSAs é autocomplete ligeiramente melhor"** — promessa entregou pouco.
- **Chat-first** é a nova expectativa mínima.
- **Multi-canal nativo**: email, Teams, Slack, mobile, web, SMS — tudo vira ticket com contexto.
- **Reports quase sempre exportados pro Excel** — ninguém confia no PSA para relatórios executivos.
- **Vendor lock-in dói** quando preço sobe 20% ou feature some.
- **Ticket triage consome 40% do dia** dos técnicos — é o problema a atacar com AI real.
- **67% dos tickets são repetitivos** — elegíveis para automação agentic.

---

## 3. O "ideal PSA" destilado

Cruzando os elogios universais, as reclamações recorrentes e os pedidos em aberto, o produto ideal é:

### Os 10 princípios

1. **Velocidade não-negociável**. Alvo: P95 < 150ms para qualquer interação. Toda UI tem optimistic update. Se um clique demora >300ms, é bug.
2. **Single-screen por tarefa principal**. Abrir ticket, registrar tempo, responder cliente e fechar — tudo numa tela. Cmd+K para navegação global. Zero modal hell.
3. **Chat-first ingestion**. Email, webchat, Teams, Slack, SMS, mobile app — todos viram threads de ticket com contexto preservado, sentiment tagging e auto-classification. Nenhum canal é segunda classe.
4. **AI que executa, não sugere**. Copilot com tools (MCP) que fecha ticket L1 completo, cria invoice, agenda follow-up — com human-in-loop para ações sensíveis. AI não é tela de sugestão.
5. **Customização sem "admin dedicado"**. Actions engine no estilo HaloPSA + workflows como código + UI visual para 80% dos casos. Devs escrevem TypeScript/Go; ops configuram sem código.
6. **Extensibilidade first-class**. Plugin system (extensions), MCP server nativo, CLI, SDKs gerados via Connect, webhooks com retry durável. Comunidade pode publicar tudo.
7. **Data portability por default**. Schema público, migrações versionadas, exports completos em formato aberto. Saída do produto não requer fornecedor.
8. **Pricing transparente e open core honesto**. AGPL para core + tier hosted comercial (modelo Plausible/Sentry/Posthog). Zero "annual renewal call" surpresa.
9. **Apple-like attention to detail**. Dark mode nativo, animações sutis, empty states trabalhados, tipografia pensada, acessibilidade WCAG AA. Single-binary porque fricção de setup mata adoção.
10. **Durabilidade embutida**. Nenhum job importante (invoice, SLA, escalação, integração) roda em cron frágil. Tudo Restate — retry automático, replay, observability, zero state management manual.

### Diferenciadores vs. concorrentes

| Dor do mercado | Concorrente sofrendo | Nosso diferencial |
|---|---|---|
| UI lenta | ConnectWise, Autotask | Go + SPA otimista + Connect RPC binário + P95 <150ms |
| Modal hell / muitos cliques | ConnectWise, Atera | Single-screen design, Cmd+K, keyboard-first |
| Workflow engine frágil | SuperOps, DeskDay | Restate.dev — workflows duráveis, replay-safe, testáveis |
| AI só sugere | todos menos DeskDay/Atera | Agents com MCP e execução real + guardrails |
| Customização precisa de admin dedicado | HaloPSA | Workflows visuais + TS/Go code + templates curados |
| Lock-in e preço opaco | ConnectWise, Autotask | AGPL + pricing público + export total |
| Open source sem polish | ITFlow | shadcn/ui + dark mode + mobile PWA desde V1 |
| Stack pesada pra self-host | Alga PSA | Single binary Go — só precisa de Postgres |
| Multi-tenancy frágil | todos | Zitadel nativo — SSO, SCIM, passkeys pros MSPs e seus clientes |

---

## 4. MVP — o mínimo para um MSP pequeno rodar em produção

Escopo de **6 meses com time pequeno (2–4 devs)**. Objetivo: um MSP de 1–20 técnicos gerenciando 10–100 clientes consegue operar totalmente.

### 4.1 Identidade e multi-tenancy (Zitadel)

- MSP tenant (a empresa que opera o PSA).
- Client tenants (cada empresa cliente do MSP) — isolados por row-level security no Postgres.
- Roles e ABAC: owner, admin, technician, dispatcher, finance, client-contact, client-admin.
- Auth: email/senha, OIDC (Google, Microsoft, Entra), passkeys, SSO via SAML/OIDC.
- Convite por link, onboarding guiado.
- Tudo delegado ao Zitadel; PSA consome via OIDC + introspection.

### 4.2 Gestão de clientes (Companies / Contacts / Sites)

- Empresa cliente + múltiplos sites + múltiplos contatos.
- Tags, custom fields, notas internas.
- Histórico unificado: tickets, invoices, atividade.
- Importação CSV.

### 4.3 Ticketing (o coração)

- CRUD completo com SLA, prioridade, categoria, tipo (incident, request, problem, change), status customizáveis.
- **Intake multi-canal**:
  - Email-to-ticket (IMAP polling + SMTP outbound).
  - Web form público por cliente com whitelabel.
  - API pública via Connect RPC.
- **Thread unificado**: respostas, notas internas, anexos, audit log.
- **Single-screen workflow**: um ticket tem tudo — timer, reply, time entry, status change, assign, close — sem abrir modal.
- Atalhos de teclado (`j/k` navegação, `a` assign, `c` close, `t` start timer, `r` reply).
- Busca full-text em corpo e metadados (Postgres tsvector + trigram).
- Saved views / filters compartilháveis.
- Mentions `@tech` gerando notificação.
- Mesclagem e split de tickets.

### 4.4 Time tracking

- Timer iniciado por ticket, pausável, com detecção de idle.
- Manual time entries retroativas.
- Marca billable/non-billable/internal.
- Approval workflow (opcional por cliente/contrato).
- Timesheet semanal + utilization report.

### 4.5 Contratos e billing

- Tipos de contrato: retainer (horas/mês), bloco de horas, T&M puro, flat fee recorrente, projeto fixo.
- Associação de contrato ao cliente + regras de rollover.
- Geração automática de invoice no fim do ciclo (via Restate workflow).
- Suporte a impostos configuráveis (simples inicialmente, internacional pós-MVP).
- Integração Stripe para pagamento online.
- PDF de invoice customizável (whitelabel MSP).
- Export para CSV e padrão contábil genérico.

### 4.6 Client portal

- Domínio whitelabel (ex: `suporte.cliente.com.br`).
- Submeter ticket, ver status, responder, anexar.
- Ver invoices, pagar com Stripe.
- Ver assets associados (básico na MVP).

### 4.7 Assets / CMDB básico

- Tipos: workstation, server, network device, software license, domain, SSL cert.
- Linkagem cliente ↔ asset ↔ ticket.
- Import via CSV; API para RMM empurrar dados.
- Alerta de expiração (domain/SSL) via workflow Restate.

### 4.8 Automação e workflows

- Triggers: ticket created, ticket updated, SLA breaching, invoice due, asset expiring, custom event.
- Actions: notify, create ticket, assign, change status, HTTP call, run script.
- UI visual (low-code) para 80% dos casos comuns.
- Workflows "de verdade" definidos em Go/TypeScript via Restate SDK para casos complexos.
- Templates curados para onboarding (ex: "ticket escalação L1→L2", "invoice fim do mês").

### 4.9 AI capabilities (MVP)

- **Ticket summarization** on-demand.
- **Response suggestion** com contexto (tickets similares, KB, cliente).
- **Time entry drafting** a partir de thread do ticket.
- **Classification**: categoria + prioridade + rota sugerida.
- Provider: OpenAI, Anthropic, ou local (Ollama) — configurável.
- Guardrails: log de toda interação AI + human-in-loop para ações de billing/external.

### 4.10 Reports (pré-built)

- SLA compliance por cliente e por técnico.
- Utilization (billable %) semanal/mensal.
- Revenue por cliente, por serviço, por contrato.
- Ticket volume + trend.
- Backlog e aging.
- Tudo exportável em CSV e via API.

### 4.11 Notificações

- Email (SMTP/Postmark/Mailgun).
- In-app realtime (WebSocket / Server-Sent Events via Connect streaming).
- Webhook outbound.
- (Pós-MVP: Slack/Teams/SMS.)

### 4.12 Observability & Ops

- Health endpoints.
- Structured logs (slog), trace propagation (OpenTelemetry).
- Migrations versionadas (goose ou atlas).
- Single binary: `./psa serve --config=config.toml` sobe tudo.
- Docker image oficial + Helm chart para k8s.
- Backup: `pg_dump` + files directory.

### Fora do MVP (explícito)

- RMM nativo (integração sim; nativo não).
- Quoting/procurement avançado.
- Mobile app nativo iOS/Android (PWA cobre MVP).
- Marketplace de plugins.
- Agentic AI completo.
- Multi-idioma (começar inglês + português).
- Integrações com Teams/Slack profundas (email forwarding cobre).
- Workflow de aprovação complexo multi-stage.

---

## 5. Evolução pós-MVP

### V1.1 (Mês 7–9) — Polimento e integrações críticas

- Integração QuickBooks + Xero.
- Integração nativa: TacticalRMM, NinjaOne (webhook ambos lados).
- Hudu como documentation source (ler).
- Email templates ricos com variables.
- Canned responses com atalhos.
- Projects & tasks (Gantt básico, Kanban).
- PWA mobile com push notifications.
- Client satisfaction (CSAT) após fechamento.

### V1.2 (Mês 10–12) — Chat-first e extensibilidade

- **Microsoft Teams app** (bot + tabs) para intake e triagem.
- **Slack app** equivalente.
- **Plugin / extension system** com sandbox (Go plugins ou WASM).
- Public plugin registry.
- MCP server nativo — qualquer LLM externo pode atuar como agente do PSA.
- Workflow builder visual completo com debugging.
- Knowledge base integrada com busca semântica (pgvector).

### V2 (Mês 13–18) — AI agentic e marketplace

- **L1 autonomous agent**: resolve account lockout, password reset, group membership, etc. end-to-end com auditoria.
- **Pattern detection**: detecta clusters de tickets similares, cria "problems" e propõe root cause.
- **Predictive SLA**: antecipa breaches e sugere reassign.
- **Smart time tracking**: detecta atividade e sugere time entries.
- **Marketplace de extensions e templates** (MSPs publicam/consomem).
- **Mobile apps nativos** (Swift + Kotlin; ou React Native / Expo).
- **White-label completo** para MSPs revender.

### V2+ (Mês 18–24) — Enterprise e ecossistema

- Compliance: SOC 2, ISO 27001 (para tier hosted).
- Multi-region deployment.
- Quoting/procurement com Ingram/Synnex feeds.
- Advanced billing: revenue recognition, subscription rampas, usage-based.
- Sales CRM básico integrado (pipeline, deals).
- Client portal completo com self-service (runbooks assistidos por AI).
- Hosted SaaS oficial (cloud gerenciado).

---

## 6. Blueprint de arquitetura

Mapeando a stack que você escolheu para as necessidades do produto.

```
┌──────────────────────────────────────────────────────────────┐
│                  Client layer (React SPA)                    │
│  TanStack Router · TanStack Query (connect-query)            │
│  shadcn/ui · Radix · Tailwind · dark mode · Cmd+K palette    │
│  Optimistic UI · Keyboard-first · WebSocket para realtime    │
└───────────────────────────┬──────────────────────────────────┘
                            │  Connect RPC (HTTP/2, protobuf)
                            │  SSE/WebSocket para streams
┌───────────────────────────▼──────────────────────────────────┐
│                     Go single binary                         │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  Connect RPC │  │   AuthN/Z    │  │  Domain svcs │       │
│  │   servers    │  │   (Zitadel   │  │  tickets,    │       │
│  │  (protobuf)  │  │   OIDC +     │  │  billing,    │       │
│  │              │  │   ABAC)      │  │  assets, …   │       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                 │                 │               │
│  ┌──────▼─────────────────▼─────────────────▼─────────┐    │
│  │            Postgres (pgx + sqlc/sqlboiler)         │    │
│  │  Row-level security · pgvector · full-text search  │    │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Restate.dev  (embedded via sidecar or external)    │  │
│  │  Durable workflows · retries · replay · idempotent  │  │
│  │  - invoice generation cycle                         │  │
│  │  - SLA tracking & escalation                        │  │
│  │  - email-to-ticket processing                       │  │
│  │  - integration webhooks (RMM, accounting, …)        │  │
│  │  - AI long-running tasks                            │  │
│  │  - asset expiration alerts                          │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  AI / MCP    │  │  Extensions  │  │   Webhooks   │     │
│  │  providers   │  │  (WASM /     │  │   outbound   │     │
│  │  OpenAI/     │  │   Go plugin) │  │  (durable    │     │
│  │  Anthropic/  │  │              │  │   via        │     │
│  │  Ollama      │  │              │  │   Restate)   │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└──────────────────────────────────────────────────────────────┘
                            │
              External: IMAP/SMTP, Stripe, RMMs,
              Teams/Slack, accounting, documentation
```

### Decisões arquiteturais chave

**Single binary real**
- Tudo compilado num binário Go: servidor HTTP, workers, migrations, CLI.
- Embedded static assets para a SPA (via `embed.FS`).
- Dependência externa obrigatória: **apenas Postgres**.
- Restate pode rodar como processo filho embedded OU externo (ambiente grande).
- Setup: `./psa serve` + `psa migrate up` e está no ar.

**Connect RPC + protobuf**
- Define todos os serviços em `.proto`.
- Gera Go server stubs + TypeScript client (`@connectrpc/connect-es`).
- TanStack Query via `@connectrpc/connect-query` — type-safe ponta a ponta, zero drift entre front e back.
- Streaming nativo para tickets realtime, timer, notificações.
- Public API oficial desde o dia 1 (mesma surface que o próprio front consome).
- SDKs automáticos em qualquer linguagem que buf suporta.

**Zitadel — por que**
- Open source, self-hostable ou cloud.
- Multi-tenant nativo (organizations) — cada cliente do MSP pode ter sua própria org com SSO próprio.
- Passkeys, MFA, OIDC, SAML, SCIM out of the box.
- Resolve o problema "cada cliente quer entrar com o Entra ID dele" que os PSAs atuais sofrem.

**Restate.dev — onde brilha**
- Qualquer operação que envolve tempo, retry ou múltiplos sistemas externos vira workflow.
- Substitui Sidekiq/Resque/cron + retry ad-hoc + event sourcing manual — tudo numa abstração.
- Exemplos concretos:
  - **Billing cycle**: fim do mês → agrega time entries → aplica contrato → gera invoice → envia email → registra no accounting externo. Cada passo retry-safe e replayable.
  - **SLA escalation**: timer por ticket que, se violar, escala para próximo tier. Sobrevive a restarts.
  - **Email-to-ticket**: IMAP poll → parse → deduplicate → enrich com AI → criar/anexar → notificar. Se falhar, replay.
  - **Integration outbound**: webhook com retry exponencial + dead-letter queue.
- Torna o sistema "self-healing" por construção — pain point frequente em PSAs legados.

**Postgres como única fonte de verdade**
- Multi-tenant via row-level security com `tenant_id` em toda tabela.
- `pgvector` para busca semântica de KB e tickets similares.
- `pg_trgm` + `tsvector` para full-text busca em tickets.
- Schemas versionados, migrations forward-only.
- Opcional: Citus / partitioning para MSPs grandes (pós-V2).

**Extensions / plugins**
- Fase 1: Go plugins (dylib) carregados em runtime — simples mas mesmo-processo.
- Fase 2: WASM runtime (Wazero) — sandboxed, cross-language.
- Toda extension tem manifesto (permissões, hooks, UI slots) + assinatura.
- Pontos de extensão: ticket lifecycle, billing rules, UI slots (sidebar, ticket panels), custom fields, report providers.

**Frontend**
- React 18/19 + Vite + TanStack Router (file-based routing, type-safe).
- TanStack Query + connect-query: cache, optimistic updates, invalidation granular.
- shadcn/ui como base, Tailwind, tema dark-first com light opcional.
- Virtualização em listas (TanStack Virtual) para listas longas de tickets.
- Command palette (Cmd+K) global.
- PWA com service worker para offline básico.

**AI stack**
- Abstração provider-agnostic: OpenAI, Anthropic, Azure OpenAI, Ollama, LocalAI, vLLM.
- MCP server exposto — qualquer LLM com tools pode operar no PSA.
- Embeddings guardados em pgvector.
- Prompt versioning + A/B + feature flags para rollouts.
- Cost tracking por ação AI (crítico para hosted tier).

---

## 7. Estratégia open source e modelo de negócio

### Licenciamento

- **Core**: AGPL-3.0 (previne fork comercial sem contribuição de volta).
- **Extensions oficiais enterprise**: BSL (Business Source License) com conversion para Apache após 4 anos.
- **SDKs, CLI, schemas**: Apache-2.0 ou MIT (amigável a integrações).
- **Templates e workflows da comunidade**: CC BY 4.0.

### Modelo comercial (opcional, no longo prazo)

Seguindo o playbook Sentry/Plausible/PostHog:

- **Self-hosted community**: grátis, AGPL, full features do core.
- **Self-hosted enterprise**: licença comercial, features avançadas (SSO enterprise, audit log avançado, multi-region, SLA).
- **Cloud SaaS**: gerenciado, pricing per-tech transparente. Zero "call for pricing".

### Governança

- RFC process público (estilo Rust/Django).
- Roadmap público (GitHub Projects).
- Decisões de arquitetura documentadas como ADRs.
- Contributor ladder transparente.
- CLA leve (DCO preferível).

### Ecossistema

- Documentation-first: SKILL da documentação investida desde o commit 1.
- CLI (`psactl`) para ops comuns.
- Homebrew, APT, Docker, Helm oficiais.
- Changelog curado semanalmente.
- Discord ou Zulip para comunidade.
- "Awesome list" de extensions, workflows e integrações.

---

## 8. Riscos e perguntas em aberto

| Risco | Mitigação |
|---|---|
| Alga PSA já existe e tem backing VC | Stack radicalmente diferente (Go single-binary vs Next.js heavyweight); foco em speed + extensibility; comunidade open core pode crescer em paralelo |
| Mercado educado a pagar $35–100/tech/mês — open source "parece barato demais" | Modelo híbrido: hosted tier paga features enterprise, não feature lock artificial |
| Integrações são o grande moat dos legados | MCP + plugin registry + CLI-first SDK baixa o custo de criar integração. Comunidade cobre os longtail. |
| Restate.dev é projeto relativamente novo — risco de desaparecer | Wrapper de abstração (interface `Workflow`) permite trocar por Temporal/River/custom se necessário. Mas Restate cresce e é a escolha certa hoje. |
| AGPL assusta MSPs conservadores | FAQ claro sobre "usar é OK, só precisa compartilhar modificações do core". Grandes MSPs provavelmente vão para tier enterprise mesmo. |
| Zitadel adiciona complexidade operacional | Disponibilizar Zitadel embedded num Docker Compose canônico; opção "managed" na cloud. |
| AI é corrida e pode virar commodity | Diferencial não é o modelo, é o **access pattern** (MCP + workflows duráveis + guardrails + dados integrados). Isso é moat real. |

### Perguntas que precisam ser decididas

1. **Nome e brand** — algo curto, pronunciável em pt/en, domínio disponível.
2. **Foco vertical inicial**: MSP "genérico" ou nicho (ex: MSPs brasileiros, MSPs de cibersegurança, internal IT teams)? Nicho acelera adoção.
3. **Quoting/procurement in-house ou via integração?** (Ingram feeds são caros e complexos.)
4. **RMM próprio no roadmap ou só integração para sempre?** (Tactical/Ninja cobrem o mercado.)
5. **Billing multi-currency / internacional já no MVP ou depois?**
6. **Fundação (CNCF-like) no longo prazo, ou empresa com foundation depois?**
7. **Como lidar com contribuições "enterprisey" que divergem do foco small/mid MSP?**

---

## 9. Próximos passos imediatos

### Fase 0 — Setup do projeto (concluída)

Gospa já existe como repo independente em `github.com/Gabrielbdd/gospa`,
gerado a partir do starter do Gofra (`gofra new --module github.com/Gabrielbdd/gospa`).
Depende do framework como módulo Go normal (`require github.com/Gabrielbdd/gofra`).
O bootstrap entregou: HTTP server via `runtime/serve`, health checks via
`runtime/health`, proto-driven config, Postgres via Compose, Dockerfile
multi-stage, e GitHub Actions CI rodando testes + build da imagem.

**Semana 1–2**
- Licença e VISION.md com os 10 princípios.
- Setup Vite + buf.build para protos dentro de `web/` e `proto/`.

**Semana 3–4**
- Esquema básico: tenants, users, companies, tickets, time_entries.
- Integração Zitadel (OIDC login funcionando).
- Primeira tela: lista de tickets + criar ticket.

**Mês 2**
- Thread de ticket com replies e time tracking.
- Email-to-ticket via Restate workflow.
- Dark mode + Cmd+K.

**Mês 3**
- Billing básico + invoice PDF.
- Client portal MVP.
- Primeiro release público (v0.1) para feedback.

**Mês 4–6**
- Reports, AI básico, automações visuais, Stripe, polish.
- Public launch (v1.0): Show HN, r/msp, MSPGeek, Discord.

### Critérios de sucesso do MVP

- Um MSP de 5 técnicos consegue migrar de ITFlow/ConnectWise Manage em < 1 semana.
- P95 de qualquer interação < 200ms em hardware modesto (4 vCPU, 8GB).
- Setup self-hosted em < 15 minutos (Docker Compose canonical).
- 100% das funcionalidades acessíveis via API pública.
- ≥ 3 plugins externos funcionando antes do v1.0.
- ≥ 50 GitHub stars orgânicos no primeiro mês do lançamento público.

---

## Apêndice A — Mapa de integrações prioritárias

| Categoria | Prioridade | Exemplos | Estratégia |
|---|---|---|---|
| Email (inbound/outbound) | P0 MVP | IMAP, SMTP, Postmark, Mailgun, SES | Nativo core |
| Payments | P0 MVP | Stripe | Nativo core |
| Accounting | P1 V1.1 | QuickBooks Online, Xero | Plugin oficial |
| RMM | P1 V1.1 | TacticalRMM, NinjaOne, Datto RMM, N-central | Plugin oficial + webhook |
| Documentation | P2 V1.2 | Hudu, IT Glue | Plugin oficial (leitura) |
| Chat ingestion | P2 V1.2 | Microsoft Teams, Slack | App oficial (bot) |
| Identity | P0 MVP | Zitadel, depois SAML genérico | Core |
| Remote support | P3 V2 | ConnectWise Control, Splashtop, TeamViewer | Plugin link-out |
| Quoting | P3 V2 | Ingram, Synnex feeds | Enterprise / plugin |
| Backup monitoring | P3 V2 | BackupRadar, veeam | Plugin via webhook |
| AI providers | P0 MVP | OpenAI, Anthropic, Ollama | Core (provider-agnostic) |

## Apêndice B — Recursos de referência

- **Concorrentes open source para estudar**: [ITFlow](https://github.com/itflow-org/itflow), [Alga PSA](https://github.com/Nine-Minds/alga-psa), [ERPNext](https://github.com/frappe/erpnext).
- **Stack technical references**: [Connect RPC](https://connectrpc.com), [Restate](https://restate.dev), [Zitadel](https://zitadel.com), [TanStack](https://tanstack.com), [shadcn/ui](https://ui.shadcn.com).
- **Comunidade MSP para validação**: r/msp, MSPGeek, Discord da OpenMSP, fóruns HaloPSA/Syncro.
- **Padrões ITIL relevantes**: Incident, Request, Problem, Change management (ITILv4).
- **Leituras obrigatórias antes de começar**:
  - "Why MSPs are quietly rage-quitting their PSA platforms" (DeskDay blog) — mapa de dores atuais.
  - r/msp reviews comparativos de ConnectWise vs HaloPSA — vocabulário real dos usuários.
  - Alga PSA docs — para entender concorrência open source direta.
  - Restate.dev patterns — para modelar workflows corretamente desde o início.

---

**Versão**: 1.2 — Blueprint rebranded para Gospa; seção de setup reflete o estado real pós-scaffold a partir do framework publicado.
**Data**: 2026-04-18
