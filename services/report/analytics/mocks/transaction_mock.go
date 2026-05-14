package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	pb "github.com/Alexandr20i/finance-tracker/gen/transaction"
)

type TransactionClient struct {
	mock.Mock
}

func (m *TransactionClient) GetBalance(
	ctx context.Context,
	userID int64,
	from, to string,
) (*pb.GetBalanceResponse, error) {
	args := m.Called(ctx, userID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.GetBalanceResponse), args.Error(1)
}

func (m *TransactionClient) GetCategoryTotals(
	ctx context.Context,
	userID int64,
	from, to string,
) (*pb.GetCategoryTotalsResponse, error) {
	args := m.Called(ctx, userID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.GetCategoryTotalsResponse), args.Error(1)
}

func (m *TransactionClient) ListAllTransactions(
	ctx context.Context,
	userID int64,
	from, to string,
) ([]*pb.Transaction, error) {
	args := m.Called(ctx, userID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*pb.Transaction), args.Error(1)
}
