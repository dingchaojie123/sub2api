package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyBillingResolverStub struct {
	sources       map[int64]APIKeyBillingSource
	setSource     *APIKeyBillingSource
	setCredential string
	setErr        error
	setCalls      []SetAPIKeyBillingSourceInput
}

func (s *apiKeyBillingResolverStub) GetBillingSubjectByAPIKey(context.Context, int64) (*OrganizationBillingSubject, error) {
	return nil, nil
}

func (s *apiKeyBillingResolverStub) AttachAPIKey(context.Context, int64, int64, int64) error {
	return nil
}

func (s *apiKeyBillingResolverStub) SetAPIKeyBillingSource(_ context.Context, apiKeyID int64, _ int64, billingType string, organizationID *int64) (*APIKeyBillingSource, string, error) {
	s.setCalls = append(s.setCalls, SetAPIKeyBillingSourceInput{APIKeyID: apiKeyID, Type: billingType, OrganizationID: organizationID})
	if s.setErr != nil {
		return nil, "", s.setErr
	}
	if s.setSource != nil {
		return s.setSource, s.setCredential, nil
	}
	return &APIKeyBillingSource{Type: billingType, OrganizationID: organizationID, Status: APIKeyBillingSourceActive}, "sk-test", nil
}

func TestSetBillingSourceValidatesInput(t *testing.T) {
	svc := &APIKeyService{}

	_, err := svc.SetBillingSource(context.Background(), 9, SetAPIKeyBillingSourceInput{APIKeyID: 1, Type: "unknown"})
	require.ErrorIs(t, err, ErrAPIKeyBillingSource)

	_, err = svc.SetBillingSource(context.Background(), 9, SetAPIKeyBillingSourceInput{APIKeyID: 1, Type: APIKeyBillingSourceOrganization})
	require.ErrorIs(t, err, ErrOrganizationIDNeeded)
}

func TestSetBillingSourceUpdatesExistingOwnedKey(t *testing.T) {
	organizationID := int64(12)
	resolver := &apiKeyBillingResolverStub{
		setSource: &APIKeyBillingSource{
			Type:             APIKeyBillingSourceOrganization,
			OrganizationID:   &organizationID,
			OrganizationName: "产品研发团队",
			Status:           APIKeyBillingSourceActive,
		},
		setCredential: "sk-existing",
	}
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 9, Key: "sk-existing", Name: "默认文本 Key"}},
		organizationBilling: resolver,
	}

	key, err := svc.SetBillingSource(context.Background(), 9, SetAPIKeyBillingSourceInput{
		APIKeyID: 205, Type: APIKeyBillingSourceOrganization, OrganizationID: &organizationID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(205), key.ID)
	require.Equal(t, APIKeyBillingSourceOrganization, key.BillingSource.Type)
	require.Equal(t, []SetAPIKeyBillingSourceInput{{APIKeyID: 205, Type: APIKeyBillingSourceOrganization, OrganizationID: &organizationID}}, resolver.setCalls)
}

func TestSetBillingSourceDoesNotExposeAnotherUsersKey(t *testing.T) {
	resolver := &apiKeyBillingResolverStub{}
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 8}},
		organizationBilling: resolver,
	}

	_, err := svc.SetBillingSource(context.Background(), 9, SetAPIKeyBillingSourceInput{APIKeyID: 205, Type: APIKeyBillingSourcePersonal})
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Empty(t, resolver.setCalls)
}

func (s *apiKeyBillingResolverStub) GetAPIKeyBillingSources(context.Context, []int64) (map[int64]APIKeyBillingSource, error) {
	return s.sources, nil
}

func TestFillBillingSourcesDistinguishesPersonalAndInactiveOrganization(t *testing.T) {
	organizationID := int64(12)
	svc := &APIKeyService{organizationBilling: &apiKeyBillingResolverStub{sources: map[int64]APIKeyBillingSource{
		2: {
			Type:             APIKeyBillingSourceOrganization,
			OrganizationID:   &organizationID,
			OrganizationName: "产品研发团队",
			Status:           APIKeyBillingSourceInactive,
		},
	}}}
	keys := []APIKey{{ID: 1}, {ID: 2}}

	require.NoError(t, svc.fillBillingSources(context.Background(), keys))
	require.Equal(t, APIKeyBillingSourcePersonal, keys[0].BillingSource.Type)
	require.Equal(t, APIKeyBillingSourceActive, keys[0].BillingSource.Status)
	require.Equal(t, APIKeyBillingSourceOrganization, keys[1].BillingSource.Type)
	require.Equal(t, APIKeyBillingSourceInactive, keys[1].BillingSource.Status)
	require.Equal(t, organizationID, *keys[1].BillingSource.OrganizationID)
}
