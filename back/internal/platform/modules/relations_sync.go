package modules

import (
	"context"
	"strings"
	"time"
)

// WAVE 5 (E2): sincronizacao INVERTIDA calendario<->tasks. O modulo tasks NAO importa
// calendar; quando uma task muda, ele avisa (por este registry) os modulos que OWNam um
// recurso espelhado por task (ex.: calendar mantem o evento-espelho). Espelha o
// RelationRegistry/RelationResolver ja existente, no sentido oposto (tasks -> dono do recurso).
// Toda a guarda anti-loop mora no handler (metodos terminais + o campo Source do snapshot).

// TaskSyncSnapshot e o estado de uma task APOS uma mudanca, passado aos handlers. Relations =
// os vinculos atuais da task (o handler filtra os do seu modulo para achar o espelho). Campos
// opcionais como ponteiro (nil = ausente). Source = ui_metadata.source da task ('calendar' =
// nasceu de um evento; o handler usa isso para nao recriar o espelho, evitando ping-pong).
type TaskSyncSnapshot struct {
	TaskID            string
	BoardID           string
	Title             string
	Status            string
	Priority          string
	Type              string
	Source            string
	DueDate           *time.Time
	ColumnID          *string
	ClientAccountID   *string
	ResponsibleUserID *string
	Relations         []RelationRef
	// Media (WAVE 6, cruzamento B) = a midia da task (os videos de ui_metadata.videos) que o dono
	// do recurso pode espelhar read-only (ex.: calendar guarda em events.linked_media). nil/vazio =
	// sem midia. Tipo neutro (sem importar tasks/calendar) para trafegar entre modulos pelo registry.
	Media []MediaSnapshot
}

// MediaSnapshot e uma referencia de midia neutra (sem acoplar tasks<->calendar) trafegada no
// TaskSyncSnapshot. Shape alinhado ao MediaItem/TaskVideo: url interna servida em /uploads/.
type MediaSnapshot struct {
	ID          string
	URL         string
	Name        string
	Type        string // "image" | "video" (video da task sempre "video")
	ContentType string
	SizeBytes   int
	PosterURL   string
}

// ResourceID devolve o resourceID do primeiro vinculo (module, resourceType) do snapshot, ou
// "" se nao houver. Ex.: snap.ResourceID("calendar", "event") = o id do evento-espelho.
func (s TaskSyncSnapshot) ResourceID(moduleID, resourceType string) string {
	moduleID = strings.TrimSpace(moduleID)
	resourceType = strings.TrimSpace(resourceType)
	for _, r := range s.Relations {
		if strings.TrimSpace(r.ModuleID) == moduleID && strings.TrimSpace(r.ResourceType) == resourceType {
			return strings.TrimSpace(r.ResourceID)
		}
	}
	return ""
}

// RelationSyncHandler e registrado por um modulo dono de um recurso espelhavel por task.
// OnTaskChanged e chamado APOS commit de create/update/move/archive da task (deleted=true no
// archive). Best-effort: erro so vira log (nunca desfaz a task).
type RelationSyncHandler interface {
	ModuleID() string
	OnTaskChanged(ctx context.Context, accountID string, snap TaskSyncSnapshot, deleted bool) error
}

// RelationSyncRegistry guarda os handlers e faz o dispatch. nil = sync desligado (no-op).
type RelationSyncRegistry struct {
	handlers []RelationSyncHandler
}

func NewRelationSyncRegistry(handlers ...RelationSyncHandler) *RelationSyncRegistry {
	registry := &RelationSyncRegistry{}
	for _, handler := range handlers {
		registry.Register(handler)
	}
	return registry
}

func (registry *RelationSyncRegistry) Register(handler RelationSyncHandler) {
	if registry == nil || handler == nil || strings.TrimSpace(handler.ModuleID()) == "" {
		return
	}
	registry.handlers = append(registry.handlers, handler)
}

// Dispatch chama todos os handlers (best-effort). Cada handler decide, pelas relations do
// snapshot e pelo Source, se o assunto e' dele. Erros ficam a cargo do handler (log interno).
func (registry *RelationSyncRegistry) Dispatch(ctx context.Context, accountID string, snap TaskSyncSnapshot, deleted bool) {
	if registry == nil {
		return
	}
	for _, handler := range registry.handlers {
		_ = handler.OnTaskChanged(ctx, accountID, snap, deleted)
	}
}
