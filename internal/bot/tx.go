package bot

import (
	"context"

	"poraclego/internal/db"
)

func (m *Manager) withQueryTx(fn func(*db.Query) error) error {
	if m == nil || m.query == nil {
		return nil
	}
	return m.query.WithTx(context.Background(), fn)
}

func (m *Manager) withAlertStateTx(fn func(*db.Query) error) error {
	if m == nil || m.query == nil {
		return nil
	}
	return m.query.WithTx(context.Background(), func(query *db.Query) error {
		if fn != nil {
			if err := fn(query); err != nil {
				return err
			}
		}
		query.AfterCommit(m.RefreshAlertState)
		return nil
	})
}
