package main

import (
	b64 "encoding/base64"
	"errors"

	keychain "github.com/keybase/go-keychain"
	"github.com/vmihailenco/msgpack/v5"
)

const SERVICE_NAME = "aws-saml-go"

type CredentialStorage[T any] interface {
	StoreEntry(name string, data *T) error
	GetEntry(name string) (*T, bool, error)
	DeleteEntry(name string) error
}

type KeyringStorage[T any] struct{}

func (k *KeyringStorage[T]) StoreEntry(name string, data *T) error {
	encoded, err := msgpack.Marshal(data)
	if err != nil {
		return err
	}
	s := b64.StdEncoding.EncodeToString(encoded)

	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(SERVICE_NAME)
	item.SetAccount(name)
	item.SetData([]byte(s))
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	err = keychain.AddItem(item)
	if err == keychain.ErrorDuplicateItem {
		err = keychain.UpdateItem(item, item)
	}

	return err
}

func (k *KeyringStorage[_]) DeleteEntry(name string) error {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(SERVICE_NAME)
	item.SetAccount(name)
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	err := keychain.DeleteItem(item)
	return err
}

func (k *KeyringStorage[T]) GetEntry(name string) (*T, bool, error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService("aws-saml-go")
	query.SetAccount(name)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, false, err
	} else if len(results) == 0 {
		// Not found
		return nil, false, nil
	} else if len(results) > 1 {
		// More than one entry found, requires user to intervene
		return nil, false, errors.New("obtained more than one keychain entry for '" + SERVICE_NAME + "', cleanup your keychain")
	} else {
		var data T
		b := results[0].Data
		l := b64.StdEncoding.DecodedLen(len(b))
		o := make([]byte, l)
		rlen, err := b64.StdEncoding.Decode(o, b)
		if err != nil {
			return nil, false, err
		}

		err = msgpack.Unmarshal(o[0:rlen], &data)
		return &data, true, err
	}
}
