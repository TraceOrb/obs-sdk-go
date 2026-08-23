package internal

import "context"

type storeContextKey struct{}

func WithStore(ctx context.Context, store *Store) context.Context {
	return context.WithValue(ctx, storeContextKey{}, store)
}

func StoreFrom(ctx context.Context) *Store {
	if ctx == nil {
		return nil
	}

	store, _ := ctx.Value(storeContextKey{}).(*Store)
	return store
}
