# Configurações portáveis dos agentes

Importação de `fila-atendimento.zip` em 2026-11-01:

- `principios-engenharia` e sete referências, preservadas integralmente;
- `revisao-dia`, preservada integralmente;
- `.agents/` estava vazio; o ZIP não continha `.codex/`, `AGENTS.md` ou `CLAUDE.md` na raiz;
- `.claude/settings.json` e `settings.local.json` continham permissões com
  credenciais, tokens e caminhos do Windows. Não foram importados. O arquivo
  compartilhado foi reconstruído com comandos de leitura e validação.
- caches, dumps, logs, sessões e dados temporários não foram importados.

`AGENTS.md` é a instrução compartilhada. `CLAUDE.md` importa esse documento.
Claude descobre `.claude/skills/`; Codex usa `.agents/skills/`, conforme a
[documentação oficial](https://developers.openai.com/blog/skills-agents-sdk).
As duas árvores são arquivos reais para funcionar também em clones Windows.
Não é necessário copiar configurações globais pessoais para `.codex/`.

Depois de clonar, abrir o agente na raiz do repositório. As skills já acompanham
o clone. Para manutenção:

```bash
# Editar a fonte em .claude/skills/, copiar para o Codex e verificar igualdade.
npm run agents:sync
npm run agents:check
```

Incluir as duas árvores e configurações compartilhadas nos commits do projeto.
Não usar `git add -f` para incluir arquivos pessoais ignorados. Autenticação dos
agentes, conexões MCP privadas, `.env` e chaves SSH são configuradas em cada máquina.

As permissões antigas não devem ser reimportadas: continham segredos reais.
Caso tenham sido compartilhadas fora de um local confiável, revogar/rotacionar
as credenciais correspondentes no serviço de origem.
