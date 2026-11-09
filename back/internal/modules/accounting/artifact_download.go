package accounting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

const maxArtifactDownloadBytes = 25 * 1024 * 1024

type ArtifactReader interface {
	OpenPrivate(context.Context, Scope, string) (io.ReadCloser, error)
}

func (s *Service) WithArtifactReader(reader ArtifactReader) *Service {
	s.artifactReader = reader
	return s
}

type artifactReference struct {
	object, kind, checksum string
	size                   int64
}

func (s *Store) artifactReference(ctx context.Context, sc Scope, execution, id string) (artifactReference, error) {
	var ref artifactReference
	tx, e := s.begin(ctx)
	if e != nil {
		return ref, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = issuerLock(ctx, tx, sc, "view"); e != nil {
		return ref, e
	}
	e = tx.QueryRow(ctx, `select storage_object_id::text,kind,sha256,size from accounting.artifacts where account_id=$1::uuid and issuer_id=$2::uuid and execution_id=$3::uuid and id=$4::uuid`, sc.AccountID, sc.IssuerID, execution, id).Scan(&ref.object, &ref.kind, &ref.checksum, &ref.size)
	if e != nil {
		return ref, dbError(e)
	}
	return ref, tx.Commit(ctx)
}
func (s *Service) DownloadArtifact(ctx context.Context, sc Scope, execution, id string) (Artifact, error) {
	var out Artifact
	if e := s.check(ctx, sc, "accounting.documents.view"); e != nil {
		return out, e
	}
	if !validID(execution) || !validID(id) {
		return out, failure("not_found", 404)
	}
	ref, e := s.store.artifactReference(ctx, sc, execution, id)
	if e != nil {
		return out, e
	}
	if s.artifactReader == nil {
		return out, failure("storage_unavailable", 503)
	}
	if ref.size < 1 || ref.size > maxArtifactDownloadBytes {
		return out, failure("artifact_size_unavailable", 413)
	}
	body, e := s.artifactReader.OpenPrivate(ctx, sc, ref.object)
	if e != nil || body == nil {
		return out, failure("storage_unavailable", 503)
	}
	defer func() { _ = body.Close() }()
	raw, e := io.ReadAll(io.LimitReader(body, ref.size+1))
	if e != nil || int64(len(raw)) != ref.size {
		return out, failure("artifact_integrity_failed", 502)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != ref.checksum {
		return out, failure("artifact_integrity_failed", 502)
	}
	// Network/storage is outside the DB transaction. Recheck access before exposure.
	if e = s.check(ctx, sc, "accounting.documents.view"); e != nil {
		return out, e
	}
	latest, e := s.store.artifactReference(ctx, sc, execution, id)
	if e != nil {
		return out, e
	}
	if latest != ref {
		return out, failure("artifact_stale", 409)
	}
	return Artifact{Kind: ref.kind, SHA256: ref.checksum, Content: raw}, nil
}
