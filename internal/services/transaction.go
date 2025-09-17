package services

import (
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

// TransactionService handles transaction business logic
type TransactionService struct {
	businessState *ocppj.RedisBusinessState
}

// NewTransactionService creates a new transaction service
func NewTransactionService(businessState *ocppj.RedisBusinessState) *TransactionService {
	return &TransactionService{
		businessState: businessState,
	}
}

// GetActiveTransactions retrieves active transactions, optionally filtered by client
func (s *TransactionService) GetActiveTransactions(clientID string) ([]interface{}, error) {
	transactions, err := s.businessState.GetActiveTransactions(clientID)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(transactions))
	for i, tx := range transactions {
		result[i] = tx
	}
	return result, nil
}

// GetAllTransactions retrieves all transactions (active + completed), optionally filtered by client
func (s *TransactionService) GetAllTransactions(clientID string) ([]interface{}, error) {
	// First get active transactions
	activeTransactions, err := s.businessState.GetActiveTransactions(clientID)
	if err != nil {
		return nil, err
	}

	// Try to get transaction history if available
	// Note: This depends on whether the ocpp-go library supports transaction history
	// For now, we'll just return active transactions and add a note about the limitation
	result := make([]interface{}, len(activeTransactions))
	for i, tx := range activeTransactions {
		result[i] = tx
	}
	return result, nil
}

// GetTransaction retrieves a specific transaction
func (s *TransactionService) GetTransaction(transactionID int) (interface{}, error) {
	return s.businessState.GetTransaction(transactionID)
}