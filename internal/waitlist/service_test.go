package waitlist

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memStore struct {
	emails []string
}

func (m *memStore) Save(_ context.Context, email string, _ time.Time) error {
	for _, existing := range m.emails {
		if existing == email {
			return nil
		}
	}
	m.emails = append(m.emails, email)
	return nil
}

func TestJoinSavesANormalizedEmailOnce(t *testing.T) {
	store := &memStore{}
	svc := NewService(store)

	require.NoError(t, svc.Join(context.Background(), "  Amina@Example.com "))
	require.NoError(t, svc.Join(context.Background(), "amina@example.com"))
	require.Equal(t, []string{"amina@example.com"}, store.emails)
}

func TestJoinRejectsABadEmail(t *testing.T) {
	svc := NewService(&memStore{})
	err := svc.Join(context.Background(), "not-an-email")
	require.Error(t, err)
}
