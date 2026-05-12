package client

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pbb "github.com/Alexandr20i/finance-tracker/gen/budget"
	pbr "github.com/Alexandr20i/finance-tracker/gen/report"
	pbt "github.com/Alexandr20i/finance-tracker/gen/transaction"
)

// Clients — все gRPC клиенты в одном месте
// Gateway держит по одному соединению на каждый сервис
type Clients struct {
	Transaction pbt.TransactionServiceClient
	Budget      pbb.BudgetServiceClient
	Report      pbr.ReportServiceClient
}

func NewClients(transactionAddr, budgetAddr, reportAddr string) (*Clients, error) {
	// Подключаемся к Transaction Service
	txConn, err := grpc.NewClient(transactionAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to transaction service: %w", err)
	}

	// Подключаемся к Budget Service
	budgetConn, err := grpc.NewClient(budgetAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to budget service: %w", err)
	}

	// Подключаемся к Report Service
	reportConn, err := grpc.NewClient(reportAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to report service: %w", err)
	}

	return &Clients{
		Transaction: pbt.NewTransactionServiceClient(txConn),
		Budget:      pbb.NewBudgetServiceClient(budgetConn),
		Report:      pbr.NewReportServiceClient(reportConn),
	}, nil
}
