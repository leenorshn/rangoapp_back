package media

import "testing"

func TestOpenS3RequiresCredentials(t *testing.T) {
	if _, err := OpenS3(Config{Region: "us-east-1"}); err == nil {
		t.Fatal("expected missing credentials to fail")
	}
}

func TestMemoryPutThenStat(t *testing.T) {
	store := NewMemory()
	ctx := t.Context()
	url, err := store.PresignPut(ctx, "company/file", "image/png", 4, 0)
	if err != nil || url != "memory://put/company/file" {
		t.Fatalf("put url %s %v", url, err)
	}
	if _, _, err := store.Stat(ctx, "company/file"); err != ErrMissing {
		t.Fatal(err)
	}
	store.Put("company/file", "image/png", []byte("png!"))
	size, kind, err := store.Stat(ctx, "company/file")
	if err != nil || size != 4 || kind != "image/png" {
		t.Fatalf("stat %d %s %v", size, kind, err)
	}
}
