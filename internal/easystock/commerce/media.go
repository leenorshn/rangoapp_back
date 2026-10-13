package commerce

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/media"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	KindProductImage   = "product_image"
	KindExpenseReceipt = "expense_receipt"
	maxMediaBytes      = 8 << 20
	mediaURLTTL        = 15 * time.Minute
)

// UploadIntent is a presigned PUT. The client sends the bytes to UploadURL, then confirms.
type UploadIntent struct {
	MediaID     uuid.UUID
	UploadURL   string
	ContentType string
	ExpiresAt   time.Time
}

// MediaView is a stored file. DownloadURL is set only when the object is ready.
type MediaView struct {
	ID          uuid.UUID
	Kind        string
	ContentType string
	ByteSize    int64
	Status      string
	FileName    string
	DownloadURL string
}

type UploadInput struct {
	Kind        string
	ContentType string
	ByteSize    int64
	FileName    string
}

func (s *Service) RequestMediaUpload(ctx context.Context, p tenant.Principal, in UploadInput) (UploadIntent, error) {
	var out UploadIntent
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		store, err := s.readyStore()
		if err != nil {
			return err
		}
		kind, contentType, fileName, err := normalizeUpload(in)
		if err != nil {
			return err
		}
		perm, err := mediaPermission(kind)
		if err != nil {
			return err
		}
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, perm); err != nil {
			return err
		}
		id := uuid.New()
		key := p.CompanyID.String() + "/" + kind + "/" + id.String()
		expires := time.Now().Add(mediaURLTTL).UTC()
		url, err := store.PresignPut(ctx, key, contentType, in.ByteSize, mediaURLTTL)
		if err != nil {
			return domain.Internal("Le lien de dépôt n'a pas pu être préparé.")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO media_objects (
				id, company_id, kind, object_key, content_type, byte_size, created_by, status, file_name
			) VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',$8)
		`, id, p.CompanyID, kind, key, contentType, in.ByteSize, p.UserID, blankNull(fileName)); err != nil {
			return err
		}
		if err := audit(ctx, tx, p, nil, "media.request", "media", id, map[string]string{"kind": kind}); err != nil {
			return err
		}
		out = UploadIntent{MediaID: id, UploadURL: url, ContentType: contentType, ExpiresAt: expires}
		return nil
	})
	return out, err
}

func (s *Service) ConfirmMediaUpload(ctx context.Context, p tenant.Principal, id uuid.UUID) (MediaView, error) {
	var out MediaView
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		store, err := s.readyStore()
		if err != nil {
			return err
		}
		if _, err := access.ActiveMember(ctx, tx, p.UserID); err != nil {
			return err
		}
		row, err := lockMedia(ctx, tx, id)
		if err != nil {
			return err
		}
		if row.createdBy != p.UserID {
			return domain.PermissionDenied("media", "Seul l'auteur du dépôt peut le confirmer.")
		}
		if row.status == "ready" {
			return s.fillDownload(ctx, store, row, &out)
		}
		size, contentType, err := store.Stat(ctx, row.key)
		if err != nil {
			if errors.Is(err, media.ErrMissing) {
				return domain.FailedPrecondition("media_missing", "Le fichier n'a pas été reçu. Envoyez-le, puis confirmez.")
			}
			return domain.Internal("Le fichier n'a pas pu être vérifié.")
		}
		if size != row.declared {
			return domain.Invalid("media_size", "La taille reçue ne correspond pas à la taille annoncée.")
		}
		if got := canonicalType(contentType); got != "" && got != row.contentType {
			return domain.Invalid("media_type", "Le type reçu ne correspond pas au type annoncé.")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE media_objects SET status = 'ready', byte_size = $2 WHERE id = $1
		`, id, size); err != nil {
			return err
		}
		row.status = "ready"
		row.byteSize = size
		if err := audit(ctx, tx, p, nil, "media.confirm", "media", id, map[string]string{"kind": row.kind}); err != nil {
			return err
		}
		return s.fillDownload(ctx, store, row, &out)
	})
	return out, err
}

func (s *Service) Media(ctx context.Context, p tenant.Principal, id uuid.UUID) (MediaView, error) {
	var out MediaView
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.ActiveMember(ctx, tx, p.UserID); err != nil {
			return err
		}
		row, err := lockMedia(ctx, tx, id)
		if err != nil {
			return err
		}
		if row.status != "ready" {
			out = viewOf(row)
			return nil
		}
		store, err := s.readyStore()
		if err != nil {
			return err
		}
		return s.fillDownload(ctx, store, row, &out)
	})
	return out, err
}

func (s *Service) SetProductImage(ctx context.Context, p tenant.Principal, productID, mediaID uuid.UUID) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.CatalogManage); err != nil {
			return err
		}
		row, err := lockMedia(ctx, tx, mediaID)
		if err != nil {
			return err
		}
		if row.kind != KindProductImage || row.status != "ready" {
			return domain.FailedPrecondition("media", "Choisissez une image de produit déjà envoyée.")
		}
		tag, err := tx.Exec(ctx, `
			UPDATE products SET image_media_id = $2
			WHERE id = $1 AND archived_at IS NULL
		`, productID, mediaID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return domain.NotFound("Produit introuvable.")
		}
		return audit(ctx, tx, p, nil, "product.image", "product", productID, map[string]string{"mediaId": mediaID.String()})
	})
}

