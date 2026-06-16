// Sessao MCP UNICA e persistente do runner.
//
// O OAuth do MCP da Meta so vale DENTRO de uma conexao viva. Por isso o runner
// mantem UMA query() do Agent SDK aberta (prompt = stream de mensagens) que
// atende o login (auth.mjs) E todos os comandos do chat (/run). Assim o token
// obtido no login persiste para todas as mensagens seguintes — sem isso cada
// /run abria conexao nova SEM token e o modelo ficava sem ferramentas reais
// (alucinava). Os turnos sao SERIALIZADOS (uma resposta por vez). Em timeout, a
// sessao e recriada (perde o login -> pede reconectar), evitando vazar a resposta
// de um turno para o proximo.

import { query } from '@anthropic-ai/claude-agent-sdk';

import {
  AssistantTimeoutError,
  buildPrompt,
  buildQueryOptions,
  collectAssistantMessage,
  collectToolResults,
  guardReply,
  metaToolCountFromInit,
  sanitizeReply,
} from './agent.mjs';
import { config } from './config.mjs';
import { refreshIfNeeded } from './oauth.mjs';
import { setAccountContext } from './omni-tools.mjs';

function userMessage(text) {
  return { type: 'user', message: { role: 'user', content: text }, parent_tool_use_id: null };
}

function deferred() {
  const box = {};
  box.promise = new Promise((resolve, reject) => {
    box.resolve = resolve;
    box.reject = reject;
  });
  return box;
}

// Fila async empurravel: vira o `prompt` (AsyncIterable) da query persistente.
function createInputQueue() {
  const items = [];
  const waiters = [];
  let ended = false;
  return {
    push(message) {
      if (ended) return;
      const waiter = waiters.shift();
      if (waiter) waiter(message);
      else items.push(message);
    },
    end() {
      ended = true;
      while (waiters.length > 0) waiters.shift()(null);
    },
    async *[Symbol.asyncIterator]() {
      for (;;) {
        if (items.length > 0) {
          yield items.shift();
          continue;
        }
        if (ended) return;
        const next = await new Promise((resolve) => waiters.push(resolve));
        if (next === null) return;
        yield next;
      }
    },
  };
}

class MetaSession {
  constructor() {
    this.input = null;
    this.query = null;
    this.abort = null;
    this.running = false;
    this.current = null;
    this.chain = Promise.resolve();
    this.optsModel = '';
    this.optsPrompt = '';
    // Quantas tools do MCP meta-ads o SDK expos no init (null = ainda nao iniciou).
    this.metaToolCount = null;
  }

  async ensure(opts = {}) {
    const model = typeof opts.model === 'string' ? opts.model.trim() : '';
    const systemPrompt = typeof opts.systemPrompt === 'string' ? opts.systemPrompt : '';
    // Settings (modelo/prompt) mudaram? recria a sessao com as novas opcoes. Com
    // o OAuth proprio (token em disco), a nova sessao nasce autenticada pelo
    // header — sem o usuario relogar.
    if (this.running && (this.optsModel !== model || this.optsPrompt !== systemPrompt)) {
      this.recreate();
    }
    if (this.running) return;
    // Refresh do token OAuth ANTES de montar as opcoes: se expira em < 60s e ha
    // refresh_token, renova; se falhar, apaga (a sessao nasce sem header e cai no
    // fallback via modelo). Best-effort — nunca bloqueia o turno.
    try {
      await refreshIfNeeded();
    } catch {
      /* refresh best-effort; segue sem header (fallback) */
    }
    this.optsModel = model;
    this.optsPrompt = systemPrompt;
    this.metaToolCount = null;
    this.input = createInputQueue();
    this.abort = new AbortController();
    this.query = query({
      prompt: this.input,
      options: buildQueryOptions(this.abort, { model, systemPrompt }),
    });
    this.running = true;
    this.consume();
  }

