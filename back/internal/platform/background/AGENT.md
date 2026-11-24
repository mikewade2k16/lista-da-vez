# background — goroutines de longa duração com recover e shutdown

`Group` (A-03): `Go(ctx, nome, fn)` roda uma vez; `Loop(ctx, nome, fn)` reinicia a
função após panic com backoff 1s..1min enquanto o contexto viver; retorno normal
encerra. Panic vira log `background_panic` com stack, nunca derruba a API.
`AddCloser` registra `Handle.Close` de módulos; `Shutdown(ctx)` roda os closers
(ordem inversa) e espera as goroutines até o prazo. Contexto raiz e prazo vêm de
`cmd/api/main.go` (`SHUTDOWN_TIMEOUT`, padrão 20s). Novo worker de background na
API: use `bg.Loop`/`bg.Go` com o `ctx` de `BuildHTTPHandler`, respeitando
`ctx.Done()` nas esperas. Não logar payloads nem segredos.
