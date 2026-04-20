package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/reapinteractive/rds-data-api-local/internal/dataapi"
)

// Handler holds the shared state needed to serve all Data API endpoints.
type Handler struct {
	db      *sql.DB
	txStore *dataapi.TransactionStore
}

// NewHandler creates a Handler wired to the given database and transaction store.
func NewHandler(db *sql.DB, txStore *dataapi.TransactionStore) *Handler {
	return &Handler{db: db, txStore: txStore}
}

// ── handlers ──────────────────────────────────────────────────────────────────

// executeSql handles POST /ExecuteSql.
// The legacy ExecuteSql operation is not implemented.
func (h *Handler) executeSql(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "NotFoundException", "NotImplemented")
}

func (h *Handler) beginTransaction(ctx context.Context, req dataapi.BeginTransactionRequest) (any, error) {
	txID, err := h.txStore.Begin(h.db)
	if err != nil {
		return nil, internalError(err.Error())
	}
	return dataapi.BeginTransactionResponse{TransactionID: txID}, nil
}

func (h *Handler) commitTransaction(ctx context.Context, req dataapi.CommitTransactionRequest) (any, error) {
	if err := h.txStore.Commit(req.TransactionID); err != nil {
		return nil, badRequest(err.Error())
	}
	return dataapi.CommitTransactionResponse{TransactionStatus: "Transaction Committed"}, nil
}

func (h *Handler) rollbackTransaction(ctx context.Context, req dataapi.RollbackTransactionRequest) (any, error) {
	if err := h.txStore.Rollback(req.TransactionID); err != nil {
		return nil, badRequest(err.Error())
	}
	return dataapi.RollbackTransactionResponse{TransactionStatus: "Rollback Complete"}, nil
}

func (h *Handler) execute(ctx context.Context, req dataapi.ExecuteStatementRequest) (any, error) {
	if strings.TrimSpace(req.TransactionID) != "" {
		tx, err := h.txStore.Get(req.TransactionID)
		if err != nil {
			return nil, badRequest(err.Error())
		}
		resp, err := dataapi.Execute(ctx, tx, &req)
		if err != nil {
			return nil, badRequest(err.Error())
		}
		return resp, nil
	}

	resp, err := dataapi.Execute(ctx, h.db, &req)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	return resp, nil
}

func (h *Handler) batchExecute(ctx context.Context, req dataapi.BatchExecuteStatementRequest) (any, error) {
	if req.ParameterSets == nil {
		req.ParameterSets = [][]dataapi.SQLParameter{}
	}

	if strings.TrimSpace(req.TransactionID) != "" {
		tx, err := h.txStore.Get(req.TransactionID)
		if err != nil {
			return nil, badRequest(err.Error())
		}
		resp, err := dataapi.BatchExecute(ctx, tx, &req)
		if err != nil {
			return nil, badRequest(err.Error())
		}
		return resp, nil
	}

	// Auto-commit: wrap the whole batch in a single transaction so either all
	// rows are inserted or none are.
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, internalError(err.Error())
	}

	resp, err := dataapi.BatchExecute(ctx, tx, &req)
	if err != nil {
		_ = tx.Rollback()
		return nil, badRequest(err.Error())
	}

	if err := tx.Commit(); err != nil {
		return nil, internalError(err.Error())
	}

	return resp, nil
}

// ── generic handler wrapper ───────────────────────────────────────────────────

// handleJSON wraps a typed handler function, taking care of JSON decoding and error
// translation so that handler methods only need to contain business logic.
func handleJSON[T any](fn func(context.Context, T) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req T
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BadRequestException", "invalid request body: "+err.Error())
			return
		}

		resp, err := fn(r.Context(), req)
		if err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) {
				writeError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			} else {
				writeError(w, http.StatusBadRequest, "BadRequestException", err.Error())
			}
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
