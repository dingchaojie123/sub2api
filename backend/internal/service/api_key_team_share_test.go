package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// apiKeyTeamSharerStub 在计费桩之上实现团队共享能力，记录调用。
type apiKeyTeamSharerStub struct {
	apiKeyBillingResolverStub
	shareCalls  []bool
	shareResult *APIKeyShareResult
	shareErr    error
	revokeCalls []int64
	revokedKeys []string
}

func (s *apiKeyTeamSharerStub) SetAPIKeyShared(_ context.Context, _ int64, _ int64, shared bool) (*APIKeyShareResult, error) {
	s.shareCalls = append(s.shareCalls, shared)
	if s.shareErr != nil {
		return nil, s.shareErr
	}
	if s.shareResult != nil {
		return s.shareResult, nil
	}
	return &APIKeyShareResult{}, nil
}

func (s *apiKeyTeamSharerStub) RevokeSharedCopies(_ context.Context, apiKeyID int64) ([]string, error) {
	s.revokeCalls = append(s.revokeCalls, apiKeyID)
	return s.revokedKeys, nil
}

func TestSetTeamShareRequiresKeyOwner(t *testing.T) {
	sharer := &apiKeyTeamSharerStub{}
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 8}},
		organizationBilling: sharer,
	}

	_, _, err := svc.SetTeamShare(context.Background(), 9, 205, true)
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Empty(t, sharer.shareCalls)
}

func TestSetTeamShareUnavailableWithoutSharer(t *testing.T) {
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 9}},
		organizationBilling: &apiKeyBillingResolverStub{},
	}

	_, _, err := svc.SetTeamShare(context.Background(), 9, 205, true)
	require.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestSetTeamShareReturnsIssuedCountsAndBillingSource(t *testing.T) {
	organizationID := int64(12)
	sharer := &apiKeyTeamSharerStub{
		apiKeyBillingResolverStub: apiKeyBillingResolverStub{sources: map[int64]APIKeyBillingSource{
			205: {Type: APIKeyBillingSourceOrganization, OrganizationID: &organizationID, Status: APIKeyBillingSourceActive, SharedWithTeam: true},
		}},
		shareResult: &APIKeyShareResult{Issued: 3, Skipped: 1},
	}
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 9, Key: "sk-owner"}},
		organizationBilling: sharer,
	}

	key, result, err := svc.SetTeamShare(context.Background(), 9, 205, true)
	require.NoError(t, err)
	require.Equal(t, []bool{true}, sharer.shareCalls)
	require.Equal(t, 3, result.Issued)
	require.Equal(t, 1, result.Skipped)
	require.True(t, key.BillingSource.SharedWithTeam)
}

func TestSetBillingSourceRevokesSharedCopiesWhenLeavingTeam(t *testing.T) {
	organizationID := int64(12)
	sharer := &apiKeyTeamSharerStub{
		apiKeyBillingResolverStub: apiKeyBillingResolverStub{sources: map[int64]APIKeyBillingSource{
			205: {Type: APIKeyBillingSourceOrganization, OrganizationID: &organizationID, Status: APIKeyBillingSourceActive, SharedWithTeam: true},
		}},
		revokedKeys: []string{"sk-member-copy"},
	}
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 9, Key: "sk-owner"}},
		organizationBilling: sharer,
	}

	_, err := svc.SetBillingSource(context.Background(), 9, SetAPIKeyBillingSourceInput{APIKeyID: 205, Type: APIKeyBillingSourcePersonal})
	require.NoError(t, err)
	require.Equal(t, []int64{205}, sharer.revokeCalls)
}

func TestSetBillingSourceKeepsSharedCopiesWhenStayingInSameTeam(t *testing.T) {
	organizationID := int64(12)
	sharer := &apiKeyTeamSharerStub{
		apiKeyBillingResolverStub: apiKeyBillingResolverStub{sources: map[int64]APIKeyBillingSource{
			205: {Type: APIKeyBillingSourceOrganization, OrganizationID: &organizationID, Status: APIKeyBillingSourceActive, SharedWithTeam: true},
		}},
	}
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 9, Key: "sk-owner"}},
		organizationBilling: sharer,
	}

	_, err := svc.SetBillingSource(context.Background(), 9, SetAPIKeyBillingSourceInput{APIKeyID: 205, Type: APIKeyBillingSourceOrganization, OrganizationID: &organizationID})
	require.NoError(t, err)
	require.Empty(t, sharer.revokeCalls)
}

func TestSetBillingSourceRevokesSharedCopiesWhenMovingToAnotherTeam(t *testing.T) {
	oldTeam, newTeam := int64(12), int64(13)
	sharer := &apiKeyTeamSharerStub{
		apiKeyBillingResolverStub: apiKeyBillingResolverStub{sources: map[int64]APIKeyBillingSource{
			205: {Type: APIKeyBillingSourceOrganization, OrganizationID: &oldTeam, Status: APIKeyBillingSourceActive, SharedWithTeam: true},
		}},
	}
	svc := &APIKeyService{
		apiKeyRepo:          &apiKeyRepoStub{apiKey: &APIKey{ID: 205, UserID: 9, Key: "sk-owner"}},
		organizationBilling: sharer,
	}

	_, err := svc.SetBillingSource(context.Background(), 9, SetAPIKeyBillingSourceInput{APIKeyID: 205, Type: APIKeyBillingSourceOrganization, OrganizationID: &newTeam})
	require.NoError(t, err)
	require.Equal(t, []int64{205}, sharer.revokeCalls)
}

func TestCreateWithShareRequiresTeamBilling(t *testing.T) {
	svc := &APIKeyService{}

	_, err := svc.Create(context.Background(), 9, CreateAPIKeyRequest{Name: "团队 Key", ShareWithTeam: true})
	require.ErrorIs(t, err, ErrAPIKeyNotTeamBilled)
}
