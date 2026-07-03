// Barril do roadmap (dado declarativo). O conteúdo real vive em ./data/*:
// types, groups, phases (phases-part1..6), modules, rules e labels. Este arquivo
// só reexporta a API pública — os 10 consumidores continuam importando de
// `~/components/roadmap/roadmap-data` sem alteração de path.
//
// Divisão feita em 2026-07-02 (AC-07): cada arquivo de dados fica ≤450 linhas.
export * from "./data/types";
export * from "./data/groups";
export * from "./data/phases";
export * from "./data/modules";
export * from "./data/rules";
export * from "./data/labels";
