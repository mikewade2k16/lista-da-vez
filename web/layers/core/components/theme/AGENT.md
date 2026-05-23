# Theme Components

## Escopo

Componentes do editor de tema em `web/layers/core/components/theme/`.

## Estrutura

- `ThemeColorInput.vue` e o wrapper fino para entrada textual de valores CSS de cor.
- `ThemeColorPicker.vue` concentra o popover visual e delega a logica para `useThemeColorPicker`.
- Conversoes puras de cor ficam em `web/app/domain/utils/color.ts`.
- Estado e acoes do picker ficam em `web/app/composables/useThemeColorPicker.ts`.

## Regras locais

- Mantenha o input textual desacoplado dos controles visuais do picker.
- Ao adicionar suporte a novos formatos de cor, prefira expandir `domain/utils/color.ts`.
- Evite acesso direto ao DOM em utils de dominio; passe resolvers opcionais quando precisar ler CSS vars.

## Atualizacoes

- 2026-05-21: B1 extraiu o picker de `ThemeColorInput.vue` para `ThemeColorPicker.vue`.