func (s *Service) ProductMedia(ctx context.Context, p tenant.Principal, productID uuid.UUID) (*MediaView, error) {
	var out *MediaView
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.ActiveMember(ctx, tx, p.UserID); err != nil {
			return err
		}
		var image pgtype.UUID
		err := tx.QueryRow(ctx, `
			SELECT image_media_id FROM products WHERE id = $1 AND archived_at IS NULL
		`, productID).Scan(&image)
		if err != nil {
			return domain.NotFound("Produit introuvable.")
		}
		if !image.Valid {
			return nil
		}
		row, err := lockMedia(ctx, tx, uuid.UUID(image.Bytes))
		if err != nil {
			return err
		}
		view := viewOf(row)
		if row.status == "ready" {
			store, err := s.readyStore()
			if err != nil {
				return err
			}
			if err := s.fillDownload(ctx, store, row, &view); err != nil {
				return err
			}
		}
		out = &view
		return nil
	})
	return out, err
}

type mediaRow struct {
	id          uuid.UUID
	kind        string
	key         string
	contentType string
	declared    int64
	byteSize    int64
	status      string
	fileName    string
	createdBy   uuid.UUID
}

func lockMedia(ctx context.Context, tx pgx.Tx, id uuid.UUID) (mediaRow, error) {
	var row mediaRow
	err := tx.QueryRow(ctx, `
		SELECT id, kind, object_key, content_type, COALESCE(byte_size, 0), status, COALESCE(file_name, ''), created_by
		FROM media_objects WHERE id = $1 FOR UPDATE
	`, id).Scan(&row.id, &row.kind, &row.key, &row.contentType, &row.declared, &row.status, &row.fileName, &row.createdBy)
	if err != nil {
		return mediaRow{}, domain.NotFound("Fichier introuvable.")
	}
	row.byteSize = row.declared
	return row, nil
}

func readyReceipt(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	row, err := lockMedia(ctx, tx, id)
	if err != nil {
		return domain.NotFound("Justificatif introuvable.")
	}
	if row.kind != KindExpenseReceipt {
		return domain.Invalid("media", "Ce fichier n'est pas un justificatif de dépense.")
	}
	if row.status != "ready" {
		return domain.FailedPrecondition("media", "Envoyez le justificatif avant d'enregistrer la dépense.")
	}
	return nil
}

func (s *Service) readyStore() (media.Store, error) {
	if s.objects == nil {
		return nil, domain.FailedPrecondition("storage", "Le stockage des fichiers n'est pas configuré.")
	}
	return s.objects, nil
}

func (s *Service) fillDownload(ctx context.Context, store media.Store, row mediaRow, out *MediaView) error {
	url, err := store.PresignGet(ctx, row.key, mediaURLTTL)
	if err != nil {
		return domain.Internal("Le lien de téléchargement n'a pas pu être préparé.")
	}
	view := viewOf(row)
	view.DownloadURL = url
	*out = view
	return nil
}

func viewOf(row mediaRow) MediaView {
	return MediaView{
		ID: row.id, Kind: row.kind, ContentType: row.contentType, ByteSize: row.byteSize,
		Status: row.status, FileName: row.fileName,
	}
}

func canonicalType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if i := strings.IndexByte(value, ';'); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return value
}

func normalizeUpload(in UploadInput) (kind, contentType, fileName string, err error) {
	kind = strings.TrimSpace(in.Kind)
	if kind != KindProductImage && kind != KindExpenseReceipt {
		return "", "", "", domain.Invalid("media_kind", "Ce type de fichier n'est pas accepté.")
	}
	contentType = canonicalType(in.ContentType)
	allowed := imageTypes
	if kind == KindExpenseReceipt {
		allowed = receiptTypes
	}
	if !allowed[contentType] {
		return "", "", "", domain.Invalid("media_type", "Ce format de fichier n'est pas accepté.")
	}
	if in.ByteSize <= 0 || in.ByteSize > maxMediaBytes {
		return "", "", "", domain.Invalid("media_size", "Le fichier doit faire entre 1 octet et 8 Mo.")
	}
	fileName, err = cleanFileName(in.FileName)
	if err != nil {
		return "", "", "", err
	}
	return kind, contentType, fileName, nil
}

func mediaPermission(kind string) (string, error) {
	switch kind {
	case KindProductImage:
		return access.CatalogManage, nil
	case KindExpenseReceipt:
		return access.ExpensesCreate, nil
	default:
		return "", domain.Invalid("media_kind", "Ce type de fichier n'est pas accepté.")
	}
}

var imageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

var receiptTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/webp":      true,
	"application/pdf": true,
}

func cleanFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) || strings.Contains(name, "..") {
		return "", domain.Invalid("file_name", "Le nom de fichier n'est pas accepté.")
	}
	if len(name) > 180 {
		return "", domain.Invalid("file_name", "Le nom de fichier est trop long.")
	}
	for _, r := range name {
		if r < 32 || r == 127 || unicode.IsControl(r) {
			return "", domain.Invalid("file_name", "Le nom de fichier n'est pas accepté.")
		}
	}
	return name, nil
}
