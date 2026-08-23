package internal

import "time"

func testMeta() CaptureMeta {
	return CaptureMeta{
		Service:      "demo",
		Env:          "test",
		MaxBodyBytes: MaxBodyBytes,
	}
}

func pastStore() *Store {
	store := NewStore()
	store.startedAt = time.Now().UTC().Add(-40 * time.Millisecond)
	return store
}
