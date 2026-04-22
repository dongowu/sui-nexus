package walrus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalrusClient_WriteAndRead(t *testing.T) {
	client := NewClient("https://walrus.testnet.sui.io")
	ctx := context.Background()

	testData := []byte(`{"analysis":"bearish","confidence":0.85}`)
	blobID, err := client.Write(ctx, testData)
	if err != nil {
		t.Skip("Walrus not available, skipping test")
	}

	assert.NotEmpty(t, blobID)
	assert.Len(t, blobID, 64) // BlobID should be 64 chars (hex)

	data, err := client.Read(ctx, blobID)
	assert.NoError(t, err)
	assert.Equal(t, testData, data)
}

func TestWalrusClient_WriteInvalid(t *testing.T) {
	client := NewClient("https://walrus.testnet.sui.io")
	ctx := context.Background()

	_, err := client.Write(ctx, nil)
	assert.Error(t, err)
}