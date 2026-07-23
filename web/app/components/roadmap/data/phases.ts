// ROADMAP_PHASES é dado declarativo volumoso — dividido em partes ≤450 linhas
// (phases-part1..6) para respeitar o limite de linhas. A ordem das fases é
// preservada pela concatenação abaixo. Fonte antiga: roadmap-data.ts.
import type { RoadmapPhase } from "./types";
import { ROADMAP_PHASES_PART1 } from "./phases-part1";
import { ROADMAP_PHASES_PART2 } from "./phases-part2";
import { ROADMAP_PHASES_PART3 } from "./phases-part3";
import { ROADMAP_PHASES_PART4 } from "./phases-part4";
import { ROADMAP_PHASES_PART5 } from "./phases-part5";
import { ROADMAP_PHASES_PART6 } from "./phases-part6";
import { ROADMAP_PHASES_PART7 } from "./phases-part7";
import { ROADMAP_PHASES_PART8 } from "./phases-part8";

export const ROADMAP_PHASES: RoadmapPhase[] = [
  ...ROADMAP_PHASES_PART1,
  ...ROADMAP_PHASES_PART2,
  ...ROADMAP_PHASES_PART3,
  ...ROADMAP_PHASES_PART4,
  ...ROADMAP_PHASES_PART5,
  ...ROADMAP_PHASES_PART6,
  ...ROADMAP_PHASES_PART7,
  ...ROADMAP_PHASES_PART8
];
