export type PhaseStatus = "pending" | "in_progress" | "done" | "blocked";

export interface RoadmapTask {
  id: string;
  label: string;
  done: boolean;
  note?: string;
}

export interface RoadmapPhase {
  id: string;
  code: string;
  title: string;
  goal: string;
  status: PhaseStatus;
  estimateWeeks: string;
  startedAt?: string;
  finishedAt?: string;
  tasks: RoadmapTask[];
  verifiable: string;
  blockers?: string[];
  group?: string;
}

export interface RoadmapGroup {
  id: string;
  label: string;
  description?: string;
}

export type ModuleStatus = "pending" | "in_progress" | "beta" | "done";
export type ModulePriority = "P0" | "P1" | "P2" | "P3";

export interface RoadmapModule {
  id: string;
  label: string;
  route: string;
  status: ModuleStatus;
  priority: ModulePriority;
  description: string;
  scope?: string[];
  dependsOn?: string[];
  category?: "atendimento" | "tools" | "operacao-comercial" | "indicadores" | "manage";
}

export type RuleCategory =
  | "frontend"
  | "backend"
  | "banco"
  | "linguagens"
  | "deploy"
  | "padroes-gerais";

export interface RoadmapRule {
  id: string;
  category: RuleCategory;
  title: string;
  body: string;
  why?: string;
  appliesWhen?: string;
}
