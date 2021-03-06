package rtfs_test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/RTradeLtd/Temporal/rtfs"
	ci "github.com/libp2p/go-libp2p-crypto"
)

/*
	if we dont disable gocache, the key generate will always be the same
	env GOCACHE=off go test -v ....
*/
func TestKeystoreManager(t *testing.T) {
	km, err := rtfs.GenerateKeystoreManager()
	if err != nil {
		t.Fatal(err)
	}

	keys, err := km.ListKeyIdentifiers()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range keys {
		_, err := km.GetPrivateKeyByName(v)
		if err != nil {
			t.Error(err)
			continue
		}
		present, err := km.CheckIfKeyIsPresent(v)
		if err != nil {
			t.Error(err)
			continue
		}
		if !present {
			t.Error("key not present when it should be")
			continue
		}
	}

	present, err := km.CheckIfKeyIsPresent("thiskeyshouldreallynotexistwithsucharandomname")
	if err != nil {
		t.Fatal(err)
	}
	if present {
		t.Fatal("key found when it should'nt have been")
	}
	// DO NOT USE 1024 IN PRODUCTION, >= 2048
	pk, _, err := ci.GenerateKeyPair(ci.RSA, 1024)
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		t.Fatal(err)
	}

	hexed := hex.EncodeToString(b)
	err = km.SavePrivateKey(hexed, pk)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetKey(t *testing.T) {
	k1 := "b6ec4a647a7738ef8eea3b21777ecf41630d6d0ac79dc36739d81e927f910a65"
	k2 := "test1"

	km, err := rtfs.GenerateKeystoreManager()
	if err != nil {
		t.Fatal(err)
	}

	present, err := km.CheckIfKeyIsPresent(k1)
	if err != nil {
		t.Fatal(err)
	}

	if !present {
		t.Error("key not present wehn it should be")
	}

	pk1, err := km.GetPrivateKeyByName(k1)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", pk1.GetPublic())

	present, err = km.CheckIfKeyIsPresent(k2)
	if err != nil {
		t.Fatal(err)
	}

	if !present {
		t.Fatal("key not present when it should be")
	}

	pk2, err := km.GetPrivateKeyByName(k2)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%+v\n", pk2.GetPublic())
}
