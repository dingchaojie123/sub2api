package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// memberDetailRepoStub 嵌入（未初始化的）OrganizationRepository，只实现成员明细这两个可选能力。
type memberDetailRepoStub struct {
	OrganizationRepository
	taskLimit, taskOffset int
	taskCalls             int
}

func (s *memberDetailRepoStub) MemberUsageDetail(_ context.Context, _, _, memberUserID int64, _, _ time.Time) (*OrganizationMemberUsageDetail, error) {
	return &OrganizationMemberUsageDetail{UserID: memberUserID}, nil
}

func (s *memberDetailRepoStub) MemberTasks(_ context.Context, _, _, _ int64, _, _ time.Time, limit, offset int) ([]OrganizationMemberTask, int64, error) {
	s.taskCalls++
	s.taskLimit, s.taskOffset = limit, offset
	return []OrganizationMemberTask{}, 0, nil
}

func TestOrganizationMemberDetailUnsupportedRepository(t *testing.T) {
	// 没实现可选接口的仓库（例如旧测试桩）：返回明确错误而不是 panic。
	svc := &OrganizationService{repo: nil}
	_, err := svc.MemberUsageDetail(context.Background(), 1, 2, 3, time.Now(), time.Now())
	require.ErrorIs(t, err, ErrOrganizationMemberDetailUnsupported)
	_, _, err = svc.MemberTasks(context.Background(), 1, 2, 3, time.Now(), time.Now(), 20, 0)
	require.ErrorIs(t, err, ErrOrganizationMemberDetailUnsupported)
}

func TestOrganizationMemberDetailDelegatesToRepository(t *testing.T) {
	repo := &memberDetailRepoStub{}
	svc := &OrganizationService{repo: repo}

	detail, err := svc.MemberUsageDetail(context.Background(), 1, 2, 9, time.Now(), time.Now())
	require.NoError(t, err)
	require.Equal(t, int64(9), detail.UserID)

	// 分页参数异常时回落到默认值，上限 100。
	for _, tc := range []struct{ limit, offset, wantLimit, wantOffset int }{
		{0, 0, 20, 0},
		{500, -5, 20, 0},
		{50, 100, 50, 100},
	} {
		_, _, err := svc.MemberTasks(context.Background(), 1, 2, 9, time.Now(), time.Now(), tc.limit, tc.offset)
		require.NoError(t, err)
		require.Equal(t, tc.wantLimit, repo.taskLimit)
		require.Equal(t, tc.wantOffset, repo.taskOffset)
	}
	require.Equal(t, 3, repo.taskCalls)
}
