package store

import (
	"testing"

	"nofx/crypto"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInferAIProvider(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		userID   string
		id       string
		provider string
		want     string
	}{
		{name: "explicit provider wins", userID: "u1", id: "u1_deepseek_2", provider: "deepseek", want: "deepseek"},
		{name: "legacy bare provider", userID: "u1", id: "deepseek", want: "deepseek"},
		{name: "first generated instance", userID: "u1", id: "u1_deepseek", want: "deepseek"},
		{name: "numbered generated instance", userID: "u1", id: "u1_deepseek_2", want: "deepseek"},
		{name: "hyphenated provider", userID: "u1", id: "u1_blockrun-base_2", want: "blockrun-base"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := inferAIProvider(tc.userID, tc.id, tc.provider); got != tc.want {
				t.Fatalf("inferAIProvider(%q, %q, %q) = %q, want %q", tc.userID, tc.id, tc.provider, got, tc.want)
			}
		})
	}
}

func TestAIModelStoreUpdateCreatesIndependentProviderInstances(t *testing.T) {
	privateKey, _, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate rsa key pair: %v", err)
	}
	dataKey, err := crypto.GenerateDataKey()
	if err != nil {
		t.Fatalf("generate data key: %v", err)
	}

	t.Setenv(crypto.EnvRSAPrivateKey, privateKey)
	t.Setenv(crypto.EnvDataEncryptionKey, dataKey)
	cs, err := crypto.NewCryptoService()
	if err != nil {
		t.Fatalf("new crypto service: %v", err)
	}
	crypto.SetGlobalCryptoService(cs)
	defer crypto.SetGlobalCryptoService(nil)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	store := NewAIModelStore(db)
	if err := store.initTables(); err != nil {
		t.Fatalf("init tables: %v", err)
	}

	userID := "user-1"
	if err := store.Update(userID, userID+"_deepseek", "deepseek", "DeepSeek AI", true, "key-1", "https://api.deepseek.com", "deepseek-chat"); err != nil {
		t.Fatalf("create first instance: %v", err)
	}
	if err := store.Update(userID, userID+"_deepseek_2", "deepseek", "DeepSeek AI #2", true, "key-2", "https://api.deepseek.com", "deepseek-reasoner"); err != nil {
		t.Fatalf("create second instance: %v", err)
	}

	models, err := store.List(userID)
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 model instances, got %d", len(models))
	}

	first, err := store.Get(userID, userID+"_deepseek")
	if err != nil {
		t.Fatalf("get first model: %v", err)
	}
	second, err := store.Get(userID, userID+"_deepseek_2")
	if err != nil {
		t.Fatalf("get second model: %v", err)
	}

	if first.CustomModelName != "deepseek-chat" {
		t.Fatalf("expected first model name to remain deepseek-chat, got %q", first.CustomModelName)
	}
	if second.CustomModelName != "deepseek-reasoner" {
		t.Fatalf("expected second model name to remain deepseek-reasoner, got %q", second.CustomModelName)
	}
	if second.Name != "DeepSeek AI #2" {
		t.Fatalf("expected second display name to remain unique, got %q", second.Name)
	}
}
