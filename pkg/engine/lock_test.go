package engine

import "testing"

func Test_acquireLock(t *testing.T) {
	path := t.TempDir() + "/update-ipsets.lock"
	first, err := acquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Release() }()

	second, err := acquireLock(path)
	if err == nil {
		_ = second.Release()
		t.Fatal("expected second lock acquisition to fail")
	}
}
