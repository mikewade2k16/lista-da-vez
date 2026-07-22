package omnichannel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

// TestStoreDeliveryE1 usa somente um banco descartavel cujo nome comece por
// omni_e1_test_. O guard impede que o fixture destrutivo rode no banco local/producao.
func TestStoreDeliveryE1(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E1_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E1_TEST_DATABASE_URL nao definido")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	var databaseName string
	if err := pool.QueryRow(ctx, `select current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("current_database: %v", err)
	}
	if !strings.HasPrefix(databaseName, "omni_e1_test_") {
		t.Fatalf("banco de teste recusado: %q", databaseName)
	}
	setupDeliveryTestSchema(t, pool)
	store := NewStore(pool)

	const (
		accountA  = "11111111-1111-4111-8111-111111111111"
		accountB  = "22222222-2222-4222-8222-222222222222"
		instanceA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		instanceB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	if _, err := pool.Exec(ctx, `insert into core.accounts(id,slug) values
		($1::uuid,'a'),($2::uuid,'b')`, accountA, accountB); err != nil {
		t.Fatalf("seed accounts: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider) values
		($3::uuid,$1::uuid,'main','evolution'),($4::uuid,$2::uuid,'main','evolution')`,
		accountA, accountB, instanceA, instanceB); err != nil {
		t.Fatalf("seed instances: %v", err)
	}

	t.Run("dedupe fromMe e isolamento de tenant", func(t *testing.T) {
		occurred := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
		first := inboundTextWrite(accountA, instanceA, "evt-a-1", "EXT-SAME", occurred, true)
		res, err := store.PersistInbound(ctx, first)
		if err != nil || res.MessageID == "" {
			t.Fatalf("primeiro fromMe: res=%+v err=%v", res, err)
		}
		second := inboundTextWrite(accountA, instanceA, "evt-a-2", "EXT-SAME", occurred.Add(time.Second), true)
		res2, err := store.PersistInbound(ctx, second)
		if err != nil || res2.MessageID != "" {
			t.Fatalf("redelivery: res=%+v err=%v", res2, err)
		}
		otherTenant := inboundTextWrite(accountB, instanceB, "evt-b-1", "EXT-SAME", occurred, true)
		if _, err := store.PersistInbound(ctx, otherTenant); err != nil {
			t.Fatalf("mesmo external id em outra conta: %v", err)
		}
		var count int
		var direction, origin string
		if err := pool.QueryRow(ctx, `select count(*), min(direction), min(origin)
			from messaging.messages where account_id=$1::uuid and external_message_id='EXT-SAME'`,
			accountA).Scan(&count, &direction, &origin); err != nil {
			t.Fatalf("consulta fromMe: %v", err)
		}
		if count != 1 || direction != "OUTBOUND" || origin != "provider_device" {
			t.Fatalf("count=%d direction=%s origin=%s", count, direction, origin)
		}
	})

	t.Run("fromMe toma conversa e replay nao renova lease", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 10, 15, 0, 0, time.UTC)
		const externalConversationID = "device-takeover@s.whatsapp.net"
		var conversationID string
		if err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 contact_phone,last_message_at,created_at)
			values ($1::uuid,$2::uuid,'main','WHATSAPP',$3,'Device Takeover','ai_active',
			 '5511888880000',$4,$4) returning id::text`,
			accountA, instanceA, externalConversationID, base).Scan(&conversationID); err != nil {
			t.Fatal(err)
		}

		write := inboundTextWrite(accountA, instanceA, "evt-device-takeover", "EXT-DEVICE-TAKEOVER", base, true)
		write.Message.ContactExternalID = externalConversationID
		write.Message.ContactPhone = "5511888880000"
		domain := NewService(store)
		persist := func() (inboundResult, error) {
			return store.PersistInboundWithTransition(ctx, write,
				func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
					return domain.decideTransition(ctx, accountA, EventMsgOutboundHuman, TransitionPayload{}, snap)
				})
		}

		first, err := persist()
		if err != nil || !first.MessageCreated || first.ConversationID != conversationID {
			t.Fatalf("first fromMe: result=%+v err=%v", first, err)
		}
		var state string
		var generation int64
		if err := pool.QueryRow(ctx, `select state,ai_generation from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`, accountA, conversationID).
			Scan(&state, &generation); err != nil {
			t.Fatal(err)
		}
		if state != "human_active" || generation != 1 {
			t.Fatalf("state=%s generation=%d", state, generation)
		}

		replay, err := persist()
		if err != nil || !replay.Duplicate {
			t.Fatalf("replay fromMe: result=%+v err=%v", replay, err)
		}
		if err := pool.QueryRow(ctx, `select state,ai_generation from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`, accountA, conversationID).
			Scan(&state, &generation); err != nil {
			t.Fatal(err)
		}
		if state != "human_active" || generation != 1 {
			t.Fatalf("replay mudou lease: state=%s generation=%d", state, generation)
		}
	})

	t.Run("identidade exata vence telefone e last_seen nao regride", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 10, 30, 0, 0, time.UTC)
		first := inboundTextWrite(accountA, instanceA, "evt-identity-a", "MSG-ID-A", base, false)
		first.Message.ContactExternalID = "contact-a"
		first.Message.ContactPhone = "5511000000000"
		firstRes, err := store.PersistInbound(ctx, first)
		if err != nil {
			t.Fatalf("contato A: %v", err)
		}
		second := inboundTextWrite(accountA, instanceA, "evt-identity-b", "MSG-ID-B", base.Add(time.Minute), false)
		second.Message.ContactExternalID = "contact-b"
		second.Message.ContactPhone = "5521000000000"
		secondRes, err := store.PersistInbound(ctx, second)
		if err != nil {
			t.Fatalf("contato B: %v", err)
		}
		if firstRes.ConversationID == secondRes.ConversationID {
			t.Fatal("telefones distintos nao deveriam compartilhar conversa")
		}

		// O telefone aponta para A, mas a identidade exata aponta para B. O match exato
		// precisa vencer para impedir a fusao silenciosa entre canais/provedores.
		conflict := inboundTextWrite(accountA, instanceA, "evt-identity-conflict", "MSG-ID-C", base.Add(2*time.Minute), false)
		conflict.Message.ContactExternalID = "contact-b"
		conflict.Message.ContactPhone = "5511000000000"
		conflictRes, err := store.PersistInbound(ctx, conflict)
		if err != nil {
			t.Fatalf("identidade exata: %v", err)
		}
		var contactID string
		if err := pool.QueryRow(ctx, `select contact_id::text from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`, accountA, conflictRes.ConversationID).Scan(&contactID); err != nil {
			t.Fatalf("contato da conversa: %v", err)
		}
		var contactBID string
		if err := pool.QueryRow(ctx, `select contact_id::text from messaging.contact_identities
			where account_id=$1::uuid and channel='WHATSAPP' and provider='evolution'
			  and instance_scope_key='main' and external_id='contact-b'`, accountA).Scan(&contactBID); err != nil {
			t.Fatalf("identidade B: %v", err)
		}
		if contactID != contactBID {
			t.Fatalf("conversa=%s identidadeB=%s", contactID, contactBID)
		}

		newer := inboundTextWrite(accountA, instanceA, "evt-identity-newer", "MSG-ID-D", base.Add(10*time.Minute), false)
		newer.Message.ContactExternalID = "contact-b"
		newer.Message.ContactPhone = "5521000000000"
		if _, err := store.PersistInbound(ctx, newer); err != nil {
			t.Fatalf("identidade nova: %v", err)
		}
		older := inboundTextWrite(accountA, instanceA, "evt-identity-older", "MSG-ID-E", base.Add(time.Minute), false)
		older.Message.ContactExternalID = "contact-b"
		older.Message.ContactPhone = "5521000000000"
		if _, err := store.PersistInbound(ctx, older); err != nil {
			t.Fatalf("identidade antiga: %v", err)
		}
		var lastSeen time.Time
		if err := pool.QueryRow(ctx, `select last_seen_at from messaging.contact_identities
			where account_id=$1::uuid and channel='WHATSAPP' and provider='evolution'
			  and instance_scope_key='main' and external_id='contact-b'`, accountA).Scan(&lastSeen); err != nil {
			t.Fatalf("last_seen identidade: %v", err)
		}
		want := base.Add(10 * time.Minute)
		if !lastSeen.Equal(want) {
			t.Fatalf("last_seen=%s want=%s", lastSeen, want)
		}
	})

	t.Run("ACK monotono e DELETED terminal", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC)
		seed := inboundTextWrite(accountA, instanceA, "evt-ack-seed", "EXT-ACK", base, true)
		if _, err := store.PersistInbound(ctx, seed); err != nil {
			t.Fatalf("seed ACK: %v", err)
		}
		applyStatus := func(eventID, status string, at time.Time) inboundResult {
			t.Helper()
			res, err := store.PersistInbound(ctx, inboundWrite{
				AccountID: accountA, Provider: "evolution", ExternalEventID: eventID,
				EventKind: "message_status", InstanceName: "main", InstanceID: instanceA,
				Status: &inboundStatusWrite{ExternalMessageID: "EXT-ACK", Status: status,
					ErrorCode: "SAFE_CODE", OccurredAt: at},
			})
			if err != nil {
				t.Fatalf("status %s: %v", status, err)
			}
			return res
		}
		if res := applyStatus("evt-ack-read", "READ", base.Add(2*time.Minute)); !res.StatusChanged {
			t.Fatal("READ deveria avancar")
		}
		if res := applyStatus("evt-ack-old", "SENT", base.Add(3*time.Minute)); res.StatusChanged {
			t.Fatal("SENT nao pode regredir READ")
		}
		if res := applyStatus("evt-ack-failed", "FAILED", base.Add(4*time.Minute)); res.StatusChanged {
			t.Fatal("FAILED nao pode substituir READ")
		}
		if res := applyStatus("evt-ack-deleted", "DELETED", base.Add(5*time.Minute)); !res.StatusChanged {
			t.Fatal("DELETED deveria ser aplicado")
		}
		if res := applyStatus("evt-ack-after-delete", "READ", base.Add(6*time.Minute)); res.StatusChanged {
			t.Fatal("DELETED deve ser terminal")
		}
		var status, errorCode string
		if err := pool.QueryRow(ctx, `select status,provider_error_code from messaging.messages
			where account_id=$1::uuid and external_message_id='EXT-ACK'`, accountA,
		).Scan(&status, &errorCode); err != nil {
			t.Fatalf("consulta ACK: %v", err)
		}
		if status != "DELETED" || errorCode != "" {
			t.Fatalf("status=%s errorCode=%q", status, errorCode)
		}
	})

	t.Run("ACK anterior a mensagem e replayado de forma monotona", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 11, 30, 0, 0, time.UTC)
		early := inboundWrite{
			AccountID: accountA, Provider: "evolution", ExternalEventID: "evt-ack-early",
			EventKind: "message_status", InstanceName: "main", InstanceID: instanceA,
			Status: &inboundStatusWrite{ExternalMessageID: "EXT-EARLY", Status: "READ",
				OccurredAt: base.Add(2 * time.Minute)},
		}
		res, err := store.PersistInbound(ctx, early)
		if err != nil || res.StatusChanged {
			t.Fatalf("early ACK: res=%+v err=%v", res, err)
		}
		message := inboundTextWrite(accountA, instanceA, "evt-ack-early-message", "EXT-EARLY", base, true)
		res, err = store.PersistInbound(ctx, message)
		if err != nil || !res.MessageCreated || !res.StatusChanged || res.ProviderStatus != "READ" {
			t.Fatalf("message replay: res=%+v err=%v", res, err)
		}
		var status string
		var replayCount int
		if err := pool.QueryRow(ctx, `select status from messaging.messages
			where account_id=$1::uuid and external_message_id='EXT-EARLY'`, accountA).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.webhook_events
			where account_id=$1::uuid and provider='evolution' and instance_name='main'
			  and external_message_id='EXT-EARLY' and provider_status='READ'`, accountA).Scan(&replayCount); err != nil {
			t.Fatal(err)
		}
		if status != "READ" || replayCount != 1 {
			t.Fatalf("status=%s replay events=%d", status, replayCount)
		}
		dup, err := store.PersistInbound(ctx, early)
		if err != nil || !dup.Duplicate {
			t.Fatalf("duplicate ACK: res=%+v err=%v", dup, err)
		}
	})

	t.Run("reply externo reconcilia quando original chega depois", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
		reply := inboundTextWrite(accountA, instanceA, "evt-reply", "EXT-REPLY", base, false)
		reply.Message.Reply = &channel.ReplyReference{
			ExternalMessageID: "EXT-ORIGINAL", Content: "original", MessageType: "TEXT",
		}
		replyRes, err := store.PersistInbound(ctx, reply)
		if err != nil {
			t.Fatalf("reply primeiro: %v", err)
		}
		original := inboundTextWrite(accountA, instanceA, "evt-original", "EXT-ORIGINAL", base.Add(time.Second), false)
		originalRes, err := store.PersistInbound(ctx, original)
		if err != nil {
			t.Fatalf("original depois: %v", err)
		}
		var linkedID, externalID string
		if err := pool.QueryRow(ctx, `select reply_to_message_id::text,reply_to_external_message_id
			from messaging.messages where account_id=$1::uuid and id=$2::uuid`,
			accountA, replyRes.MessageID).Scan(&linkedID, &externalID); err != nil {
			t.Fatalf("consulta reply: %v", err)
		}
		if linkedID != originalRes.MessageID || externalID != "EXT-ORIGINAL" {
			t.Fatalf("linked=%s original=%s external=%s", linkedID, originalRes.MessageID, externalID)
		}
	})

	t.Run("eco fromMe e reconciliado com mensagem local", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 13, 0, 0, 0, time.UTC)
		echo := inboundTextWrite(accountA, instanceA, "evt-race-echo", "EXT-RACE", base, true)
		echoRes, err := store.PersistInbound(ctx, echo)
		if err != nil {
			t.Fatalf("echo: %v", err)
		}
		var localID string
		if err := pool.QueryRow(ctx, `insert into messaging.messages
			(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,created_at)
			values ($1::uuid,$2::uuid,$3::uuid,'main','OUTBOUND','TEXT','local','PENDING','human',$4)
			returning id::text`, accountA, echoRes.ConversationID, instanceA, base.Add(time.Second),
		).Scan(&localID); err != nil {
			t.Fatalf("local: %v", err)
		}
		var dependentID string
		if err := pool.QueryRow(ctx, `insert into messaging.messages
			(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,reply_to_message_id,reply_to_external_message_id,created_at)
			values ($1::uuid,$2::uuid,$3::uuid,'main','INBOUND','TEXT','reply','SENT','contact',
			 $4::uuid,'EXT-RACE',$5) returning id::text`,
			accountA, echoRes.ConversationID, instanceA, echoRes.MessageID, base.Add(2*time.Second),
		).Scan(&dependentID); err != nil {
			t.Fatalf("dependent: %v", err)
		}
		if _, err := store.PersistInbound(ctx, inboundWrite{
			AccountID: accountA, Provider: "evolution", ExternalEventID: "evt-race-read",
			EventKind: "message_status", InstanceName: "main", InstanceID: instanceA,
			Status: &inboundStatusWrite{ExternalMessageID: "EXT-RACE", Status: "READ",
				OccurredAt: base.Add(3 * time.Second)},
		}); err != nil {
			t.Fatalf("echo READ: %v", err)
		}
		if _, err := store.MarkMessageSent(ctx, accountA, localID, "EXT-RACE"); err != nil {
			t.Fatalf("MarkMessageSent: %v", err)
		}
		var count int
		if err := pool.QueryRow(ctx, `select count(*) from messaging.messages
			where account_id=$1::uuid and external_message_id='EXT-RACE'`, accountA).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("external duplicado: %d", count)
		}
		var canonicalID, status, replyTarget string
		if err := pool.QueryRow(ctx, `select id::text,status from messaging.messages
			where account_id=$1::uuid and external_message_id='EXT-RACE'`, accountA,
		).Scan(&canonicalID, &status); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select reply_to_message_id::text from messaging.messages
			where account_id=$1::uuid and id=$2::uuid`, accountA, dependentID).Scan(&replyTarget); err != nil {
			t.Fatal(err)
		}
		if canonicalID != localID || status != "READ" || replyTarget != localID {
			t.Fatalf("canonical=%s local=%s status=%s replyTarget=%s", canonicalID, localID, status, replyTarget)
		}
	})

	t.Run("eco FAILED nao e mascarado como SENT no merge local", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 13, 30, 0, 0, time.UTC)
		echo := inboundTextWrite(accountA, instanceA, "evt-failed-echo", "EXT-FAILED-ECHO", base, true)
		echoRes, err := store.PersistInbound(ctx, echo)
		if err != nil {
			t.Fatalf("echo: %v", err)
		}
		if _, err := store.PersistInbound(ctx, inboundWrite{
			AccountID: accountA, Provider: "evolution", ExternalEventID: "evt-echo-failed-status",
			EventKind: "message_status", InstanceName: "main", InstanceID: instanceA,
			Status: &inboundStatusWrite{ExternalMessageID: "EXT-FAILED-ECHO", Status: "FAILED",
				ErrorCode: "SAFE_FAILURE", OccurredAt: base.Add(time.Second)},
		}); err != nil {
			t.Fatalf("echo FAILED: %v", err)
		}
		var localID string
		if err := pool.QueryRow(ctx, `insert into messaging.messages
			(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,created_at)
			values ($1::uuid,$2::uuid,$3::uuid,'main','OUTBOUND','TEXT','local failed echo',
			 'PENDING','human',$4) returning id::text`,
			accountA, echoRes.ConversationID, instanceA, base.Add(2*time.Second)).Scan(&localID); err != nil {
			t.Fatalf("local: %v", err)
		}
		if _, err := store.MarkMessageSent(ctx, accountA, localID, "EXT-FAILED-ECHO"); err != nil {
			t.Fatalf("MarkMessageSent: %v", err)
		}
		var status, providerCode string
		if err := pool.QueryRow(ctx, `select status,provider_error_code from messaging.messages
			where account_id=$1::uuid and id=$2::uuid`, accountA, localID).
			Scan(&status, &providerCode); err != nil {
			t.Fatal(err)
		}
		if status != "FAILED" || providerCode != "SAFE_FAILURE" {
			t.Fatalf("status=%s providerCode=%s", status, providerCode)
		}
	})

	t.Run("midia pronta, falha e retry usam uma unica linha tenant scoped", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
		readyWrite := inboundTextWrite(accountA, instanceA, "evt-media-ready", "EXT-MEDIA-READY", base, false)
		readyWrite.Message.MessageType = "IMAGE"
		readyWrite.Message.MediaMimeType = "image/png"
		readyWrite.Message.MediaFileName = "ready.png"
		readyRes, err := store.PersistInbound(ctx, readyWrite)
		if err != nil {
			t.Fatalf("seed media ready: %v", err)
		}
		fetch, err := store.GetMediaFetchData(ctx, accountA, readyRes.MessageID)
		if err != nil || fetch.ExternalMessageID != "EXT-MEDIA-READY" || fetch.MaxBytes <= 0 {
			t.Fatalf("fetch data = %+v err=%v", fetch, err)
		}
		if _, err := store.GetMediaFetchData(ctx, accountB, readyRes.MessageID); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("outra conta = %v, want ErrNoRows", err)
		}
		view, err := store.UpdateFetchedMedia(ctx, accountA, readyRes.ConversationID, readyRes.MessageID,
			StoredMedia{StorageKey: "acc/conv/msg.png", MimeType: "image/png", FileName: "ready.png", SizeBytes: 4, SHA256: "abcd"})
		if err != nil || view.MediaState != "ready" || view.CanRetryMedia {
			t.Fatalf("ready view=%+v err=%v", view, err)
		}

		failedWrite := inboundTextWrite(accountA, instanceA, "evt-media-failed", "EXT-MEDIA-FAILED", base.Add(time.Second), false)
		failedWrite.Message.MessageType = "AUDIO"
		failedWrite.Message.MediaMimeType = "audio/ogg"
		failedRes, err := store.PersistInbound(ctx, failedWrite)
		if err != nil {
			t.Fatalf("seed media failed: %v", err)
		}
		failed, err := store.MarkMediaFetchFailed(ctx, accountA, failedRes.ConversationID, failedRes.MessageID, "provider_not_ready")
		if err != nil || failed.MediaState != "failed" || !failed.CanRetryMedia {
			t.Fatalf("failed view=%+v err=%v", failed, err)
		}
		if _, err := pool.Exec(ctx, `update messaging.outbox set status='dead', attempts=4
			where account_id=$1::uuid and idempotency_key=$2`, accountA, "media-fetch:"+failedRes.MessageID); err != nil {
			t.Fatalf("dead seed: %v", err)
		}
		retried, err := store.RetryMediaFetch(ctx, accountA, failedRes.ConversationID, failedRes.MessageID)
		if err != nil || retried.MediaState != "pending" || retried.CanRetryMedia {
			t.Fatalf("retried view=%+v err=%v", retried, err)
		}
		var count, attempts int
		var status string
		if err := pool.QueryRow(ctx, `select count(*),min(status),min(attempts)
			from messaging.outbox where account_id=$1::uuid and idempotency_key=$2 and kind=$3`,
			accountA, "media-fetch:"+failedRes.MessageID, MediaFetchJobKind).Scan(&count, &status, &attempts); err != nil {
			t.Fatalf("retry outbox: %v", err)
		}
		if count != 1 || status != "pending" || attempts != 0 {
			t.Fatalf("outbox count=%d status=%s attempts=%d", count, status, attempts)
		}
	})

	t.Run("auditoria de midia aceita somente o vocabulario E1", func(t *testing.T) {
		for _, eventType := range []string{"MESSAGE_MEDIA_READY", "MESSAGE_MEDIA_FAILED", "MESSAGE_MEDIA_RETRY"} {
			if err := store.InsertAudit(ctx, accountA, "", "", "", eventType, nil); err != nil {
				t.Fatalf("InsertAudit(%s): %v", eventType, err)
			}
		}
		if err := store.InsertAudit(ctx, accountA, "", "", "", "MESSAGE_MEDIA_UNKNOWN", nil); err == nil {
			t.Fatal("evento de auditoria desconhecido deveria ser rejeitado")
		}
		var count int
		if err := pool.QueryRow(ctx, `select count(*) from messaging.audit_events
			where account_id=$1::uuid and event_type like 'MESSAGE_MEDIA_%'`, accountA).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			t.Fatalf("eventos de midia=%d, want 3", count)
		}
	})

	t.Run("conversas usam filtros e cursor estavel sem vazar tenant", func(t *testing.T) {
		const (
			queueID = "33333333-3333-4333-8333-333333333333"
			userID  = "44444444-4444-4444-8444-444444444444"
		)
		base := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
		insertConversation := func(accountID, instanceID, externalID, name, content string, at time.Time) string {
			t.Helper()
			var conversationID string
			if err := pool.QueryRow(ctx, `insert into messaging.conversations
				(account_id,instance_id,instance_scope_key,assigned_to_id,channel,external_id,
				 contact_name,state,queue_id,assigned_user_id,last_message_at,created_at)
				values ($1::uuid,$2::uuid,'main',$3::text,'WHATSAPP',$4,$5,'human_active',$6::uuid,$3::uuid,$7,$7)
				returning id::text`, accountID, instanceID, userID, externalID, name, queueID, at).Scan(&conversationID); err != nil {
				t.Fatalf("insert conversation: %v", err)
			}
			if _, err := pool.Exec(ctx, `insert into messaging.messages
				(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
				 content,status,origin,created_at)
				values ($1::uuid,$2::uuid,$3::uuid,'main','INBOUND','TEXT',$4,'SENT','contact',$5)`,
				accountID, conversationID, instanceID, content, at); err != nil {
				t.Fatalf("insert message: %v", err)
			}
			return conversationID
		}
		newerID := insertConversation(accountA, instanceA, "cursor-new", "Needle New", "needle alpha", base.Add(time.Minute))
		olderID := insertConversation(accountA, instanceA, "cursor-old", "Needle Old", "needle beta", base)
		_ = insertConversation(accountB, instanceB, "cursor-other", "Needle Other", "needle tenant", base.Add(2*time.Minute))

		svc := NewService(store)
		filter := ConversationPageFilter{Limit: 1, Search: "  Needle  ", Channel: "WHATSAPP",
			Status: "OPEN", QueueID: queueID, ResponsibleID: userID}
		first, err := svc.ListConversations(ctx, accountA, Caller{IsAdmin: true}, filter)
		if err != nil {
			t.Fatalf("first page: %v", err)
		}
		if len(first.Conversations) != 1 || first.Conversations[0].ID != newerID || !first.HasMore || first.NextCursor == "" {
			t.Fatalf("first page = %+v", first)
		}
		filter.BeforeCursor = first.NextCursor
		second, err := svc.ListConversations(ctx, accountA, Caller{IsAdmin: true}, filter)
		if err != nil {
			t.Fatalf("second page: %v", err)
		}
		if len(second.Conversations) != 1 || second.Conversations[0].ID != olderID || second.HasMore {
			t.Fatalf("second page = %+v", second)
		}
		other, err := svc.ListConversations(ctx, accountB, Caller{IsAdmin: true}, ConversationPageFilter{
			Limit: 10, Search: "needle", QueueID: queueID, ResponsibleID: userID,
		})
		if err != nil || len(other.Conversations) != 1 || other.Conversations[0].ID == newerID || other.Conversations[0].ID == olderID {
			t.Fatalf("other tenant page=%+v err=%v", other, err)
		}
	})

	t.Run("takeover invalida merge mensagem e outbox de IA por geracao", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 16, 0, 0, 0, time.UTC)
		var conversationID string
		if err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 last_message_at,created_at)
			values ($1::uuid,$2::uuid,'main','WHATSAPP','ai-lease','AI Lease','ai_active',$3,$3)
			returning id::text`, accountA, instanceA, base).Scan(&conversationID); err != nil {
			t.Fatalf("seed ai conversation: %v", err)
		}
		committed, err := store.CommitAITriage(ctx, accountA, conversationID, 0, map[string]any{"accepted": true})
		if err != nil || !committed {
			t.Fatalf("commit inicial: committed=%v err=%v", committed, err)
		}
		message, created, err := store.CreateAIOutboundMessage(ctx, accountA, conversationID,
			"resposta da IA", "run-1", "ai-reply:run-1", 0)
		if err != nil || !created || message.Origin != "ai" {
			t.Fatalf("create ai: message=%+v created=%v err=%v", message, created, err)
		}
		duplicate, created, err := store.CreateAIOutboundMessage(ctx, accountA, conversationID,
			"resposta repetida", "run-1", "ai-reply:run-1", 0)
		if err != nil || created || duplicate.ID != message.ID {
			t.Fatalf("duplicate ai: message=%+v created=%v err=%v", duplicate, created, err)
		}
		if _, err := pool.Exec(ctx, `update messaging.outbox set status='processing',
			locked_at=now(), locked_by='e1-test'
			where account_id=$1::uuid and idempotency_key='ai-reply:run-1'`, accountA); err != nil {
			t.Fatalf("claim ai job: %v", err)
		}
		if _, err := store.ApplyTransition(ctx, accountA, conversationID,
			func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
				return stateUpdate{State: StateHumanActive, QueueID: snap.QueueID,
					DepartmentID: snap.DepartmentID, AssignedUserID: snap.AssignedUserID,
					InvalidateAI: true}, nil, nil
			}); err != nil {
			t.Fatalf("takeover: %v", err)
		}
		committed, err = store.CommitAITriage(ctx, accountA, conversationID, 0, map[string]any{"stale": true})
		if err != nil || committed {
			t.Fatalf("stale merge: committed=%v err=%v", committed, err)
		}
		if _, _, err := store.CreateAIOutboundMessage(ctx, accountA, conversationID,
			"nao pode sair", "run-stale", "ai-reply:run-stale", 0); !errors.Is(err, ErrAILeaseInvalid) {
			t.Fatalf("stale outbound = %v", err)
		}
		providerCalls := 0
		if _, err := store.DispatchOutbound(ctx, accountA, message.ID, func(outboundSendData) (string, error) {
			providerCalls++
			return "EXT-SHOULD-NOT-SEND", nil
		}); !errors.Is(err, ErrAILeaseInvalid) || providerCalls != 0 {
			t.Fatalf("dispatch cancelado err=%v providerCalls=%d", err, providerCalls)
		}
		var generation, messages, outboxes int
		var state, messageStatus, outboxStatus, providerCode string
		var fields []byte
		if err := pool.QueryRow(ctx, `select state,ai_generation,extracted_fields
			from messaging.conversations where account_id=$1::uuid and id=$2::uuid`, accountA, conversationID).
			Scan(&state, &generation, &fields); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*),min(status),min(provider_error_code)
			from messaging.messages where account_id=$1::uuid and conversation_id=$2::uuid and origin='ai'`,
			accountA, conversationID).Scan(&messages, &messageStatus, &providerCode); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*),min(status) from messaging.outbox
			where account_id=$1::uuid and ordering_key=$2 and kind=$3`, accountA, conversationID,
			OutboundJobKind).Scan(&outboxes, &outboxStatus); err != nil {
			t.Fatal(err)
		}
		if state != "human_active" || generation != 1 || strings.Contains(string(fields), "stale") ||
			messages != 1 || messageStatus != "FAILED" || providerCode != "ai_handoff_canceled" ||
			outboxes != 1 || outboxStatus != "dead" {
			t.Fatalf("state=%s gen=%d fields=%s messages=%d/%s/%s outbox=%d/%s",
				state, generation, fields, messages, messageStatus, providerCode, outboxes, outboxStatus)
		}
	})

	t.Run("dispatch com lock termina antes do takeover concorrente", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 16, 30, 0, 0, time.UTC)
		var conversationID string
		if err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 last_message_at,created_at)
			values ($1::uuid,$2::uuid,'main','WHATSAPP','ai-lock','AI Lock','ai_active',$3,$3)
			returning id::text`, accountA, instanceA, base).Scan(&conversationID); err != nil {
			t.Fatal(err)
		}
		message, created, err := store.CreateAIOutboundMessage(ctx, accountA, conversationID,
			"resposta serializada", "run-lock", "ai-reply:run-lock", 0)
		if err != nil || !created {
			t.Fatalf("create ai lock: created=%v err=%v", created, err)
		}
		sendStarted := make(chan struct{})
		releaseSend := make(chan struct{})
		dispatchDone := make(chan error, 1)
		go func() {
			_, dispatchErr := store.DispatchOutbound(context.Background(), accountA, message.ID,
				func(outboundSendData) (string, error) {
					close(sendStarted)
					<-releaseSend
					return "EXT-AI-LOCK", nil
				})
			dispatchDone <- dispatchErr
		}()
		select {
		case <-sendStarted:
		case <-time.After(3 * time.Second):
			t.Fatal("dispatch nao obteve lock")
		}
		takeoverDone := make(chan error, 1)
		go func() {
			_, transitionErr := store.ApplyTransition(context.Background(), accountA, conversationID,
				func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
					return stateUpdate{State: StateHumanActive, QueueID: snap.QueueID,
						DepartmentID: snap.DepartmentID, AssignedUserID: snap.AssignedUserID,
						InvalidateAI: true}, nil, nil
				})
			takeoverDone <- transitionErr
		}()
		select {
		case err := <-takeoverDone:
			t.Fatalf("takeover atravessou lock do dispatch: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		close(releaseSend)
		if err := <-dispatchDone; err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if err := <-takeoverDone; err != nil {
			t.Fatalf("takeover: %v", err)
		}
		var state, status string
		if err := pool.QueryRow(ctx, `select c.state,m.status from messaging.conversations c
			join messaging.messages m on m.conversation_id=c.id and m.account_id=c.account_id
			where c.account_id=$1::uuid and c.id=$2::uuid and m.id=$3::uuid`,
			accountA, conversationID, message.ID).Scan(&state, &status); err != nil {
			t.Fatal(err)
		}
		if state != "human_active" || status != "SENT" {
			t.Fatalf("state=%s status=%s", state, status)
		}
	})

	t.Run("erro do provider libera lock da conversa", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 16, 45, 0, 0, time.UTC)
		var conversationID, messageID string
		if err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 contact_phone,last_message_at,created_at)
			values ($1::uuid,$2::uuid,'main','WHATSAPP','provider-error','Provider Error','human_active',
			 '5511999999999',$3,$3) returning id::text`, accountA, instanceA, base).Scan(&conversationID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `insert into messaging.messages
			(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,created_at)
			values ($1::uuid,$2::uuid,$3::uuid,'main','OUTBOUND','TEXT','provider error',
			 'PENDING','human',$4) returning id::text`, accountA, conversationID, instanceA, base,
		).Scan(&messageID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DispatchOutbound(ctx, accountA, messageID,
			func(outboundSendData) (string, error) { return "", context.DeadlineExceeded }); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("dispatch err=%v", err)
		}
		lockCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if _, err := store.ApplyTransition(lockCtx, accountA, conversationID,
			func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
				return stateUpdate{State: StateClosed, QueueID: snap.QueueID,
					DepartmentID: snap.DepartmentID, AssignedUserID: snap.AssignedUserID}, nil, nil
			}); err != nil {
			t.Fatalf("lock nao foi liberado: %v", err)
		}
	})

	t.Run("envio humano toma conversa e replay idempotente nao renova lease", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 17, 0, 0, 0, time.UTC)
		const userID = "55555555-5555-4555-8555-555555555555"
		var conversationID string
		if err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 contact_phone,last_message_at,created_at)
			values ($1::uuid,$2::uuid,'main','WHATSAPP','human-send','Human Send','ai_active',
			 '5511999999999',$3,$3) returning id::text`, accountA, instanceA, base).Scan(&conversationID); err != nil {
			t.Fatal(err)
		}
		send := NewSendService(store, nil, nil, nil)
		principal := auth.Principal{UserID: userID, Role: auth.RolePlatformAdmin, AccountID: accountA}
		input := SendMessageInput{Type: "TEXT", Content: "atendimento humano", IdempotencyKey: "human-rbac-idem"}
		first, _, err := send.SendMessage(ctx, accountA, principal, conversationID, input)
		if err != nil {
			t.Fatalf("first human send: %v", err)
		}
		second, _, err := send.SendMessage(ctx, accountA, principal, conversationID, input)
		if err != nil || second.ID != first.ID {
			t.Fatalf("idempotent send: first=%s second=%s err=%v", first.ID, second.ID, err)
		}
		var state, assigned string
		var generation int64
		if err := pool.QueryRow(ctx, `select state,assigned_user_id::text,ai_generation
			from messaging.conversations where account_id=$1::uuid and id=$2::uuid`,
			accountA, conversationID).Scan(&state, &assigned, &generation); err != nil {
			t.Fatal(err)
		}
		if state != "human_active" || assigned != userID || generation != 1 {
			t.Fatalf("state=%s assigned=%s generation=%d", state, assigned, generation)
		}

		// Owner tem escopo administrativo de dados, mas nao bypassa o RBAC canonico como
		// platform_admin. Sem grant efetivo, a feature reply deve responder 403.
		denied := auth.Principal{UserID: userID, Role: auth.RoleOwner, AccountID: accountA}
		if _, _, err := send.SendMessage(ctx, accountA, denied, conversationID,
			SendMessageInput{Type: "TEXT", Content: "sem permissao"}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("permission err=%v, want ErrForbidden", err)
		}
		media := NewMediaService(store, nil, nil, nil, nil, nil)
		if _, err := media.RetryMedia(ctx, accountA, denied, conversationID, first.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("media retry permission err=%v, want ErrForbidden", err)
		}
		if _, _, err := send.SendMessage(ctx, accountA, denied,
			"00000000-0000-4000-8000-000000000001",
			SendMessageInput{Type: "TEXT", Content: "fora do tenant"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("cross-account err=%v, want ErrNotFound", err)
		}
	})

	t.Run("envio humano concorrente toma uma vez e nao duplica midia", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC)
		const userID = "55555555-5555-4555-8555-555555555555"
		var conversationID string
		if err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 contact_phone,last_message_at,created_at)
			values ($1::uuid,$2::uuid,'main','WHATSAPP','human-send-concurrent',
			 'Human Send Concurrent','ai_active','5511888888888',$3,$3) returning id::text`,
			accountA, instanceA, base).Scan(&conversationID); err != nil {
			t.Fatal(err)
		}
		// Pool deliberadamente minimo: o limitador de locks precisa deixar uma conexao livre
		// para a secao critica, sem deadlock por esgotamento do proprio pool.
		constrainedConfig, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		constrainedConfig.MaxConns = 2
		constrainedPool, err := pgxpool.NewWithConfig(ctx, constrainedConfig)
		if err != nil {
			t.Fatal(err)
		}
		defer constrainedPool.Close()
		constrainedStore := NewStore(constrainedPool)
		mediaRoot := t.TempDir()
		send := NewSendService(constrainedStore, NewDiskMediaStorage(mediaRoot), nil, nil)
		principal := auth.Principal{UserID: userID, Role: auth.RolePlatformAdmin, AccountID: accountA}
		input := SendMessageInput{
			Type:           "IMAGE",
			MediaURL:       "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
			MediaMimeType:  "image/png",
			MediaFileName:  "idempotent.png",
			IdempotencyKey: "human-media-concurrent-idem",
		}

		type sendResult struct {
			view MessageView
			err  error
		}
		start := make(chan struct{})
		results := make(chan sendResult, 2)
		for range 2 {
			go func() {
				<-start
				view, _, sendErr := send.SendMessage(ctx, accountA, principal, conversationID, input)
				results <- sendResult{view: view, err: sendErr}
			}()
		}
		close(start)
		first, second := <-results, <-results
		if first.err != nil || second.err != nil {
			t.Fatalf("concurrent sends: first=%v second=%v", first.err, second.err)
		}
		if first.view.ID == "" || second.view.ID != first.view.ID {
			t.Fatalf("ids concorrentes: first=%q second=%q", first.view.ID, second.view.ID)
		}

		var generation int64
		var messageCount, outboxCount int
		if err := pool.QueryRow(ctx, `select ai_generation from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`, accountA, conversationID).Scan(&generation); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.messages
			where account_id=$1::uuid and conversation_id=$2::uuid`, accountA, conversationID).Scan(&messageCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.outbox
			where account_id=$1::uuid and idempotency_key=$2`,
			accountA, input.IdempotencyKey).Scan(&outboxCount); err != nil {
			t.Fatal(err)
		}
		if generation != 1 || messageCount != 1 || outboxCount != 1 {
			t.Fatalf("generation=%d messages=%d outbox=%d", generation, messageCount, outboxCount)
		}

		fileCount := countRegularFiles(t, mediaRoot)
		if fileCount != 1 {
			t.Fatalf("arquivos de midia=%d, want 1", fileCount)
		}
	})

	t.Run("falha da outbox reverte takeover mensagem e midia antes do retry", func(t *testing.T) {
		base := time.Date(2026, 7, 20, 18, 30, 0, 0, time.UTC)
		const userID = "55555555-5555-4555-8555-555555555555"
		var conversationID string
		if err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 contact_phone,last_message_at,created_at)
			values ($1::uuid,$2::uuid,'main','WHATSAPP','human-send-failpoint',
			 'Human Send Failpoint','ai_active','5511777777777',$3,$3) returning id::text`,
			accountA, instanceA, base).Scan(&conversationID); err != nil {
			t.Fatal(err)
		}
		mediaRoot := t.TempDir()
		send := NewSendService(store, NewDiskMediaStorage(mediaRoot), nil, nil)
		principal := auth.Principal{UserID: userID, Role: auth.RolePlatformAdmin, AccountID: accountA}
		input := SendMessageInput{
			Type:           "IMAGE",
			MediaURL:       "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
			MediaMimeType:  "image/png",
			MediaFileName:  "failpoint.png",
			IdempotencyKey: "human-media-failpoint-idem",
		}

		if _, err := pool.Exec(ctx, `create or replace function messaging.e1_fail_outbox_insert()
			returns trigger language plpgsql as $$
			begin
				if new.idempotency_key = 'human-media-failpoint-idem' then
					raise exception 'e1 forced outbox failure';
				end if;
				return new;
			end $$;
			create trigger e1_fail_outbox_insert before insert on messaging.outbox
			for each row execute function messaging.e1_fail_outbox_insert()`); err != nil {
			t.Fatal(err)
		}
		dropFailpoint := func() {
			_, _ = pool.Exec(context.Background(), `drop trigger if exists e1_fail_outbox_insert on messaging.outbox;
				drop function if exists messaging.e1_fail_outbox_insert()`)
		}
		t.Cleanup(dropFailpoint)

		if _, _, err := send.SendMessage(ctx, accountA, principal, conversationID, input); err == nil {
			t.Fatal("failpoint da outbox deveria falhar o POST inteiro")
		}
		var generation int64
		var state string
		var messageCount, outboxCount int
		if err := pool.QueryRow(ctx, `select state,ai_generation from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`, accountA, conversationID).
			Scan(&state, &generation); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.messages
			where account_id=$1::uuid and conversation_id=$2::uuid`, accountA, conversationID).
			Scan(&messageCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.outbox
			where account_id=$1::uuid and idempotency_key=$2`, accountA, input.IdempotencyKey).
			Scan(&outboxCount); err != nil {
			t.Fatal(err)
		}
		if state != "ai_active" || generation != 0 || messageCount != 0 || outboxCount != 0 ||
			countRegularFiles(t, mediaRoot) != 0 {
			t.Fatalf("rollback incompleto: state=%s generation=%d messages=%d outbox=%d files=%d",
				state, generation, messageCount, outboxCount, countRegularFiles(t, mediaRoot))
		}

		dropFailpoint()
		first, _, err := send.SendMessage(ctx, accountA, principal, conversationID, input)
		if err != nil {
			t.Fatalf("retry apos rollback: %v", err)
		}
		second, _, err := send.SendMessage(ctx, accountA, principal, conversationID, input)
		if err != nil || second.ID != first.ID {
			t.Fatalf("replay apos retry: first=%s second=%s err=%v", first.ID, second.ID, err)
		}
		if err := pool.QueryRow(ctx, `select ai_generation from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`, accountA, conversationID).Scan(&generation); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.messages
			where account_id=$1::uuid and conversation_id=$2::uuid`, accountA, conversationID).
			Scan(&messageCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.outbox
			where account_id=$1::uuid and idempotency_key=$2`, accountA, input.IdempotencyKey).
			Scan(&outboxCount); err != nil {
			t.Fatal(err)
		}
		if generation != 1 || messageCount != 1 || outboxCount != 1 || countRegularFiles(t, mediaRoot) != 1 {
			t.Fatalf("retry duplicou efeito: generation=%d messages=%d outbox=%d files=%d",
				generation, messageCount, outboxCount, countRegularFiles(t, mediaRoot))
		}
	})
}

func countRegularFiles(t *testing.T, root string) int {
	t.Helper()
	count := 0
	if err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return count
}

func inboundTextWrite(accountID, instanceID, eventID, externalID string, at time.Time, fromMe bool) inboundWrite {
	return inboundWrite{
		AccountID: accountID, Provider: "evolution", ExternalEventID: eventID,
		EventKind: "message_received", InstanceName: "main", InstanceID: instanceID,
		PayloadMasked: []byte(`{"kind":"message_received"}`),
		Message: &inboundMessageWrite{
			ExternalMessageID: externalID, Channel: "WHATSAPP",
			ContactExternalID: "5511999999999@s.whatsapp.net", ContactPhone: "5511999999999",
			ContactName: "Contato", MessageType: "TEXT", Content: eventID,
			OccurredAt: at, FromMe: fromMe,
		},
	}
}

func setupDeliveryTestSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	ddl := `
		create extension if not exists pgcrypto;
		drop schema if exists messaging cascade;
		drop schema if exists core cascade;
		create schema core;
		create schema messaging;
		create table core.accounts(id uuid primary key, slug text not null);
		create table core.permissions(key text primary key, deprecated_at timestamptz);
		create table core.user_role_assignments(account_id uuid not null,user_id uuid not null,role_id uuid not null);
		create table core.role_permissions(role_id uuid not null,permission_key text not null);
		create table core.user_permission_overrides(account_id uuid not null,user_id uuid not null,
			permission_key text not null,effect text not null,is_active boolean not null default true);
		create table messaging.whatsapp_instances(
			id uuid primary key default gen_random_uuid(), account_id uuid not null,
			instance_name text not null, provider text not null, provider_config jsonb not null default '{}',
			display_name text, credentials_ciphertext text, is_active boolean not null default true,
			unique(account_id,instance_name));
		create table messaging.account_config(
			account_id uuid primary key, max_upload_mb int not null default 500);
		create table messaging.contacts(
			id uuid primary key default gen_random_uuid(), account_id uuid not null, name text not null,
			phone text, avatar_url text, source text not null, first_seen_at timestamptz not null,
			last_seen_at timestamptz not null, first_channel text not null, last_channel text not null,
			relationship_status text not null, classification_source text not null default 'backfill',
			classification_confidence numeric(4,3), archived_at timestamptz, merged_into_contact_id uuid,
			updated_at timestamptz not null default now());
		create unique index contacts_phone_uq on messaging.contacts(account_id,phone)
			where phone is not null and phone <> '';
		create table messaging.contact_identities(
			id uuid primary key default gen_random_uuid(), account_id uuid not null, contact_id uuid not null,
			channel text not null, provider text not null, instance_scope_key text not null,
			external_id text not null, display_name text, avatar_url text, first_seen_at timestamptz not null,
			last_seen_at timestamptz not null, updated_at timestamptz not null default now(),
			unique(account_id,channel,provider,instance_scope_key,external_id));
		create table messaging.conversations(
			id uuid primary key default gen_random_uuid(), account_id uuid not null, instance_id uuid,
			instance_scope_key text not null, assigned_to_id text, contact_id uuid, channel text not null, external_id text not null,
			contact_name text, contact_phone text, contact_avatar_url text, state text not null,
			queue_id uuid, department_id uuid, assigned_user_id uuid,
			ai_generation bigint not null default 0, extracted_fields jsonb not null default '{}',
			last_message_at timestamptz not null, created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			unique(account_id,external_id,channel,instance_scope_key));
		create table messaging.messages(
			id uuid primary key default gen_random_uuid(), account_id uuid not null,
			conversation_id uuid not null references messaging.conversations(id) on delete cascade,
			instance_id uuid, instance_scope_key text not null, sender_user_id uuid,
			direction text not null, message_type text not null, sender_name text, sender_avatar_url text,
			content text not null, media_url text, media_mime_type text, media_file_name text,
			media_file_size_bytes int, media_caption text, media_duration_seconds int,
			media_storage_key text, media_source_kind text, metadata_json jsonb,
			external_message_id text, status text not null, origin text not null,
			reply_to_message_id uuid references messaging.messages(id) on delete set null,
			reply_to_external_message_id text, provider_status_at timestamptz,
			provider_error_code text not null default '', created_at timestamptz not null default now(),
			updated_at timestamptz not null default now());
		create unique index messages_external_uq on messaging.messages
			(account_id,instance_scope_key,external_message_id)
			where external_message_id is not null and btrim(external_message_id) <> '';
		create table messaging.webhook_events(
			id uuid primary key default gen_random_uuid(), account_id uuid not null, provider text not null,
			external_event_id text not null, event_kind text not null, instance_name text not null,
			payload_masked jsonb, external_message_id text, provider_status text,
			provider_status_at timestamptz, provider_error_code text not null default '',
			unique(account_id,provider,external_event_id));
		create table messaging.contact_touchpoints(
			id uuid primary key default gen_random_uuid(), account_id uuid not null, contact_id uuid not null,
			conversation_id uuid, message_id uuid, channel text not null, provider text not null,
			external_event_id text, source_kind text not null, occurred_at timestamptz not null);
		create unique index touchpoints_event_uq on messaging.contact_touchpoints(account_id,provider,external_event_id)
			where external_event_id is not null and external_event_id <> '';
		create table messaging.audit_events(
			id uuid primary key default gen_random_uuid(), account_id uuid not null,
			actor_user_id uuid, conversation_id uuid, message_id uuid, event_type text not null,
			payload_json jsonb, created_at timestamptz not null default now(),
			constraint messaging_audit_events_event_type_e1_check check (event_type in (
				'MESSAGE_OUTBOUND_QUEUED','MESSAGE_OUTBOUND_SENT','MESSAGE_OUTBOUND_FAILED',
				'CONVERSATION_STATUS_CHANGED','CONVERSATION_ASSIGNED',
				'MESSAGE_FORWARDED','MESSAGE_DELETED_FOR_ALL',
				'MESSAGE_MEDIA_READY','MESSAGE_MEDIA_FAILED','MESSAGE_MEDIA_RETRY')));
		create table messaging.outbox(
			id uuid primary key default gen_random_uuid(), account_id uuid not null, ordering_key text not null,
			idempotency_key text not null, kind text not null, payload jsonb not null,
			status text not null default 'pending', attempts int not null default 0,
			max_attempts int not null default 3, run_after timestamptz not null default now(),
			locked_at timestamptz, locked_by text not null default '', last_error text not null default '',
			created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
			unique(account_id,idempotency_key));`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatalf("fixture schema: %v", err)
	}
}
