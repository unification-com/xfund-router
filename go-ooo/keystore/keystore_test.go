package keystore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-ooo/keystore"

	"github.com/stretchr/testify/require"
)

func getCurentPath() string {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return pwd
}

func loadKeystoreFromFile() *keystore.Keystorage {
	pwd := getCurentPath()
	kPath, _ := filepath.Abs(fmt.Sprintf(`%s/../tests_data/.go-ooo/keystore.json`, pwd))

	ks, _ := keystore.NewKeyStorageNoLogger(kPath)

	return ks
}

func loadTmpKeystore() *keystore.Keystorage {
	now := time.Now()
	kPath, _ := filepath.Abs(fmt.Sprintf(`/tmp/go-ooo_test_keystore%d.json`, now.Nanosecond()))
	ks, _ := keystore.NewKeyStorageNoLogger(kPath)
	return ks
}

func TestNewKeyStorageNoLoggerWithExistingKeystore(t *testing.T) {

	pwd := getCurentPath()
	kPath, _ := filepath.Abs(fmt.Sprintf(`%s/../tests_data/.go-ooo/keystore.json`, pwd))

	_, err := keystore.NewKeyStorageNoLogger(kPath)

	require.NoError(t, err)
}

func TestExists(t *testing.T) {
	ks := loadKeystoreFromFile()
	ks.KeyStore.SetToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")

	require.True(t, ks.Exists())
}

func TestGetFirst(t *testing.T) {
	ks := loadKeystoreFromFile()
	ks.KeyStore.SetToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")

	pk := ks.GetFirst()

	require.Equal(t, "test", pk.Account)
	require.Equal(t, "OhSRG5J4ehgdmPUJK2jiItgIOlQtPafWDJonAhOW1i5KgnmjYC6hlHYi8VNVRquSssRvJYu9T6qBawv0e3yZN1_x7Abu46oLKw4TrEKdSCT7_g==", pk.CipherPrivate)
	require.Equal(t, "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", pk.Private)
}

func TestGetByUsername(t *testing.T) {
	ks := loadKeystoreFromFile()
	ks.KeyStore.SetToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")
	pk := ks.GetByUsername("test")

	require.Equal(t, "test", pk.Account)
	require.Equal(t, "OhSRG5J4ehgdmPUJK2jiItgIOlQtPafWDJonAhOW1i5KgnmjYC6hlHYi8VNVRquSssRvJYu9T6qBawv0e3yZN1_x7Abu46oLKw4TrEKdSCT7_g==", pk.CipherPrivate)
	require.Equal(t, "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", pk.Private)
}

func TestExistsByUsername(t *testing.T) {
	ks := loadKeystoreFromFile()
	ks.KeyStore.SetToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")
	require.True(t, ks.ExistsByUsername("test"))
}

func TestGeneratePrivate(t *testing.T) {
	ks := loadTmpKeystore()
	pk, err := ks.GeneratePrivate("test")

	require.NoError(t, err)
	require.NotNil(t, pk)
}

func TestAddExisting(t *testing.T) {
	ks := loadTmpKeystore()
	err := ks.AddExisting("test", "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913")
	require.NoError(t, err)
}

func TestGetByAccount(t *testing.T) {
	ks := loadKeystoreFromFile()
	ks.KeyStore.SetToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")

	ksm, err := ks.GetByAccount("test")

	require.NoError(t, err)

	require.Equal(t, "test", ksm.Account)
	require.Equal(t, "OhSRG5J4ehgdmPUJK2jiItgIOlQtPafWDJonAhOW1i5KgnmjYC6hlHYi8VNVRquSssRvJYu9T6qBawv0e3yZN1_x7Abu46oLKw4TrEKdSCT7_g==", ksm.CipherPrivate)
	require.Equal(t, "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", ksm.Private)
}

func TestGenerateToken(t *testing.T) {
	ks := loadTmpKeystore()

	token, err := ks.GenerateToken()
	require.NoError(t, err)
	require.NotNil(t, token)
}

func TestCheckToken_ExistingKeystore(t *testing.T) {
	ks := loadKeystoreFromFile()

	err := ks.CheckToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")

	require.NoError(t, err)

	require.Equal(t, "0d1bd3us45hi8j6bhno4ca00z5pk5i5t", ks.KeyStore.Token)
}

func TestCheckToken_NewKeystore(t *testing.T) {
	ks := loadTmpKeystore()

	_, _ = ks.GeneratePrivate("test")

	token, _ := ks.GenerateToken()

	err := ks.CheckToken(token)

	require.NoError(t, err)

	require.Equal(t, token, ks.KeyStore.Token)
}

func TestSelectPrivateKey_NoToken(t *testing.T) {
	ks := loadKeystoreFromFile()
	_ = ks.SelectPrivateKey("test")

	require.NotEqual(t, "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", ks.KeyStore.PrivateKey)
}

func TestSelectPrivateKey_WithToken(t *testing.T) {
	ks := loadKeystoreFromFile()
	_ = ks.CheckToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")
	_ = ks.SelectPrivateKey("test")

	require.Equal(t, "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", ks.KeyStore.PrivateKey)
}

func TestGetSelectedPrivateKey_NoToken(t *testing.T) {
	ks := loadKeystoreFromFile()
	_ = ks.SelectPrivateKey("test")

	require.NotEqual(t, "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", ks.GetSelectedPrivateKey())
}

func TestGetSelectedPrivateKey_WithToken(t *testing.T) {
	ks := loadKeystoreFromFile()
	_ = ks.CheckToken("0d1bd3us45hi8j6bhno4ca00z5pk5i5t")
	_ = ks.SelectPrivateKey("test")

	require.Equal(t, "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", ks.GetSelectedPrivateKey())
}
