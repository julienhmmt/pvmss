package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrap_FreshDB_NotComplete(t *testing.T) {
	db := openTestDB(t)
	done, err := db.IsBootstrapComplete()
	require.NoError(t, err)
	assert.False(t, done)
}

func TestBootstrap_CompleteBootstrap_SetsFlag(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CompleteBootstrap("v1.0.0"))

	done, err := db.IsBootstrapComplete()
	require.NoError(t, err)
	assert.True(t, done)
}

func TestBootstrap_CompleteBootstrap_Idempotent(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CompleteBootstrap("v1.0.0"))
	require.NoError(t, db.CompleteBootstrap("v1.1.0"), "second call must not error")

	done, err := db.IsBootstrapComplete()
	require.NoError(t, err)
	assert.True(t, done)
}
