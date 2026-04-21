// Mock client for the tickets list. Shaped exactly like the future
// TicketsService.ListTickets RPC so the swap is a one-line change in
// listTickets() — replace the in-memory filter with a Connect call.

export type TicketPriority = "P1" | "P2" | "P3" | "P4";

export type TicketView = "all" | "unassigned" | "mine" | "risk" | "closed";

export type Ticket = {
  id: string;
  priority: TicketPriority;
  // sla_seconds: positive = remaining, negative = overdue, null = paused.
  // Closed tickets always carry null and use closed_days_ago instead.
  slaSeconds: number | null;
  status: string;
  statusTone: string;
  subject: string;
  clientName: string;
  requesterName: string;
  assigneeHandle: string | null;
  updatedMinutesAgo: number;
  isNew?: boolean;
  closed?: boolean;
  closedDaysAgo?: number;
};

export type ListTicketsRequest = {
  view: TicketView;
  companyName?: string;
  query?: string;
  pageToken?: string;
  pageSize?: number;
};

export type ListTicketsResponse = {
  tickets: Ticket[];
  nextPageToken: string;
  totalCount: number;
};

export type TicketCounts = Record<TicketView, number>;

export type CreateTicketRequest = {
  subject: string;
  priority: TicketPriority;
  clientName: string;
};

export type CreateTicketResponse = {
  ticket: Ticket;
};

const TEAM_HANDLES = ["maria", "joao", "carla", "you"] as const;
export type TeamMember = {
  handle: (typeof TEAM_HANDLES)[number] | string;
  name: string;
  tone: string;
};

export const TEAM: TeamMember[] = [
  { handle: "maria", name: "Maria Silva", tone: "#3291ff" },
  { handle: "joao", name: "João Pereira", tone: "#a1a1a1" },
  { handle: "carla", name: "Carla Souza", tone: "#454545" },
  { handle: "you", name: "Você", tone: "#0cce6b" },
];

