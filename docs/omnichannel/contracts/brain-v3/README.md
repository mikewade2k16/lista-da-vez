# Contrato `brain.v3` — fechamento e handoff do MVP

Status: autorado e validado localmente. Compatibilidade `brain.v2` preservada por versão
publicada do agente. O rollout ocorre somente quando `workflowContractVersion=brain.v3`.

## Autoridade

O n8n/modelo devolve uma proposta. Somente o Go pode:

- aceitar ou rejeitar `decision=close`;
- criar `messaging.handoffs` e selecionar a fila por policy determinística;
- mudar `messaging.conversations.state`;
- criar mensagem/outbox e enviar ao provider.

O workflow não possui node Evolution, WAHA, Meta ou PostgreSQL e permanece stateless, com
`saveDataSuccessExecution=none`, `saveDataErrorExecution=none`, `staticData=null` e `pinData={}`.

## Request

`brain.request.v3` mantém todos os campos do v2 e acrescenta, quando disponível:

```json
{
  "schemaVersion": "brain.request.v3",
  "client": { "id": "uuid-do-cliente" },
  "businessContext": {
    "clientId": "uuid-do-cliente",
    "segment": "...",
    "positioning": "...",
    "objectives": "...",
    "brandVoice": "..."
  }
}
```

O Go resolve cliente e contexto pelo vínculo tenant-scoped `automation_profiles`. Chaves de
provider continuam somente no token efêmero do gateway e nunca entram em `request`.

## Result

O v3 aceita `continue_ai`, `handoff`, `no_reply` e `close`. Todo resultado v3 precisa trazer:

```json
{
  "schemaVersion": "brain.result.v3",
  "decision": "close",
  "closure": {
    "requested": true,
    "humanRequested": false,
    "sensitiveTopic": false,
    "reason": "demanda resolvida"
  }
}
```

`closure.requested=true` é obrigatório para `decision=close`. Isso ainda não autoriza o
encerramento. O Go revalida sob lock:

1. perfil existente e automação habilitada;
2. fechamento automático habilitado;
3. confiança mínima;
4. campos obrigatórios, quando configurado;
5. ausência de pedido humano, quando configurado;
6. ausência de assunto sensível, quando configurado;
7. `ai_generation` capturada igual à atual — gate estrutural não desligável.

Cada proposta gera `messaging.ai_close_evaluations`, aceita ou rejeitada, sem prompt ou texto da
conversa. Um fechamento aceito cria a resposta final e a outbox na mesma transação, preserva
somente essa mensagem e invalida todas as demais ações de IA.

## Compatibilidade e rollout

- agentes publicados com `brain.v2` continuam enviando `brain.request.v2` e não podem propor close;
- o workflow Omnichannel aceita request v2/v3 e devolve result na mesma geração do contrato;
- um agente passa para v3 somente por nova versão publicada/configurada no painel;
- resultado atrasado, identidade divergente ou JSON fora do schema é rejeitado no Go.
