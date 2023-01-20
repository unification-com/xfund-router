package keystore_test

import (
	"testing"

	"go-ooo/keystore"

	"github.com/stretchr/testify/require"
)

func TestSetGetAccount(t *testing.T) {
	testKeyStorageKeyModel := &keystore.KeyStorageKeyModel{}

	testKeyStorageKeyModel.SetAccount("testacc")

	testAcc := testKeyStorageKeyModel.GetAccount()

	require.Equal(t, "testacc", testAcc, "expected: testacc, got %s", testAcc)
}

func TestSetGetCipherPrivate(t *testing.T) {
	testKeyStorageKeyModel := &keystore.KeyStorageKeyModel{}

	testKeyStorageKeyModel.SetCipherPrivate("test")

	testAcc := testKeyStorageKeyModel.GetCipherPrivate()

	require.Equal(t, "test", testAcc, "expected: test, got %s", testAcc)
}

func TestSetGetPrivate(t *testing.T) {
	testKeyStorageKeyModel := &keystore.KeyStorageKeyModel{}

	testKeyStorageKeyModel.SetPrivate("test")

	testAcc := testKeyStorageKeyModel.GetPrivate()

	require.Equal(t, "test", testAcc, "expected: test, got %s", testAcc)
}