const SEED: Ticket[] = [
  { id: "TKT-2041", priority: "P1", slaSeconds: -23 * 60, status: "Em andamento", statusTone: "#3291ff", subject: "Exchange — fila de relay crescendo, impacto amplo", clientName: "Northside Dental", requesterName: "Deepa Maddox", assigneeHandle: "you", updatedMinutesAgo: 2, isNew: true },
  { id: "TKT-2040", priority: "P1", slaSeconds: 47 * 60, status: "Novo", statusTone: "#0070f3", subject: "Servidor de arquivos inacessível — filial Campinas", clientName: "Acme Mfg", requesterName: "Pedro Nogueira", assigneeHandle: null, updatedMinutesAgo: 6, isNew: true },
  { id: "TKT-2039", priority: "P2", slaSeconds: 92 * 60, status: "Em andamento", statusTone: "#3291ff", subject: "VPN desconecta ao suspender — 3 usuários", clientName: "Acme Mfg", requesterName: "Sofia Park", assigneeHandle: "maria", updatedMinutesAgo: 14 },
  { id: "TKT-2038", priority: "P2", slaSeconds: null, status: "Aguardando", statusTone: "#f5a623", subject: "Job de backup Synology falhando há 2 dias", clientName: "Viridis Labs", requesterName: "Helena Ito", assigneeHandle: "joao", updatedMinutesAgo: 38 },
  { id: "TKT-2037", priority: "P2", slaSeconds: 3 * 3600, status: "Em andamento", statusTone: "#3291ff", subject: "Firewall rule request — GitHub runners → prod", clientName: "Acme Mfg", requesterName: "Rahul Verma", assigneeHandle: "you", updatedMinutesAgo: 48 },
  { id: "TKT-2036", priority: "P3", slaSeconds: 5 * 3600, status: "Novo", statusTone: "#0070f3", subject: "Onboarding — K. Chen, notebook + licenças", clientName: "Halden Legal", requesterName: "Marcos Halden", assigneeHandle: null, updatedMinutesAgo: 62 },
  { id: "TKT-2035", priority: "P3", slaSeconds: 11 * 3600, status: "Em andamento", statusTone: "#3291ff", subject: "Impressora do comercial não aceita driver via GPO", clientName: "Kronos Holdings", requesterName: "Ana Kronos", assigneeHandle: "carla", updatedMinutesAgo: 95 },
  { id: "TKT-2034", priority: "P3", slaSeconds: null, status: "Aguardando", statusTone: "#f5a623", subject: "Solicitação — reset de MFA para funcionária afastada", clientName: "Stellar Group", requesterName: "João Stellar", assigneeHandle: "maria", updatedMinutesAgo: 130 },
  { id: "TKT-2033", priority: "P3", slaSeconds: 18 * 3600, status: "Em andamento", statusTone: "#3291ff", subject: "Reconciliação de licenças Office 365 do último trimestre", clientName: "Stellar Group", requesterName: "Miguel Hale", assigneeHandle: "joao", updatedMinutesAgo: 180 },
  { id: "TKT-2032", priority: "P4", slaSeconds: 26 * 3600, status: "Novo", statusTone: "#0070f3", subject: "Dúvida — como exportar relatório mensal em CSV", clientName: "Marlow Architects", requesterName: "Beatriz Marlow", assigneeHandle: null, updatedMinutesAgo: 240 },
  { id: "TKT-2031", priority: "P4", slaSeconds: 2 * 24 * 3600, status: "Em andamento", statusTone: "#3291ff", subject: "Atualização de firmware nos switches sala-02 (agendar janela)", clientName: "Baltic Freight", requesterName: "Igor Baltic", assigneeHandle: "carla", updatedMinutesAgo: 360 },
  { id: "TKT-2030", priority: "P2", slaSeconds: 4200, status: "Em revisão", statusTone: "#3291ff", subject: "Slack → Gospa webhook quebrado após reinstall", clientName: "Northside Dental", requesterName: "Deepa Maddox", assigneeHandle: "you", updatedMinutesAgo: 15 },

  { id: "TKT-2029", priority: "P2", slaSeconds: null, status: "Resolvido", statusTone: "#0cce6b", subject: "VPN lentidão intermitente — site primário", clientName: "Acme Mfg", requesterName: "Sofia Park", assigneeHandle: "maria", updatedMinutesAgo: 60 * 24 * 2, closedDaysAgo: 2, closed: true },
  { id: "TKT-2028", priority: "P3", slaSeconds: null, status: "Fechado", statusTone: "#8f8f8f", subject: "Substituição de monitor defeituoso — sala 04", clientName: "Halden Legal", requesterName: "Marcos Halden", assigneeHandle: "you", updatedMinutesAgo: 60 * 24 * 3, closedDaysAgo: 3, closed: true },
  { id: "TKT-2027", priority: "P1", slaSeconds: null, status: "Resolvido", statusTone: "#0cce6b", subject: "Exchange: queue backlog após patch de segurança", clientName: "Northside Dental", requesterName: "Deepa Maddox", assigneeHandle: "you", updatedMinutesAgo: 60 * 24 * 4, closedDaysAgo: 4, closed: true },
  { id: "TKT-2026", priority: "P3", slaSeconds: null, status: "Fechado", statusTone: "#8f8f8f", subject: "Onboarding — J. Pires, setup Office 365", clientName: "Acme Mfg", requesterName: "Pedro Nogueira", assigneeHandle: "joao", updatedMinutesAgo: 60 * 24 * 7, closedDaysAgo: 7, closed: true },
  { id: "TKT-2025", priority: "P2", slaSeconds: null, status: "Resolvido", statusTone: "#0cce6b", subject: "Backup Synology — rotação de credenciais SMB", clientName: "Viridis Labs", requesterName: "Helena Ito", assigneeHandle: "joao", updatedMinutesAgo: 60 * 24 * 9, closedDaysAgo: 9, closed: true },
  { id: "TKT-2024", priority: "P3", slaSeconds: null, status: "Fechado", statusTone: "#8f8f8f", subject: "Reset MFA em massa — time financeiro", clientName: "Stellar Group", requesterName: "Miguel Hale", assigneeHandle: "carla", updatedMinutesAgo: 60 * 24 * 12, closedDaysAgo: 12, closed: true },
  { id: "TKT-2023", priority: "P4", slaSeconds: null, status: "Fechado", statusTone: "#8f8f8f", subject: "Dúvida sobre relatório de custos — respondida", clientName: "Marlow Architects", requesterName: "Beatriz Marlow", assigneeHandle: "maria", updatedMinutesAgo: 60 * 24 * 15, closedDaysAgo: 15, closed: true },
  { id: "TKT-2022", priority: "P2", slaSeconds: null, status: "Resolvido", statusTone: "#0cce6b", subject: "Firewall rule — ambiente de staging → banco", clientName: "Acme Mfg", requesterName: "Rahul Verma", assigneeHandle: "you", updatedMinutesAgo: 60 * 24 * 18, closedDaysAgo: 18, closed: true },
  { id: "TKT-2021", priority: "P3", slaSeconds: null, status: "Fechado", statusTone: "#8f8f8f", subject: "Impressora 3º andar — driver atualizado via GPO", clientName: "Kronos Holdings", requesterName: "Ana Kronos", assigneeHandle: "carla", updatedMinutesAgo: 60 * 24 * 22, closedDaysAgo: 22, closed: true },
  { id: "TKT-2020", priority: "P3", slaSeconds: null, status: "Resolvido", statusTone: "#0cce6b", subject: "Slack webhook — reinstalação e validação", clientName: "Northside Dental", requesterName: "Deepa Maddox", assigneeHandle: "you", updatedMinutesAgo: 60 * 24 * 27, closedDaysAgo: 27, closed: true },
];