  async consume() {
    try {
      for await (const message of this.query) {
        // Auto-checagem de conexao: o SDK emite system/init com a lista de tools.
        // 0 tools meta-ads => o MCP da Meta nao conectou/autenticou nesta sessao.
        if (message.type === 'system' && message.subtype === 'init') {
          this.metaToolCount = metaToolCountFromInit(message);
          console.log(
            `[meta-ads-assistant] sessao MCP iniciada | tools meta-ads disponiveis: ${this.metaToolCount}`,
          );
          continue;
        }
        const state = this.current;
        if (!state) continue;
        if (message.type === 'assistant') {
          collectAssistantMessage(message, state);
        } else if (message.type === 'user') {
          collectToolResults(message, state);
        } else if (message.type === 'result') {
          if (message.subtype === 'success') {
            state.resultText = typeof message.result === 'string' ? message.result.trim() : '';
          } else if (Array.isArray(message.errors)) {
            state.resultErrors = message.errors;
          }
          state.done.resolve();
        }
      }
    } catch {
      /* stream quebrou; sera recriada no proximo turno */
    } finally {
      this.running = false;
    }
  }

  recreate() {
    try {
      this.abort?.abort();
    } catch {
      /* noop */
    }
    try {
      this.query?.close?.();
    } catch {
      /* noop */
    }
    this.running = false;
    this.current = null;
  }

  // hasMetaTools: a sessao viva expos tools do MCP da Meta (login in-session ok)?
  // Usado pelo /healthz para distinguir fallback 'session' de 'none'.
  hasMetaTools() {
    return this.running && typeof this.metaToolCount === 'number' && this.metaToolCount > 0;
  }

  // run executa UM turno (serializado pela chain). Retorna { reply, actions }.
  // opts = { model, systemPrompt } das configuracoes da account; control pode
  // trazer accountId (contexto da conta para as tools do bridge omni).
  run(prompt, opts = {}, control = {}) {
    const exec = () => this.execTurn(prompt, opts, control);
    const result = this.chain.then(exec, exec);
    this.chain = result.then(
      () => {},
      () => {},
    );
    return result;
  }

  async execTurn(prompt, opts, control = {}) {
    // Contexto da conta para as tools do bridge omni (turnos serializados => seguro).
    setAccountContext(typeof control.accountId === 'string' ? control.accountId : '');
    await this.ensure(opts);
    const state = {
      actions: [],
      actionIndexByToolUseId: new Map(),
      lastAssistantText: '',
      resultText: '',
      resultErrors: [],
      done: deferred(),
    };
    this.current = state;
    let timer;
    const timeout = new Promise((_, reject) => {
      timer = setTimeout(
        () => reject(new AssistantTimeoutError(config.timeoutMs)),
        config.timeoutMs,
      );
    });
    try {
      this.input.push(userMessage(prompt));
      await Promise.race([state.done.promise, timeout]);
      const raw = sanitizeReply(state.resultText || state.lastAssistantText);
      // Login (guard:false) usa tools de auth e nao deve passar pela trava.
      if (control.guard === false) {
        return { reply: raw, actions: state.actions };
      }
      // TRAVA anti-invencao: sem ferramenta real + resposta com cara de dado => bloqueia.
      const { reply, suppressed } = guardReply(raw, state.actions);
      if (suppressed) {
        console.warn('[meta-ads-assistant] resposta sem ferramenta real bloqueada (anti-invencao).');
      }
      return { reply, actions: state.actions };
    } catch (err) {
      // Timeout/erro: recria a sessao para nao vazar a resposta ao proximo turno.
      this.recreate();
      throw err;
    } finally {
      clearTimeout(timer);
      this.current = null;
    }
  }
}

export const session = new MetaSession();

// recreateSession forca o descarte da sessao MCP atual. Apos o OAuth completar
// (token novo em disco), o proximo turno cria uma sessao que ja nasce com o
// header Authorization — sem o usuario relogar pelo modelo.
export function recreateSession() {
  session.recreate();
}

// runAssistant: consumido pelo /run (server.mjs). Monta o prompt (historico +
// conta de anuncios) e roda na sessao unica persistente. accountId vai no contexto
// do turno para as tools do bridge omni (feed do Instagram).
export async function runAssistant({
  prompt,
  history = [],
  adAccountId = '',
  accountId = '',
  model = '',
  systemPrompt = '',
}) {
  return session.run(buildPrompt({ prompt, history, adAccountId }), { model, systemPrompt }, { accountId });
}
