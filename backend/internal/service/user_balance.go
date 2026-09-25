package service

import (
	"context"
	"fmt"
)

type UserBalance struct {
	UserID         int64   `json:"user_id"`
	DisplayBalance float64 `json:"display_balance"`
}

// GetBalance exposes only the customer-facing balance, excluding internal billing amounts.
func (s *UserService) GetBalance(ctx context.Context, userID int64) (*UserBalance, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user balance: %w", err)
	}
	return &UserBalance{
		UserID:         user.ID,
		DisplayBalance: user.DisplayBalance,
	}, nil
}