let store: Ticket[] = [...SEED];

function isAtRisk(t: Ticket): boolean {
  return t.slaSeconds !== null && !t.closed && t.slaSeconds < 2 * 3600;
}

function priorityRank(p: TicketPriority): number {
  return parseInt(p.slice(1), 10);
}

function applyFilters(req: ListTicketsRequest): Ticket[] {
  let list = store.slice();
  if (req.view === "closed") {
    list = list.filter((t) => t.closed);
  } else {
    list = list.filter((t) => !t.closed);
    if (req.view === "unassigned") list = list.filter((t) => !t.assigneeHandle);
    else if (req.view === "mine") list = list.filter((t) => t.assigneeHandle === "you");
    else if (req.view === "risk") list = list.filter(isAtRisk);
  }
  if (req.companyName) list = list.filter((t) => t.clientName === req.companyName);
  const needle = req.query?.trim().toLowerCase();
  if (needle) {
    list = list.filter(
      (t) =>
        t.id.toLowerCase().includes(needle) ||
        t.subject.toLowerCase().includes(needle) ||
        t.requesterName.toLowerCase().includes(needle) ||
        t.clientName.toLowerCase().includes(needle),
    );
  }
  if (req.view === "closed") {
    list.sort((a, b) => (a.closedDaysAgo ?? 0) - (b.closedDaysAgo ?? 0));
  } else {
    list.sort((a, b) => {
      const ar = isAtRisk(a) ? 0 : 1;
      const br = isAtRisk(b) ? 0 : 1;
      if (ar !== br) return ar - br;
      if (ar === 0) {
        const aSla = a.slaSeconds ?? Number.POSITIVE_INFINITY;
        const bSla = b.slaSeconds ?? Number.POSITIVE_INFINITY;
        if (aSla !== bSla) return aSla - bSla;
      }
      const pa = priorityRank(a.priority);
      const pb = priorityRank(b.priority);
      if (pa !== pb) return pa - pb;
      return a.updatedMinutesAgo - b.updatedMinutesAgo;
    });
  }
  return list;
}

export async function listTickets(
  req: ListTicketsRequest,
): Promise<ListTicketsResponse> {
  await new Promise((r) => setTimeout(r, 120));
  const all = applyFilters(req);
  return { tickets: all, nextPageToken: "", totalCount: all.length };
}

export async function getTicketCounts(): Promise<TicketCounts> {
  await new Promise((r) => setTimeout(r, 120));
  const open = store.filter((t) => !t.closed);
  return {
    all: open.length,
    unassigned: open.filter((t) => !t.assigneeHandle).length,
    mine: open.filter((t) => t.assigneeHandle === "you").length,
    risk: open.filter(isAtRisk).length,
    closed: store.filter((t) => t.closed).length,
  };
}

export async function listClientNames(): Promise<
  { name: string; ticketCount: number }[]
> {
  await new Promise((r) => setTimeout(r, 50));
  const counts: Record<string, number> = {};
  store.forEach((t) => {
    counts[t.clientName] = (counts[t.clientName] ?? 0) + 1;
  });
  return Object.entries(counts)
    .map(([name, ticketCount]) => ({ name, ticketCount }))
    .sort((a, b) => b.ticketCount - a.ticketCount);
}

export async function createTicket(
  req: CreateTicketRequest,
): Promise<CreateTicketResponse> {
  await new Promise((r) => setTimeout(r, 150));
  const nextId = `TKT-${2042 + (store.length - SEED.length)}`;
  const slaSeconds =
    req.priority === "P1" ? 4 * 3600 : req.priority === "P2" ? 8 * 3600 : 24 * 3600;
  const ticket: Ticket = {
    id: nextId,
    priority: req.priority,
    slaSeconds,
    status: "Novo",
    statusTone: "#0070f3",
    subject: req.subject,
    clientName: req.clientName,
    requesterName: "Novo contato",
    assigneeHandle: null,
    updatedMinutesAgo: 0,
    isNew: true,
  };
  store = [ticket, ...store];
  return { ticket };
}
