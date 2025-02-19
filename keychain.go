package main

import (
	"errors"

	"github.com/keybase/go-keychain"
	"github.com/vmihailenco/msgpack/v5"
)

const SERVICE_NAME = "aws-google-go"

func storeEntry[T any](name string, data *T) error {

	encoded, err := msgpack.Marshal(data)
	if err != nil {
		return err
	}
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(SERVICE_NAME)
	item.SetAccount(name)
	item.SetData(encoded)
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	err = keychain.AddItem(item)
	if err == keychain.ErrorDuplicateItem {
		err = keychain.UpdateItem(item, item)
	}

	return err
}

func getEntry[T any](name string) (*T, bool, error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService("aws-google-go")
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
		err := msgpack.Unmarshal(results[0].Data, &data)
		return &data, true, err
	}
}
