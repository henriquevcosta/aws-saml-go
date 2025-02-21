package main

type NoopCredentialStorage[T any] struct {
}

func (c *NoopCredentialStorage[T]) StoreEntry(name string, data *T) error {
	return nil
}

func (c *NoopCredentialStorage[T]) GetEntry(name string) (*T, bool, error) {
	return nil, false, nil
}

func (c *NoopCredentialStorage[T]) DeleteEntry(name string) error {
	return nil
}
