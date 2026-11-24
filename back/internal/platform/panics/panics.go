// Package panics conta os panics recuperados pelo processo. E um pacote folha
// (sem dependencias internas) para httpapi, background e jobs poderem registrar
// sem criar ciclo com platform/observability, que le o contador.
package panics

import "sync/atomic"

var total atomic.Int64

// Record registra um panic recuperado. source e informativo ("http", "background",
// "jobs") e hoje nao separa o contador.
func Record(source string) {
	_ = source
	total.Add(1)
}

// Total devolve quantos panics foram recuperados desde o inicio do processo.
func Total() int64 { return total.Load() }
